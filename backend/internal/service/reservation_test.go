package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
)

// Tests UNITARIOS de las reglas de reservas y stock: sin MongoDB ni HTTP.

type reservationEnv struct {
	watches *fakeWatches
	moves   *fakeMovements
	res     *fakeReservations
	clock   *fixedClock
	stock   *StockService
	svc     *ReservationService
}

func newReservationEnv() *reservationEnv {
	e := &reservationEnv{watches: newFakeWatches(), moves: &fakeMovements{}, res: newFakeReservations(),
		clock: &fixedClock{now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}}
	e.stock = NewStockService(e.watches, e.moves, nil, e.clock.Now)
	e.svc = NewReservationService(e.res, e.watches, fakeCustomerLookup{}, e.stock, nil, e.clock.Now)
	return e
}

func (e *reservationEnv) pending(t *testing.T, w *domain.Watch, expires *time.Time) *domain.Reservation {
	t.Helper()
	r, err := e.svc.CreateByAdmin(context.Background(), dto.AdminReservationRequest{
		WatchID: w.ID.Hex(), CustomerName: "Cliente", Phone: "351", ExpiresAt: expires,
	}, "admin@test")
	if err != nil {
		t.Fatalf("no se pudo crear la reserva: %v", err)
	}
	return r
}

func wantKind(t *testing.T, err, kind error) {
	t.Helper()
	if !errors.Is(err, kind) {
		t.Fatalf("esperaba error %v, obtuvo %v", kind, err)
	}
}

func TestPendingDoesNotHoldAndConfirmHoldsWithSnapshot(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("GA-2100-1A1", 2, 150000)

	r := e.pending(t, w, nil)
	if r.StockHeld || e.watches.stock(w.ID) != 2 || len(e.moves.items) != 0 {
		t.Fatal("una reserva pendiente no retiene stock ni genera movimiento")
	}
	if r.PriceAtReservation != 150000 || r.WatchModelSnapshot != "GA-2100-1A1" || r.WatchImageSnapshot == "" {
		t.Errorf("snapshot incorrecto: %+v", r)
	}

	// El Admin cambia el precio después: la reserva conserva el acordado.
	e.watches.items[w.ID].PriceARS = 170000

	r, err := e.svc.Confirm(ctx, r.ID, "admin@test")
	if err != nil {
		t.Fatal(err)
	}
	if !r.StockHeld || r.ConfirmedAt == nil || e.watches.stock(w.ID) != 1 || r.PriceAtReservation != 150000 {
		t.Fatalf("confirmar debe retener 1 unidad sin tocar el snapshot: %+v", r)
	}
	hold := e.moves.ofType(domain.MoveReservationHold)
	if len(hold) != 1 || hold[0].StockBefore != 2 || hold[0].StockAfter != 1 || hold[0].CreatedBy != "admin@test" {
		t.Errorf("movimiento de retención incorrecto: %+v", hold)
	}
}

func TestCompletedDoesNotReleaseStock(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("F-91W-1", 1, 30000)
	r := e.pending(t, w, nil)
	r, _ = e.svc.Confirm(ctx, r.ID, "a")
	dep := int64(10000)
	if _, err := e.svc.MarkDepositPaid(ctx, r.ID, &dep, "", "a"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("la seña exige método: %v", err)
	}
	r, err := e.svc.MarkDepositPaid(ctx, r.ID, &dep, domain.DepositCash, "a")
	if err != nil || r.DepositPaidAt == nil || r.DepositMethod != domain.DepositCash {
		t.Fatalf("seña mal registrada: %v %+v", err, r)
	}
	r, err = e.svc.Complete(ctx, r.ID, "a")
	if err != nil || r.StockHeld || r.CompletedAt == nil {
		t.Fatalf("completar falló: %v %+v", err, r)
	}
	if e.watches.stock(w.ID) != 0 {
		t.Error("completar NO debe devolver la unidad")
	}
	sale := e.moves.ofType(domain.MoveSale)
	if len(sale) != 1 || sale[0].QuantityDelta != 0 {
		t.Errorf("la venta se anota con delta 0: %+v", sale)
	}
	_, err = e.svc.ChangeStatus(ctx, r.ID, domain.ReservationPending, nil, "", "a")
	wantKind(t, err, domain.ErrInvalidTransition)
}

