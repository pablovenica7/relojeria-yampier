package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
)

type reservationStore interface {
	Insert(ctx context.Context, r *domain.Reservation) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Reservation, error)
	FindForCustomer(ctx context.Context, id, customerID primitive.ObjectID) (*domain.Reservation, error)
	ApplyStatusChange(ctx context.Context, id primitive.ObjectID, c domain.StatusChange) (*domain.Reservation, error)
	ExpireNextDue(ctx context.Context, now time.Time, only *primitive.ObjectID) (*domain.Reservation, error)
	UpdateDetails(ctx context.Context, id primitive.ObjectID, status string, c domain.DetailsChange) (*domain.Reservation, error)
	List(ctx context.Context, f domain.ReservationFilter, page, limit int) ([]domain.Reservation, int64, error)
	ListByCustomer(ctx context.Context, customerID primitive.ObjectID, limit int) ([]domain.Reservation, error)
	CountForCustomer(ctx context.Context, customerID primitive.ObjectID, watchID *primitive.ObjectID, statuses []string) (int64, error)
}

type watchFinder interface {
	FindByID(ctx context.Context, id primitive.ObjectID, activeOnly bool) (*domain.Watch, error)
}

type customerLookup interface {
	FindByIDs(ctx context.Context, ids []primitive.ObjectID) ([]domain.Customer, error)
}

// ReservationService concentra TODAS las reglas de reservas:
//
//   - pending no retiene stock; confirmed retiene 1 unidad (atómico);
//   - deposit_paid sigue retenida; completed no la devuelve (es una venta);
//   - cancelled y expired devuelven la unidad si estaba retenida, una sola vez;
//   - las transiciones válidas están en domain.CanTransition;
//   - el cliente solo puede cancelar sus reservas pendientes;
//   - los snapshots (nombre, modelo, imagen, precio) se congelan al reservar.
//
// Ningún handler cambia el estado de una reserva: todo pasa por acá.
type ReservationService struct {
	reservations reservationStore
	watches      watchFinder
	customers    customerLookup
	stock        *StockService
	log          *slog.Logger
	now          Clock
}

func NewReservationService(reservations reservationStore, watches watchFinder, customers customerLookup, stock *StockService, log *slog.Logger, now Clock) *ReservationService {
	return &ReservationService{reservations: reservations, watches: watches, customers: customers,
		stock: stock, log: loggerOrDefault(log), now: clockOrNow(now)}
}

// ---------- Creación ----------

func validateAdminReservation(in *dto.AdminReservationRequest, now time.Time) string {
	in.CustomerName = domain.CollapseSpaces(in.CustomerName)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Email = strings.TrimSpace(in.Email)
	in.Notes = strings.TrimSpace(in.Notes)
	in.WatchID = strings.TrimSpace(in.WatchID)
	in.InquiryID = strings.TrimSpace(in.InquiryID)
	if in.Status == "" {
		in.Status = domain.ReservationPending
	}
	switch {
	case !primitive.IsValidObjectID(in.WatchID):
		return "Elegí un reloj válido"
	case in.InquiryID != "" && !primitive.IsValidObjectID(in.InquiryID):
		return "Consulta inválida"
	case in.CustomerName == "":
		return "El nombre del cliente es obligatorio"
	case len(in.CustomerName) > 120:
		return "El nombre del cliente es demasiado largo"
	case in.Phone == "" && in.Email == "":
		return "Indicá al menos un teléfono o un email del cliente"
	case len(in.Phone) > 40:
		return "El teléfono es demasiado largo"
	case in.Email != "" && (!domain.ValidEmail(in.Email) || len(in.Email) > 190):
		return "El email no es válido"
	case len(in.Notes) > domain.MaxReservationNotes:
		return "Las notas son demasiado largas (máximo 2000 caracteres)"
	case in.Status != domain.ReservationPending && in.Status != domain.ReservationConfirmed:
		return "Una reserva nueva solo puede crearse como pendiente o confirmada"
	case in.DepositAmount < 0 || in.DepositAmount > domain.MaxPriceARS:
		return "La seña es inválida"
	case in.PriceAtReservation < 0 || in.PriceAtReservation > domain.MaxPriceARS:
		return "El precio acordado es inválido"
	}
	return domain.ValidateExpiry(in.ExpiresAt, now)
}

func (s *ReservationService) activeWatch(ctx context.Context, id primitive.ObjectID) (*domain.Watch, error) {
	w, err := s.watches.FindByID(ctx, id, true)
	if isNotFound(err) {
		return nil, errWatchNotFound
	}
	return w, err
}

