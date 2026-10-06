package app

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/auth"
	"relojeria-yampier/internal/database"
	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
	"relojeria-yampier/internal/service"
)

// ---------- Clientes: registro, login, perfil, roles ----------

func TestRegisterLoginAndProfile(t *testing.T) {
	ta := newTestApp(t)
	token, cust := ta.register("Ana@Example.com")
	if cust.Email != "ana@example.com" || cust.PrivacyVersion != domain.PrivacyPolicyVersion || cust.MarketingConsent || cust.LegalName != "Ana Pérez" {
		t.Errorf("cliente mal registrado: %+v", cust)
	}
	var errBody map[string]string
	if code := ta.do("POST", "/api/auth/register", "", registerBody("ANA@example.com "), &errBody); code != http.StatusConflict {
		t.Errorf("email duplicado debería dar 409, dio %d", code)
	}

	var login authResp
	if code := ta.do("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "secreta123"}, &login); code != 200 || login.Token == "" {
		t.Fatalf("login válido falló: %d", code)
	}
	var bad1, bad2 map[string]string
	c1 := ta.do("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "otra-clave1"}, &bad1)
	c2 := ta.do("POST", "/api/auth/login", "", map[string]string{"email": "nadie@example.com", "password": "otra-clave1"}, &bad2)
	if c1 != 401 || c2 != 401 || bad1["error"] != bad2["error"] || bad1["code"] != domain.CodeInvalidCredentials {
		t.Errorf("login inválido debe ser 401 con el mismo mensaje: %d %v / %d %v", c1, bad1, c2, bad2)
	}

	var me map[string]any
	if code := ta.do("GET", "/api/auth/me", token, nil, &me); code != 200 || me["email"] != "ana@example.com" {
		t.Fatalf("no pudo leer su perfil: %d %v", code, me)
	}
	for _, k := range []string{"passwordHash", "tokenVersion", "active"} {
		if _, ok := me[k]; ok {
			t.Errorf("el perfil nunca debe exponer %s", k)
		}
	}

	ta.register("beto@example.com")
	if code := ta.do("PUT", "/api/auth/me", token, registerBody("beto@example.com"), nil); code != http.StatusConflict {
		t.Errorf("tomar el email de otra cuenta debería dar 409, dio %d", code)
	}
	upd := registerBody("ana@example.com")
	upd["phone"] = "351 999 0000"
	upd["id"] = primitive.NewObjectID().Hex()
	upd["role"] = "admin"
	var updated dto.CustomerResponse
	if code := ta.do("PUT", "/api/auth/me", token, upd, &updated); code != 200 || updated.Phone != "351 999 0000" || updated.ID != cust.ID {
		t.Errorf("edición de perfil incorrecta: %d %+v", code, updated)
	}

	if code := ta.do("GET", "/api/auth/me", "", nil, nil); code != 401 {
		t.Errorf("perfil sin sesión debería dar 401, dio %d", code)
	}
	if code := ta.do("GET", "/api/admin/reservations", token, nil, nil); code != http.StatusForbidden {
		t.Errorf("un cliente NUNCA debe acceder al Admin: esperaba 403, dio %d", code)
	}
	if code := ta.do("GET", "/api/auth/me", ta.adminToken(), nil, nil); code != http.StatusForbidden {
		t.Errorf("un token Admin no es una cuenta de cliente: esperaba 403, dio %d", code)
	}
}

// ---------- Flujo completo: reserva → confirmación → seña → cancelación ----------

func TestCustomerReservationFlow(t *testing.T) {
	ta := newTestApp(t)
	w := ta.insertWatch("GA-2100-1A1", 1, 150000)
	anaTok, _ := ta.register("ana@example.com")
	betoTok, _ := ta.register("beto@example.com")
	adminTok := ta.adminToken()
	req := map[string]any{"watchId": w.ID.Hex(), "termsAccepted": true, "notes": "Paso el sábado"}

	if code := ta.do("POST", "/api/reservations", "", req, nil); code != 401 {
		t.Errorf("reservar sin sesión debería dar 401, dio %d", code)
	}
	if code := ta.do("POST", "/api/reservations", anaTok, map[string]any{"watchId": w.ID.Hex()}, nil); code != 400 {
		t.Errorf("sin aceptar términos debería dar 400, dio %d", code)
	}

	var view dto.CustomerReservationResponse
	if code := ta.do("POST", "/api/reservations", anaTok, req, &view); code != http.StatusCreated {
		t.Fatalf("crear reserva falló: %d", code)
	}
	if view.Status != domain.ReservationPending || !view.CanCancel || view.PriceAtReservation != 150000 {
		t.Errorf("reserva mal creada: %+v", view)
	}
	if ta.stockOf(w.ID) != 1 {
		t.Error("una reserva pendiente NO debe descontar stock")
	}
	rid, _ := primitive.ObjectIDFromHex(view.ID)
	if stored := ta.reservation(rid); stored.TermsVersion != domain.TermsVersion || stored.TermsAcceptedAt == nil || stored.Source != domain.ReservationSourceWeb {
		t.Errorf("no se registró la aceptación de términos: %+v", stored)
	}
	if code := ta.do("POST", "/api/reservations", anaTok, req, nil); code != http.StatusConflict {
		t.Errorf("reserva duplicada debería dar 409, dio %d", code)
	}

	var anaList, betoList []dto.CustomerReservationResponse
	ta.do("GET", "/api/reservations", anaTok, nil, &anaList)
	ta.do("GET", "/api/reservations", betoTok, nil, &betoList)
	if len(anaList) != 1 || len(betoList) != 0 {
		t.Errorf("cada cliente debe ver solo sus reservas: ana=%d beto=%d", len(anaList), len(betoList))
	}
	if code := ta.do("GET", "/api/reservations/"+view.ID, betoTok, nil, nil); code != 404 {
		t.Errorf("leer una reserva ajena debería dar 404, dio %d", code)
	}
	if code := ta.do("POST", "/api/reservations/"+view.ID+"/cancel", betoTok, nil, nil); code != 404 {
		t.Errorf("cancelar una reserva ajena debería dar 404, dio %d", code)
	}
	if code := ta.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", anaTok, map[string]string{"status": "confirmed"}, nil); code != 403 {
		t.Errorf("el cliente no puede confirmar: esperaba 403, dio %d", code)
	}

	var confirmed domain.Reservation
	if code := ta.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", adminTok, map[string]string{"status": "confirmed"}, &confirmed); code != 200 || confirmed.ConfirmedAt == nil {
		t.Fatalf("el Admin no pudo confirmar: %d %+v", code, confirmed)
	}
	if ta.stockOf(w.ID) != 0 {
		t.Error("confirmar debe descontar una unidad")
	}

	if code := ta.do("POST", "/api/reservations/"+view.ID+"/cancel", anaTok, nil, nil); code != http.StatusConflict {
		t.Errorf("cancelar una confirmada desde la web debería dar 409, dio %d", code)
	}
	if code := ta.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", adminTok, map[string]any{"status": "deposit_paid", "depositAmount": 20000}, nil); code != 400 {
		t.Errorf("seña sin método debería dar 400, dio %d", code)
	}
	if code := ta.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", adminTok,
		map[string]any{"status": "deposit_paid", "depositAmount": 20000, "depositMethod": "bank_transfer"}, nil); code != 200 {
		t.Fatalf("registrar seña falló: %d", code)
	}
	var mine dto.CustomerReservationResponse
	ta.do("GET", "/api/reservations/"+view.ID, anaTok, nil, &mine)
	if mine.Status != domain.ReservationDepositPaid || mine.DepositAmount != 20000 || mine.DepositPaidAt == nil || mine.CanCancel {
		t.Errorf("vista del cliente incorrecta tras la seña: %+v", mine)
	}

	var adminList struct {
		Items []dto.AdminReservationResponse `json:"items"`
	}
	ta.do("GET", "/api/admin/reservations", adminTok, nil, &adminList)
	if len(adminList.Items) != 1 || adminList.Items[0].Customer == nil || adminList.Items[0].Customer.DocumentNumber != "30123456" {
		t.Errorf("el Admin debería ver los datos del cliente: %+v", adminList.Items)
	}

	if code := ta.do("PATCH", "/api/admin/reservations/"+view.ID+"/status", adminTok, map[string]string{"status": "cancelled"}, nil); code != 200 {
		t.Fatalf("cancelación admin falló: %d", code)
	}
	if ta.stockOf(w.ID) != 1 {
		t.Error("cancelar una reserva confirmada debe devolver la unidad")
	}
}

