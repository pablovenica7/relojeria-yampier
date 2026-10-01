package main

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Watch representa un reloj del catálogo.
//
// Dinero: PriceARS es un entero en pesos argentinos (sin centavos). Nunca se
// usa punto flotante para dinero. 0 significa "precio a consultar".
//
// Stock: StockQuantity es la única fuente de verdad. La disponibilidad que ve
// el cliente (en stock / última unidad / sin stock) se deriva de ella en
// cada respuesta (campo Availability, que no se guarda en Mongo).
//
// Archivado: un reloj con ArchivedAt != nil no aparece en el catálogo público
// pero se conserva, porque puede estar vinculado a consultas o reservas.
type Watch struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Brand         string             `bson:"brand" json:"brand"`
	BrandKey      string             `bson:"brandKey" json:"-"` // marca normalizada para filtrar con índice
	Name          string             `bson:"name" json:"name"`
	Model         string             `bson:"model" json:"model"`          // código comercial, ej: GA-2100-1A1
	ModelKey      string             `bson:"modelKey,omitempty" json:"-"` // clave única normalizada, ej: GA21001A1
	Gender        string             `bson:"gender" json:"gender"`        // "hombre" | "mujer"
	Description   string             `bson:"description" json:"description"`
	Image         string             `bson:"image" json:"image"` // ruta pública, ej: /uploads/archivo.jpg
	Specs         []string           `bson:"specs" json:"specs"`
	PriceARS      int64              `bson:"priceARS" json:"priceARS"`
	StockQuantity int                `bson:"stockQuantity" json:"stockQuantity"`
	Availability  string             `bson:"-" json:"availability"`
	ArchivedAt    *time.Time         `bson:"archivedAt,omitempty" json:"archivedAt,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// WatchInput es el cuerpo esperado al crear o editar un reloj.
type WatchInput struct {
	Brand         string   `json:"brand"`
	Name          string   `json:"name"`
	Model         string   `json:"model"`
	Gender        string   `json:"gender"`
	Description   string   `json:"description"`
	Image         string   `json:"image"`
	Specs         []string `json:"specs"`
	PriceARS      int64    `json:"priceARS"`
	StockQuantity *int     `json:"stockQuantity"`
	// StockQuantityBase es la cantidad que el Admin vio al abrir el formulario.
	// Si viene, el stock solo se pisa si no cambió mientras tanto (por ejemplo
	// por una reserva confirmada en paralelo); si cambió, se responde 409.
	StockQuantityBase *int `json:"stockQuantityBase,omitempty"`
	// Tipo y motivo del movimiento si cambia el stock (por defecto
	// manual_adjustment). Quedan en el historial de stock.
	StockMoveType   string `json:"stockMoveType,omitempty"`
	StockMoveReason string `json:"stockMoveReason,omitempty"`
}

// Estados de una consulta.
const (
	InquiryNew       = "new"
	InquiryContacted = "contacted"
	InquiryClosed    = "closed"
)

func validInquiryStatus(s string) bool {
	return s == InquiryNew || s == InquiryContacted || s == InquiryClosed
}

// Inquiry representa una consulta enviada desde el sitio. Una consulta solo
// expresa interés: nunca modifica el stock.
type Inquiry struct {
	ID                 primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name               string              `bson:"name" json:"name"`
	Email              string              `bson:"email" json:"email"`
	Phone              string              `bson:"phone" json:"phone"`
	Message            string              `bson:"message" json:"message"`
	Status             string              `bson:"status" json:"status"`
	AdminNotes         string              `bson:"adminNotes" json:"adminNotes"`
	WatchID            *primitive.ObjectID `bson:"watchId,omitempty" json:"watchId,omitempty"`
	WatchNameSnapshot  string              `bson:"watchNameSnapshot,omitempty" json:"watchNameSnapshot,omitempty"`
	WatchModelSnapshot string              `bson:"watchModelSnapshot,omitempty" json:"watchModelSnapshot,omitempty"`
	ContactedAt        *time.Time          `bson:"contactedAt,omitempty" json:"contactedAt,omitempty"`
	ClosedAt           *time.Time          `bson:"closedAt,omitempty" json:"closedAt,omitempty"`
	CreatedAt          time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

// InquiryInput es el cuerpo público de POST /api/inquiries.
type InquiryInput struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Message string `json:"message"`
	WatchID string `json:"watchId"`
}

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
	// Reservas anteriores a este cambio no los tienen (quedan vacíos).
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

// Métodos de seña: solo registro administrativo, no hay cobro online.
const (
	DepositCash         = "cash"
	DepositBankTransfer = "bank_transfer"
	DepositInStoreCard  = "in_store_card"
	DepositOther        = "other"
)

func validDepositMethod(m string) bool {
	switch m {
	case DepositCash, DepositBankTransfer, DepositInStoreCard, DepositOther:
		return true
	}
	return false
}

// Address es un domicilio de facturación estructurado (para facturar más
// adelante sin tener que interpretar texto libre).
type Address struct {
	Street     string `bson:"street" json:"street"`
	Number     string `bson:"number" json:"number"`
	Floor      string `bson:"floor,omitempty" json:"floor"`
	Apartment  string `bson:"apartment,omitempty" json:"apartment"`
	PostalCode string `bson:"postalCode" json:"postalCode"`
	City       string `bson:"city" json:"city"`
	Province   string `bson:"province" json:"province"`
	Country    string `bson:"country" json:"country"`
}

// Customer es una cuenta de cliente. Es independiente de AdminUser: otra
// colección, otro login y otro rol en el token.
//
// Minimización de datos: solo lo necesario para identificar y contactar al
// cliente, gestionar reservas y facturar más adelante. No se guardan fecha de
// nacimiento, género, imágenes de documentos ni datos sensibles.
type Customer struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CustomerType string             `bson:"customerType" json:"customerType"` // "individual" (empresas: preparado, no habilitado)
	FirstName    string             `bson:"firstName" json:"firstName"`
	LastName     string             `bson:"lastName" json:"lastName"`
	BusinessName string             `bson:"businessName,omitempty" json:"businessName,omitempty"`
	Email        string             `bson:"email" json:"email"`
	Phone        string             `bson:"phone" json:"phone"`
	PasswordHash string             `bson:"passwordHash" json:"-"`

	DocumentType   string   `bson:"documentType" json:"documentType"`
	DocumentNumber string   `bson:"documentNumber" json:"documentNumber"`
	TaxCondition   string   `bson:"taxCondition" json:"taxCondition"`
	TaxIDType      string   `bson:"taxIdType,omitempty" json:"taxIdType"` // cuit | cuil | cdi
	TaxID          string   `bson:"taxId,omitempty" json:"taxId"`         // 11 dígitos, sin guiones
	BillingAddress *Address `bson:"billingAddress,omitempty" json:"billingAddress"`

	PrivacyAcceptedAt  time.Time  `bson:"privacyAcceptedAt" json:"privacyAcceptedAt"`
	PrivacyVersion     string     `bson:"privacyVersion" json:"privacyVersion"`
	MarketingConsent   bool       `bson:"marketingConsent" json:"marketingConsent"`
	MarketingConsentAt *time.Time `bson:"marketingConsentAt,omitempty" json:"marketingConsentAt,omitempty"`

	Active        bool       `bson:"active" json:"-"`
	DeactivatedAt *time.Time `bson:"deactivatedAt,omitempty" json:"-"`
	// TokenVersion se incrementa al cambiar/restablecer la contraseña o al
	// desactivar la cuenta: invalida todas las sesiones (JWT) anteriores.
	TokenVersion int       `bson:"tokenVersion" json:"-"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time `bson:"updatedAt" json:"updatedAt"`
}

