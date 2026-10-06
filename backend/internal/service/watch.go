package service

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
)

type watchStore interface {
	List(ctx context.Context, f domain.CatalogFilter) ([]domain.Watch, int64, error)
	Count(ctx context.Context, scope, availability string) (int64, error)
	FindByID(ctx context.Context, id primitive.ObjectID, activeOnly bool) (*domain.Watch, error)
	CountByModelKey(ctx context.Context, key string, exclude *primitive.ObjectID) (int64, error)
	Insert(ctx context.Context, w *domain.Watch) error
	UpdateDetails(ctx context.Context, id primitive.ObjectID, w *domain.Watch) error
	SetArchived(ctx context.Context, id primitive.ObjectID, archivedAt *time.Time, now time.Time) error
	DeleteArchived(ctx context.Context, id primitive.ObjectID) error
}

type watchReservationCounter interface {
	CountForWatch(ctx context.Context, watchID primitive.ObjectID, statuses []string) (int64, error)
}

// WatchService gestiona el catálogo: alta, edición, archivado y consultas.
// El stock lo modifica siempre StockService (con su libro de movimientos).
type WatchService struct {
	watches      watchStore
	reservations watchReservationCounter
	stock        *StockService
	now          Clock
}

func NewWatchService(watches watchStore, reservations watchReservationCounter, stock *StockService, now Clock) *WatchService {
	return &WatchService{watches: watches, reservations: reservations, stock: stock, now: clockOrNow(now)}
}

var errDuplicateModel = domain.Conflict(domain.CodeDuplicateModel, "Ya existe un reloj con ese modelo")

// validateWatch normaliza y valida el alta/edición. Toda regla importante
// vive acá, no solo en el frontend.
func validateWatch(in *dto.WatchRequest) string {
	in.Brand = domain.CollapseSpaces(in.Brand)
	in.Name = domain.CollapseSpaces(in.Name)
	in.Model = domain.NormalizeModel(in.Model)
	in.Description = strings.TrimSpace(in.Description)
	in.Image = strings.TrimSpace(in.Image)
	in.StockMoveReason = strings.TrimSpace(in.StockMoveReason)
	if in.StockMoveType == "" {
		in.StockMoveType = domain.MoveManualAdjustment
	}
	specs := make([]string, 0, len(in.Specs))
	for _, s := range in.Specs {
		if s = strings.TrimSpace(s); s != "" {
			specs = append(specs, s)
		}
	}
	in.Specs = specs

	switch {
	case in.Name == "":
		return "El nombre es obligatorio"
	case len(in.Name) > 120:
		return "El nombre es demasiado largo (máximo 120 caracteres)"
	case in.Brand == "":
		return "La marca es obligatoria"
	case len(in.Brand) > 60:
		return "La marca es demasiado larga"
	case in.Model == "":
		return "El modelo es obligatorio"
	case !domain.ValidModel(in.Model):
		return "El modelo solo puede tener letras, números, espacios y - . / _"
	case !domain.ValidGender(in.Gender):
		return "El género debe ser 'hombre' o 'mujer'"
	case len(in.Description) > 4000:
		return "La descripción es demasiado larga"
	case len(in.Specs) > 40:
		return "Demasiadas especificaciones (máximo 40)"
	case in.PriceARS < 0:
		return "El precio no puede ser negativo"
	case in.PriceARS > domain.MaxPriceARS:
		return "El precio es demasiado alto"
	case in.StockQuantity == nil:
		return "La cantidad en stock es obligatoria"
	case *in.StockQuantity < 0:
		return "El stock no puede ser negativo"
	case *in.StockQuantity > domain.MaxStockQuantity:
		return "La cantidad en stock es demasiado alta"
	case in.StockQuantityBase != nil && *in.StockQuantityBase < 0:
		return "Stock base inválido"
	case !domain.ValidManualMoveType(in.StockMoveType):
		return "Tipo de movimiento de stock inválido"
	case len(in.StockMoveReason) > 300:
		return "El motivo del movimiento es demasiado largo"
	case !domain.ValidImagePath(in.Image):
		return "La imagen debe ser una ruta /uploads/..., /images/... o una URL https"
	}
	for _, s := range in.Specs {
		if len(s) > 200 {
			return "Cada especificación debe tener como máximo 200 caracteres"
		}
	}
	return ""
}