func TestCustomerCancelsPendingAndOutOfStockRequest(t *testing.T) {
	ta := newTestApp(t)
	w := ta.insertWatch("F-91W-1", 2, 30000)
	empty := ta.insertWatch("LA670WA-1", 0, 40000)
	tok, _ := ta.register("ana@example.com")
	var view dto.CustomerReservationResponse
	ta.do("POST", "/api/reservations", tok, map[string]any{"watchId": w.ID.Hex(), "termsAccepted": true}, &view)
	var after dto.CustomerReservationResponse
	if code := ta.do("POST", "/api/reservations/"+view.ID+"/cancel", tok, nil, &after); code != 200 || after.Status != domain.ReservationCancelled {
		t.Fatalf("el cliente debería poder cancelar su pendiente: %d %+v", code, after)
	}
	if ta.stockOf(w.ID) != 2 {
		t.Error("cancelar una pendiente no debe modificar stock")
	}
	if code := ta.do("POST", "/api/reservations", tok, map[string]any{"watchId": empty.ID.Hex(), "termsAccepted": true}, nil); code != http.StatusConflict {
		t.Errorf("reservar un reloj sin stock debería dar 409, dio %d", code)
	}
}

// ---------- Concurrencia y atomicidad contra MongoDB real ----------

func TestLastUnitCannotBeReservedTwiceOnMongo(t *testing.T) {
	ta := newTestApp(t)
	ctx := context.Background()
	w := ta.insertWatch("A168WA-1W", 1, 50000)
	const n = 8
	ids := make([]primitive.ObjectID, n)
	for i := range ids {
		ids[i] = ta.newReservation(w, domain.ReservationPending, nil).ID
	}
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = ta.app.Services.Reservations.Confirm(ctx, ids[i], "admin@example.com")
		}(i)
	}
	wg.Wait()
	confirmed := 0
	for _, err := range results {
		if err == nil {
			confirmed++
		} else if !errors.Is(err, domain.ErrOutOfStock) {
			t.Errorf("el resto debe fallar por falta de stock: %v", err)
		}
	}
	if confirmed != 1 || ta.stockOf(w.ID) != 0 {
		t.Errorf("solo una reserva se queda con la última unidad: %d confirmadas, stock %d", confirmed, ta.stockOf(w.ID))
	}
}

