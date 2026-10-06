package service

import (
	"context"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/domain"
)

// Fakes en memoria para testear reglas de negocio sin MongoDB. Cada método
// toma el mutex completo, así reproduce la atomicidad de las operaciones
// condicionales de Mongo (condición + cambio en un solo paso).

type fakeWatches struct {
	mu    sync.Mutex
	items map[primitive.ObjectID]*domain.Watch
}

func newFakeWatches() *fakeWatches {
	return &fakeWatches{items: map[primitive.ObjectID]*domain.Watch{}}
}

func (f *fakeWatches) add(name string, qty int, price int64) *domain.Watch {
	f.mu.Lock()
	defer f.mu.Unlock()
	w := &domain.Watch{ID: primitive.NewObjectID(), Name: name, Model: name, ModelKey: domain.ModelKeyFor(name),
		StockQuantity: qty, PriceARS: price, Image: "/images/" + name + ".webp"}
	f.items[w.ID] = w
	return w
}

func (f *fakeWatches) stock(id primitive.ObjectID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.items[id].StockQuantity
}

func (f *fakeWatches) AddStock(_ context.Context, id primitive.ObjectID, delta int, requireActive bool, _ time.Time) (*domain.Watch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.items[id]
	if !ok || (delta < 0 && w.StockQuantity < -delta) || (requireActive && w.ArchivedAt != nil) {
		return nil, domain.ErrNotFound
	}
	w.StockQuantity += delta
	cp := *w
	return &cp, nil
}

func (f *fakeWatches) SetStock(_ context.Context, id primitive.ObjectID, base *int, qty int, _ time.Time) (*domain.Watch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.items[id]
	if !ok || (base != nil && w.StockQuantity != *base) {
		return nil, domain.ErrNotFound
	}
	before := *w
	w.StockQuantity = qty
	return &before, nil
}

func (f *fakeWatches) FindByID(_ context.Context, id primitive.ObjectID, activeOnly bool) (*domain.Watch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.items[id]
	if !ok || (activeOnly && w.ArchivedAt != nil) {
		return nil, domain.ErrNotFound
	}
	cp := *w
	return &cp, nil
}

type fakeMovements struct {
	mu    sync.Mutex
	items []domain.StockMovement
	keys  map[string]bool
}

func (f *fakeMovements) Insert(_ context.Context, m *domain.StockMovement) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.keys == nil {
		f.keys = map[string]bool{}
	}
	if m.IdempotencyKey != "" {
		if f.keys[m.IdempotencyKey] {
			return domain.ErrDuplicate
		}
		f.keys[m.IdempotencyKey] = true
	}
	f.items = append(f.items, *m)
	return nil
}

func (f *fakeMovements) List(context.Context, domain.MovementFilter, int, int) ([]domain.StockMovement, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.StockMovement(nil), f.items...), int64(len(f.items)), nil
}

func (f *fakeMovements) ofType(t string) []domain.StockMovement {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.StockMovement
	for _, m := range f.items {
		if m.Type == t {
			out = append(out, m)
		}
	}
	return out
}

type fakeReservations struct {
	mu    sync.Mutex
	items map[primitive.ObjectID]*domain.Reservation
}

func newFakeReservations() *fakeReservations {
	return &fakeReservations{items: map[primitive.ObjectID]*domain.Reservation{}}
}

func (f *fakeReservations) Insert(_ context.Context, r *domain.Reservation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r.ID = primitive.NewObjectID()
	cp := *r
	f.items[r.ID] = &cp
	return nil
}

func (f *fakeReservations) get(id primitive.ObjectID) domain.Reservation {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.items[id]
}

func (f *fakeReservations) FindByID(_ context.Context, id primitive.ObjectID) (*domain.Reservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (f *fakeReservations) FindForCustomer(ctx context.Context, id, customerID primitive.ObjectID) (*domain.Reservation, error) {
	r, err := f.FindByID(ctx, id)
	if err != nil || r.CustomerID == nil || *r.CustomerID != customerID {
		return nil, domain.ErrNotFound
	}
	return r, nil
}

func (f *fakeReservations) ApplyStatusChange(_ context.Context, id primitive.ObjectID, c domain.StatusChange) (*domain.Reservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.items[id]
	if !ok || r.Status != c.FromStatus || r.StockHeld != c.FromStockHeld {
		return nil, domain.ErrNotFound
	}
	r.Status, r.StockHeld, r.DepositAmount, r.UpdatedAt = c.To, c.StockHeld, c.DepositAmount, c.At
	if c.DepositPaidAt != nil {
		r.DepositMethod, r.DepositPaidAt = c.DepositMethod, c.DepositPaidAt
	}
	at := c.At
	switch c.To {
	case domain.ReservationConfirmed:
		r.ConfirmedAt = &at
	case domain.ReservationCompleted:
		r.CompletedAt = &at
	case domain.ReservationCancelled:
		r.CancelledAt = &at
	case domain.ReservationExpired:
		r.ExpiredAt = &at
	}
	cp := *r
	return &cp, nil
}

func (f *fakeReservations) ExpireNextDue(_ context.Context, now time.Time, only *primitive.ObjectID) (*domain.Reservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, r := range f.items {
		if only != nil && id != *only {
			continue
		}
		if domain.Expirable(r.Status) && r.ExpiresAt != nil && !r.ExpiresAt.After(now) {
			before := *r
			r.Status, r.StockHeld, r.ExpiredAt = domain.ReservationExpired, false, &now
			return &before, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeReservations) UpdateDetails(_ context.Context, id primitive.ObjectID, status string, c domain.DetailsChange) (*domain.Reservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.items[id]
	if !ok || r.Status != status {
		return nil, domain.ErrNotFound
	}
	if c.Notes != nil {
		r.Notes = *c.Notes
	}
	if c.DepositAmount != nil {
		r.DepositAmount = *c.DepositAmount
	}
	if c.ClearExpiry {
		r.ExpiresAt = nil
	} else if c.ExpiresAt != nil {
		r.ExpiresAt = c.ExpiresAt
	}
	cp := *r
	return &cp, nil
}

func (f *fakeReservations) List(context.Context, domain.ReservationFilter, int, int) ([]domain.Reservation, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Reservation
	for _, r := range f.items {
		out = append(out, *r)
	}
	return out, int64(len(out)), nil
}

func (f *fakeReservations) ListByCustomer(_ context.Context, customerID primitive.ObjectID, _ int) ([]domain.Reservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Reservation
	for _, r := range f.items {
		if r.CustomerID != nil && *r.CustomerID == customerID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeReservations) CountForCustomer(_ context.Context, customerID primitive.ObjectID, watchID *primitive.ObjectID, statuses []string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, r := range f.items {
		if r.CustomerID != nil && *r.CustomerID == customerID && contains(statuses, r.Status) && (watchID == nil || r.WatchID == *watchID) {
			n++
		}
	}
	return n, nil
}

type fakeCustomerLookup struct{}

func (fakeCustomerLookup) FindByIDs(context.Context, []primitive.ObjectID) ([]domain.Customer, error) {
	return nil, nil
}

// fixedClock es un reloj controlable.
type fixedClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
