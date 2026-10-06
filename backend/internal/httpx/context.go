package httpx

import (
	"context"

	"relojeria-yampier/internal/domain"
)

// Claves de contexto (tipos propios: no colisionan con otros paquetes).
type (
	requestIDKey struct{}
	adminKey     struct{}
	customerKey  struct{}
)

// WithRequestID / RequestID: identificador del request (logs y X-Request-ID).
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// WithAdmin / AdminEmail: admin autenticado (para auditoría de movimientos).
func WithAdmin(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, adminKey{}, email)
}

func AdminEmail(ctx context.Context) string {
	e, _ := ctx.Value(adminKey{}).(string)
	return e
}

// WithCustomer / Customer: cliente autenticado (cargado por el middleware).
func WithCustomer(ctx context.Context, c *domain.Customer) context.Context {
	return context.WithValue(ctx, customerKey{}, c)
}

func Customer(ctx context.Context) *domain.Customer {
	c, _ := ctx.Value(customerKey{}).(*domain.Customer)
	return c
}