func TestConcurrentCancelReleasesOnceOnMongo(t *testing.T) {
	ta := newTestApp(t)
	ctx := context.Background()
	w := ta.insertWatch("MTP-V002D-1B", 1, 0)
	r := ta.newReservation(w, domain.ReservationConfirmed, nil)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = ta.app.Services.Reservations.Cancel(ctx, r.ID, "admin@example.com")
		}(i)
	}
	wg.Wait()
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Errorf("exactamente una cancelación debe aplicarse: %v", errs)
	}
	if got := ta.stockOf(w.ID); got != 1 {
		t.Errorf("la unidad se devuelve una sola vez, stock=%d", got)
	}
	if n := len(ta.movements(w.ID)); n != 2 { // hold + release
		t.Errorf("se esperaban 2 movimientos (retención y liberación), hay %d", n)
	}
}

func TestDuplicateModelRejectedByUniqueIndex(t *testing.T) {
	ta := newTestApp(t)
	ta.insertWatch("GA-2100-1A1", 1, 0)
	// El índice único es la garantía real, aunque se saltee la validación previa.
	_, err := ta.col(database.CollWatches).InsertOne(context.Background(), domain.Watch{Name: "x", Model: "GA2100 1A1", ModelKey: domain.ModelKeyFor("GA2100 1A1")})
	if err == nil {
		t.Fatal("el índice único debería rechazar el duplicado")
	}
	var resp map[string]string
	body := map[string]any{"brand": "Casio", "name": "dup", "model": "ga 2100-1a1", "gender": "hombre", "stockQuantity": 1}
	if code := ta.do("POST", "/api/admin/watches", ta.adminToken(), body, &resp); code != http.StatusConflict || resp["code"] != domain.CodeDuplicateModel {
		t.Errorf("modelo duplicado debería dar 409 DUPLICATE_MODEL: %d %v", code, resp)
	}
}