func TestCancelReleasesOnlyIfHeldAndOnlyOnce(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("MTP-V002D-1B", 1, 0)
	r := e.pending(t, w, nil)
	r, _ = e.svc.Confirm(ctx, r.ID, "a")

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.svc.Cancel(ctx, r.ID, "a")
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok != 1 || e.watches.stock(w.ID) != 1 {
		t.Errorf("solo una cancelación se aplica y devuelve una unidad: ok=%d stock=%d", ok, e.watches.stock(w.ID))
	}
	if n := len(e.moves.ofType(domain.MoveReservationRelease)); n != 1 {
		t.Errorf("debe haber un solo movimiento de liberación, hay %d", n)
	}

	p := e.pending(t, w, nil)
	if _, err := e.svc.Cancel(ctx, p.ID, "a"); err != nil || e.watches.stock(w.ID) != 1 {
		t.Error("cancelar una pendiente no toca el stock")
	}
}

func TestLastUnitCannotBeConfirmedTwice(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("A168WA-1W", 1, 0)
	const n = 10
	ids := make([]primitive.ObjectID, n)
	for i := range ids {
		ids[i] = e.pending(t, w, nil).ID
	}
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = e.svc.Confirm(ctx, ids[i], "a")
		}(i)
	}
	wg.Wait()
	confirmed := 0
	for _, err := range results {
		if err == nil {
			confirmed++
		} else if !errors.Is(err, domain.ErrOutOfStock) {
			t.Errorf("el resto debe fallar por falta de stock: %v", err)
		}
	}
	if confirmed != 1 || e.watches.stock(w.ID) != 0 {
		t.Errorf("solo una reserva se queda con la última unidad: confirmadas=%d stock=%d", confirmed, e.watches.stock(w.ID))
	}
}

func TestOutOfStockAndArchivedCannotBeHeld(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("LA670WA-1", 0, 0)
	_, err := e.svc.CreateByAdmin(ctx, dto.AdminReservationRequest{WatchID: w.ID.Hex(), CustomerName: "A", Phone: "1", Status: domain.ReservationConfirmed}, "a")
	wantKind(t, err, domain.ErrOutOfStock)
	if len(e.res.items) != 0 || e.watches.stock(w.ID) != 0 {
		t.Error("una reserva confirmada sin stock no se guarda ni deja stock negativo")
	}
}

func TestExpirationReleasesHeldStock(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("LTP-V002D-7B", 2, 0)
	soon := e.clock.Now().Add(time.Minute)
	held := e.pending(t, w, &soon)
	held, _ = e.svc.Confirm(ctx, held.ID, "a")
	notHeld := e.pending(t, w, &soon)

	e.clock.Advance(2 * time.Minute)
	n, err := e.svc.ExpireDue(ctx, nil)
	if err != nil || n != 2 {
		t.Fatalf("debían vencer 2, vencieron %d (%v)", n, err)
	}
	if e.watches.stock(w.ID) != 2 {
		t.Errorf("solo la confirmada devuelve su unidad: stock=%d", e.watches.stock(w.ID))
	}
	for _, id := range []primitive.ObjectID{held.ID, notHeld.ID} {
		if r := e.res.get(id); r.Status != domain.ReservationExpired || r.StockHeld {
			t.Errorf("reserva %s debería estar vencida: %+v", id.Hex(), r)
		}
	}
	rel := e.moves.ofType(domain.MoveReservationRelease)
	if len(rel) != 1 || rel[0].CreatedBy != domain.SystemActor {
		t.Errorf("la liberación por vencimiento la anota el sistema: %+v", rel)
	}
	if n, _ := e.svc.ExpireDue(ctx, nil); n != 0 {
		t.Error("el vencimiento debe ser idempotente")
	}
}

