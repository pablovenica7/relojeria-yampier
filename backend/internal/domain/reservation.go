package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Reservation es una reserva coordinada con el local (no hay pago online).
// Los campos *Snapshot y PriceAtReservation congelan los datos del reloj al
// momento de reservar: si después cambia el precio o el nombre, la reserva
// conserva los valores acordados.
type Reservation struct {
	ID                 primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	WatchID            primitive.ObjectID  `bson:"watchId" json:"watchId"`
	WatchNameSnapshot  string              `bson:"watchNameSnapshot" json:"watchNameSnapshot"`
	WatchModelSnapshot string              `bson:"watchModelSnapshot" json:"watchModelSnapshot"`
	PriceAtReservation int64               `bson:"priceAtReservation" json:"priceAtReservation"`
	InquiryID          *primitive.ObjectID `bson:"inquiryId,omitempty" json:"inquiryId,omitempty"`
	CustomerName       string              `bson:"customerName" json:"customerName"`
	Phone              string              `bson:"phone" json:"phone"`
	Email              string              `bson:"email" json:"email"`
	Status             string              `bson:"status" json:"status"`
	// StockHeld indica si esta reserva tiene retenida una unidad del reloj.
	// Es lo que garantiza que una unidad se devuelva al stock una sola vez.
	StockHeld     bool       `bson:"stockHeld" json:"stockHeld"`
	DepositAmount int64      `bson:"depositAmount" json:"depositAmount"`
	DepositMethod string     `bson:"depositMethod,omitempty" json:"depositMethod,omitempty"`
	DepositPaidAt *time.Time `bson:"depositPaidAt,omitempty" json:"depositPaidAt,omitempty"`
	// Momento de cada cambio de estado (no depender solo de updatedAt).
	ConfirmedAt *time.Time `bson:"confirmedAt,omitempty" json:"confirmedAt,omitempty"`
	CompletedAt *time.Time `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
	CancelledAt *time.Time `bson:"cancelledAt,omitempty" json:"cancelledAt,omitempty"`
	ExpiredAt   *time.Time `bson:"expiredAt,omitempty" json:"expiredAt,omitempty"`
	Notes       string     `bson:"notes" json:"notes"` // notas internas del local (nunca se muestran al cliente)
	ExpiresAt   *time.Time `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time  `bson:"updatedAt" json:"updatedAt"`

	// Reservas solicitadas por un cliente desde la web.
	Source             string              `bson:"source,omitempty" json:"source,omitempty"` // "web" | "admin"
	CustomerID         *primitive.ObjectID `bson:"customerId,omitempty" json:"customerId,omitempty"`
	CustomerNotes      string              `bson:"customerNotes,omitempty" json:"customerNotes,omitempty"`
	WatchImageSnapshot string              `bson:"watchImageSnapshot,omitempty" json:"watchImageSnapshot,omitempty"`
	TermsAcceptedAt    *time.Time          `bson:"termsAcceptedAt,omitempty" json:"termsAcceptedAt,omitempty"`
	TermsVersion       string              `bson:"termsVersion,omitempty" json:"termsVersion,omitempty"`
}

// Estados de una reserva. Única definición: el resto del código usa estas
// constantes, nunca strings sueltos.
const (
	ReservationPending     = "pending"      // pedido registrado, sin unidad retenida
	ReservationConfirmed   = "confirmed"    // el local confirmó: retiene 1 unidad
	ReservationDepositPaid = "deposit_paid" // seña recibida: sigue retenida
	ReservationCompleted   = "completed"    // pagado y retirado en el local (venta)
	ReservationCancelled   = "cancelled"
	ReservationExpired     = "expired"
)

// Origen de la reserva.
const (
	ReservationSourceWeb   = "web"
	ReservationSourceAdmin = "admin"
)

// Reglas de negocio de reservas.
const (
	MaxReservationDays    = 90   // vencimiento máximo
	MaxReservationNotes   = 2000 // notas internas
	MaxCustomerNotes      = 500  // comentario del cliente
	WebPendingDays        = 7    // una solicitud web pendiente vence sola
	MaxPendingPerCustomer = 3    // anti-abuso
)

// transitions define las transiciones permitidas. Los estados finales
// (completed, cancelled, expired) no tienen salida: una reserva cerrada no se
// reabre; si el cliente vuelve, se crea una reserva nueva.
var transitions = map[string][]string{
	ReservationPending:     {ReservationConfirmed, ReservationCancelled, ReservationExpired},
	ReservationConfirmed:   {ReservationDepositPaid, ReservationCompleted, ReservationCancelled, ReservationExpired},
	ReservationDepositPaid: {ReservationCompleted, ReservationCancelled},
}

// ValidReservationStatus indica si el estado existe.
func ValidReservationStatus(s string) bool {
	switch s {
	case ReservationPending, ReservationConfirmed, ReservationDepositPaid,
		ReservationCompleted, ReservationCancelled, ReservationExpired:
		return true
	}
	return false
}

// CanTransition indica si se puede pasar de un estado a otro.
func CanTransition(from, to string) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// HoldsStock indica si una reserva en ese estado tiene una unidad retenida.
func HoldsStock(status string) bool {
	return status == ReservationConfirmed || status == ReservationDepositPaid
}

// Expirable: solo las pendientes o confirmadas vencen solas. Con seña paga,
// lo decide el local.
func Expirable(status string) bool {
	return status == ReservationPending || status == ReservationConfirmed
}

// IsOpen: la reserva sigue activa (no está cerrada).
func IsOpen(status string) bool {
	return status == ReservationPending || status == ReservationConfirmed || status == ReservationDepositPaid
}

// OpenStatuses son los estados de una reserva activa.
var OpenStatuses = []string{ReservationPending, ReservationConfirmed, ReservationDepositPaid}

// Métodos de seña: solo registro administrativo, no hay cobro online.
const (
	DepositCash         = "cash"
	DepositBankTransfer = "bank_transfer"
	DepositInStoreCard  = "in_store_card"
	DepositOther        = "other"
)

// ValidDepositMethod indica si el método de seña existe.
func ValidDepositMethod(m string) bool {
	switch m {
	case DepositCash, DepositBankTransfer, DepositInStoreCard, DepositOther:
		return true
	}
	return false
}

// ValidateExpiry valida un vencimiento opcional: futuro y dentro del máximo.
func ValidateExpiry(t *time.Time, now time.Time) string {
	if t == nil {
		return ""
	}
	if !t.After(now) {
		return "El vencimiento debe ser una fecha futura"
	}
	if t.After(now.AddDate(0, 0, MaxReservationDays)) {
		return "El vencimiento no puede superar los 90 días"
	}
	return ""
}

// ReservationFilter filtra el listado del Admin.
type ReservationFilter struct {
	Status      string
	WatchID     *primitive.ObjectID
	CreatedFrom *time.Time // inclusive
	CreatedTo   *time.Time // exclusivo
}

// StatusChange describe un cambio de estado ya validado, que el repository
// aplica de forma condicional (solo si la reserva sigue en FromStatus y con
// el mismo StockHeld leído).
type StatusChange struct {
	FromStatus    string
	FromStockHeld bool
	To            string
	StockHeld     bool
	DepositAmount int64
	DepositMethod string     // solo al registrar seña
	DepositPaidAt *time.Time // solo al registrar seña
	At            time.Time  // fecha del cambio (confirmedAt/completedAt/... y updatedAt)
}

// DetailsChange son los campos editables de una reserva (no el estado).
type DetailsChange struct {
	Notes         *string
	DepositAmount *int64
	ExpiresAt     *time.Time
	ClearExpiry   bool
	At            time.Time
}
