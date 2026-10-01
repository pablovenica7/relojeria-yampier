package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

// jwtSecretBytes se fija una única vez al arrancar, desde setJWTSecret (ver main.go).
// No hay valor por defecto: si JWT_SECRET no está definido, el proceso no arranca
// (ver requireJWTSecret en main.go). Esto evita que producción quede corriendo
// con una clave insegura conocida públicamente en el código fuente.
var jwtSecretBytes []byte

func setJWTSecret(secret string) {
	jwtSecretBytes = []byte(secret)
}

// Roles de sesión. Admin y cliente usan el mismo mecanismo (JWT firmado con
// HMAC) pero cada middleware exige su rol: un token de cliente NUNCA da
// acceso al panel Admin, y viceversa.
const (
	RoleAdmin    = "admin"
	RoleCustomer = "customer"

	adminTokenTTL    = 24 * time.Hour
	customerTokenTTL = 72 * time.Hour
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type claims struct {
	Email string `json:"email,omitempty"`
	Role  string `json:"role"`
	// TV (tokenVersion) debe coincidir con la versión guardada en la cuenta.
	// Al cambiar/restablecer la contraseña o desactivar la cuenta, la versión
	// sube y todos los tokens emitidos antes dejan de valer.
	TV int `json:"tv"`
	jwt.RegisteredClaims
}

func signToken(c claims, ttl time.Duration) (string, error) {
	now := time.Now()
	c.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))
	c.IssuedAt = jwt.NewNumericDate(now)
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(jwtSecretBytes)
}

func generateAdminToken(a *AdminUser) (string, error) {
	return signToken(claims{Email: a.Email, Role: RoleAdmin, TV: a.TokenVersion}, adminTokenTTL)
}

// generateCustomerToken identifica al cliente por su id (subject). No lleva
// datos personales: el token puede quedar en el navegador del cliente.
func generateCustomerToken(c *Customer) (string, error) {
	return signToken(claims{Role: RoleCustomer, TV: c.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{Subject: c.ID.Hex()}}, customerTokenTTL)
}

func parseToken(tokenStr string) (*claims, error) {
	c := &claims{}
	_, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		// Exigimos HMAC explícitamente: sin esto, un token armado a mano con
		// "alg": "none" u otro algoritmo podría eludir la verificación de firma
		// (ataque de confusión de algoritmo).
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("algoritmo de firma no soportado")
		}
		return jwtSecretBytes, nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	return c, nil
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	return strings.TrimPrefix(header, "Bearer "), true
}

// postLogin valida credenciales contra la colección admins y devuelve un JWT.
func postLogin(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	var admin AdminUser
	err := adminsCol().FindOne(r.Context(), bson.M{"email": in.Email}).Decode(&admin)
	if err != nil {
		// Mismo mensaje y costo similar que con contraseña incorrecta: no se
		// revela si el email existe.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(in.Password))
		writeError(w, http.StatusUnauthorized, "Credenciales inválidas")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(in.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "Credenciales inválidas")
		return
	}

	token, err := generateAdminToken(&admin)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "email": admin.Email})
}

type adminCtxKey struct{}

// adminEmail devuelve el email del admin autenticado (para auditoría).
func adminEmail(r *http.Request) string {
	e, _ := r.Context().Value(adminCtxKey{}).(string)
	return e
}

// requireAuth protege las rutas del panel Admin: token válido, rol admin y
// tokenVersion vigente (la cuenta sigue existiendo y no se invalidaron sus
// sesiones). Un token de cliente válido recibe 403.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "No autenticado")
			return
		}
		c, err := parseToken(token)
		if err != nil || c.Role == "" {
			writeAPIError(w, &apiError{Status: http.StatusUnauthorized, Code: CodeSessionExpired, Message: "Sesión inválida o expirada"})
			return
		}
		if c.Role != RoleAdmin {
			writeError(w, http.StatusForbidden, "No tenés permiso para esta acción")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()
		var admin AdminUser
		if err := adminsCol().FindOne(ctx, bson.M{"email": c.Email}).Decode(&admin); err != nil || admin.TokenVersion != c.TV {
			writeAPIError(w, &apiError{Status: http.StatusUnauthorized, Code: CodeSessionExpired, Message: "Sesión inválida o expirada"})
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), adminCtxKey{}, admin.Email)))
	}
}

type customerCtxKey struct{}

// currentCustomer devuelve el cliente autenticado (lo carga requireCustomer).
func currentCustomer(r *http.Request) *Customer {
	c, _ := r.Context().Value(customerCtxKey{}).(*Customer)
	return c
}

// requireCustomer protege las rutas del cliente: token con rol customer y
// cuenta existente y activa. El cliente se identifica SIEMPRE por el token,
// nunca por un id enviado en la URL o el cuerpo: así no puede operar sobre
// datos de otra persona.
func requireCustomer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Iniciá sesión para continuar")
			return
		}
		c, err := parseToken(token)
		if err != nil {
			writeAPIError(w, &apiError{Status: http.StatusUnauthorized, Code: CodeSessionExpired, Message: "Tu sesión venció. Iniciá sesión de nuevo."})
			return
		}
		if c.Role != RoleCustomer {
			writeError(w, http.StatusForbidden, "Esta sección es para cuentas de clientes")
			return
		}
		id, err := primitive.ObjectIDFromHex(c.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Sesión inválida")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()
		var cust Customer
		if err := customersCol().FindOne(ctx, bson.M{"_id": id, "active": true}).Decode(&cust); err != nil || cust.TokenVersion != c.TV {
			writeAPIError(w, &apiError{Status: http.StatusUnauthorized, Code: CodeSessionExpired, Message: "Tu sesión ya no es válida. Iniciá sesión de nuevo."})
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), customerCtxKey{}, &cust)))
	}
}
