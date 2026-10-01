package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

// Versiones de los textos legales vigentes. Cuando cambie el texto de la
// Política de Privacidad o de los Términos de Reserva, actualizar la versión
// aquí y en frontend/src/utils/legal.js: cada aceptación queda registrada con
// la versión que el usuario vio.
const (
	PrivacyPolicyVersion = "2026-10-01-borrador"
	TermsVersion         = "2026-10-01-borrador"
)

// Tipos de cliente. Hoy solo se habilita "individual"; "company" queda
// preparado en el modelo (businessName) para cuando se acepten empresas.
const (
	CustomerIndividual = "individual"
	CustomerCompany    = "company"
)

// Condiciones fiscales (únicas definiciones; el frontend usa las mismas claves).
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

func validTaxCondition(s string) bool {
	switch s {
	case TaxConsumerFinal, TaxMonotributo, TaxResponsableInscripto, TaxExento, TaxOther:
		return true
	}
	return false
}

// requiresTaxID: estas condiciones siempre tienen CUIT/CUIL/CDI.
func requiresTaxID(cond string) bool {
	return cond == TaxMonotributo || cond == TaxResponsableInscripto || cond == TaxExento
}

func isTaxIDType(s string) bool { return s == DocCUIT || s == DocCUIL || s == DocCDI }

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

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// validCUIT verifica longitud y dígito verificador (módulo 11) de un
// CUIT/CUIL/CDI. No consulta a ARCA: solo descarta errores de tipeo.
func validCUIT(d string) bool {
	if len(d) != 11 || digitsOnly(d) != d {
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

func validatePassword(p string) string {
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

func validatePersonName(v, field string) string {
	if v == "" {
		return "Ingresá tu " + field
	}
	if utf8.RuneCountInString(v) > 60 || !nameRe.MatchString(v) {
		return "El " + field + " no es válido"
	}
	return ""
}

// normalizeDocument devuelve el número normalizado o un mensaje de error.
func normalizeDocument(docType, number string) (string, string) {
	switch docType {
	case DocDNI:
		d := digitsOnly(number)
		if len(d) < 7 || len(d) > 8 {
			return "", "El DNI debe tener 7 u 8 dígitos"
		}
		return d, ""
	case DocCUIT, DocCUIL, DocCDI:
		d := digitsOnly(number)
		if !validCUIT(d) {
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

// normalizeAddress valida un domicilio. Un domicilio totalmente vacío se
// considera "no informado" (es opcional hasta que se necesite facturar).
func normalizeAddress(a *Address) (*Address, string) {
	if a == nil {
		return nil, ""
	}
	t := Address{
		Street: collapseSpaces(a.Street), Number: collapseSpaces(a.Number),
		Floor: collapseSpaces(a.Floor), Apartment: collapseSpaces(a.Apartment),
		PostalCode: strings.ToUpper(collapseSpaces(a.PostalCode)), City: collapseSpaces(a.City),
		Province: collapseSpaces(a.Province), Country: collapseSpaces(a.Country),
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

// CustomerInput es el cuerpo de registro y de edición del perfil.
type CustomerInput struct {
	FirstName        string   `json:"firstName"`
	LastName         string   `json:"lastName"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	Password         string   `json:"password"`
	DocumentType     string   `json:"documentType"`
	DocumentNumber   string   `json:"documentNumber"`
	TaxCondition     string   `json:"taxCondition"`
	TaxIDType        string   `json:"taxIdType"`
	TaxID            string   `json:"taxId"`
	BillingAddress   *Address `json:"billingAddress"`
	PrivacyAccepted  bool     `json:"privacyAccepted"`
	MarketingConsent bool     `json:"marketingConsent"`
}

// validateCustomerInput normaliza y valida. register=true exige contraseña y
// aceptación de la Política de Privacidad.
func validateCustomerInput(in *CustomerInput, register bool) string {
	in.FirstName = collapseSpaces(in.FirstName)
	in.LastName = collapseSpaces(in.LastName)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = collapseSpaces(in.Phone)
	in.DocumentType = strings.TrimSpace(in.DocumentType)
	in.TaxCondition = strings.TrimSpace(in.TaxCondition)
	in.TaxIDType = strings.TrimSpace(in.TaxIDType)

	if msg := validatePersonName(in.FirstName, "nombre"); msg != "" {
		return msg
	}
	if msg := validatePersonName(in.LastName, "apellido"); msg != "" {
		return msg
	}
	if in.Email == "" || len(in.Email) > 190 || !emailRe.MatchString(in.Email) {
		return "Ingresá un email válido"
	}
	if n := len(digitsOnly(in.Phone)); !phoneCharsRe.MatchString(in.Phone) || n < 8 || n > 15 {
		return "Ingresá un teléfono válido (con código de área, ej: 351 123 4567)"
	}
	if register {
		if msg := validatePassword(in.Password); msg != "" {
			return msg
		}
		if !in.PrivacyAccepted {
			return "Para crear la cuenta tenés que leer y aceptar la Política de Privacidad"
		}
	}

	doc, msg := normalizeDocument(in.DocumentType, in.DocumentNumber)
	if msg != "" {
		return msg
	}
	in.DocumentNumber = doc

	if !validTaxCondition(in.TaxCondition) {
		return "Elegí una condición fiscal válida"
	}
	in.TaxID = digitsOnly(in.TaxID)
	switch {
	case in.TaxID == "" && requiresTaxID(in.TaxCondition):
		return "Para esta condición fiscal ingresá tu CUIT / CUIL / CDI"
	case in.TaxID == "":
		in.TaxIDType = ""
	case !isTaxIDType(in.TaxIDType):
		return "Indicá si el número fiscal es CUIT, CUIL o CDI"
	case !validCUIT(in.TaxID):
		return "El CUIT / CUIL / CDI no es válido (11 dígitos, revisá el verificador)"
	}

	addr, msg := normalizeAddress(in.BillingAddress)
	if msg != "" {
		return msg
	}
	in.BillingAddress = addr
	return ""
}

// dummyHash se compara cuando el email no existe, para que un login fallido
// tarde lo mismo exista o no la cuenta (dificulta enumerar usuarios).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("cuenta-inexistente-yampier"), bcrypt.DefaultCost)

func findCustomerByEmail(ctx context.Context, email string) (*Customer, error) {
	var c Customer
	if err := customersCol().FindOne(ctx, bson.M{"email": email}).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

type authResponse struct {
	Token    string    `json:"token"`
	Customer *Customer `json:"customer"`
}

// registerCustomer crea la cuenta. Devuelve 409 genérico si el email ya
// existe (sin confirmarlo explícitamente).
func registerCustomer(ctx context.Context, in CustomerInput, now time.Time) (*Customer, error) {
	if msg := validateCustomerInput(&in, true); msg != "" {
		return nil, errBadRequest(msg)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	c := Customer{
		CustomerType: CustomerIndividual,
		FirstName:    in.FirstName, LastName: in.LastName,
		Email: in.Email, Phone: in.Phone, PasswordHash: string(hash),
		DocumentType: in.DocumentType, DocumentNumber: in.DocumentNumber,
		TaxCondition: in.TaxCondition, TaxIDType: in.TaxIDType, TaxID: in.TaxID,
		BillingAddress:    in.BillingAddress,
		PrivacyAcceptedAt: now, PrivacyVersion: PrivacyPolicyVersion,
		MarketingConsent: in.MarketingConsent,
		Active:           true, CreatedAt: now, UpdatedAt: now,
	}
	if in.MarketingConsent {
		c.MarketingConsentAt = &now
	}
	res, err := customersCol().InsertOne(ctx, c)
	if mongo.IsDuplicateKeyError(err) {
		return nil, errConflict("No pudimos crear la cuenta con esos datos. Si ya tenés una cuenta, iniciá sesión.")
	}
	if err != nil {
		return nil, err
	}
	c.ID = res.InsertedID.(primitive.ObjectID)
	return &c, nil
}

// POST /api/auth/register  (público, con rate limit)
func postRegister(w http.ResponseWriter, r *http.Request) {
	var in CustomerInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	c, err := registerCustomer(ctx, in, time.Now())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	token, err := generateCustomerToken(c)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: token, Customer: c})
}

// authenticateCustomer devuelve el cliente si email y contraseña coinciden.
// Ante cualquier fallo (email inexistente, contraseña incorrecta, cuenta
// inactiva) responde el mismo error, con costo de tiempo similar.
func authenticateCustomer(ctx context.Context, email, password string) (*Customer, error) {
	invalid := &apiError{Status: http.StatusUnauthorized, Message: "Email o contraseña incorrectos", Code: "INVALID_CREDENTIALS"}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" || len(password) > 72 {
		return nil, invalid
	}
	c, err := findCustomerByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, invalid
	}
	if bcrypt.CompareHashAndPassword([]byte(c.PasswordHash), []byte(password)) != nil || !c.Active {
		return nil, invalid
	}
	return c, nil
}

// POST /api/auth/login  (público, con rate limit) — login de CLIENTES.
// Separado de /api/admin/login: otra colección y otro rol en el token.
func postCustomerLogin(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	c, err := authenticateCustomer(ctx, in.Email, in.Password)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	token, err := generateCustomerToken(c)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, Customer: c})
}

// GET /api/auth/me  (cliente) — siempre el perfil del dueño del token.
func getMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentCustomer(r))
}

// updateCustomerProfile actualiza los datos editables. No se pueden cambiar
// id, rol, contraseña (tiene su propio endpoint) ni la aceptación de privacidad.
func updateCustomerProfile(ctx context.Context, current *Customer, in CustomerInput, now time.Time) (*Customer, error) {
	if msg := validateCustomerInput(&in, false); msg != "" {
		return nil, errBadRequest(msg)
	}
	if in.Email != current.Email {
		n, err := customersCol().CountDocuments(ctx, bson.M{"email": in.Email, "_id": bson.M{"$ne": current.ID}})
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, errConflict("Ese email no está disponible")
		}
	}
	set := bson.M{
		"firstName": in.FirstName, "lastName": in.LastName, "email": in.Email, "phone": in.Phone,
		"documentType": in.DocumentType, "documentNumber": in.DocumentNumber,
		"taxCondition": in.TaxCondition, "taxIdType": in.TaxIDType, "taxId": in.TaxID,
		"marketingConsent": in.MarketingConsent, "updatedAt": now,
	}
	update := bson.M{"$set": set}
	if in.BillingAddress != nil {
		set["billingAddress"] = in.BillingAddress
	} else {
		update["$unset"] = bson.M{"billingAddress": ""}
	}
	// Se registra cuándo cambió el consentimiento comercial (alta o baja).
	if in.MarketingConsent != current.MarketingConsent {
		set["marketingConsentAt"] = now
	}
	var out Customer
	err := customersCol().FindOneAndUpdate(ctx, bson.M{"_id": current.ID}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if mongo.IsDuplicateKeyError(err) {
		return nil, errConflict("Ese email no está disponible")
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// PUT /api/auth/me  (cliente)
func putMe(w http.ResponseWriter, r *http.Request) {
	var in CustomerInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	c, err := updateCustomerProfile(ctx, currentCustomer(r), in, time.Now())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// POST /api/auth/me/password  (cliente) — exige la contraseña actual.
func postChangePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	c := currentCustomer(r)
	if bcrypt.CompareHashAndPassword([]byte(c.PasswordHash), []byte(in.CurrentPassword)) != nil {
		writeError(w, http.StatusBadRequest, "La contraseña actual no es correcta")
		return
	}
	if msg := validatePassword(in.NewPassword); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	// Sube tokenVersion: todas las sesiones abiertas (otros dispositivos)
	// quedan cerradas. Esta sesión recibe un token nuevo para seguir.
	var updated Customer
	err = customersCol().FindOneAndUpdate(ctx, bson.M{"_id": c.ID, "tokenVersion": c.TokenVersion}, bson.M{
		"$set": bson.M{"passwordHash": string(hash), "updatedAt": time.Now()},
		"$inc": bson.M{"tokenVersion": 1},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeAPIError(w, &apiError{Status: http.StatusUnauthorized, Code: CodeSessionExpired, Message: "Tu sesión ya no es válida. Iniciá sesión de nuevo."})
		return
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	token, err := generateCustomerToken(&updated)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "token": token})
}
