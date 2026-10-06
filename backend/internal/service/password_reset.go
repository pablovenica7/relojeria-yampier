package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/auth"
	"relojeria-yampier/internal/domain"
)

// Mailer envía emails transaccionales (implementaciones en internal/mail).
type Mailer interface {
	Send(to, subject, body string) error
}

type resetStore interface {
	CountRecentUnused(ctx context.Context, customerID primitive.ObjectID, since time.Time) (int64, error)
	InvalidateAll(ctx context.Context, customerID primitive.ObjectID, at time.Time) error
	Insert(ctx context.Context, p *domain.PasswordReset) error
	Consume(ctx context.Context, tokenHash string, now time.Time) (*domain.PasswordReset, error)
}

type resetCustomerStore interface {
	FindByEmail(ctx context.Context, email string) (*domain.Customer, error)
	ReplacePassword(ctx context.Context, id primitive.ObjectID, expectedVersion *int, onlyActive bool, hash string, at time.Time) (*domain.Customer, error)
}

// Reglas de la recuperación de contraseña.
const (
	ResetTokenTTL      = 30 * time.Minute
	resetRequestMinGap = 2 * time.Minute // no generar enlaces en ráfaga para la misma cuenta
)

// PasswordResetService implementa "olvidé mi contraseña":
//   - token aleatorio (32 bytes de crypto/rand), NO un JWT;
//   - en la base solo se guarda su hash SHA-256;
//   - vence en 30 minutos y es de un solo uso (consumo atómico);
//   - pedir un enlace responde siempre igual, exista o no la cuenta;
//   - al restablecer sube tokenVersion: se cierran todas las sesiones.
type PasswordResetService struct {
	customers resetCustomerStore
	resets    resetStore
	mailer    Mailer
	publicURL string
	log       *slog.Logger
	now       Clock
}

func NewPasswordResetService(customers resetCustomerStore, resets resetStore, mailer Mailer, publicURL string, log *slog.Logger, now Clock) *PasswordResetService {
	return &PasswordResetService{customers: customers, resets: resets, mailer: mailer,
		publicURL: strings.TrimRight(publicURL, "/"), log: loggerOrDefault(log), now: clockOrNow(now)}
}

// HashResetToken es el hash que se guarda (el token en claro nunca se persiste).
func HashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Request crea un token si la cuenta existe y está activa, y lo envía por
// email en segundo plano (así la respuesta tarda lo mismo exista o no la
// cuenta). Devuelve el token en claro SOLO para los tests: el controller
// nunca lo incluye en la respuesta.
func (s *PasswordResetService) Request(ctx context.Context, email string) (string, error) {
	now := s.now()
	email = domain.NormalizeEmail(email)
	if email == "" || !domain.ValidEmail(email) {
		return "", nil
	}
	c, err := s.customers.FindByEmail(ctx, email)
	if isNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !c.Active {
		return "", nil
	}
	recent, err := s.resets.CountRecentUnused(ctx, c.ID, now.Add(-resetRequestMinGap))
	if err != nil {
		return "", err
	}
	if recent > 0 {
		return "", nil // ya se envió uno hace instantes
	}
	// Un solo enlace vigente por cuenta: los anteriores quedan inutilizados.
	if err := s.resets.InvalidateAll(ctx, c.ID, now); err != nil {
		return "", err
	}
	token, err := newResetToken()
	if err != nil {
		return "", err
	}
	if err := s.resets.Insert(ctx, &domain.PasswordReset{
		CustomerID: c.ID, TokenHash: HashResetToken(token), ExpiresAt: now.Add(ResetTokenTTL), CreatedAt: now,
	}); err != nil {
		return "", err
	}
	link := s.publicURL + "/restablecer-contrasena?token=" + url.QueryEscape(token)
	body := "Hola " + c.FirstName + ",\n\n" +
		"Recibimos un pedido para restablecer la contraseña de tu cuenta en Relojería Yampier.\n" +
		"Usá este enlace dentro de los próximos 30 minutos:\n\n" + link + "\n\n" +
		"Si no lo pediste, ignorá este mensaje: tu contraseña no cambia.\n"
	to := c.Email
	go func() {
		if err := s.mailer.Send(to, "Restablecer contraseña · Relojería Yampier", body); err != nil {
			// Sin el token ni el email del cliente en el log.
			s.log.Error("no se pudo enviar el email de recuperación", "error", err)
		}
	}()
	return token, nil
}

// Reset consume el token (una sola vez) y cambia la contraseña.
func (s *PasswordResetService) Reset(ctx context.Context, token, newPassword string) error {
	invalid := &domain.Error{Kind: domain.ErrValidation, Code: domain.CodeInvalidResetToken,
		Message: "El enlace no es válido o ya venció. Pedí uno nuevo."}
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 100 {
		return invalid
	}
	// Se valida la contraseña ANTES de consumir el token, para no gastarlo
	// por un error de tipeo.
	if msg := domain.ValidatePassword(newPassword); msg != "" {
		return domain.Validation(msg)
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := s.now()
	pr, err := s.resets.Consume(ctx, HashResetToken(token), now)
	if isNotFound(err) {
		return invalid
	}
	if err != nil {
		return err
	}
	if _, err := s.customers.ReplacePassword(ctx, pr.CustomerID, nil, true, hash, now); err != nil {
		if isNotFound(err) {
			return invalid
		}
		return err
	}
	return nil
}