// CreateByAdmin registra una reserva desde el panel. Si se crea ya
// confirmada, retiene una unidad.
func (s *ReservationService) CreateByAdmin(ctx context.Context, in dto.AdminReservationRequest, actor string) (*domain.Reservation, error) {
	now := s.now()
	if msg := validateAdminReservation(&in, now); msg != "" {
		return nil, domain.Validation(msg)
	}
	watchID, _ := primitive.ObjectIDFromHex(in.WatchID)
	watch, err := s.activeWatch(ctx, watchID)
	if err != nil {
		return nil, err
	}
	price := watch.PriceARS
	if in.PriceAtReservation > 0 {
		price = in.PriceAtReservation
	}
	if price > 0 && in.DepositAmount > price {
		return nil, domain.Validation("La seña no puede superar el precio")
	}
	res := &domain.Reservation{
		WatchID: watch.ID, WatchNameSnapshot: watch.Name, WatchModelSnapshot: watch.Model,
		WatchImageSnapshot: watch.Image, PriceAtReservation: price,
		CustomerName: in.CustomerName, Phone: in.Phone, Email: in.Email,
		Status: in.Status, DepositAmount: in.DepositAmount, Notes: in.Notes, ExpiresAt: in.ExpiresAt,
		Source: domain.ReservationSourceAdmin, CreatedAt: now, UpdatedAt: now,
	}
	if in.InquiryID != "" {
		iid, _ := primitive.ObjectIDFromHex(in.InquiryID)
		res.InquiryID = &iid
	}

	var held *domain.Watch
	if domain.HoldsStock(res.Status) {
		if held, err = s.stock.HoldUnit(ctx, watch.ID); err != nil {
			return nil, err
		}
		res.StockHeld = true
		res.ConfirmedAt = &now
	}
	if err := s.reservations.Insert(ctx, res); err != nil {
		if held != nil {
			s.stock.CompensateHold(ctx, watch.ID)
		}
		return nil, err
	}
	if held != nil {
		s.stock.RecordHold(ctx, held, res.ID, "Reserva creada confirmada", actor)
	}
	return res, nil
}

// RequestByCustomer registra una SOLICITUD de reserva de un cliente: queda
// pendiente y NO descuenta stock. Solo el Admin puede confirmarla.
func (s *ReservationService) RequestByCustomer(ctx context.Context, cust *domain.Customer, in dto.CustomerReservationRequest) (*domain.Reservation, error) {
	now := s.now()
	in.Notes = strings.TrimSpace(in.Notes)
	in.WatchID = strings.TrimSpace(in.WatchID)
	if !primitive.IsValidObjectID(in.WatchID) {
		return nil, domain.Validation("Reloj inválido")
	}
	if !in.TermsAccepted {
		return nil, domain.Validation("Para solicitar la reserva tenés que aceptar los términos de reserva")
	}
	if len(in.Notes) > domain.MaxCustomerNotes {
		return nil, domain.Validation("El comentario es demasiado largo (máximo 500 caracteres)")
	}
	watchID, _ := primitive.ObjectIDFromHex(in.WatchID)
	watch, err := s.activeWatch(ctx, watchID)
	if err != nil {
		return nil, err
	}
	if watch.StockQuantity < 1 {
		return nil, domain.Conflict("", "Este reloj no tiene stock en este momento. Podés consultarnos por reposición.")
	}
	dup, err := s.reservations.CountForCustomer(ctx, cust.ID, &watch.ID, domain.OpenStatuses)
	if err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, domain.Conflict("", "Ya tenés una reserva activa para este reloj. Podés verla en Mis reservas.")
	}
	pending, err := s.reservations.CountForCustomer(ctx, cust.ID, nil, []string{domain.ReservationPending})
	if err != nil {
		return nil, err
	}
	if pending >= domain.MaxPendingPerCustomer {
		return nil, domain.Conflict("", "Tenés varias solicitudes pendientes. Esperá a que las revisemos o cancelá alguna.")
	}

	expires := now.AddDate(0, 0, domain.WebPendingDays)
	res := &domain.Reservation{
		WatchID: watch.ID, WatchNameSnapshot: watch.Name, WatchModelSnapshot: watch.Model,
		WatchImageSnapshot: watch.Image, PriceAtReservation: watch.PriceARS,
		CustomerID: &cust.ID, CustomerName: cust.FirstName + " " + cust.LastName,
		Phone: cust.Phone, Email: cust.Email, CustomerNotes: in.Notes,
		Status: domain.ReservationPending, ExpiresAt: &expires, Source: domain.ReservationSourceWeb,
		TermsAcceptedAt: &now, TermsVersion: domain.TermsVersion, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.reservations.Insert(ctx, res); err != nil {
		return nil, err
	}
	return res, nil
}

