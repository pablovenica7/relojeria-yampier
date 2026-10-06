package domain

import "errors"

// Tipos de error de dominio. Los services devuelven *Error con uno de estos
// tipos (Kind); la capa HTTP (httpx.WriteError) traduce el tipo a status
// HTTP. Ninguna capa compara mensajes de texto.
var (
	ErrValidation        = errors.New("validation")
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrOutOfStock        = errors.New("out of stock")
	ErrInvalidTransition = errors.New("invalid transition")
	ErrConcurrentUpdate  = errors.New("concurrent update")
	// ErrDuplicate lo devuelven los repositories ante una clave única repetida.
	ErrDuplicate = errors.New("duplicate")
)

// Códigos estables que ve el cliente en {"error": "...", "code": "..."}.
// Los genéricos dependen del tipo; estos son los específicos de negocio.
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
	CodeInvalidCredentials  = "INVALID_CREDENTIALS"
)

// Error es un error de negocio con mensaje para el usuario.
type Error struct {
	Kind    error          // uno de los Err* de arriba
	Code    string         // opcional: código específico; si falta, se usa el genérico del Kind
	Message string         // texto seguro para mostrar al usuario
	Extra   map[string]any // datos adicionales seguros (ej: cantidad de reservas)
}

func (e *Error) Error() string { return e.Message }

// Unwrap permite errors.Is(err, domain.ErrNotFound).
func (e *Error) Unwrap() error { return e.Kind }

// Constructores de uso común.
func Validation(msg string) error { return &Error{Kind: ErrValidation, Message: msg} }
func NotFound(code, msg string) error {
	return &Error{Kind: ErrNotFound, Code: code, Message: msg}
}
func Conflict(code, msg string) error {
	return &Error{Kind: ErrConflict, Code: code, Message: msg}
}
func Unauthorized(code, msg string) error {
	return &Error{Kind: ErrUnauthorized, Code: code, Message: msg}
}
func Forbidden(msg string) error { return &Error{Kind: ErrForbidden, Message: msg} }