// ---------- Ledger de stock, vencimiento, archivado ----------

func TestStockLedgerAndExpirationOnMongo(t *testing.T) {
	ta := newTestApp(t)
	ctx := context.Background()
	w := ta.insertWatch("LTP-V002D-7B", 2, 0)
	soon := time.Now().Add(time.Hour)
	held := ta.newReservation(w, domain.ReservationConfirmed, &soon)
	notHeld := ta.newReservation(w, domain.ReservationPending, &soon)
	if ta.stockOf(w.ID) != 1 {
		t.Fatal("la confirmada retiene una unidad")
	}

	// Pasa el tiempo: el vencimiento ya ocurrió.
	ta.backdate(database.CollReservations, bson.M{}, bson.M{"expiresAt": time.Now().Add(-time.Minute)})
	n, err := ta.app.Services.Reservations.ExpireDue(ctx, nil)
	if err != nil || n != 2 {
		t.Fatalf("debían vencer 2, vencieron %d (%v)", n, err)
	}
	if ta.stockOf(w.ID) != 2 {
		t.Error("vencer la confirmada devuelve su unidad")
	}
	for _, id := range []primitive.ObjectID{held.ID, notHeld.ID} {
		if r := ta.reservation(id); r.Status != domain.ReservationExpired || r.StockHeld || r.ExpiredAt == nil {
			t.Errorf("reserva %s debería estar vencida: %+v", id.Hex(), r)
		}
	}
	ms := ta.movements(w.ID)
	hold, rel := findMove(ms, domain.MoveReservationHold), findMove(ms, domain.MoveReservationRelease)
	if hold == nil || hold.StockBefore != 2 || hold.StockAfter != 1 || rel == nil || rel.CreatedBy != domain.SystemActor {
		t.Errorf("libro de stock incorrecto: %+v", ms)
	}

	// Ajuste desde el formulario con base: 2 → 5.
	body := map[string]any{"brand": "Casio", "name": w.Name, "model": w.Model, "gender": "hombre",
		"stockQuantity": 5, "stockQuantityBase": 2, "stockMoveReason": "Conteo físico"}
	tok := ta.adminToken()
	if code := ta.do("PUT", "/api/admin/watches/"+w.ID.Hex(), tok, body, nil); code != 200 {
		t.Fatalf("edición falló: %d", code)
	}
	if m := findMove(ta.movements(w.ID), domain.MoveManualAdjustment); m == nil || m.QuantityDelta != 3 || m.CreatedBy != "admin@example.com" {
		t.Errorf("ajuste 2 → 5 mal registrado: %+v", m)
	}
	body["stockQuantityBase"] = 2 // ya no es 2: alguien lo cambió
	body["stockQuantity"] = 9
	var conflict map[string]string
	if code := ta.do("PUT", "/api/admin/watches/"+w.ID.Hex(), tok, body, &conflict); code != 409 || conflict["code"] != domain.CodeConcurrentUpdate {
		t.Errorf("edición con stock desactualizado debería dar 409: %d %v", code, conflict)
	}
	var page struct {
		Items []domain.StockMovement `json:"items"`
	}
	if code := ta.do("GET", "/api/admin/stock-movements?watchId="+w.ID.Hex(), tok, nil, &page); code != 200 || len(page.Items) != 3 {
		t.Errorf("el historial debería listar 3 movimientos: %d %d", code, len(page.Items))
	}
}

