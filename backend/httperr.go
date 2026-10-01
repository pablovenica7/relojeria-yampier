package main

import (
	"errors"
	"log"
	"net/http"
)

// Formato de error de toda la API:
//
//	{"error": "Mensaje legible para mostrar", "code": "CODIGO_ESTABLE"}
//
// "error" se mantiene como string (compatibilidad con el frontend existente);
// "code" permite que el cliente reaccione a casos puntuales sin comparar
// textos (por ejemplo ACTIVE_RESERVATIONS al archivar un reloj).

// Códigos genéricos por status, usados cuando no hay uno específico.
var defaultErrorCodes = map[int]string{
	http.StatusBadRequest:          "INVALID_INPUT",
	http.StatusUnauthorized:        "UNAUTHENTICATED",
	http.StatusForbidden:           "FORBIDDEN",
	http.StatusNotFound:            "NOT_FOUND",
	http.StatusConflict:            "CONFLICT",
	http.StatusTooManyRequests:     "RATE_LIMITED",
	http.StatusInternalServerError: "INTERNAL_ERROR",
	http.StatusServiceUnavailable:  "SERVICE_UNAVAILABLE",
}

// Códigos específicos de negocio.
const (
	CodeReservationNotFound = "RESERVATION_NOT_FOUND"
	CodeWatchNotFound       = "WATCH_NOT_FOUND"
	CodeOutOfStock          = "OUT_OF_STOCK"
	CodeInvalidTransition   = "INVALID_TRANSITION"
	CodeConcurrentUpdate    = "CONCURRENT_UPDATE"
	CodeDuplicateModel      = "DUPLICATE_MODEL"
	CodeActiveReservations  = "ACTIVE_RESERVATIONS"
	CodeInvalidResetToken   = "INVALID_RESET_TOKEN"
	CodeSessionExpired      = "SESSION_EXPIRED"
)

// apiError es un error de negocio con su status HTTP. Las funciones de
// dominio (reservas, stock) devuelven apiError para que los handlers no
// tengan que adivinar qué código corresponde.
type apiError struct {
	Status  int
	Message string
	Code    string
	Extra   map[string]any // datos adicionales seguros (ej: cantidad de reservas)
}

func (e *apiError) Error() string { return e.Message }

func errBadRequest(msg string) error { return &apiError{Status: http.StatusBadRequest, Message: msg} }
func errNotFound(msg string) error   { return &apiError{Status: http.StatusNotFound, Message: msg} }
func errConflict(msg string) error   { return &apiError{Status: http.StatusConflict, Message: msg} }

// errCode crea un error con código específico.
func errCode(status int, code, msg string) error {
	return &apiError{Status: status, Message: msg, Code: code}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeAPIError(w, &apiError{Status: status, Message: msg})
}

func writeAPIError(w http.ResponseWriter, e *apiError) {
	code := e.Code
	if code == "" {
		code = defaultErrorCodes[e.Status]
		if code == "" {
			code = "ERROR"
		}
	}
	body := map[string]any{"error": e.Message, "code": code}
	for k, v := range e.Extra {
		body[k] = v
	}
	writeJSON(w, e.Status, body)
}

// writeErr responde con el status del apiError, o 500 genérico para
// cualquier otro error (el detalle interno solo va al log, nunca al cliente).
func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeAPIError(w, ae)
		return
	}
	log.Printf("error interno request_id=%s: %v", requestIDFrom(r), err)
	writeError(w, http.StatusInternalServerError, "Error interno del servidor")
}

// apiNotFound responde JSON (y no el texto plano de net/http) para rutas
// /api/ que no existen.
func apiNotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "Recurso no encontrado")
}