// ---------- Transiciones ----------

// transition son las opciones de un cambio de estado.
type transition struct {
	to            string
	depositAmount *int64
	depositMethod string
	actor         string
	// allowedFrom restringe desde qué estados se acepta (cancelación del cliente).
	allowedFrom []string
	// customerID exige que la reserva sea de ese cliente (ajena → 404).
	customerID *primitive.ObjectID
}

// Confirm confirma una reserva pendiente: retiene una unidad.
func (s *ReservationService) Confirm(ctx context.Context, id primitive.ObjectID, actor string) (*domain.Reservation, error) {
	return s.apply(ctx, id, transition{to: domain.ReservationConfirmed, actor: actor})
}

// MarkDepositPaid registra la seña (monto y método; solo administrativo).
func (s *ReservationService) MarkDepositPaid(ctx context.Context, id primitive.ObjectID, amount *int64, method, actor string) (*domain.Reservation, error) {
	return s.apply(ctx, id, transition{to: domain.ReservationDepositPaid, depositAmount: amount, depositMethod: method, actor: actor})
}

// Complete cierra la venta: la unidad retenida queda vendida.
func (s *ReservationService) Complete(ctx context.Context, id primitive.ObjectID, actor string) (*domain.Reservation, error) {
	return s.apply(ctx, id, transition{to: domain.ReservationCompleted, actor: actor})
}

// Cancel cancela (Admin): si retenía una unidad, la devuelve.
func (s *ReservationService) Cancel(ctx context.Context, id primitive.ObjectID, actor string) (*domain.Reservation, error) {
	return s.apply(ctx, id, transition{to: domain.ReservationCancelled, actor: actor})
}

// CancelByCustomer cancela una reserva del cliente, solo si está pendiente.
// Las confirmadas o con seña se cancelan hablando con el local.
func (s *ReservationService) CancelByCustomer(ctx context.Context, customerID, id primitive.ObjectID) (*domain.Reservation, error) {
	return s.apply(ctx, id, transition{to: domain.ReservationCancelled, actor: "cliente",
		allowedFrom: []string{domain.ReservationPending}, customerID: &customerID})
}

// ChangeStatus es la entrada del panel Admin (PATCH .../status): elige la
// operación según el estado pedido. "expired" manual también se acepta.
func (s *ReservationService) ChangeStatus(ctx context.Context, id primitive.ObjectID, status string, depositAmount *int64, depositMethod, actor string) (*domain.Reservation, error) {
	status = strings.TrimSpace(status)
	switch status {
	case domain.ReservationConfirmed:
		return s.Confirm(ctx, id, actor)
	case domain.ReservationDepositPaid:
		return s.MarkDepositPaid(ctx, id, depositAmount, strings.TrimSpace(depositMethod), actor)
	case domain.ReservationCompleted:
		return s.Complete(ctx, id, actor)
	case domain.ReservationCancelled:
		return s.Cancel(ctx, id, actor)
	}
	if !domain.ValidReservationStatus(status) {
		return nil, domain.Validation("Estado de reserva inválido")
	}
	return s.apply(ctx, id, transition{to: status, depositAmount: depositAmount, actor: actor})
}

