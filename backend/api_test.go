package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Tests HTTP de punta a punta contra el router real (middlewares de auth,
// rate limiting y handlers). Usan la base temporal de setupTestDB.

type apiClient struct {
	t *testing.T
	h http.Handler
}

func newAPI(t *testing.T) *apiClient {
	setupTestDB(t)
	setJWTSecret("clave-de-test-api")
	return &apiClient{t: t, h: newRouter(parseTrustedProxies(""))}
}

func (a *apiClient) do(method, path, token string, body any, out any) int {
	a.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if out != nil {
		_ = json.Unmarshal(rec.Body.Bytes(), out)
	}
	return rec.Code
}

// adminToken crea un admin en la base de prueba y devuelve su token.
func (a *apiClient) adminToken() string {
	a.t.Helper()
	admin := AdminUser{Email: "admin@example.com", CreatedAt: time.Now()}
	_, _ = adminsCol().InsertOne(context.Background(), admin)
	tok, err := generateAdminToken(&admin)
	if err != nil {
		a.t.Fatal(err)
	}
	return tok
}

func registerBody(email string) map[string]any {
	return map[string]any{
		"firstName": "Ana", "lastName": "Pérez", "email": email, "phone": "351 555 1234",
		"password": "secreta123", "documentType": "dni", "documentNumber": "30123456",
		"taxCondition": "consumer_final", "privacyAccepted": true,
	}
}

func (a *apiClient) register(email string) (string, *Customer) {
	a.t.Helper()
	var res authResponse
	if code := a.do("POST", "/api/auth/register", "", registerBody(email), &res); code != http.StatusCreated {
		a.t.Fatalf("registro falló: %d", code)
	}
	return res.Token, res.Customer
}

func TestRegisterLoginAndProfile(t *testing.T) {
	api := newAPI(t)

	token, cust := api.register("Ana@Example.com")
	if cust.Email != "ana@example.com" || cust.PrivacyVersion != PrivacyPolicyVersion || cust.MarketingConsent {
		t.Errorf("cliente mal registrado: %+v", cust)
	}

	// Email duplicado (con otro casing): 409 y mensaje genérico.
	var errBody map[string]string
	if code := api.do("POST", "/api/auth/register", "", registerBody("ANA@example.com "), &errBody); code != http.StatusConflict {
		t.Errorf("email duplicado debería dar 409, dio %d", code)
	}

	// Login válido e inválido (mismo mensaje para email inexistente).
	var login authResponse
	if code := api.do("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "secreta123"}, &login); code != 200 || login.Token == "" {
		t.Fatalf("login válido falló: %d", code)
	}
	var bad1, bad2 map[string]string
	c1 := api.do("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "otra-clave1"}, &bad1)
	c2 := api.do("POST", "/api/auth/login", "", map[string]string{"email": "nadie@example.com", "password": "otra-clave1"}, &bad2)
	if c1 != 401 || c2 != 401 || bad1["error"] != bad2["error"] {
		t.Errorf("login inválido debe ser 401 con el mismo mensaje: %d %q / %d %q", c1, bad1["error"], c2, bad2["error"])
	}

	// Perfil propio.
	var me map[string]any
	if code := api.do("GET", "/api/auth/me", token, nil, &me); code != 200 || me["email"] != "ana@example.com" {
		t.Fatalf("no pudo leer su perfil: %d %v", code, me)
	}
	if _, ok := me["passwordHash"]; ok {
		t.Error("el perfil nunca debe exponer passwordHash")
	}

	// Editar perfil: no se puede tocar id/rol; email nuevo de otra cuenta → 409.
	api.register("beto@example.com")
	upd := registerBody("beto@example.com")
	if code := api.do("PUT", "/api/auth/me", token, upd, nil); code != http.StatusConflict {
		t.Errorf("tomar el email de otra cuenta debería dar 409, dio %d", code)
	}
	upd = registerBody("ana@example.com")
	upd["phone"] = "351 999 0000"
	upd["id"] = primitive.NewObjectID().Hex()
	upd["role"] = "admin"
	var updated Customer
	if code := api.do("PUT", "/api/auth/me", token, upd, &updated); code != 200 || updated.Phone != "351 999 0000" || updated.ID != cust.ID {
		t.Errorf("edición de perfil incorrecta: %d %+v", code, updated)
	}

	// Sin token / token de cliente en el Admin.
	if code := api.do("GET", "/api/auth/me", "", nil, nil); code != 401 {
		t.Errorf("perfil sin sesión debería dar 401, dio %d", code)
	}
	if code := api.do("GET", "/api/admin/reservations", token, nil, nil); code != http.StatusForbidden {
		t.Errorf("un cliente NUNCA debe acceder al Admin: esperaba 403, dio %d", code)
	}
	adminTok := api.adminToken()
	if code := api.do("GET", "/api/auth/me", adminTok, nil, nil); code != http.StatusForbidden {
		t.Errorf("un token Admin no es una cuenta de cliente: esperaba 403, dio %d", code)
	}
}

func TestCustomerReservationFlow(t *testing.T) {
	api := newAPI(t)
	w := insertTestWatch(t, "GA-2100-1A1", 1, 150000)
	anaTok, _ := api.register("ana@example.com")
	betoTok, _ := api.register("beto@example.com")
	adminTok := api.adminToken()

	req := map[string]any{"watchId": w.ID.Hex(), "termsAccepted": true, "notes": "Paso el sábado"}

	// Sin sesión: 401. Sin aceptar términos: 400.
	if code := api.do("POST", "/api/reservations", "", req, nil); code != 401 {
		t.Errorf("reservar sin sesión debería dar 401, dio %d", code)
	}
	if code := api.do("POST", "/api/reservations", anaTok, map[string]any{"watchId": w.ID.Hex()}, nil); code != 400 {
		t.Errorf("sin aceptar términos debería dar 400, dio %d", code)
	}

	// Solicitud válida: pending, sin tocar stock, con términos registrados.
	var view CustomerReservationView
	if code := api.do("POST", "/api/reservations", anaTok, req, &view); code != http.StatusCreated {
		t.Fatalf("crear reserva falló: %d", code)
	}
	if view.Status != ReservationPending || !view.CanCancel || view.PriceAtReservation != 150000 {
		t.Errorf("reserva mal creada: %+v", view)
	}
	if stockOf(t, w.ID) != 1 {
		t.Error("una reserva pendiente NO debe descontar stock")
	}
	rid, _ := primitive.ObjectIDFromHex(view.ID)
	stored, _ := findReservation(context.Background(), rid)
	if stored.TermsVersion != TermsVersion || stored.TermsAcceptedAt == nil || stored.Source != ReservationSourceWeb {
		t.Errorf("no se registró la aceptación de términos: %+v", stored)
	}

	// Duplicada para el mismo reloj: 409.
	if code := api.do("POST", "/api/reservations", anaTok, req, nil); code != http.StatusConflict {
		t.Errorf("reserva duplicada debería dar 409, dio %d", code)
	}

	// Cada cliente ve solo lo suyo; la reserva ajena da 404 (no 403).
	var anaList, betoList []CustomerReservationView
	api.do("GET", "/api/reservations", anaTok, nil, &anaList)
	api.do("GET", "/api/reservations", betoTok, nil, &betoList)
	if len(anaList) != 1 || len(betoList) != 0 {
		t.Errorf("cada cliente debe ver solo sus reservas: ana=%d beto=%d", len(anaList), len(betoList))
	}
	if code := api.do("GET", "/api/reservations/"+view.ID, betoTok, nil, nil); code != 404 {
		t.Errorf("leer una reserva ajena debería dar 404, dio %d", code)
	}
	if code := api.do("POST", "/api/reservations/"+view.ID+"/cancel", betoTok, nil, nil); code != 404 {
		t.Errorf("cancelar una reserva ajena debería dar 404, dio %d", code)
	}

	// El cliente no puede confirmar su reserva (no tiene acceso a la ruta Admin).
	if code := api.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", anaTok, map[string]string{"status": "confirmed"}, nil); code != 403 {
		t.Errorf("el cliente no puede confirmar: esperaba 403, dio %d", code)
	}

	// El Admin confirma: recién ahí se descuenta la unidad.
	var adminView map[string]any
	if code := api.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", adminTok, map[string]string{"status": "confirmed"}, &adminView); code != 200 {
		t.Fatalf("el Admin no pudo confirmar: %d %v", code, adminView)
	}
	if stockOf(t, w.ID) != 0 {
		t.Error("confirmar debe descontar una unidad")
	}

	// Confirmada: el cliente ya no puede cancelarla desde la web.
	var cancelErr map[string]string
	if code := api.do("POST", "/api/reservations/"+view.ID+"/cancel", anaTok, nil, &cancelErr); code != http.StatusConflict {
		t.Errorf("cancelar una confirmada desde la web debería dar 409, dio %d", code)
	}
	if stockOf(t, w.ID) != 0 {
		t.Error("un intento de cancelación rechazado no debe tocar stock")
	}

	// Seña: exige método válido; queda registrada fecha y método.
	if code := api.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", adminTok, map[string]any{"status": "deposit_paid", "depositAmount": 20000}, nil); code != 400 {
		t.Errorf("seña sin método debería dar 400, dio %d", code)
	}
	if code := api.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", adminTok,
		map[string]any{"status": "deposit_paid", "depositAmount": 20000, "depositMethod": "bank_transfer"}, nil); code != 200 {
		t.Fatalf("registrar seña falló: %d", code)
	}
	var mine CustomerReservationView
	api.do("GET", "/api/reservations/"+view.ID, anaTok, nil, &mine)
	if mine.Status != ReservationDepositPaid || mine.DepositAmount != 20000 || mine.DepositPaidAt == nil || mine.CanCancel {
		t.Errorf("vista del cliente incorrecta tras la seña: %+v", mine)
	}

	// El Admin ve los datos completos del cliente en la reserva.
	var adminList struct {
		Items []adminReservation `json:"items"`
	}
	api.do("GET", "/api/admin/reservations", adminTok, nil, &adminList)
	if len(adminList.Items) != 1 || adminList.Items[0].Customer == nil || adminList.Items[0].Customer.DocumentNumber != "30123456" {
		t.Errorf("el Admin debería ver los datos del cliente: %+v", adminList.Items)
	}

	// Admin cancela una reserva con unidad retenida: se libera el stock.
	if code := api.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", adminTok, map[string]string{"status": "cancelled"}, nil); code != 200 {
		t.Fatalf("cancelación admin falló: %d", code)
	}
	if stockOf(t, w.ID) != 1 {
		t.Error("cancelar una reserva confirmada debe devolver la unidad")
	}
}