// ensureModelAvailable: 409 si otro reloj ya usa ese modelo. El índice único
// es la garantía real ante altas simultáneas; esto da un mensaje más claro.
func (s *WatchService) ensureModelAvailable(ctx context.Context, key string, exclude *primitive.ObjectID) error {
	n, err := s.watches.CountByModelKey(ctx, key, exclude)
	if err != nil {
		return err
	}
	if n > 0 {
		return errDuplicateModel
	}
	return nil
}

func normalizeAll(items []domain.Watch) {
	for i := range items {
		items[i].Normalize()
	}
}

// ListPublic devuelve una página del catálogo público (solo activos).
func (s *WatchService) ListPublic(ctx context.Context, f domain.CatalogFilter) (dto.Page[domain.Watch], error) {
	f.Scope = domain.ScopeActive
	items, total, err := s.watches.List(ctx, f)
	if err != nil {
		return dto.Page[domain.Watch]{}, err
	}
	normalizeAll(items)
	return dto.NewPage(items, f.Page, f.Limit, total), nil
}

// ListAdmin devuelve la página del Admin con contadores del alcance elegido.
func (s *WatchService) ListAdmin(ctx context.Context, f domain.CatalogFilter) (dto.AdminWatchList, error) {
	items, total, err := s.watches.List(ctx, f)
	if err != nil {
		return dto.AdminWatchList{}, err
	}
	normalizeAll(items)
	var stats domain.CatalogStats
	for _, c := range []struct {
		dst          *int64
		availability string
	}{
		{&stats.Total, ""},
		{&stats.OutOfStock, domain.AvailabilityOutOfStock},
		{&stats.LastUnit, domain.AvailabilityLastUnit},
	} {
		if *c.dst, err = s.watches.Count(ctx, f.Scope, c.availability); err != nil {
			return dto.AdminWatchList{}, err
		}
	}
	return dto.AdminWatchList{Page: dto.NewPage(items, f.Page, f.Limit, total), Stats: stats}, nil
}

// Get devuelve un reloj. Para el público, uno archivado no existe.
func (s *WatchService) Get(ctx context.Context, id primitive.ObjectID, publicOnly bool) (*domain.Watch, error) {
	w, err := s.watches.FindByID(ctx, id, publicOnly)
	if isNotFound(err) {
		return nil, errWatchNotFound
	}
	if err != nil {
		return nil, err
	}
	w.Normalize()
	return w, nil
}