// apply es el único lugar donde una reserva cambia de estado.
//
// Orden de los pasos (sin transacciones): al RETENER, primero se descuenta
// stock y después se marca la reserva; al LIBERAR, primero se marca la
// reserva y después se devuelve la unidad. Si el proceso se cae en el medio,
// el peor caso es una unidad "perdida" (stock por debajo del real), nunca
// una unidad reservada dos veces.
func (s *ReservationService) apply(ctx context.Context, id primitive.ObjectID, t transition) (*domain.Reservation, error) {
	now := s.now()
	// Si ya venció, se marca como vencida antes de cualquier otro cambio.
	if _, err := s.ExpireDue(ctx, &id); err != nil {
		return nil, err
	}
	r, err := s.reservations.FindByID(ctx, id)
	if isNotFound(err) {
		return nil, errReservationNotFound
	}
	if err != nil {
		return nil, err
	}
	if t.customerID != nil && (r.CustomerID == nil || *r.CustomerID != *t.customerID) {
		return nil, domain.NotFound("", "Reserva no encontrada")
	}
	if t.allowedFrom != nil && !contains(t.allowedFrom, r.Status) {
		return nil, domain.Conflict("", "Esta reserva ya no se puede cancelar desde la web. Contactanos para cancelarla.")
	}
	if !domain.CanTransition(r.Status, t.to) {
		return nil, &domain.Error{Kind: domain.ErrInvalidTransition, Code: domain.CodeInvalidTransition,
			Message: "No se puede pasar una reserva de \"" + r.Status + "\" a \"" + t.to + "\""}
	}

	deposit := r.DepositAmount
	if t.depositAmount != nil {
		deposit = *t.depositAmount
	}
	if deposit < 0 || deposit > domain.MaxPriceARS || (r.PriceAtReservation > 0 && deposit > r.PriceAtReservation) {
		return nil, domain.Validation("La seña es inválida o supera el precio")
	}
	change := domain.StatusChange{
		FromStatus: r.Status, FromStockHeld: r.StockHeld, To: t.to,
		StockHeld: domain.HoldsStock(t.to), DepositAmount: deposit, At: now,
	}
	if t.to == domain.ReservationDepositPaid {
		if deposit <= 0 {
			return nil, domain.Validation("Indicá el monto de la seña recibida")
		}
		if !domain.ValidDepositMethod(t.depositMethod) {
			return nil, domain.Validation("Indicá cómo se recibió la seña (efectivo, transferencia, tarjeta en el local u otro)")
		}
		change.DepositMethod = t.depositMethod
		change.DepositPaidAt = &now
	}

	needsHold := domain.HoldsStock(t.to) && !r.StockHeld
	needsRelease := (t.to == domain.ReservationCancelled || t.to == domain.ReservationExpired) && r.StockHeld

	var held *domain.Watch
	if needsHold {
		if held, err = s.stock.HoldUnit(ctx, r.WatchID); err != nil {
			return nil, err
		}
	}
	out, err := s.reservations.ApplyStatusChange(ctx, r.ID, change)
	if err != nil {
		if held != nil {
			s.stock.CompensateHold(ctx, r.WatchID)
		}
		if isNotFound(err) {
			return nil, &domain.Error{Kind: domain.ErrConcurrentUpdate, Code: domain.CodeConcurrentUpdate,
				Message: "La reserva fue modificada al mismo tiempo. Recargá y volvé a intentar."}
		}
		return nil, err
	}

	actor := t.actor
	if actor == "" {
		actor = domain.SystemActor
	}
	switch {
	case held != nil:
		s.stock.RecordHold(ctx, held, r.ID, "Reserva confirmada", actor)
	case needsRelease:
		reason := "Reserva cancelada"
		if t.to == domain.ReservationExpired {
			reason = "Reserva vencida"
		}
		s.stock.ReleaseUnit(ctx, r.WatchID, r.ID, reason, actor)
	case t.to == domain.ReservationCompleted && r.StockHeld:
		s.stock.RecordSale(ctx, r.WatchID, r.ID, actor)
	}
	return out, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ---------- Vencimiento ----------

// ExpireDue vence las reservas pendientes/confirmadas cuyo plazo pasó (o
// solo la indicada) y devuelve al stock las unidades retenidas. Cada una se
// vence con una operación atómica condicional: si una cancelación ocurre al
// mismo tiempo, la unidad se devuelve una sola vez.
func (s *ReservationService) ExpireDue(ctx context.Context, only *primitive.ObjectID) (int, error) {
	now := s.now()
	n := 0
	for {
		before, err := s.reservations.ExpireNextDue(ctx, now, only)
		if isNotFound(err) {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		if before.StockHeld {
			s.stock.ReleaseUnit(ctx, before.WatchID, before.ID, "Reserva vencida automáticamente", domain.SystemActor)
		}
		n++
	}
}

// RunExpirationSweeper vence reservas periódicamente hasta que ctx termine.
// Además, el listado del Admin y cada cambio de estado vencen en el momento,
// así que el intervalo solo define cuánto puede tardar en volver una unidad
// al stock si nadie usa el Admin.
func (s *ReservationService) RunExpirationSweeper(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c, cancel := context.WithTimeout(ctx, 30*time.Second)
			if n, err := s.ExpireDue(c, nil); err != nil {
				s.log.Error("no se pudieron vencer reservas", "error", err)
			} else if n > 0 {
				s.log.Info("reservas vencidas automáticamente", "count", n)
			}
			cancel()
		}
	}
}

// ---------- Datos editables ----------

