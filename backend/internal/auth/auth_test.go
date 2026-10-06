package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTokensAreRoleScoped(t *testing.T) {
	m := NewTokenManager("clave-de-test")
	adminTok, _ := m.IssueAdmin("admin@example.com", 3)
	c, err := m.Parse(adminTok)
	if err != nil || c.Role != RoleAdmin || c.Email != "admin@example.com" || c.TV != 3 {
		t.Fatalf("token admin incorrecto: %v %+v", err, c)
	}
	id := primitive.NewObjectID()
	custTok, _ := m.IssueCustomer(id, 1)
	c, err = m.Parse(custTok)
	if err != nil || c.Role != RoleCustomer || c.Subject != id.Hex() || c.Email != "" {
		t.Fatalf("token de cliente incorrecto (no debe llevar datos personales): %v %+v", err, c)
	}
}

func TestRejectsTamperedExpiredAndForeignTokens(t *testing.T) {
	m := NewTokenManager("clave-de-test")
	tok, _ := m.IssueAdmin("admin@example.com", 0)
	if _, err := m.Parse(tok + "x"); err == nil {
		t.Error("un token alterado no debería pasar la validación")
	}
	if _, err := NewTokenManager("otra-clave").Parse(tok); err == nil {
		t.Error("un token firmado con otra clave no debería pasar")
	}
	m.now = func() time.Time { return time.Now().Add(-48 * time.Hour) }
	old, _ := m.IssueAdmin("admin@example.com", 0)
	m.now = time.Now
	if _, err := m.Parse(old); err == nil {
		t.Error("un token vencido no debería pasar")
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{Role: RoleAdmin}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := m.Parse(none); err == nil {
		t.Error("alg=none nunca debe aceptarse")
	}
}

func TestPasswords(t *testing.T) {
	h, err := HashPassword("secreta123")
	if err != nil || h == "secreta123" {
		t.Fatal("el hash no puede ser la contraseña")
	}
	if !CheckPassword(h, "secreta123") || CheckPassword(h, "otra") {
		t.Error("comparación de contraseña incorrecta")
	}
}