func TestCustomerCancelsPendingWithoutStockChange(t *testing.T) {
	api := newAPI(t)
	w := insertTestWatch(t, "F-91W-1", 2, 30000)
	tok, _ := api.register("ana@example.com")
	var view CustomerReservationView
	api.do("POST", "/api/reservations", tok, map[string]any{"watchId": w.ID.Hex(), "termsAccepted": true}, &view)
	var after CustomerReservationView
	if code := api.do("POST", "/api/reservations/"+view.ID+"/cancel", tok, nil, &after); code != 200 || after.Status != ReservationCancelled {
		t.Fatalf("el cliente debería poder cancelar su pendiente: %d %+v", code, after)
	}
	if stockOf(t, w.ID) != 2 {
		t.Error("cancelar una pendiente no debe modificar stock")
	}
}

func TestCannotRequestReservationWithoutStock(t *testing.T) {
	api := newAPI(t)
	w := insertTestWatch(t, "LA670WA-1", 0, 40000)
	tok, _ := api.register("ana@example.com")
	if code := api.do("POST", "/api/reservations", tok, map[string]any{"watchId": w.ID.Hex(), "termsAccepted": true}, nil); code != http.StatusConflict {
		t.Errorf("reservar un reloj sin stock debería dar 409, dio %d", code)
	}
}

func TestPendingWebReservationExpires(t *testing.T) {
	api := newAPI(t)
	w := insertTestWatch(t, "MTP-V002D-1B", 1, 0)
	tok, cust := api.register("ana@example.com")
	var view CustomerReservationView
	api.do("POST", "/api/reservations", tok, map[string]any{"watchId": w.ID.Hex(), "termsAccepted": true}, &view)
	if view.ExpiresAt == nil {
		t.Fatal("una solicitud web debe tener vencimiento")
	}
	items, err := listCustomerReservations(context.Background(), cust.ID, time.Now().AddDate(0, 0, webPendingDays+1))
	if err != nil || len(items) != 1 || items[0].Status != ReservationExpired {
		t.Errorf("la solicitud pendiente debería vencer: %v %+v", err, items)
	}
	if stockOf(t, w.ID) != 1 {
		t.Error("vencer una pendiente no debe modificar stock")
	}
}