// UpdateDetails edita notas, seña acordada y vencimiento. El estado se
// cambia solo con las transiciones de arriba.
func (s *ReservationService) UpdateDetails(ctx context.Context, id primitive.ObjectID, in dto.ReservationDetailsRequest) (*domain.Reservation, error) {
	now := s.now()
	if _, err := s.ExpireDue(ctx, &id); err != nil {
		return nil, err
	}
	current, err := s.reservations.FindByID(ctx, id)
	if isNotFound(err) {
		return nil, errReservationNotFound
	}
	if err != nil {
		return nil, err
	}
	change := domain.DetailsChange{At: now, ClearExpiry: in.ClearExpiry}
	if in.Notes != nil {
		notes := strings.TrimSpace(*in.Notes)
		if len(notes) > domain.MaxReservationNotes {
			return nil, domain.Validation("Las notas son demasiado largas (máximo 2000 caracteres)")
		}
		change.Notes = &notes
	}
	if in.DepositAmount != nil {
		d := *in.DepositAmount
		if !domain.IsOpen(current.Status) {
			return nil, domain.Conflict("", "La reserva está cerrada: solo se pueden editar las notas")
		}
		if d < 0 || d > domain.MaxPriceARS || (current.PriceAtReservation > 0 && d > current.PriceAtReservation) {
			return nil, domain.Validation("La seña es inválida o supera el precio")
		}
		if current.Status == domain.ReservationDepositPaid && d == 0 {
			return nil, domain.Validation("Una reserva con seña paga debe tener un monto de seña")
		}
		change.DepositAmount = &d
	}
	if in.ExpiresAt != nil || in.ClearExpiry {
		if !domain.Expirable(current.Status) {
			return nil, domain.Conflict("", "Solo las reservas pendientes o confirmadas tienen vencimiento")
		}
		if !in.ClearExpiry {
			if msg := domain.ValidateExpiry(in.ExpiresAt, now); msg != "" {
				return nil, domain.Validation(msg)
			}
			change.ExpiresAt = in.ExpiresAt
		}
	}
	out, err := s.reservations.UpdateDetails(ctx, id, current.Status, change)
	if isNotFound(err) {
		return nil, domain.Conflict("", "La reserva fue modificada al mismo tiempo. Recargá y volvé a intentar.")
	}
	return out, err
}

// ---------- Consultas ----------

// ListForAdmin vence lo que corresponda y devuelve la página con los datos
// completos de los clientes (solo para el panel).
func (s *ReservationService) ListForAdmin(ctx context.Context, f domain.ReservationFilter, page, limit int) (dto.Page[dto.AdminReservationResponse], error) {
	if _, err := s.ExpireDue(ctx, nil); err != nil {
		return dto.Page[dto.AdminReservationResponse]{}, err
	}
	items, total, err := s.reservations.List(ctx, f, page, limit)
	if err != nil {
		return dto.Page[dto.AdminReservationResponse]{}, err
	}
	var ids []primitive.ObjectID
	for _, r := range items {
		if r.CustomerID != nil {
			ids = append(ids, *r.CustomerID)
		}
	}
	custs, err := s.customers.FindByIDs(ctx, ids)
	if err != nil {
		return dto.Page[dto.AdminReservationResponse]{}, err
	}
	byID := map[primitive.ObjectID]*domain.Customer{}
	for i := range custs {
		byID[custs[i].ID] = &custs[i]
	}
	out := make([]dto.AdminReservationResponse, 0, len(items))
	for _, r := range items {
		ar := dto.AdminReservationResponse{Reservation: r}
		if r.CustomerID != nil {
			ar.Customer = dto.NewCustomerResponse(byID[*r.CustomerID])
		}
		out = append(out, ar)
	}
	return dto.NewPage(out, page, limit, total), nil
}

// ListForCustomer devuelve SOLO las reservas del cliente indicado.
func (s *ReservationService) ListForCustomer(ctx context.Context, customerID primitive.ObjectID) ([]dto.CustomerReservationResponse, error) {
	if _, err := s.ExpireDue(ctx, nil); err != nil {
		return nil, err
	}
	items, err := s.reservations.ListByCustomer(ctx, customerID, 100)
	if err != nil {
		return nil, err
	}
	out := make([]dto.CustomerReservationResponse, 0, len(items))
	for i := range items {
		out = append(out, dto.NewCustomerReservationResponse(&items[i]))
	}
	return out, nil
}

// GetForCustomer devuelve una reserva del cliente; ajena o inexistente → 404
// (no se distingue, para no revelar reservas de otros).
func (s *ReservationService) GetForCustomer(ctx context.Context, customerID, id primitive.ObjectID) (*domain.Reservation, error) {
	r, err := s.reservations.FindForCustomer(ctx, id, customerID)
	if isNotFound(err) {
		return nil, domain.NotFound("", "Reserva no encontrada")
	}
	return r, err
}
