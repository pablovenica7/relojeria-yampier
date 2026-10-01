package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

// Recuperación de contraseña de CLIENTES.
//
//   - El token es aleatorio (32 bytes de crypto/rand), NO un JWT.
//   - En la base solo se guarda su hash SHA-256: quien lea la base no puede usarlo.
//   - Vence a los 30 minutos y es de un solo uso (se marca usado con una
//     operación condicional atómica: dos usos simultáneos no pueden ganar ambos).
//   - Pedir un enlace siempre responde lo mismo, exista o no la cuenta.
//   - Al restablecer, sube tokenVersion: se cierran todas las sesiones abiertas.

const (
	resetTokenTTL      = 30 * time.Minute
	resetRequestMinGap = 2 * time.Minute // no generar enlaces en ráfaga para la misma cuenta
)

var appMailer Mailer = disabledMailer{}

func base64Std(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func hashResetToken(token string) string {
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

// publicURL es la URL del sitio para armar el enlace (APP_PUBLIC_URL).
func publicURL() string {
	u := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_PUBLIC_URL")), "/")
	if u == "" {
		u = "http://localhost:5173"
	}
	return u
}

// requestPasswordReset crea un token para la cuenta (si existe y está activa)
// y lo envía por email. Devuelve el token en claro solo para los tests; el
// handler NUNCA lo incluye en la respuesta.
func requestPasswordReset(ctx context.Context, email string, now time.Time) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !emailRe.MatchString(email) {
		return "", nil
	}
	c, err := findCustomerByEmail(ctx, email)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !c.Active {
		return "", nil
	}
	recent, err := passwordResetsCol().CountDocuments(ctx, bson.M{
		"customerId": c.ID, "usedAt": nil, "createdAt": bson.M{"$gt": now.Add(-resetRequestMinGap)},
	})
	if err != nil {
		return "", err
	}
	if recent > 0 {
		return "", nil // ya se envió uno hace instantes
	}
	// Un solo enlace vigente por cuenta: los anteriores quedan inutilizados.
	if _, err := passwordResetsCol().UpdateMany(ctx, bson.M{"customerId": c.ID, "usedAt": nil},
		bson.M{"$set": bson.M{"usedAt": now}}); err != nil {
		return "", err
	}
	token, err := newResetToken()
	if err != nil {
		return "", err
	}
	if _, err := passwordResetsCol().InsertOne(ctx, PasswordReset{
		CustomerID: c.ID, TokenHash: hashResetToken(token), ExpiresAt: now.Add(resetTokenTTL), CreatedAt: now,
	}); err != nil {
		return "", err
	}
	link := publicURL() + "/restablecer-contrasena?token=" + url.QueryEscape(token)
	body := "Hola " + c.FirstName + ",\n\n" +
		"Recibimos un pedido para restablecer la contraseña de tu cuenta en Relojería Yampier.\n" +
		"Usá este enlace dentro de los próximos 30 minutos:\n\n" + link + "\n\n" +
		"Si no lo pediste, ignorá este mensaje: tu contraseña no cambia.\n"
	// Envío en segundo plano: la respuesta tarda lo mismo exista o no la
	// cuenta (no se puede deducir por el tiempo de respuesta) y un SMTP lento
	// no bloquea el request. Un error queda en el log, sin el token.
	to := c.Email
	go func() {
		if err := appMailer.Send(to, "Restablecer contraseña · Relojería Yampier", body); err != nil {
			log.Printf("No se pudo enviar el email de recuperación: %v", err)
		}
	}()
	return token, nil
}

// resetPassword consume el token (una sola vez) y cambia la contraseña.
func resetPassword(ctx context.Context, token, newPassword string, now time.Time) error {
	invalid := errCode(http.StatusBadRequest, CodeInvalidResetToken, "El enlace no es válido o ya venció. Pedí uno nuevo.")
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 100 {
		return invalid
	}
	// Se valida la contraseña ANTES de consumir el token, para no gastarlo
	// por un error de tipeo.
	if msg := validatePassword(newPassword); msg != "" {
		return errBadRequest(msg)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	var pr PasswordReset
	err = passwordResetsCol().FindOneAndUpdate(ctx,
		bson.M{"tokenHash": hashResetToken(token), "usedAt": nil, "expiresAt": bson.M{"$gt": now}},
		bson.M{"$set": bson.M{"usedAt": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&pr)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return invalid
	}
	if err != nil {
		return err
	}
	res, err := customersCol().UpdateOne(ctx, bson.M{"_id": pr.CustomerID, "active": true}, bson.M{
		"$set": bson.M{"passwordHash": string(hash), "updatedAt": now},
		"$inc": bson.M{"tokenVersion": 1},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return invalid
	}
	return nil
}

// POST /api/auth/password/forgot  (público, rate limit) — respuesta siempre igual.
func postForgotPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	if _, err := requestPasswordReset(ctx, in.Email, time.Now()); err != nil {
		log.Printf("error en recuperación de contraseña request_id=%s: %v", requestIDFrom(r), err)
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Si el email corresponde a una cuenta, vas a recibir un enlace para restablecer la contraseña.",
	})
}

// POST /api/auth/password/reset  (público, rate limit) — body {token, newPassword}
func postResetPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	if err := resetPassword(ctx, in.Token, in.NewPassword, time.Now()); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
