package service

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/auth"
	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
)

type customerStore interface {
	Insert(ctx context.Context, c *domain.Customer) error
	FindActiveByID(ctx context.Context, id primitive.ObjectID) (*domain.Customer, error)
	FindByEmail(ctx context.Context, email string) (*domain.Customer, error)
	EmailTakenByOther(ctx context.Context, email string, self primitive.ObjectID) (bool, error)
	UpdateProfile(ctx context.Context, id primitive.ObjectID, c domain.ProfileChange) (*domain.Customer, error)
	ReplacePassword(ctx context.Context, id primitive.ObjectID, expectedVersion *int, onlyActive bool, hash string, at time.Time) (*domain.Customer, error)
	SetActive(ctx context.Context, id primitive.ObjectID, active bool, at time.Time) (*domain.Customer, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Customer, error)
	List(ctx context.Context, f domain.CustomerFilter, page, limit int) ([]domain.Customer, int64, error)
}

// customerTokens emite sesiones de cliente (implementado por auth.TokenManager).
type customerTokens interface {
	IssueCustomer(id primitive.ObjectID, tokenVersion int) (string, error)
}

// CustomerService gestiona cuentas de clientes: registro, login, perfil,
// contraseña y activación. Separado del Admin: otra colección y otro rol.
type CustomerService struct {
	customers customerStore
	tokens    customerTokens
	now       Clock
}

func NewCustomerService(customers customerStore, tokens customerTokens, now Clock) *CustomerService {
	return &CustomerService{customers: customers, tokens: tokens, now: clockOrNow(now)}
}

var (
	errInvalidCredentials = &domain.Error{Kind: domain.ErrUnauthorized, Code: domain.CodeInvalidCredentials, Message: "Email o contraseña incorrectos"}
	errSessionInvalid     = domain.Unauthorized(domain.CodeSessionExpired, "Tu sesión ya no es válida. Iniciá sesión de nuevo.")
)

// validateCustomer normaliza y valida. register=true exige contraseña y
// aceptación de la Política de Privacidad.
func validateCustomer(in *dto.CustomerRequest, register bool) string {
	in.FirstName = domain.CollapseSpaces(in.FirstName)
	in.LastName = domain.CollapseSpaces(in.LastName)
	in.Email = domain.NormalizeEmail(in.Email)
	in.Phone = domain.CollapseSpaces(in.Phone)
	in.DocumentType = strings.TrimSpace(in.DocumentType)
	in.TaxCondition = strings.TrimSpace(in.TaxCondition)
	in.TaxIDType = strings.TrimSpace(in.TaxIDType)

	if msg := domain.ValidatePersonName(in.FirstName, "nombre"); msg != "" {
		return msg
	}
	if msg := domain.ValidatePersonName(in.LastName, "apellido"); msg != "" {
		return msg
	}
	if in.Email == "" || len(in.Email) > 190 || !domain.ValidEmail(in.Email) {
		return "Ingresá un email válido"
	}
	if !domain.ValidPhone(in.Phone) {
		return "Ingresá un teléfono válido (con código de área, ej: 351 123 4567)"
	}
	if register {
		if msg := domain.ValidatePassword(in.Password); msg != "" {
			return msg
		}
		if !in.PrivacyAccepted {
			return "Para crear la cuenta tenés que leer y aceptar la Política de Privacidad"
		}
	}
	doc, msg := domain.NormalizeDocument(in.DocumentType, in.DocumentNumber)
	if msg != "" {
		return msg
	}
	in.DocumentNumber = doc
	if !domain.ValidTaxCondition(in.TaxCondition) {
		return "Elegí una condición fiscal válida"
	}
	in.TaxID = domain.DigitsOnly(in.TaxID)
	switch {
	case in.TaxID == "" && domain.RequiresTaxID(in.TaxCondition):
		return "Para esta condición fiscal ingresá tu CUIT / CUIL / CDI"
	case in.TaxID == "":
		in.TaxIDType = ""
	case !domain.IsTaxIDType(in.TaxIDType):
		return "Indicá si el número fiscal es CUIT, CUIL o CDI"
	case !domain.ValidCUIT(in.TaxID):
		return "El CUIT / CUIL / CDI no es válido (11 dígitos, revisá el verificador)"
	}
	addr, msg := domain.NormalizeAddress(in.BillingAddress)
	if msg != "" {
		return msg
	}
	in.BillingAddress = addr
	return ""
}