func TestCustomerRequestRulesAndOwnership(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("GA-2100-1A1", 1, 150000)
	ana := &domain.Customer{ID: primitive.NewObjectID(), FirstName: "Ana", LastName: "P", Phone: "351", Email: "a@b.com"}
	beto := &domain.Customer{ID: primitive.NewObjectID()}

	_, err := e.svc.RequestByCustomer(ctx, ana, dto.CustomerReservationRequest{WatchID: w.ID.Hex()})
	wantKind(t, err, domain.ErrValidation) // sin aceptar términos

	r, err := e.svc.RequestByCustomer(ctx, ana, dto.CustomerReservationRequest{WatchID: w.ID.Hex(), TermsAccepted: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != domain.ReservationPending || r.StockHeld || r.TermsVersion != domain.TermsVersion || r.ExpiresAt == nil || e.watches.stock(w.ID) != 1 {
		t.Fatalf("la solicitud debe quedar pendiente sin retener stock: %+v", r)
	}
	_, err = e.svc.RequestByCustomer(ctx, ana, dto.CustomerReservationRequest{WatchID: w.ID.Hex(), TermsAccepted: true})
	wantKind(t, err, domain.ErrConflict) // duplicada

	// Otro cliente no puede verla ni cancelarla (404, no 403).
	_, err = e.svc.GetForCustomer(ctx, beto.ID, r.ID)
	wantKind(t, err, domain.ErrNotFound)
	_, err = e.svc.CancelByCustomer(ctx, beto.ID, r.ID)
	wantKind(t, err, domain.ErrNotFound)

	// Confirmada: el cliente ya no puede cancelarla.
	if _, err := e.svc.Confirm(ctx, r.ID, "a"); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.CancelByCustomer(ctx, ana.ID, r.ID)
	wantKind(t, err, domain.ErrConflict)
	if e.watches.stock(w.ID) != 0 {
		t.Error("un intento de cancelación rechazado no toca el stock")
	}
}

func TestAdjustStockNeverNegative(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("F-91W-1", 1, 0)
	_, err := e.stock.Adjust(ctx, w.ID, -2, "", "", "a")
	wantKind(t, err, domain.ErrConflict)
	_, err = e.stock.Adjust(ctx, primitive.NewObjectID(), 1, "", "", "a")
	wantKind(t, err, domain.ErrNotFound)
	_, err = e.stock.Adjust(ctx, w.ID, 0, "", "", "a")
	wantKind(t, err, domain.ErrValidation)
	after, err := e.stock.Adjust(ctx, w.ID, 3, domain.MoveStockEntry, "Ingreso", "a")
	if err != nil || after.StockQuantity != 4 {
		t.Fatalf("ajuste válido falló: %v", err)
	}
	m := e.moves.ofType(domain.MoveStockEntry)
	if len(m) != 1 || m[0].StockBefore != 1 || m[0].StockAfter != 4 {
		t.Errorf("movimiento mal registrado: %+v", m)
	}
}

func TestSetQuantityDetectsConcurrentChange(t *testing.T) {
	e := newReservationEnv()
	ctx := context.Background()
	w := e.watches.add("MTP", 5, 0)
	base := 4 // el Admin vio 4, pero ahora hay 5
	err := e.stock.SetQuantity(ctx, w.ID, &base, 8, domain.MoveManualAdjustment, "", "a")
	wantKind(t, err, domain.ErrConcurrentUpdate)
	base = 5
	if err := e.stock.SetQuantity(ctx, w.ID, &base, 8, domain.MoveManualAdjustment, "Conteo", "a"); err != nil || e.watches.stock(w.ID) != 8 {
		t.Fatalf("ajuste con base correcta falló: %v", err)
	}
	if m := e.moves.ofType(domain.MoveManualAdjustment); len(m) != 1 || m[0].QuantityDelta != 3 {
		t.Errorf("ajuste 5 → 8 mal registrado: %+v", m)
	}
}
