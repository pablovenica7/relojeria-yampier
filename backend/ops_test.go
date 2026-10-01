package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func movementsOf(t *testing.T, watchID primitive.ObjectID) []StockMovement {
	t.Helper()
	cur, err := stockMovementsCol().Find(context.Background(), bson.M{"watchId": watchID})
	if err != nil {
		t.Fatal(err)
	}
	var out []StockMovement
	_ = cur.All(context.Background(), &out)
	return out
}

func findMove(ms []StockMovement, typ string) *StockMovement {
	for i := range ms {
		if ms[i].Type == typ {
			return &ms[i]
		}
	}
	return nil
}

func TestStockLedgerFollowsReservationLifecycle(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	w := insertTestWatch(t, "GA-2100-1A1", 2, 150000)
	admin := transitionOpts{Actor: "admin@example.com"}

	// Pendiente: no hay movimiento. Confirmada: reservation_hold -1 (2 → 1).
	r := newReservation(t, w, ReservationPending, nil)
	if len(movementsOf(t, w.ID)) != 0 {
		t.Fatal("una reserva pendiente no genera movimiento de stock")
	}
	r, err := transitionReservation(ctx, r.ID, ReservationConfirmed, admin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if r.ConfirmedAt == nil {
		t.Error("falta confirmedAt")
	}
	hold := findMove(movementsOf(t, w.ID), MoveReservationHold)
	if hold == nil || hold.QuantityDelta != -1 || hold.StockBefore != 2 || hold.StockAfter != 1 ||
		hold.ReservationID == nil || *hold.ReservationID != r.ID || hold.CreatedBy != "admin@example.com" {
		t.Fatalf("movimiento de retención incorrecto: %+v", hold)
	}

	// Completar: venta con delta 0 (no descuenta dos veces) y stock intacto.
	r, err = transitionReservation(ctx, r.ID, ReservationCompleted, admin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if r.CompletedAt == nil || stockOf(t, w.ID) != 1 {
		t.Error("completar no debe devolver stock y debe registrar completedAt")
	}
	sale := findMove(movementsOf(t, w.ID), MoveSale)
	if sale == nil || sale.QuantityDelta != 0 || sale.StockBefore != 1 || sale.StockAfter != 1 {
		t.Fatalf("movimiento de venta incorrecto: %+v", sale)
	}

	// Otra reserva confirmada y cancelada: hold -1 y release +1.
	r2 := newReservation(t, w, ReservationConfirmed, nil)
	r2, err = transitionReservation(ctx, r2.ID, ReservationCancelled, admin, time.Now())
	if err != nil || r2.CancelledAt == nil {
		t.Fatalf("cancelación falló: %v", err)
	}
	ms := movementsOf(t, w.ID)
	if len(ms) != 4 || findMove(ms, MoveReservationRelease) == nil || stockOf(t, w.ID) != 1 {
		t.Errorf("se esperaban 4 movimientos (hold, sale, hold, release) y stock 1: %d / %d", len(ms), stockOf(t, w.ID))
	}

	// Reintento del mismo movimiento (misma clave): no se duplica.
	rel := findMove(ms, MoveReservationRelease)
	recordMovement(ctx, *rel)
	if len(movementsOf(t, w.ID)) != 4 {
		t.Error("un reintento no debe duplicar el movimiento")
	}
}

func TestStockLedgerOnExpirationAndManualAdjustment(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	w := insertTestWatch(t, "F-91W-1", 1, 30000)
	soon := time.Now().Add(time.Minute)
	r := newReservation(t, w, ReservationConfirmed, &soon)
	if _, err := expireDueReservations(ctx, time.Now().Add(2*time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	got, _ := findReservation(ctx, r.ID)
	if got.Status != ReservationExpired || got.ExpiredAt == nil || stockOf(t, w.ID) != 1 {
		t.Fatalf("vencimiento incorrecto: %+v stock=%d", got, stockOf(t, w.ID))
	}
	rel := findMove(movementsOf(t, w.ID), MoveReservationRelease)
	if rel == nil || rel.CreatedBy != systemActor || rel.StockAfter != 1 {
		t.Errorf("el vencimiento debe anotar la liberación como sistema: %+v", rel)
	}

	if _, err := adjustStock(ctx, w.ID, 3, MoveStockEntry, "Ingreso proveedor", "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	entry := findMove(movementsOf(t, w.ID), MoveStockEntry)
	if entry == nil || entry.QuantityDelta != 3 || entry.StockBefore != 1 || entry.StockAfter != 4 || entry.Reason != "Ingreso proveedor" {
		t.Errorf("ingreso de stock mal registrado: %+v", entry)
	}
}

func TestEditingStockFromFormRecordsMovement(t *testing.T) {
	api := newAPI(t)
	w := insertTestWatch(t, "MTP-V002D-1B", 5, 0)
	tok := api.adminToken()
	body := map[string]any{"brand": "Casio", "name": w.Name, "model": w.Model, "gender": "hombre",
		"stockQuantity": 8, "stockQuantityBase": 5, "stockMoveReason": "Conteo físico"}
	if code := api.do("PUT", "/api/admin/watches/"+w.ID.Hex(), tok, body, nil); code != 200 {
		t.Fatalf("edición falló: %d", code)
	}
	m := findMove(movementsOf(t, w.ID), MoveManualAdjustment)
	if m == nil || m.QuantityDelta != 3 || m.StockBefore != 5 || m.StockAfter != 8 || m.CreatedBy != "admin@example.com" {
		t.Errorf("ajuste 5 → 8 mal registrado: %+v", m)
	}
	var page struct {
		Items []StockMovement `json:"items"`
	}
	if code := api.do("GET", "/api/admin/stock-movements?watchId="+w.ID.Hex(), tok, nil, &page); code != 200 || len(page.Items) != 1 {
		t.Errorf("el historial debería listar el movimiento: %d %d", code, len(page.Items))
	}
}

func TestArchiveWithActiveReservationsRequiresConfirmation(t *testing.T) {
	api := newAPI(t)
	w := insertTestWatch(t, "LTP-V002D-7B", 3, 0)
	tok := api.adminToken()
	newReservation(t, w, ReservationConfirmed, nil)
	newReservation(t, w, ReservationConfirmed, nil)

	var resp map[string]any
	if code := api.do("PATCH", "/api/admin/watches/"+w.ID.Hex()+"/archive", tok, nil, &resp); code != http.StatusConflict {
		t.Fatalf("archivar con reservas activas sin confirmar debería dar 409, dio %d", code)
	}
	if resp["code"] != CodeActiveReservations || resp["activeReservations"] != float64(2) {
		t.Errorf("respuesta incorrecta: %v", resp)
	}
	if code := api.do("PATCH", "/api/admin/watches/"+w.ID.Hex()+"/archive?confirm=true", tok, nil, nil); code != 200 {
		t.Errorf("con confirmación explícita debería archivar, dio %d", code)
	}
}

func TestTokenVersionInvalidatesOldSessions(t *testing.T) {
	api := newAPI(t)
	oldTok, cust := api.register("ana@example.com")

	// Cambio de contraseña: el token viejo deja de valer; el nuevo funciona.
	var resp map[string]string
	if code := api.do("POST", "/api/auth/me/password", oldTok,
		map[string]string{"currentPassword": "secreta123", "newPassword": "nuevaClave99"}, &resp); code != 200 || resp["token"] == "" {
		t.Fatalf("cambio de contraseña falló: %d", code)
	}
	if code := api.do("GET", "/api/auth/me", oldTok, nil, nil); code != 401 {
		t.Errorf("un JWT con tokenVersion viejo debería dar 401, dio %d", code)
	}
	if code := api.do("GET", "/api/auth/me", resp["token"], nil, nil); code != 200 {
		t.Errorf("el token nuevo debería funcionar, dio %d", code)
	}

	// Desactivación por el Admin: corta la sesión y el login.
	admin := api.adminToken()
	if code := api.do("PATCH", "/api/admin/customers/"+cust.ID.Hex()+"/deactivate", admin, nil, nil); code != 200 {
		t.Fatalf("desactivar falló: %d", code)
	}
	if code := api.do("GET", "/api/auth/me", resp["token"], nil, nil); code != 401 {
		t.Errorf("una cuenta desactivada no debe conservar sesiones, dio %d", code)
	}
	if code := api.do("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "nuevaClave99"}, nil); code != 401 {
		t.Errorf("una cuenta desactivada no puede iniciar sesión, dio %d", code)
	}
	if n, _ := customersCol().CountDocuments(context.Background(), bson.M{"_id": cust.ID}); n != 1 {
		t.Error("desactivar nunca debe borrar al cliente")
	}
}

func TestExpiredJWTIsRejected(t *testing.T) {
	api := newAPI(t)
	_, cust := api.register("ana@example.com")
	expired, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{Role: RoleCustomer, RegisteredClaims: jwt.RegisteredClaims{
		Subject: cust.ID.Hex(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}}).SignedString(jwtSecretBytes)
	var resp map[string]string
	if code := api.do("GET", "/api/auth/me", expired, nil, &resp); code != 401 || resp["code"] != CodeSessionExpired {
		t.Errorf("un JWT vencido debe dar 401 SESSION_EXPIRED: %d %v", code, resp)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	api := newAPI(t)
	ctx := context.Background()
	sessionTok, _ := api.register("ana@example.com")

	// Respuesta idéntica exista o no la cuenta.
	var r1, r2 map[string]string
	c1 := api.do("POST", "/api/auth/password/forgot", "", map[string]string{"email": "nadie@example.com"}, &r1)
	c2 := api.do("POST", "/api/auth/password/forgot", "", map[string]string{"email": "ana@example.com"}, &r2)
	if c1 != 200 || c2 != 200 || r1["message"] != r2["message"] {
		t.Errorf("la respuesta debe ser genérica: %d %v / %d %v", c1, r1, c2, r2)
	}

	// En la base solo queda el hash, nunca el token.
	token, err := requestPasswordReset(ctx, "ana@example.com", time.Now().Add(5*time.Minute))
	if err != nil || token == "" {
		t.Fatalf("no se generó el token: %v", err)
	}
	if n, _ := passwordResetsCol().CountDocuments(ctx, bson.M{"tokenHash": token}); n != 0 {
		t.Fatal("el token no debe guardarse en claro")
	}

	// Contraseña inválida: no consume el token.
	if code := api.do("POST", "/api/auth/password/reset", "", map[string]string{"token": token, "newPassword": "corta"}, nil); code != 400 {
		t.Errorf("contraseña inválida debería dar 400, dio %d", code)
	}
	// Uso válido.
	if code := api.do("POST", "/api/auth/password/reset", "", map[string]string{"token": token, "newPassword": "otraClave77"}, nil); code != 200 {
		t.Fatalf("reset válido falló: %d", code)
	}
	// Un solo uso.
	var again map[string]string
	if code := api.do("POST", "/api/auth/password/reset", "", map[string]string{"token": token, "newPassword": "otraClave88"}, &again); code != 400 || again["code"] != CodeInvalidResetToken {
		t.Errorf("el token no puede usarse dos veces: %d %v", code, again)
	}
	// Las sesiones anteriores se cierran; la nueva contraseña funciona.
	if code := api.do("GET", "/api/auth/me", sessionTok, nil, nil); code != 401 {
		t.Errorf("restablecer debe invalidar sesiones previas, dio %d", code)
	}
	if code := api.do("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "otraClave77"}, nil); code != 200 {
		t.Errorf("login con la nueva contraseña falló: %d", code)
	}

	// Token vencido.
	expiredTok, _ := requestPasswordReset(ctx, "ana@example.com", time.Now().Add(-2*resetTokenTTL))
	if err := resetPassword(ctx, expiredTok, "valida123", time.Now()); err == nil {
		t.Error("un token vencido no debe funcionar")
	}
}

func TestCustomerCannotTouchAdminOperations(t *testing.T) {
	api := newAPI(t)
	w := insertTestWatch(t, "A168WA-1W", 1, 0)
	tok, _ := api.register("ana@example.com")
	for _, c := range []struct{ method, path string }{
		{"PATCH", "/api/admin/watches/" + w.ID.Hex() + "/stock"},
		{"PUT", "/api/admin/watches/" + w.ID.Hex()},
		{"GET", "/api/admin/stock-movements"},
		{"GET", "/api/admin/dashboard"},
		{"GET", "/api/admin/customers"},
	} {
		if code := api.do(c.method, c.path, tok, map[string]int{"delta": 5}, nil); code != http.StatusForbidden {
			t.Errorf("%s %s con token de cliente debería dar 403, dio %d", c.method, c.path, code)
		}
	}
	if stockOf(t, w.ID) != 1 {
		t.Error("un cliente nunca debe poder modificar stock")
	}
}

func TestInquiryTimestampsSnapshotsAndLiteralNotes(t *testing.T) {
	api := newAPI(t)
	w := insertTestWatch(t, "LA670WA-1", 1, 0)
	admin := api.adminToken()
	if code := api.do("POST", "/api/inquiries", "", map[string]string{
		"name": "Ana", "email": "ana@example.com", "message": "¿Lo tienen en stock?", "watchId": w.ID.Hex(),
	}, nil); code != 201 {
		t.Fatalf("consulta falló: %d", code)
	}
	var list []Inquiry
	api.do("GET", "/api/admin/inquiries", admin, nil, &list)
	if len(list) != 1 || list[0].WatchModelSnapshot != "LA670WA-1" {
		t.Fatalf("falta el snapshot del modelo: %+v", list)
	}
	var q Inquiry
	api.do("PATCH", "/api/admin/inquiries/"+list[0].ID.Hex(), admin, map[string]string{"status": "contacted", "adminNotes": "$100 de seña"}, &q)
	if q.ContactedAt == nil || q.AdminNotes != "$100 de seña" {
		t.Fatalf("contactedAt o nota literal incorrectos: %+v", q)
	}
	first := *q.ContactedAt
	time.Sleep(10 * time.Millisecond)
	api.do("PATCH", "/api/admin/inquiries/"+list[0].ID.Hex(), admin, map[string]string{"status": "contacted"}, &q)
	if !q.ContactedAt.Equal(first) {
		t.Error("contactedAt debe conservar la primera vez")
	}
	api.do("PATCH", "/api/admin/inquiries/"+list[0].ID.Hex(), admin, map[string]string{"status": "closed"}, &q)
	if q.ClosedAt == nil {
		t.Error("falta closedAt")
	}
}

func TestDashboardAndJSONNotFound(t *testing.T) {
	api := newAPI(t)
	admin := api.adminToken()
	w := insertTestWatch(t, "GA-2100-1A1", 1, 0)
	insertTestWatch(t, "F-91W-1", 0, 0)
	soon := time.Now().Add(time.Hour)
	newReservation(t, w, ReservationPending, &soon)

	var d map[string]any
	if code := api.do("GET", "/api/admin/dashboard", admin, nil, &d); code != 200 {
		t.Fatalf("dashboard falló: %d", code)
	}
	if d["pendingReservations"] != float64(1) || d["outOfStock"] != float64(1) || d["lastUnit"] != float64(1) {
		t.Errorf("métricas incorrectas: %v", d)
	}
	if exp, _ := d["expiringSoon"].([]any); len(exp) != 1 {
		t.Errorf("debería listar la reserva por vencer: %v", d["expiringSoon"])
	}

	var nf map[string]string
	if code := api.do("GET", "/api/no-existe", "", nil, &nf); code != 404 || nf["code"] != "NOT_FOUND" {
		t.Errorf("ruta inexistente debería dar 404 JSON con código: %d %v", code, nf)
	}
}