// Register crea la cuenta y devuelve una sesión. Email repetido → 409
// genérico (sin confirmar explícitamente que la cuenta existe).
func (s *CustomerService) Register(ctx context.Context, in dto.CustomerRequest) (string, *domain.Customer, error) {
	if msg := validateCustomer(&in, true); msg != "" {
		return "", nil, domain.Validation(msg)
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return "", nil, err
	}
	now := s.now()
	c := &domain.Customer{
		CustomerType: domain.CustomerIndividual, FirstName: in.FirstName, LastName: in.LastName,
		Email: in.Email, Phone: in.Phone, PasswordHash: hash,
		DocumentType: in.DocumentType, DocumentNumber: in.DocumentNumber,
		TaxCondition: in.TaxCondition, TaxIDType: in.TaxIDType, TaxID: in.TaxID,
		BillingAddress:    in.BillingAddress,
		PrivacyAcceptedAt: now, PrivacyVersion: domain.PrivacyPolicyVersion,
		MarketingConsent: in.MarketingConsent, Active: true, CreatedAt: now, UpdatedAt: now,
	}
	if in.MarketingConsent {
		c.MarketingConsentAt = &now
	}
	if err := s.customers.Insert(ctx, c); err != nil {
		if isDuplicate(err) {
			return "", nil, domain.Conflict("", "No pudimos crear la cuenta con esos datos. Si ya tenés una cuenta, iniciá sesión.")
		}
		return "", nil, err
	}
	token, err := s.tokens.IssueCustomer(c.ID, c.TokenVersion)
	return token, c, err
}

// Login valida credenciales. Ante cualquier fallo (email inexistente,
// contraseña incorrecta, cuenta inactiva) responde el mismo error, con costo
// de tiempo similar.
func (s *CustomerService) Login(ctx context.Context, email, password string) (string, *domain.Customer, error) {
	email = domain.NormalizeEmail(email)
	if email == "" || password == "" || len(password) > 72 {
		return "", nil, errInvalidCredentials
	}
	c, err := s.customers.FindByEmail(ctx, email)
	if isNotFound(err) {
		auth.SpendComparisonTime(password)
		return "", nil, errInvalidCredentials
	}
	if err != nil {
		return "", nil, err
	}
	if !auth.CheckPassword(c.PasswordHash, password) || !c.Active {
		return "", nil, errInvalidCredentials
	}
	token, err := s.tokens.IssueCustomer(c.ID, c.TokenVersion)
	return token, c, err
}

// Authenticate valida que la sesión siga vigente: cuenta activa y misma
// tokenVersion que el token (si cambió la contraseña o se desactivó, no).
func (s *CustomerService) Authenticate(ctx context.Context, id primitive.ObjectID, tokenVersion int) (*domain.Customer, error) {
	c, err := s.customers.FindActiveByID(ctx, id)
	if isNotFound(err) || (err == nil && c.TokenVersion != tokenVersion) {
		return nil, errSessionInvalid
	}
	return c, err
}

// UpdateProfile actualiza los datos editables. No se pueden cambiar id, rol,
// contraseña (tiene su propio flujo) ni la aceptación de privacidad.
func (s *CustomerService) UpdateProfile(ctx context.Context, current *domain.Customer, in dto.CustomerRequest) (*domain.Customer, error) {
	if msg := validateCustomer(&in, false); msg != "" {
		return nil, domain.Validation(msg)
	}
	emailTaken := domain.Conflict("", "Ese email no está disponible")
	if in.Email != current.Email {
		taken, err := s.customers.EmailTakenByOther(ctx, in.Email, current.ID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, emailTaken
		}
	}
	out, err := s.customers.UpdateProfile(ctx, current.ID, domain.ProfileChange{
		FirstName: in.FirstName, LastName: in.LastName, Email: in.Email, Phone: in.Phone,
		DocumentType: in.DocumentType, DocumentNumber: in.DocumentNumber,
		TaxCondition: in.TaxCondition, TaxIDType: in.TaxIDType, TaxID: in.TaxID,
		BillingAddress: in.BillingAddress, MarketingConsent: in.MarketingConsent,
		// Se registra cuándo cambió el consentimiento comercial (alta o baja).
		MarketingConsentChanged: in.MarketingConsent != current.MarketingConsent,
		At:                      s.now(),
	})
	if isDuplicate(err) {
		return nil, emailTaken
	}
	return out, err
}