func TestArchiveWithActiveReservationsRequiresConfirmation(t *testing.T) {
	ta := newTestApp(t)
	w := ta.insertWatch("LTP-V002D-7B", 3, 0)
	tok := ta.adminToken()
	ta.newReservation(w, domain.ReservationConfirmed, nil)
	ta.newReservation(w, domain.ReservationConfirmed, nil)
	var resp map[string]any
	if code := ta.do("PATCH", "/api/admin/watches/"+w.ID.Hex()+"/archive", tok, nil, &resp); code != http.StatusConflict {
		t.Fatalf("archivar con reservas activas sin confirmar debería dar 409, dio %d", code)
	}
	if resp["code"] != domain.CodeActiveReservations || resp["activeReservations"] != float64(2) {
		t.Errorf("respuesta incorrecta: %v", resp)
	}
	if code := ta.do("PATCH", "/api/admin/watches/"+w.ID.Hex()+"/archive?confirm=true", tok, nil, nil); code != 200 {
		t.Errorf("con confirmación explícita debería archivar, dio %d", code)
	}
	if code := ta.do("GET", "/api/watches/"+w.ID.Hex(), "", nil, nil); code != 404 {
		t.Errorf("un reloj archivado no se ve en el catálogo público, dio %d", code)
	}
}

// ---------- Sesiones y recuperación de contraseña ----------

func TestTokenVersionAndDeactivation(t *testing.T) {
	ta := newTestApp(t)
	oldTok, cust := ta.register("ana@example.com")
	var resp map[string]string
	if code := ta.do("POST", "/api/auth/me/password", oldTok,
		map[string]string{"currentPassword": "secreta123", "newPassword": "nuevaClave99"}, &resp); code != 200 || resp["token"] == "" {
		t.Fatalf("cambio de contraseña falló: %d", code)
	}
	if code := ta.do("GET", "/api/auth/me", oldTok, nil, nil); code != 401 {
		t.Errorf("un JWT con tokenVersion viejo debería dar 401, dio %d", code)
	}
	if code := ta.do("GET", "/api/auth/me", resp["token"], nil, nil); code != 200 {
		t.Errorf("el token nuevo debería funcionar, dio %d", code)
	}
	admin := ta.adminToken()
	if code := ta.do("PATCH", "/api/admin/customers/"+cust.ID+"/deactivate", admin, nil, nil); code != 200 {
		t.Fatalf("desactivar falló: %d", code)
	}
	if code := ta.do("GET", "/api/auth/me", resp["token"], nil, nil); code != 401 {
		t.Errorf("una cuenta desactivada no conserva sesiones, dio %d", code)
	}
	if code := ta.do("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "nuevaClave99"}, nil); code != 401 {
		t.Errorf("una cuenta desactivada no puede iniciar sesión, dio %d", code)
	}
	if n, _ := ta.col(database.CollCustomers).CountDocuments(context.Background(), bson.M{}); n != 1 {
		t.Error("desactivar nunca borra al cliente")
	}
}