// Create da de alta un reloj y anota su stock inicial en el libro.
func (s *WatchService) Create(ctx context.Context, in dto.WatchRequest, actor string) (*domain.Watch, error) {
	if msg := validateWatch(&in); msg != "" {
		return nil, domain.Validation(msg)
	}
	key := domain.ModelKeyFor(in.Model)
	if err := s.ensureModelAvailable(ctx, key, nil); err != nil {
		return nil, err
	}
	now := s.now()
	w := &domain.Watch{
		Brand: in.Brand, BrandKey: domain.BrandKeyFor(in.Brand), Name: in.Name,
		Model: in.Model, ModelKey: key, Gender: in.Gender,
		Description: in.Description, Image: in.Image, Specs: in.Specs,
		PriceARS: in.PriceARS, StockQuantity: *in.StockQuantity, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.watches.Insert(ctx, w); err != nil {
		if isDuplicate(err) {
			return nil, errDuplicateModel
		}
		return nil, err
	}
	s.stock.RecordInitialEntry(ctx, w, actor)
	w.Normalize()
	return w, nil
}

// Update edita un reloj. El stock se cambia en una operación atómica propia
// (con su movimiento): si base == nueva cantidad no se toca, así guardar el
// formulario no deshace una reserva confirmada mientras se editaba.
func (s *WatchService) Update(ctx context.Context, id primitive.ObjectID, in dto.WatchRequest, actor string) (*domain.Watch, error) {
	if msg := validateWatch(&in); msg != "" {
		return nil, domain.Validation(msg)
	}
	key := domain.ModelKeyFor(in.Model)
	if err := s.ensureModelAvailable(ctx, key, &id); err != nil {
		return nil, err
	}
	if in.StockQuantityBase == nil || *in.StockQuantityBase != *in.StockQuantity {
		if err := s.stock.SetQuantity(ctx, id, in.StockQuantityBase, *in.StockQuantity, in.StockMoveType, in.StockMoveReason, actor); err != nil {
			return nil, err
		}
	}
	details := &domain.Watch{
		Brand: in.Brand, BrandKey: domain.BrandKeyFor(in.Brand), Name: in.Name,
		Model: in.Model, ModelKey: key, Gender: in.Gender,
		Description: in.Description, Image: in.Image, Specs: in.Specs,
		PriceARS: in.PriceARS, UpdatedAt: s.now(),
	}
	if err := s.watches.UpdateDetails(ctx, id, details); err != nil {
		switch {
		case isDuplicate(err):
			return nil, errDuplicateModel
		case isNotFound(err):
			return nil, errWatchNotFound
		}
		return nil, err
	}
	return s.Get(ctx, id, false)
}

// AdjustStock delega en StockService (atómico, nunca negativo).
func (s *WatchService) AdjustStock(ctx context.Context, id primitive.ObjectID, delta int, moveType, reason, actor string) (*domain.Watch, error) {
	w, err := s.stock.Adjust(ctx, id, delta, moveType, reason, actor)
	if err != nil {
		return nil, err
	}
	w.Normalize()
	return w, nil
}

// Archive saca un reloj del catálogo público sin borrarlo. Si tiene reservas
// confirmadas o con seña, exige confirmación explícita (no se bloquea: el
// negocio puede necesitarlo).
func (s *WatchService) Archive(ctx context.Context, id primitive.ObjectID, confirmed bool) (*domain.Watch, error) {
	if !confirmed {
		n, err := s.reservations.CountForWatch(ctx, id, []string{domain.ReservationConfirmed, domain.ReservationDepositPaid})
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, &domain.Error{Kind: domain.ErrConflict, Code: domain.CodeActiveReservations,
				Message: "Este reloj tiene reservas activas. Confirmá si querés archivarlo igualmente.",
				Extra:   map[string]any{"activeReservations": n}}
		}
	}
	now := s.now()
	return s.setArchived(ctx, id, &now)
}

// Restore vuelve a publicar un reloj archivado.
func (s *WatchService) Restore(ctx context.Context, id primitive.ObjectID) (*domain.Watch, error) {
	return s.setArchived(ctx, id, nil)
}

func (s *WatchService) setArchived(ctx context.Context, id primitive.ObjectID, at *time.Time) (*domain.Watch, error) {
	if err := s.watches.SetArchived(ctx, id, at, s.now()); err != nil {
		if isNotFound(err) {
			return nil, domain.NotFound("", "Reloj no encontrado")
		}
		return nil, err
	}
	return s.Get(ctx, id, false)
}

// Delete es el borrado físico EXCEPCIONAL: solo relojes archivados y sin
// reservas (las consultas conservan su propio snapshot).
func (s *WatchService) Delete(ctx context.Context, id primitive.ObjectID) error {
	w, err := s.Get(ctx, id, false)
	if err != nil {
		return err
	}
	if !w.IsArchived() {
		return domain.Conflict("", "Archivá el reloj antes de eliminarlo definitivamente")
	}
	n, err := s.reservations.CountForWatch(ctx, id, nil)
	if err != nil {
		return err
	}
	if n > 0 {
		return domain.Conflict("", "El reloj tiene reservas asociadas: se mantiene archivado")
	}
	if err := s.watches.DeleteArchived(ctx, id); err != nil {
		if isNotFound(err) {
			return domain.NotFound("", "Reloj no encontrado")
		}
		return err
	}
	return nil
}