// ChangePassword exige la contraseña actual, sube tokenVersion (cierra las
// demás sesiones) y devuelve un token nuevo para la sesión actual.
func (s *CustomerService) ChangePassword(ctx context.Context, current *domain.Customer, currentPassword, newPassword string) (string, error) {
	if !auth.CheckPassword(current.PasswordHash, currentPassword) {
		return "", domain.Validation("La contraseña actual no es correcta")
	}
	if msg := domain.ValidatePassword(newPassword); msg != "" {
		return "", domain.Validation(msg)
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return "", err
	}
	version := current.TokenVersion
	updated, err := s.customers.ReplacePassword(ctx, current.ID, &version, false, hash, s.now())
	if isNotFound(err) {
		return "", errSessionInvalid
	}
	if err != nil {
		return "", err
	}
	return s.tokens.IssueCustomer(updated.ID, updated.TokenVersion)
}

// SetActive desactiva (soft delete: impide login y cierra sesiones, conserva
// el historial) o reactiva una cuenta. Nunca se borra un cliente.
func (s *CustomerService) SetActive(ctx context.Context, id primitive.ObjectID, active bool) (*domain.Customer, error) {
	c, err := s.customers.SetActive(ctx, id, active, s.now())
	if isNotFound(err) {
		if _, ferr := s.customers.FindByID(ctx, id); isNotFound(ferr) {
			return nil, domain.NotFound("", "Cliente no encontrado")
		}
		return nil, domain.Conflict("", "La cuenta ya estaba en ese estado")
	}
	return c, err
}

// ListForAdmin devuelve una página de clientes con el estado de la cuenta.
func (s *CustomerService) ListForAdmin(ctx context.Context, f domain.CustomerFilter, page, limit int) (dto.Page[dto.AdminCustomerResponse], error) {
	items, total, err := s.customers.List(ctx, f, page, limit)
	if err != nil {
		return dto.Page[dto.AdminCustomerResponse]{}, err
	}
	out := make([]dto.AdminCustomerResponse, 0, len(items))
	for i := range items {
		out = append(out, dto.NewAdminCustomerResponse(&items[i]))
	}
	return dto.NewPage(out, page, limit, total), nil
}

// ---------- Admin ----------

type adminStore interface {
	FindByEmail(ctx context.Context, email string) (*domain.AdminUser, error)
}

type adminTokens interface {
	IssueAdmin(email string, tokenVersion int) (string, error)
}

// AdminAuthService autentica usuarios del panel.
type AdminAuthService struct {
	admins adminStore
	tokens adminTokens
}

func NewAdminAuthService(admins adminStore, tokens adminTokens) *AdminAuthService {
	return &AdminAuthService{admins: admins, tokens: tokens}
}

// Login valida credenciales del Admin (mismo mensaje y costo si el email no existe).
func (s *AdminAuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	invalid := domain.Unauthorized("", "Credenciales inválidas")
	email = domain.NormalizeEmail(email)
	a, err := s.admins.FindByEmail(ctx, email)
	if isNotFound(err) {
		auth.SpendComparisonTime(password)
		return "", "", invalid
	}
	if err != nil {
		return "", "", err
	}
	if !auth.CheckPassword(a.PasswordHash, password) {
		return "", "", invalid
	}
	token, err := s.tokens.IssueAdmin(a.Email, a.TokenVersion)
	return token, a.Email, err
}

// Authenticate valida que la cuenta Admin del token exista y que su
// tokenVersion siga vigente.
func (s *AdminAuthService) Authenticate(ctx context.Context, email string, tokenVersion int) (*domain.AdminUser, error) {
	a, err := s.admins.FindByEmail(ctx, email)
	if isNotFound(err) || (err == nil && a.TokenVersion != tokenVersion) {
		return nil, domain.Unauthorized(domain.CodeSessionExpired, "Sesión inválida o expirada")
	}
	return a, err
}
