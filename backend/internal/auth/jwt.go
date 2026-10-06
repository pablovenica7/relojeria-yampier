// Package auth genera y valida sesiones (JWT) y contraseñas. No conoce
// MongoDB ni HTTP: la verificación de que la cuenta existe y que su
// tokenVersion sigue vigente la hace el middleware con los services.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Roles de sesión. Admin y cliente usan el mismo mecanismo, pero cada
// middleware exige su rol: un token de cliente NUNCA da acceso al Admin.
const (
	RoleAdmin    = "admin"
	RoleCustomer = "customer"

	AdminTokenTTL    = 24 * time.Hour
	CustomerTokenTTL = 72 * time.Hour
)

// Claims del token.
type Claims struct {
	Email string `json:"email,omitempty"`
	Role  string `json:"role"`
	// TV (tokenVersion) debe coincidir con la versión guardada en la cuenta:
	// al cambiar/restablecer la contraseña o desactivar la cuenta, la versión
	// sube y todos los tokens anteriores dejan de valer.
	TV int `json:"tv"`
	jwt.RegisteredClaims
}

// TokenManager firma y valida tokens con una clave HMAC.
type TokenManager struct {
	secret []byte
	now    func() time.Time
}

// NewTokenManager crea el gestor. La clave viene de la configuración
// (JWT_SECRET, obligatoria): no hay clave por defecto.
func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret), now: time.Now}
}

func (m *TokenManager) sign(c Claims, ttl time.Duration) (string, error) {
	now := m.now()
	c.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))
	c.IssuedAt = jwt.NewNumericDate(now)
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
}

// IssueAdmin emite un token de Admin.
func (m *TokenManager) IssueAdmin(email string, tokenVersion int) (string, error) {
	return m.sign(Claims{Email: email, Role: RoleAdmin, TV: tokenVersion}, AdminTokenTTL)
}

// IssueCustomer emite un token de cliente. Identifica al cliente por su id
// (subject) y no lleva datos personales: puede quedar en el navegador.
func (m *TokenManager) IssueCustomer(id primitive.ObjectID, tokenVersion int) (string, error) {
	return m.sign(Claims{Role: RoleCustomer, TV: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{Subject: id.Hex()}}, CustomerTokenTTL)
}

// Parse valida firma, algoritmo y vencimiento.
func (m *TokenManager) Parse(token string) (*Claims, error) {
	c := &Claims{}
	_, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (interface{}, error) {
		// Se exige HMAC: sin esto, un token con "alg": "none" u otro algoritmo
		// podría eludir la verificación de firma (confusión de algoritmo).
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("algoritmo de firma no soportado")
		}
		return m.secret, nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	return c, nil
}
