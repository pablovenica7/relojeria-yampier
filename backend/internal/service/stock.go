package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/domain"
)

// stockStore es lo que StockService necesita del catálogo: solo operaciones
// atómicas de stock (la condición y el cambio en una única operación).
type stockStore interface {
	AddStock(ctx context.Context, id primitive.ObjectID, delta int, requireActive bool, at time.Time) (*domain.Watch, error)
	SetStock(ctx context.Context, id primitive.ObjectID, base *int, qty int, at time.Time) (*domain.Watch, error)
	FindByID(ctx context.Context, id primitive.ObjectID, activeOnly bool) (*domain.Watch, error)
}

type movementStore interface {
	Insert(ctx context.Context, m *domain.StockMovement) error
	List(ctx context.Context, f domain.MovementFilter, page, limit int) ([]domain.StockMovement, int64, error)
}

// StockService es el ÚNICO punto que modifica stockQuantity y el que anota
// cada cambio en el libro de movimientos.
//
// Consistencia sin transacciones (MongoDB corre sin replica set):
//   - Cada cambio de stock es una operación atómica condicional: retener la
//     última unidad desde dos requests a la vez solo puede ganarla uno.
//   - El movimiento se anota DESPUÉS de que el cambio quedó firme, con el
//     stock anterior/posterior que devolvió la misma operación atómica. Si el
//     proceso se cae en el medio, puede faltar una línea del libro, pero
//     nunca se anota un movimiento que no ocurrió.
//   - Las compensaciones (retener y devolver por un conflicto) dejan el stock
//     igual y no se anotan.
//   - Los movimientos de reservas llevan una clave de idempotencia
//     (tipo:reserva) con índice único: un reintento no duplica la línea.
type StockService struct {
	watches stockStore
	moves   movementStore
	log     *slog.Logger
	now     Clock
}

func NewStockService(watches stockStore, moves movementStore, log *slog.Logger, now Clock) *StockService {
	return &StockService{watches: watches, moves: moves, log: loggerOrDefault(log), now: clockOrNow(now)}
}

// HoldUnit descuenta una unidad de un reloj activo y devuelve el reloj
// después del cambio. Sin stock (o archivado) → ErrOutOfStock. No anota el
// movimiento: lo hace quien confirma la reserva cuando ésta quedó firme.
func (s *StockService) HoldUnit(ctx context.Context, watchID primitive.ObjectID) (*domain.Watch, error) {
	after, err := s.watches.AddStock(ctx, watchID, -1, true, s.now())
	if isNotFound(err) {
		return nil, &domain.Error{Kind: domain.ErrOutOfStock, Code: domain.CodeOutOfStock, Message: "No hay stock disponible para este reloj"}
	}
	return after, err
}

// CompensateHold devuelve una unidad retenida por una operación que no
// llegó a completarse (stock neto sin cambios: no se anota).
func (s *StockService) CompensateHold(ctx context.Context, watchID primitive.ObjectID) {
	if _, err := s.watches.AddStock(ctx, watchID, 1, false, s.now()); err != nil {
		s.log.Error("ATENCIÓN: no se pudo compensar la retención de stock", "watch_id", watchID.Hex(), "error", err)
	}
}

// RecordHold anota la retención de una unidad por una reserva.
func (s *StockService) RecordHold(ctx context.Context, after *domain.Watch, reservationID primitive.ObjectID, reason, actor string) {
	s.record(ctx, after, domain.MoveReservationHold, -1, &reservationID, reason, actor)
}

// ReleaseUnit devuelve una unidad de una reserva cancelada o vencida y anota
// el movimiento. Si falla, la reserva ya quedó cerrada: se registra en el log
// para corregir el stock a mano (stock por debajo del real, nunca de más).
func (s *StockService) ReleaseUnit(ctx context.Context, watchID, reservationID primitive.ObjectID, reason, actor string) {
	after, err := s.watches.AddStock(ctx, watchID, 1, false, s.now())
	if err != nil {
		s.log.Error("ATENCIÓN: no se pudo devolver 1 unidad al stock", "watch_id", watchID.Hex(), "error", err)
		return
	}
	s.record(ctx, after, domain.MoveReservationRelease, 1, &reservationID, reason, actor)
}

