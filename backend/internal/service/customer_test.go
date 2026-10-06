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

type fakeCustomers struct {
	mu    sync.Mutex
	items map[primitive.ObjectID]*domain.Customer
}

func newFakeCustomers() *fakeCustomers {
	return &fakeCustomers{items: map[primitive.ObjectID]*domain.Customer{}}
}

func (f *fakeCustomers) Insert(_ context.Context, c *domain.Customer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.items {
		if x.Email == c.Email {
			return domain.ErrDuplicate
		}
	}
	c.ID = primitive.NewObjectID()
	cp := *c
	f.items[c.ID] = &cp
	return nil
}

func (f *fakeCustomers) FindByID(_ context.Context, id primitive.ObjectID) (*domain.Customer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (f *fakeCustomers) FindActiveByID(ctx context.Context, id primitive.ObjectID) (*domain.Customer, error) {
	c, err := f.FindByID(ctx, id)
	if err != nil || !c.Active {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeCustomers) FindByEmail(_ context.Context, email string) (*domain.Customer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.items {
		if c.Email == email {
			cp := *c
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeCustomers) EmailTakenByOther(ctx context.Context, email string, self primitive.ObjectID) (bool, error) {
	c, err := f.FindByEmail(ctx, email)
	return err == nil && c.ID != self, nil
}

func (f *fakeCustomers) UpdateProfile(_ context.Context, id primitive.ObjectID, ch domain.ProfileChange) (*domain.Customer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.items[id]
	c.FirstName, c.LastName, c.Email, c.Phone = ch.FirstName, ch.LastName, ch.Email, ch.Phone
	cp := *c
	return &cp, nil
}

func (f *fakeCustomers) ReplacePassword(_ context.Context, id primitive.ObjectID, expected *int, onlyActive bool, hash string, _ time.Time) (*domain.Customer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.items[id]
	if !ok || (expected != nil && c.TokenVersion != *expected) || (onlyActive && !c.Active) {
		return nil, domain.ErrNotFound
	}
	c.PasswordHash = hash
	c.TokenVersion++
	cp := *c
	return &cp, nil
}

func (f *fakeCustomers) SetActive(_ context.Context, id primitive.ObjectID, active bool, at time.Time) (*domain.Customer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.items[id]
	if !ok || c.Active == active {
		return nil, domain.ErrNotFound
	}
	c.Active = active
	if !active {
		c.TokenVersion++
		c.DeactivatedAt = &at
	}
	cp := *c
	return &cp, nil
}

func (f *fakeCustomers) List(context.Context, domain.CustomerFilter, int, int) ([]domain.Customer, int64, error) {
	return nil, 0, nil
}

type fakeTokens struct{}

func (fakeTokens) IssueCustomer(id primitive.ObjectID, tv int) (string, error) {
	return id.Hex() + ":" + string(rune('0'+tv)), nil
}

func registerReq(email string) dto.CustomerRequest {
	return dto.CustomerRequest{FirstName: "Ana", LastName: "Pérez", Email: email, Phone: "351 555 1234",
		Password: "secreta123", DocumentType: domain.DocDNI, DocumentNumber: "30.123.456",
		TaxCondition: domain.TaxConsumerFinal, PrivacyAccepted: true}
}

func TestRegisterLoginAndSessions(t *testing.T) {
	store := newFakeCustomers()
	svc := NewCustomerService(store, fakeTokens{}, nil)
	ctx := context.Background()

	_, c, err := svc.Register(ctx, registerReq("  Ana@Example.COM "))
	if err != nil || c.Email != "ana@example.com" || c.DocumentNumber != "30123456" || c.PasswordHash == "secreta123" {
		t.Fatalf("registro incorrecto: %v %+v", err, c)
	}
	_, _, err = svc.Register(ctx, registerReq("ana@example.com"))
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("email duplicado debe dar conflicto: %v", err)
	}
	noPrivacy := registerReq("x@y.com")
	noPrivacy.PrivacyAccepted = false
	if _, _, err := svc.Register(ctx, noPrivacy); !errors.Is(err, domain.ErrValidation) {
		t.Error("sin aceptar la privacidad no se crea la cuenta")
	}

	// Mismo error para contraseña incorrecta y email inexistente.
	_, _, e1 := svc.Login(ctx, "ana@example.com", "otra-clave1")
	_, _, e2 := svc.Login(ctx, "nadie@example.com", "otra-clave1")
	if !errors.Is(e1, domain.ErrUnauthorized) || e1.Error() != e2.Error() {
		t.Errorf("login inválido debe responder igual: %v / %v", e1, e2)
	}
	if _, _, err := svc.Login(ctx, "ANA@example.com", "secreta123"); err != nil {
		t.Fatalf("login válido falló: %v", err)
	}

	// Cambiar la contraseña invalida la versión anterior de las sesiones.
	if _, err := svc.Authenticate(ctx, c.ID, 0); err != nil {
		t.Fatal("la sesión inicial debería ser válida")
	}
	current, _ := store.FindByID(ctx, c.ID)
	if _, err := svc.ChangePassword(ctx, current, "secreta123", "nuevaClave99"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, c.ID, 0); !errors.Is(err, domain.ErrUnauthorized) {
		t.Error("una sesión con tokenVersion vieja debe rechazarse")
	}

	// Desactivar: corta login y sesiones, sin borrar.
	if _, err := svc.SetActive(ctx, c.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(ctx, "ana@example.com", "nuevaClave99"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Error("una cuenta desactivada no puede iniciar sesión")
	}
	if _, err := svc.SetActive(ctx, c.ID, false); !errors.Is(err, domain.ErrConflict) {
		t.Error("desactivar dos veces debe informar que ya estaba así")
	}
}

type fakeResets struct {
	mu    sync.Mutex
	items []*domain.PasswordReset
}

func (f *fakeResets) CountRecentUnused(_ context.Context, id primitive.ObjectID, since time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, p := range f.items {
		if p.CustomerID == id && p.UsedAt == nil && p.CreatedAt.After(since) {
			n++
		}
	}
	return n, nil
}

func (f *fakeResets) InvalidateAll(_ context.Context, id primitive.ObjectID, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.items {
		if p.CustomerID == id && p.UsedAt == nil {
			p.UsedAt = &at
		}
	}
	return nil
}

func (f *fakeResets) Insert(_ context.Context, p *domain.PasswordReset) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *p
	f.items = append(f.items, &cp)
	return nil
}

func (f *fakeResets) Consume(_ context.Context, hash string, now time.Time) (*domain.PasswordReset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.items {
		if p.TokenHash == hash && p.UsedAt == nil && p.ExpiresAt.After(now) {
			p.UsedAt = &now
			return p, nil
		}
	}
	return nil, domain.ErrNotFound
}

type nopMailer struct{}

func (nopMailer) Send(string, string, string) error { return nil }

func TestPasswordResetSingleUseAndExpiry(t *testing.T) {
	store := newFakeCustomers()
	ctx := context.Background()
	clock := &fixedClock{now: time.Now()}
	_, c, _ := NewCustomerService(store, fakeTokens{}, clock.Now).Register(ctx, registerReq("ana@example.com"))
	resets := &fakeResets{}
	svc := NewPasswordResetService(store, resets, nopMailer{}, "https://yampier.test", nil, clock.Now)

	if tok, err := svc.Request(ctx, "nadie@example.com"); err != nil || tok != "" {
		t.Error("un email inexistente no genera token (y no da error)")
	}
	token, err := svc.Request(ctx, "ana@example.com")
	if err != nil || token == "" {
		t.Fatalf("no se generó token: %v", err)
	}
	if resets.items[0].TokenHash == token || resets.items[0].TokenHash != HashResetToken(token) {
		t.Fatal("solo debe guardarse el hash del token")
	}
	if err := svc.Reset(ctx, token, "corta"); !errors.Is(err, domain.ErrValidation) {
		t.Error("contraseña inválida debe rechazarse sin consumir el token")
	}
	if err := svc.Reset(ctx, token, "otraClave77"); err != nil {
		t.Fatalf("reset válido falló: %v", err)
	}
	var de *domain.Error
	if err := svc.Reset(ctx, token, "otraClave88"); !errors.As(err, &de) || de.Code != domain.CodeInvalidResetToken {
		t.Errorf("el token es de un solo uso: %v", err)
	}
	if u, _ := store.FindByID(ctx, c.ID); u.TokenVersion != 1 {
		t.Error("restablecer debe invalidar las sesiones anteriores")
	}

	clock.Advance(3 * time.Minute)
	token2, _ := svc.Request(ctx, "ana@example.com")
	clock.Advance(ResetTokenTTL + time.Minute)
	if err := svc.Reset(ctx, token2, "valida123"); err == nil {
		t.Error("un token vencido no debe funcionar")
	}
}
