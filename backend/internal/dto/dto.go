// Package dto define los cuerpos HTTP de entrada (requests) y salida
// (responses). Los nombres JSON son el contrato con el frontend: cambiarlos
// rompe la aplicación.
//
// Las entidades que la API ya devolvía tal cual (Watch, Reservation para el
// Admin, Inquiry, StockMovement) se siguen devolviendo desde domain, sin un
// DTO espejo que solo copiaría campos. Customer sí tiene DTO porque guarda
// datos que nunca deben salir (passwordHash, tokenVersion, estado interno).
package dto

import (
	"math"
	"time"

	"relojeria-yampier/internal/domain"
)

// Page es el formato de los listados paginados.
type Page[T any] struct {
	Items []T   `json:"items"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
	Pages int   `json:"pages"`
}

// NewPage arma una página calculando la cantidad total de páginas.
func NewPage[T any](items []T, page, limit int, total int64) Page[T] {
	if items == nil {
		items = []T{}
	}
	pages := 0
	if limit > 0 {
		pages = int(math.Ceil(float64(total) / float64(limit)))
	}
	return Page[T]{Items: items, Page: page, Limit: limit, Total: total, Pages: pages}
}

// ---------- Relojes ----------

// WatchRequest es el cuerpo de alta/edición de un reloj (Admin).
type WatchRequest struct {
	Brand         string   `json:"brand"`
	Name          string   `json:"name"`
	Model         string   `json:"model"`
	Gender        string   `json:"gender"`
	Description   string   `json:"description"`
	Image         string   `json:"image"`
	Specs         []string `json:"specs"`
	PriceARS      int64    `json:"priceARS"`
	StockQuantity *int     `json:"stockQuantity"`
	// StockQuantityBase es la cantidad que el Admin vio al abrir el formulario:
	// el stock solo se pisa si no cambió mientras tanto.
	StockQuantityBase *int   `json:"stockQuantityBase,omitempty"`
	StockMoveType     string `json:"stockMoveType,omitempty"`
	StockMoveReason   string `json:"stockMoveReason,omitempty"`
}

// StockAdjustRequest es el cuerpo de PATCH /api/admin/watches/{id}/stock.
type StockAdjustRequest struct {
	Delta  int    `json:"delta"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// AdminWatchList es el listado del Admin: página + contadores.
type AdminWatchList struct {
	Page[domain.Watch]
	Stats domain.CatalogStats `json:"stats"`
}

// ---------- Clientes y autenticación ----------

// LoginRequest es el cuerpo de los logins (Admin y clientes).
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CustomerRequest es el cuerpo de registro y de edición del perfil.
type CustomerRequest struct {
	FirstName        string          `json:"firstName"`
	LastName         string          `json:"lastName"`
	Email            string          `json:"email"`
	Phone            string          `json:"phone"`
	Password         string          `json:"password"`
	DocumentType     string          `json:"documentType"`
	DocumentNumber   string          `json:"documentNumber"`
	TaxCondition     string          `json:"taxCondition"`
	TaxIDType        string          `json:"taxIdType"`
	TaxID            string          `json:"taxId"`
	BillingAddress   *domain.Address `json:"billingAddress"`
	PrivacyAccepted  bool            `json:"privacyAccepted"`
	MarketingConsent bool            `json:"marketingConsent"`
}

// ChangePasswordRequest es el cuerpo de POST /api/auth/me/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ForgotPasswordRequest / ResetPasswordRequest: recuperación de contraseña.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

// CustomerResponse es lo que la API muestra de un cliente (a sí mismo o al
// Admin). Nunca incluye passwordHash, tokenVersion ni estado interno.
type CustomerResponse struct {
	ID                 string          `json:"id"`
	CustomerType       string          `json:"customerType"`
	FirstName          string          `json:"firstName"`
	LastName           string          `json:"lastName"`
	BusinessName       string          `json:"businessName,omitempty"`
	Email              string          `json:"email"`
	Phone              string          `json:"phone"`
	DocumentType       string          `json:"documentType"`
	DocumentNumber     string          `json:"documentNumber"`
	TaxCondition       string          `json:"taxCondition"`
	TaxIDType          string          `json:"taxIdType"`
	TaxID              string          `json:"taxId"`
	BillingAddress     *domain.Address `json:"billingAddress"`
	PrivacyAcceptedAt  time.Time       `json:"privacyAcceptedAt"`
	PrivacyVersion     string          `json:"privacyVersion"`
	MarketingConsent   bool            `json:"marketingConsent"`
	MarketingConsentAt *time.Time      `json:"marketingConsentAt,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	LegalName          string          `json:"legalName"`
}

// NewCustomerResponse mapea la entidad a su respuesta pública.
func NewCustomerResponse(c *domain.Customer) *CustomerResponse {
	if c == nil {
		return nil
	}
	return &CustomerResponse{
		ID: c.ID.Hex(), CustomerType: c.CustomerType, FirstName: c.FirstName, LastName: c.LastName,
		BusinessName: c.BusinessName, Email: c.Email, Phone: c.Phone,
		DocumentType: c.DocumentType, DocumentNumber: c.DocumentNumber, TaxCondition: c.TaxCondition,
		TaxIDType: c.TaxIDType, TaxID: c.TaxID, BillingAddress: c.BillingAddress,
		PrivacyAcceptedAt: c.PrivacyAcceptedAt, PrivacyVersion: c.PrivacyVersion,
		MarketingConsent: c.MarketingConsent, MarketingConsentAt: c.MarketingConsentAt,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, LegalName: c.LegalName(),
	}
}

// AdminCustomerResponse agrega el estado de la cuenta (solo para el Admin).
type AdminCustomerResponse struct {
	CustomerResponse
	Active        bool       `json:"active"`
	DeactivatedAt *time.Time `json:"deactivatedAt,omitempty"`
}

// NewAdminCustomerResponse mapea la entidad a la vista del Admin.
func NewAdminCustomerResponse(c *domain.Customer) AdminCustomerResponse {
	return AdminCustomerResponse{CustomerResponse: *NewCustomerResponse(c), Active: c.Active, DeactivatedAt: c.DeactivatedAt}
}

// AuthResponse es la respuesta de registro y login de clientes.
type AuthResponse struct {
	Token    string            `json:"token"`
	Customer *CustomerResponse `json:"customer"`
}

// ---------- Reservas ----------

// AdminReservationRequest es el cuerpo de POST /api/admin/reservations.
type AdminReservationRequest struct {
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

// CustomerReservationRequest es el cuerpo de POST /api/reservations.
type CustomerReservationRequest struct {
	WatchID       string `json:"watchId"`
	TermsAccepted bool   `json:"termsAccepted"`
	Notes         string `json:"notes"`
}

// ReservationStatusRequest es el cuerpo de PATCH /api/admin/reservations/{id}/status.
type ReservationStatusRequest struct {
	Status        string `json:"status"`
	DepositAmount *int64 `json:"depositAmount"`
	DepositMethod string `json:"depositMethod"`
}

// ReservationDetailsRequest es el cuerpo de PATCH /api/admin/reservations/{id}.
type ReservationDetailsRequest struct {
	Notes         *string    `json:"notes"`
	DepositAmount *int64     `json:"depositAmount"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	ClearExpiry   bool       `json:"clearExpiry"`
}

// AdminReservationResponse es la reserva completa + datos del cliente.
type AdminReservationResponse struct {
	domain.Reservation
	Customer *CustomerResponse `json:"customer,omitempty"`
}

// CustomerReservationResponse es lo que ve el cliente de su reserva: sin
// notas internas del local ni datos de stock interno.
type CustomerReservationResponse struct {
	ID                 string     `json:"id"`
	WatchID            string     `json:"watchId"`
	WatchName          string     `json:"watchName"`
	WatchModel         string     `json:"watchModel"`
	WatchImage         string     `json:"watchImage"`
	PriceAtReservation int64      `json:"priceAtReservation"`
	Status             string     `json:"status"`
	DepositAmount      int64      `json:"depositAmount"`
	DepositPaidAt      *time.Time `json:"depositPaidAt,omitempty"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	CustomerNotes      string     `json:"customerNotes,omitempty"`
	CanCancel          bool       `json:"canCancel"`
	CreatedAt          time.Time  `json:"createdAt"`
}

// NewCustomerReservationResponse mapea la reserva a la vista del cliente.
func NewCustomerReservationResponse(r *domain.Reservation) CustomerReservationResponse {
	v := CustomerReservationResponse{
		ID: r.ID.Hex(), WatchID: r.WatchID.Hex(), WatchName: r.WatchNameSnapshot,
		WatchModel: r.WatchModelSnapshot, WatchImage: r.WatchImageSnapshot,
		PriceAtReservation: r.PriceAtReservation, Status: r.Status,
		DepositAmount: r.DepositAmount, DepositPaidAt: r.DepositPaidAt, CustomerNotes: r.CustomerNotes,
		CanCancel: r.Status == domain.ReservationPending, CreatedAt: r.CreatedAt,
	}
	if domain.Expirable(r.Status) {
		v.ExpiresAt = r.ExpiresAt
	}
	return v
}

// ---------- Consultas ----------

// InquiryRequest es el cuerpo público de POST /api/inquiries.
type InquiryRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Message string `json:"message"`
	WatchID string `json:"watchId"`
}

// InquiryUpdateRequest es el cuerpo de PATCH /api/admin/inquiries/{id}.
type InquiryUpdateRequest struct {
	Status     *string `json:"status"`
	AdminNotes *string `json:"adminNotes"`
}

// ---------- Dashboard ----------

// ExpiringReservation es una reserva que vence pronto (dashboard).
type ExpiringReservation struct {
	ID           string    `json:"id"`
	WatchName    string    `json:"watchName"`
	CustomerName string    `json:"customerName"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// Dashboard son los números de la operación diaria.
type Dashboard struct {
	PendingReservations     int64                 `json:"pendingReservations"`
	ConfirmedReservations   int64                 `json:"confirmedReservations"`
	DepositPaidReservations int64                 `json:"depositPaidReservations"`
	OutOfStock              int64                 `json:"outOfStock"`
	LastUnit                int64                 `json:"lastUnit"`
	NewInquiries            int64                 `json:"newInquiries"`
	ExpiringSoon            []ExpiringReservation `json:"expiringSoon"`
}
