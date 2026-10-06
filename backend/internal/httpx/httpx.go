// Package httpx tiene los helpers HTTP compartidos por controllers y
// middleware: respuestas JSON, decodificación con límite de tamaño y el
// mapeo ÚNICO de errores de dominio a status HTTP.
//
// Formato de error de toda la API (contrato con el frontend):
//
//	{"error": "Mensaje para mostrar", "code": "CODIGO_ESTABLE"}
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/domain"
)

// WriteJSON responde con status y cuerpo JSON.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Códigos genéricos por status (si el error no trae uno específico).
var defaultCodes = map[int]string{
	http.StatusBadRequest:          "INVALID_INPUT",
	http.StatusUnauthorized:        "UNAUTHENTICATED",
	http.StatusForbidden:           "FORBIDDEN",
	http.StatusNotFound:            "NOT_FOUND",
	http.StatusConflict:            "CONFLICT",
	http.StatusTooManyRequests:     "RATE_LIMITED",
	http.StatusInternalServerError: "INTERNAL_ERROR",
	http.StatusServiceUnavailable:  "SERVICE_UNAVAILABLE",
}

// WriteMessage responde un error con mensaje (y código opcional).
func WriteMessage(w http.ResponseWriter, status int, code, msg string) {
	writeBody(w, status, code, msg, nil)
}

func writeBody(w http.ResponseWriter, status int, code, msg string, extra map[string]any) {
	if code == "" {
		code = defaultCodes[status]
		if code == "" {
			code = "ERROR"
		}
	}
	body := map[string]any{"error": msg, "code": code}
	for k, v := range extra {
		body[k] = v
	}
	WriteJSON(w, status, body)
}

// StatusFor traduce el tipo de error de dominio a status HTTP.
func StatusFor(kind error) int {
	switch {
	case errors.Is(kind, domain.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(kind, domain.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(kind, domain.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(kind, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(kind, domain.ErrConflict), errors.Is(kind, domain.ErrOutOfStock),
		errors.Is(kind, domain.ErrInvalidTransition), errors.Is(kind, domain.ErrConcurrentUpdate),
		errors.Is(kind, domain.ErrDuplicate):
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// WriteError responde un error. Los errores de dominio se traducen a su
// status y mensaje seguro; cualquier otro es un 500 genérico: el detalle
// interno (por ejemplo, un error de Mongo) solo va al log, nunca al cliente.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var de *domain.Error
	if errors.As(err, &de) {
		writeBody(w, StatusFor(de.Kind), de.Code, de.Message, de.Extra)
		return
	}
	slog.Error("error interno", "request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
	WriteMessage(w, http.StatusInternalServerError, "", "Error interno del servidor")
}

// DecodeJSON lee el cuerpo como JSON con un límite de tamaño. Un error acá es
// de FORMATO (400), no de negocio.
func DecodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes)).Decode(dst)
}

// PathID lee un ObjectID de la ruta ({id}).
func PathID(r *http.Request, name string) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(r.PathValue(name))
	return id, err == nil
}
