package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Customer es una cuenta de cliente. Es independiente de AdminUser: otra
// colección, otro login y otro rol en el token. No tiene tags json: la API
// expone dto.CustomerResponse (nunca passwordHash ni tokenVersion).
//
// Minimización de datos: solo lo necesario para identificar y contactar al
// cliente, gestionar reservas y facturar más adelante.
type Customer struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	CustomerType string             `bson:"customerType"` // "individual" (empresas: preparado, no habilitado)
	FirstName    string             `bson:"firstName"`
	LastName     string             `bson:"lastName"`
	BusinessName string             `bson:"businessName,omitempty"`
	Email        string             `bson:"email"`
	Phone        string             `bson:"phone"`
	PasswordHash string             `bson:"passwordHash"`

	DocumentType   string   `bson:"documentType"`
	DocumentNumber string   `bson:"documentNumber"`
	TaxCondition   string   `bson:"taxCondition"`
	TaxIDType      string   `bson:"taxIdType,omitempty"` // cuit | cuil | cdi
	TaxID          string   `bson:"taxId,omitempty"`     // 11 dígitos, sin guiones
	BillingAddress *Address `bson:"billingAddress,omitempty"`

	PrivacyAcceptedAt  time.Time  `bson:"privacyAcceptedAt"`
	PrivacyVersion     string     `bson:"privacyVersion"`
	MarketingConsent   bool       `bson:"marketingConsent"`
	MarketingConsentAt *time.Time `bson:"marketingConsentAt,omitempty"`

	Active        bool       `bson:"active"`
	DeactivatedAt *time.Time `bson:"deactivatedAt,omitempty"`
	// TokenVersion se incrementa al cambiar/restablecer la contraseña o al
	// desactivar la cuenta: invalida todas las sesiones (JWT) anteriores.
	TokenVersion int       `bson:"tokenVersion"`
	CreatedAt    time.Time `bson:"createdAt"`
	UpdatedAt    time.Time `bson:"updatedAt"`
}

// Address es un domicilio de facturación estructurado.
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

// LegalName es el nombre a usar en una factura.
func (c *Customer) LegalName() string {
	if c.CustomerType == CustomerCompany && c.BusinessName != "" {
		return c.BusinessName
	}
	return c.FirstName + " " + c.LastName
}

// Versiones de los textos legales vigentes. Cuando cambie el texto, subir la
// versión aquí y en frontend/src/utils/legal.js.
const (
	PrivacyPolicyVersion = "2026-10-01-borrador"
	TermsVersion         = "2026-10-01-borrador"
)

// Tipos de cliente.
const (
	CustomerIndividual = "individual"
	CustomerCompany    = "company"
)

// Condiciones fiscales.
const (
	TaxConsumerFinal        = "consumer_final"
	TaxMonotributo          = "monotributo"
	TaxResponsableInscripto = "responsable_inscripto"
	TaxExento               = "exento"
	TaxOther                = "other"
)

// Tipos de documento de identidad.
const (
	DocDNI             = "dni"
	DocCUIT            = "cuit"
	DocCUIL            = "cuil"
	DocCDI             = "cdi"
	DocPassport        = "passport"
	DocForeignDocument = "foreign_document"
)

// ValidTaxCondition indica si la condición fiscal existe.
func ValidTaxCondition(s string) bool {
	switch s {
	case TaxConsumerFinal, TaxMonotributo, TaxResponsableInscripto, TaxExento, TaxOther:
		return true
	}
	return false
}

// RequiresTaxID: estas condiciones siempre tienen CUIT/CUIL/CDI.
func RequiresTaxID(cond string) bool {
	return cond == TaxMonotributo || cond == TaxResponsableInscripto || cond == TaxExento
}

// IsTaxIDType indica si el tipo es CUIT/CUIL/CDI.
func IsTaxIDType(s string) bool { return s == DocCUIT || s == DocCUIL || s == DocCDI }

var argentineProvinces = []string{
	"Buenos Aires", "Ciudad Autónoma de Buenos Aires", "Catamarca", "Chaco", "Chubut",
	"Córdoba", "Corrientes", "Entre Ríos", "Formosa", "Jujuy", "La Pampa", "La Rioja",
	"Mendoza", "Misiones", "Neuquén", "Río Negro", "Salta", "San Juan", "San Luis",
	"Santa Cruz", "Santa Fe", "Santiago del Estero", "Tierra del Fuego", "Tucumán",
}

func canonicalProvince(p string) (string, bool) {
	for _, prov := range argentineProvinces {
		if strings.EqualFold(prov, p) {
			return prov, true
		}
	}
	return "", false
}

// ValidCUIT verifica longitud y dígito verificador (módulo 11) de un
// CUIT/CUIL/CDI. No consulta a ARCA: solo descarta errores de tipeo.
func ValidCUIT(d string) bool {
	if len(d) != 11 || DigitsOnly(d) != d {
		return false
	}
	weights := []int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i, w := range weights {
		sum += int(d[i]-'0') * w
	}
	check := 11 - sum%11
	switch check {
	case 11:
		check = 0
	case 10:
		return false
	}
	return int(d[10]-'0') == check
}