// RecordSale anota una venta completada con delta 0: la unidad ya se había
// descontado al confirmar, así que no se descuenta dos veces.
func (s *StockService) RecordSale(ctx context.Context, watchID, reservationID primitive.ObjectID, actor string) {
	w, err := s.watches.FindByID(ctx, watchID, false)
	if err != nil {
		s.log.Error("no se pudo anotar la venta en el libro de stock", "watch_id", watchID.Hex(), "error", err)
		return
	}
	s.record(ctx, w, domain.MoveSale, 0, &reservationID, "Venta completada en el local", actor)
}

// RecordInitialEntry anota el stock con el que se dio de alta un reloj.
func (s *StockService) RecordInitialEntry(ctx context.Context, w *domain.Watch, actor string) {
	if w.StockQuantity > 0 {
		s.record(ctx, w, domain.MoveStockEntry, w.StockQuantity, nil, "Alta del reloj", actor)
	}
}

// Adjust suma o resta unidades a mano (Admin), sin permitir negativos.
func (s *StockService) Adjust(ctx context.Context, watchID primitive.ObjectID, delta int, moveType, reason, actor string) (*domain.Watch, error) {
	if delta == 0 || delta > domain.MaxStockQuantity || delta < -domain.MaxStockQuantity {
		return nil, domain.Validation("Indicá un ajuste de stock entero distinto de cero")
	}
	if moveType == "" {
		moveType = domain.MoveManualAdjustment
	}
	reason = strings.TrimSpace(reason)
	if !domain.ValidManualMoveType(moveType) || len(reason) > 300 {
		return nil, domain.Validation("Tipo de movimiento o motivo inválido")
	}
	after, err := s.watches.AddStock(ctx, watchID, delta, false, s.now())
	if isNotFound(err) {
		if _, ferr := s.watches.FindByID(ctx, watchID, false); ferr != nil {
			if isNotFound(ferr) {
				return nil, errWatchNotFound
			}
			return nil, ferr
		}
		return nil, domain.Conflict("", "El stock no puede quedar negativo")
	}
	if err != nil {
		return nil, err
	}
	s.record(ctx, after, moveType, delta, nil, reason, actor)
	return after, nil
}

// SetQuantity fija la cantidad desde el formulario del Admin. Con base, solo
// se aplica si nadie cambió el stock desde que se abrió el formulario (por
// ejemplo, una reserva confirmada en paralelo).
func (s *StockService) SetQuantity(ctx context.Context, watchID primitive.ObjectID, base *int, qty int, moveType, reason, actor string) error {
	before, err := s.watches.SetStock(ctx, watchID, base, qty, s.now())
	if isNotFound(err) {
		if _, ferr := s.watches.FindByID(ctx, watchID, false); ferr == nil {
			return &domain.Error{Kind: domain.ErrConcurrentUpdate, Code: domain.CodeConcurrentUpdate,
				Message: "El stock cambió mientras editabas (por ejemplo, por una reserva). Recargá el reloj y volvé a intentar."}
		}
		return errWatchNotFound
	}
	if err != nil {
		return err
	}
	if delta := qty - before.StockQuantity; delta != 0 {
		after := *before
		after.StockQuantity = qty
		s.record(ctx, &after, moveType, delta, nil, reason, actor)
	}
	return nil
}

// ListMovements devuelve el historial filtrado.
func (s *StockService) ListMovements(ctx context.Context, f domain.MovementFilter, page, limit int) ([]domain.StockMovement, int64, error) {
	return s.moves.List(ctx, f, page, limit)
}

// record arma y guarda el movimiento a partir del reloj DESPUÉS del cambio.
func (s *StockService) record(ctx context.Context, after *domain.Watch, moveType string, delta int, reservationID *primitive.ObjectID, reason, actor string) {
	if actor == "" {
		actor = domain.SystemActor
	}
	m := &domain.StockMovement{
		WatchID: after.ID, WatchNameSnapshot: after.Name, Type: moveType,
		QuantityDelta: delta, StockBefore: after.StockQuantity - delta, StockAfter: after.StockQuantity,
		ReservationID: reservationID, Reason: reason, CreatedBy: actor, CreatedAt: s.now(),
	}
	if reservationID != nil {
		m.IdempotencyKey = moveType + ":" + reservationID.Hex()
	}
	if err := s.moves.Insert(ctx, m); err != nil && !isDuplicate(err) {
		// El cambio de stock ya ocurrió y no se revierte por no poder anotarlo.
		s.log.Error("ATENCIÓN: no se pudo registrar el movimiento de stock", "type", moveType, "watch_id", after.ID.Hex(), "error", err)
	}
}
