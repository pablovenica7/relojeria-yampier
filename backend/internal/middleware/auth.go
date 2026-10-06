package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/auth"
	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/httpx"
)

// AdminVerifier valida que la cuenta Admin del token siga vigente.
type AdminVerifier interface {
	Authenticate(ctx context.Context, email string, tokenVersion int) (*domain.AdminUser, error)
}

// CustomerVerifier carga al cliente del token si su sesión sigue vigente.
type CustomerVerifier interface {
	Authenticate(ctx context.Context, id primitive.ObjectID, tokenVersion int) (*domain.Customer, error)
}

// Auth valida sesiones. La parte común (leer el Bearer y validar el JWT) es
// una sola; RequireAdmin y RequireCustomer solo cambian el rol exigido y
// cómo se verifica la cuenta.
type Auth struct {
	tokens    *auth.TokenManager
	admins    AdminVerifier
	customers CustomerVerifier
}

func NewAuth(tokens *auth.TokenManager, admins AdminVerifier, customers CustomerVerifier) *Auth {
	return &Auth{tokens: tokens, admins: admins, customers: customers}
}

func bearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	return strings.TrimPrefix(h, "Bearer "), true
}

// claims valida el token y el rol. Escribe la respuesta de error y devuelve
// nil si no corresponde continuar.
func (a *Auth) claims(w http.ResponseWriter, r *http.Request, role, noTokenMsg, invalidMsg, wrongRoleMsg string) *auth.Claims {
	token, ok := bearer(r)
	if !ok {
		httpx.WriteMessage(w, http.StatusUnauthorized, "", noTokenMsg)
		return nil
	}
	c, err := a.tokens.Parse(token)
	if err != nil || c.Role == "" {
		httpx.WriteMessage(w, http.StatusUnauthorized, domain.CodeSessionExpired, invalidMsg)
		return nil
	}
	if c.Role != role {
		// Autenticado, pero sin permiso: un token de cliente nunca abre el
		// Admin, y uno de Admin no es una cuenta de cliente.
		httpx.WriteMessage(w, http.StatusForbidden, "", wrongRoleMsg)
		return nil
	}
	return c
}

// RequireAdmin protege el panel: token válido, rol admin y tokenVersion vigente.
func (a *Auth) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := a.claims(w, r, auth.RoleAdmin, "No autenticado", "Sesión inválida o expirada", "No tenés permiso para esta acción")
		if c == nil {
			return
		}
		admin, err := a.admins.Authenticate(r.Context(), c.Email, c.TV)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		next(w, r.WithContext(httpx.WithAdmin(r.Context(), admin.Email)))
	}
}

// RequireCustomer protege las rutas de clientes. El cliente se identifica
// SIEMPRE por el token, nunca por un id de la URL o del cuerpo: así no puede
// operar sobre datos de otra persona.
func (a *Auth) RequireCustomer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := a.claims(w, r, auth.RoleCustomer, "Iniciá sesión para continuar",
			"Tu sesión venció. Iniciá sesión de nuevo.", "Esta sección es para cuentas de clientes")
		if c == nil {
			return
		}
		id, err := primitive.ObjectIDFromHex(c.Subject)
		if err != nil {
			httpx.WriteMessage(w, http.StatusUnauthorized, "", "Sesión inválida")
			return
		}
		cust, err := a.customers.Authenticate(r.Context(), id, c.TV)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		next(w, r.WithContext(httpx.WithCustomer(r.Context(), cust)))
	}
}