// LegalName es el nombre a usar en una factura: razón social para empresas,
// nombre y apellido para personas.
func (c *Customer) LegalName() string {
	if c.CustomerType == CustomerCompany && c.BusinessName != "" {
		return c.BusinessName
	}
	return c.FirstName + " " + c.LastName
}

// MarshalJSON agrega legalName (derivado, no se guarda).
func (c Customer) MarshalJSON() ([]byte, error) {
	type alias Customer
	return json.Marshal(struct {
		alias
		LegalName string `json:"legalName"`
	}{alias(c), c.LegalName()})
}

// ReservationInput es el cuerpo de POST /api/admin/reservations.
type ReservationInput struct {
	WatchID            string     `json:"watchId"`
	InquiryID          string     `json:"inquiryId"`
	CustomerName       string     `json:"customerName"`
	Phone              string     `json:"phone"`
	Email              string     `json:"email"`
	Status             string     `json:"status"` // "pending" (por defecto) o "confirmed"
	DepositAmount      int64      `json:"depositAmount"`
	PriceAtReservation int64      `json:"priceAtReservation"` // opcional: precio acordado si difiere del de lista
	Notes              string     `json:"notes"`
	ExpiresAt          *time.Time `json:"expiresAt"`
}

// AdminUser representa a un usuario con acceso al panel de administración.
type AdminUser struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"passwordHash" json:"-"`
	TokenVersion int                `bson:"tokenVersion" json:"-"` // ver Customer.TokenVersion
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
}

// Tipos de movimiento de stock.
const (
	MoveStockEntry         = "stock_entry"         // ingreso de mercadería
	MoveManualAdjustment   = "manual_adjustment"   // ajuste desde el Admin
	MoveReservationHold    = "reservation_hold"    // reserva confirmada: -1
	MoveReservationRelease = "reservation_release" // reserva cancelada/vencida: +1
	MoveSale               = "sale"                // venta completada (delta 0: la unidad ya estaba retenida)
	MoveCorrection         = "correction"          // corrección de inventario
)

func validManualMoveType(t string) bool {
	return t == MoveStockEntry || t == MoveManualAdjustment || t == MoveCorrection
}

// StockMovement es una línea del libro de inventario. Cada cambio de
// stockQuantity genera exactamente un movimiento (ver stock_ledger.go).
type StockMovement struct {
	ID                primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	WatchID           primitive.ObjectID  `bson:"watchId" json:"watchId"`
	WatchNameSnapshot string              `bson:"watchNameSnapshot" json:"watchName"`
	Type              string              `bson:"type" json:"type"`
	QuantityDelta     int                 `bson:"quantityDelta" json:"quantityDelta"`
	StockBefore       int                 `bson:"stockBefore" json:"stockBefore"`
	StockAfter        int                 `bson:"stockAfter" json:"stockAfter"`
	ReservationID     *primitive.ObjectID `bson:"reservationId,omitempty" json:"reservationId,omitempty"`
	Reason            string              `bson:"reason,omitempty" json:"reason,omitempty"`
	CreatedBy         string              `bson:"createdBy" json:"createdBy"` // email del admin o "sistema"
	// IdempotencyKey evita duplicar el movimiento de una reserva ante
	// reintentos (índice único parcial). Vacío en ajustes manuales.
	IdempotencyKey string    `bson:"idempotencyKey,omitempty" json:"-"`
	CreatedAt      time.Time `bson:"createdAt" json:"createdAt"`
}

// PasswordReset es un pedido de restablecimiento de contraseña. Se guarda
// solo el hash SHA-256 del token: con una copia de la base no se puede usar.
type PasswordReset struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	CustomerID primitive.ObjectID `bson:"customerId"`
	TokenHash  string             `bson:"tokenHash"`
	ExpiresAt  time.Time          `bson:"expiresAt"`
	UsedAt     *time.Time         `bson:"usedAt,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt"`
}