var (
	nameRe       = regexp.MustCompile(`^\p{L}[\p{L}'’ .-]*$`)
	phoneCharsRe = regexp.MustCompile(`^\+?[0-9 ()-]+$`)
	docAlnumRe   = regexp.MustCompile(`^[A-Z0-9]{5,20}$`)
	streetNumRe  = regexp.MustCompile(`^[0-9A-Za-z/ -]{1,10}$`)
	arPostalRe   = regexp.MustCompile(`^([A-Z]\d{4}[A-Z]{3}|\d{4})$`)
	postalRe     = regexp.MustCompile(`^[A-Z0-9 -]{3,10}$`)
)

// ValidatePassword devuelve un mensaje si la contraseña no cumple la política.
func ValidatePassword(p string) string {
	if utf8.RuneCountInString(p) < 8 {
		return "La contraseña debe tener al menos 8 caracteres"
	}
	if len(p) > 72 { // límite de bcrypt
		return "La contraseña es demasiado larga (máximo 72 caracteres)"
	}
	var letter, digit bool
	for _, r := range p {
		letter = letter || unicode.IsLetter(r)
		digit = digit || unicode.IsDigit(r)
	}
	if !letter || !digit {
		return "La contraseña debe combinar letras y números"
	}
	return ""
}

// ValidatePersonName valida nombre o apellido.
func ValidatePersonName(v, field string) string {
	if v == "" {
		return "Ingresá tu " + field
	}
	if utf8.RuneCountInString(v) > 60 || !nameRe.MatchString(v) {
		return "El " + field + " no es válido"
	}
	return ""
}

// ValidPhone: caracteres permitidos y entre 8 y 15 dígitos.
func ValidPhone(p string) bool {
	n := len(DigitsOnly(p))
	return phoneCharsRe.MatchString(p) && n >= 8 && n <= 15
}

// NormalizeDocument devuelve el número normalizado o un mensaje de error.
func NormalizeDocument(docType, number string) (string, string) {
	switch docType {
	case DocDNI:
		d := DigitsOnly(number)
		if len(d) < 7 || len(d) > 8 {
			return "", "El DNI debe tener 7 u 8 dígitos"
		}
		return d, ""
	case DocCUIT, DocCUIL, DocCDI:
		d := DigitsOnly(number)
		if !ValidCUIT(d) {
			return "", "El número de " + strings.ToUpper(docType) + " no es válido (11 dígitos, revisá el verificador)"
		}
		return d, ""
	case DocPassport, DocForeignDocument:
		d := strings.ToUpper(strings.NewReplacer(" ", "", "-", "", ".", "").Replace(number))
		if !docAlnumRe.MatchString(d) {
			return "", "El número de documento debe tener entre 5 y 20 letras o números"
		}
		return d, ""
	}
	return "", "Elegí un tipo de documento válido"
}

// NormalizeAddress valida un domicilio. Un domicilio totalmente vacío se
// considera "no informado" (es opcional hasta que se necesite facturar).
func NormalizeAddress(a *Address) (*Address, string) {
	if a == nil {
		return nil, ""
	}
	t := Address{
		Street: CollapseSpaces(a.Street), Number: CollapseSpaces(a.Number),
		Floor: CollapseSpaces(a.Floor), Apartment: CollapseSpaces(a.Apartment),
		PostalCode: strings.ToUpper(CollapseSpaces(a.PostalCode)), City: CollapseSpaces(a.City),
		Province: CollapseSpaces(a.Province), Country: CollapseSpaces(a.Country),
	}
	if t == (Address{Country: t.Country}) && (t.Country == "" || strings.EqualFold(t.Country, "Argentina")) {
		return nil, ""
	}
	if t.Country == "" {
		t.Country = "Argentina"
	}
	switch {
	case t.Street == "" || len(t.Street) > 120:
		return nil, "Ingresá la calle del domicilio"
	case !streetNumRe.MatchString(t.Number):
		return nil, "Ingresá la altura del domicilio (o S/N)"
	case len(t.Floor) > 10 || len(t.Apartment) > 10:
		return nil, "Piso y departamento: máximo 10 caracteres"
	case t.City == "" || len(t.City) > 80:
		return nil, "Ingresá la ciudad o localidad"
	case len(t.Country) > 60:
		return nil, "País inválido"
	}
	if strings.EqualFold(t.Country, "Argentina") {
		t.Country = "Argentina"
		prov, ok := canonicalProvince(t.Province)
		if !ok {
			return nil, "Elegí una provincia válida"
		}
		t.Province = prov
		if !arPostalRe.MatchString(t.PostalCode) {
			return nil, "El código postal debe tener 4 dígitos o formato CPA (ej: X5000ABC)"
		}
	} else {
		if t.Province == "" || len(t.Province) > 80 {
			return nil, "Ingresá la provincia o estado"
		}
		if !postalRe.MatchString(t.PostalCode) {
			return nil, "Código postal inválido"
		}
	}
	return &t, ""
}

// CustomerFilter filtra el listado de clientes del Admin.
type CustomerFilter struct {
	Search string
	Active *bool // nil = todas
}

// ProfileChange son los datos editables del perfil, ya validados.
type ProfileChange struct {
	FirstName, LastName, Email, Phone string
	DocumentType, DocumentNumber      string
	TaxCondition, TaxIDType, TaxID    string
	BillingAddress                    *Address
	MarketingConsent                  bool
	MarketingConsentChanged           bool
	At                                time.Time
}