func TestExpiredJWTIsRejected(t *testing.T) {
	ta := newTestApp(t)
	_, cust := ta.register("ana@example.com")
	expired, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{Role: auth.RoleCustomer, RegisteredClaims: jwt.RegisteredClaims{
		Subject: cust.ID, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}}).SignedString([]byte("clave-de-test-api"))
	var resp map[string]string
	if code := ta.do("GET", "/api/auth/me", expired, nil, &resp); code != 401 || resp["code"] != domain.CodeSessionExpired {
		t.Errorf("un JWT vencido debe dar 401 SESSION_EXPIRED: %d %v", code, resp)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	ta := newTestApp(t)
	ctx := context.Background()
	sessionTok, _ := ta.register("ana@example.com")

	var r1, r2 map[string]string
	c1 := ta.do("POST", "/api/auth/password/forgot", "", map[string]string{"email": "nadie@example.com"}, &r1)
	c2 := ta.do("POST", "/api/auth/password/forgot", "", map[string]string{"email": "ana@example.com"}, &r2)
	if c1 != 200 || c2 != 200 || r1["message"] != r2["message"] {
		t.Errorf("la respuesta debe ser genérica: %d %v / %d %v", c1, r1, c2, r2)
	}
	// Pasan unos minutos (sin ráfaga de pedidos) y se genera un token.
	ta.backdate(database.CollPasswordResets, bson.M{}, bson.M{"createdAt": time.Now().Add(-10 * time.Minute)})
	token, err := ta.app.Services.PasswordReset.Request(ctx, "ana@example.com")
	if err != nil || token == "" {
		t.Fatalf("no se generó el token: %v", err)
	}
	if n, _ := ta.col(database.CollPasswordResets).CountDocuments(ctx, bson.M{"tokenHash": token}); n != 0 {
		t.Fatal("el token no debe guardarse en claro")
	}
	if code := ta.do("POST", "/api/auth/password/reset", "", map[string]string{"token": token, "newPassword": "corta"}, nil); code != 400 {
		t.Errorf("contraseña inválida debería dar 400, dio %d", code)
	}
	if code := ta.do("POST", "/api/auth/password/reset", "", map[string]string{"token": token, "newPassword": "otraClave77"}, nil); code != 200 {
		t.Fatalf("reset válido falló: %d", code)
	}
	var again map[string]string
	if code := ta.do("POST", "/api/auth/password/reset", "", map[string]string{"token": token, "newPassword": "otraClave88"}, &again); code != 400 || again["code"] != domain.CodeInvalidResetToken {
		t.Errorf("el token no puede usarse dos veces: %d %v", code, again)
	}
	if code := ta.do("GET", "/api/auth/me", sessionTok, nil, nil); code != 401 {
		t.Errorf("restablecer debe invalidar sesiones previas, dio %d", code)
	}
	if code := ta.do("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "otraClave77"}, nil); code != 200 {
		t.Errorf("login con la nueva contraseña falló: %d", code)
	}

	// Token vencido.
	ta.backdate(database.CollPasswordResets, bson.M{}, bson.M{"createdAt": time.Now().Add(-time.Hour)})
	expiredTok, _ := ta.app.Services.PasswordReset.Request(ctx, "ana@example.com")
	ta.backdate(database.CollPasswordResets, bson.M{"tokenHash": service.HashResetToken(expiredTok)}, bson.M{"expiresAt": time.Now().Add(-time.Minute)})
	if code := ta.do("POST", "/api/auth/password/reset", "", map[string]string{"token": expiredTok, "newPassword": "valida123"}, nil); code != 400 {
		t.Errorf("un token vencido no debe funcionar, dio %d", code)
	}
}

// ---------- Autorización, consultas, dashboard y contratos generales ----------

func TestCustomerCannotTouchAdminOperations(t *testing.T) {
	ta := newTestApp(t)
	w := ta.insertWatch("A168WA-1W", 1, 0)
	tok, _ := ta.register("ana@example.com")
	for _, c := range []struct{ method, path string }{
		{"PATCH", "/api/admin/watches/" + w.ID.Hex() + "/stock"},
		{"PUT", "/api/admin/watches/" + w.ID.Hex()},
		{"GET", "/api/admin/stock-movements"},
		{"GET", "/api/admin/dashboard"},
		{"GET", "/api/admin/customers"},
	} {
		if code := ta.do(c.method, c.path, tok, map[string]int{"delta": 5}, nil); code != http.StatusForbidden {
			t.Errorf("%s %s con token de cliente debería dar 403, dio %d", c.method, c.path, code)
		}
	}
	if ta.stockOf(w.ID) != 1 {
		t.Error("un cliente nunca debe poder modificar stock")
	}
}

func TestInquiriesTimestampsSnapshotsAndLiteralNotes(t *testing.T) {
	ta := newTestApp(t)
	w := ta.insertWatch("LA670WA-1", 1, 0)
	admin := ta.adminToken()
	if code := ta.do("POST", "/api/inquiries", "", map[string]string{
		"name": "Ana", "email": "ana@example.com", "message": "¿Lo tienen en stock?", "watchId": w.ID.Hex(),
	}, nil); code != 201 {
		t.Fatalf("consulta falló: %d", code)
	}
	if ta.stockOf(w.ID) != 1 {
		t.Error("una consulta nunca descuenta stock")
	}
	var list []domain.Inquiry
	ta.do("GET", "/api/admin/inquiries", admin, nil, &list)
	if len(list) != 1 || list[0].WatchModelSnapshot != "LA670WA-1" || list[0].WatchNameSnapshot != w.Name {
		t.Fatalf("faltan los snapshots del reloj: %+v", list)
	}
	var q domain.Inquiry
	ta.do("PATCH", "/api/admin/inquiries/"+list[0].ID.Hex(), admin, map[string]string{"status": "contacted", "adminNotes": "$100 de seña"}, &q)
	if q.ContactedAt == nil || q.AdminNotes != "$100 de seña" {
		t.Fatalf("contactedAt o nota literal incorrectos: %+v", q)
	}
	first := *q.ContactedAt
	time.Sleep(10 * time.Millisecond)
	ta.do("PATCH", "/api/admin/inquiries/"+list[0].ID.Hex(), admin, map[string]string{"status": "contacted"}, &q)
	if !q.ContactedAt.Equal(first) {
		t.Error("contactedAt debe conservar la primera vez")
	}
}

func TestCatalogDashboardHealthAndJSONErrors(t *testing.T) {
	ta := newTestApp(t)
	admin := ta.adminToken()
	w := ta.insertWatch("GA-2100-1A1", 1, 0)
	ta.insertWatch("F-91W-1", 0, 0)
	soon := time.Now().Add(time.Hour)
	ta.newReservation(w, domain.ReservationPending, &soon)

	var page dto.Page[domain.Watch]
	if code := ta.do("GET", "/api/watches?search=ga2100&limit=10", "", nil, &page); code != 200 || page.Total != 1 || page.Items[0].Availability != domain.AvailabilityLastUnit {
		t.Errorf("búsqueda del catálogo incorrecta: %d %+v", code, page)
	}
	if code := ta.do("GET", "/api/watches?limit=999999", "", nil, nil); code != 400 {
		t.Errorf("límite excesivo debería dar 400, dio %d", code)
	}
	var d dto.Dashboard
	if code := ta.do("GET", "/api/admin/dashboard", admin, nil, &d); code != 200 {
		t.Fatalf("dashboard falló: %d", code)
	}
	if d.PendingReservations != 1 || d.OutOfStock != 1 || d.LastUnit != 1 || len(d.ExpiringSoon) != 1 {
		t.Errorf("métricas incorrectas: %+v", d)
	}
	var health map[string]string
	if code := ta.do("GET", "/api/health", "", nil, &health); code != 200 || health["database"] != "ok" {
		t.Errorf("health incorrecto: %d %v", code, health)
	}
	var nf map[string]string
	if code := ta.do("GET", "/api/no-existe", "", nil, &nf); code != 404 || nf["code"] != "NOT_FOUND" {
		t.Errorf("ruta inexistente debería dar 404 JSON con código: %d %v", code, nf)
	}
}
