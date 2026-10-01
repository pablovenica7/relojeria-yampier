package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---------- Lógica pura (sin base de datos) ----------

func TestReservationTransitions(t *testing.T) {
	valid := [][2]string{
		{ReservationPending, ReservationConfirmed},
		{ReservationPending, ReservationCancelled},
		{ReservationConfirmed, ReservationDepositPaid},
		{ReservationConfirmed, ReservationCompleted},
		{ReservationConfirmed, ReservationCancelled},
		{ReservationConfirmed, ReservationExpired},
		{ReservationDepositPaid, ReservationCompleted},
		{ReservationDepositPaid, ReservationCancelled},
	}
	for _, tr := range valid {
		if !canTransition(tr[0], tr[1]) {
			t.Errorf("%s -> %s debería estar permitida", tr[0], tr[1])
		}
	}
	invalid := [][2]string{
		{ReservationCompleted, ReservationPending},
		{ReservationCompleted, ReservationCancelled},
		{ReservationCancelled, ReservationConfirmed},
		{ReservationExpired, ReservationConfirmed},
		{ReservationPending, ReservationCompleted},   // hay que confirmar antes
		{ReservationPending, ReservationDepositPaid}, // idem
		{ReservationDepositPaid, ReservationExpired}, // con seña no vence sola
		{ReservationDepositPaid, ReservationPending},
		{ReservationConfirmed, ReservationConfirmed},
	}
	for _, tr := range invalid {
		if canTransition(tr[0], tr[1]) {
			t.Errorf("%s -> %s NO debería estar permitida", tr[0], tr[1])
		}
	}
}

func TestValidateReservationInput(t *testing.T) {
	now := time.Now()
	base := func() ReservationInput {
		return ReservationInput{WatchID: primitive.NewObjectID().Hex(), CustomerName: "Ana", Phone: "351 000 0000"}
	}
	in := base()
	if msg := validateReservationInput(&in, now); msg != "" || in.Status != ReservationPending {
		t.Fatalf("entrada válida rechazada: %q (status=%q)", msg, in.Status)
	}
	past := now.Add(-time.Hour)
	far := now.AddDate(0, 0, 120)
	bad := []func(*ReservationInput){
		func(in *ReservationInput) { in.WatchID = "x" },
		func(in *ReservationInput) { in.CustomerName = " " },
		func(in *ReservationInput) { in.Phone, in.Email = "", "" },
		func(in *ReservationInput) { in.Email = "no-email" },
		func(in *ReservationInput) { in.Status = ReservationCompleted },
		func(in *ReservationInput) { in.DepositAmount = -5 },
		func(in *ReservationInput) { in.ExpiresAt = &past },
		func(in *ReservationInput) { in.ExpiresAt = &far },
	}
	for i, mutate := range bad {
		in := base()
		mutate(&in)
		if validateReservationInput(&in, now) == "" {
			t.Errorf("caso inválido #%d aceptado", i)
		}
	}
}

// ---------- Integración con MongoDB ----------
//
// Usan una base temporal (yampier_test_<id>) que se borra al terminar. Nunca
// tocan la base real. Si no hay Mongo disponible (MONGO_TEST_URI o
// localhost:27017), se saltean con un aviso.

func setupTestDB(t *testing.T) {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(2*time.Second))
	if err == nil {
		err = client.Ping(ctx, nil)
	}
	if err != nil {
		t.Skipf("MongoDB no disponible en %s: se omite el test de integración (%v)", uri, err)
	}
	prevClient, prevDB := mongoClient, database
	mongoClient = client
	database = client.Database("yampier_test_" + primitive.NewObjectID().Hex())
	ensureIndexes()
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = database.Drop(c)
		_ = client.Disconnect(c)
		mongoClient, database = prevClient, prevDB
	})
}

func insertTestWatch(t *testing.T, model string, qty int, price int64) *Watch {
	t.Helper()
	now := time.Now()
	w := Watch{Brand: "Casio", BrandKey: "casio", Name: "Casio " + model, Model: model, ModelKey: modelKeyFor(model),
		Gender: "hombre", PriceARS: price, StockQuantity: qty, CreatedAt: now, UpdatedAt: now}
	res, err := watchesCol().InsertOne(context.Background(), w)
	if err != nil {
		t.Fatalf("no se pudo insertar el reloj de prueba: %v", err)
	}
	w.ID = res.InsertedID.(primitive.ObjectID)
	return &w
}

func stockOf(t *testing.T, id primitive.ObjectID) int {
	t.Helper()
	w, err := findWatch(context.Background(), bson.M{"_id": id})
	if err != nil {
		t.Fatalf("no se pudo leer el reloj: %v", err)
	}
	return w.StockQuantity
}

func newReservation(t *testing.T, w *Watch, status string, expires *time.Time) *Reservation {
	t.Helper()
	r, err := createReservation(context.Background(), ReservationInput{
		WatchID: w.ID.Hex(), CustomerName: "Cliente", Phone: "351", Status: status, ExpiresAt: expires,
	}, "test@admin", time.Now())
	if err != nil {
		t.Fatalf("no se pudo crear la reserva: %v", err)
	}
	return r
}

func wantStatus(t *testing.T, err error, status int) {
	t.Helper()
	var ae *apiError
	if !errors.As(err, &ae) || ae.Status != status {
		t.Fatalf("esperaba error HTTP %d, obtuvo %v", status, err)
	}
}

func TestInquiryDoesNotTouchStockAndLinksWatch(t *testing.T) {
	setupTestDB(t)
	w := insertTestWatch(t, "F-91W-1", 1, 30000)
	q, err := buildInquiry(context.Background(), InquiryInput{Name: "Ana", Email: "a@b.com", Message: "Hola, ¿lo tienen?", WatchID: w.ID.Hex()}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if q.WatchID == nil || *q.WatchID != w.ID || q.WatchNameSnapshot != w.Name || q.Status != InquiryNew {
		t.Errorf("la consulta no quedó vinculada al reloj: %+v", q)
	}
	if stockOf(t, w.ID) != 1 {
		t.Error("una consulta nunca debe descontar stock")
	}
	_, err = buildInquiry(context.Background(), InquiryInput{Name: "Ana", WatchID: primitive.NewObjectID().Hex()}, time.Now())
	wantStatus(t, err, http.StatusNotFound)
}

func TestReservationHoldsStockOnConfirmAndSnapshotsPrice(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	w := insertTestWatch(t, "GA-2100-1A1", 2, 150000)

	r := newReservation(t, w, ReservationPending, nil)
	if r.StockHeld || stockOf(t, w.ID) != 2 {
		t.Fatal("una reserva pendiente no retiene stock")
	}
	if r.PriceAtReservation != 150000 || r.WatchModelSnapshot != "GA-2100-1A1" {
		t.Errorf("snapshot incorrecto: %+v", r)
	}

	// El Admin cambia el precio: la reserva conserva el acordado.
	_, _ = watchesCol().UpdateOne(ctx, bson.M{"_id": w.ID}, bson.M{"$set": bson.M{"priceARS": 170000, "name": "Otro nombre"}})

	r, err := transitionReservation(ctx, r.ID, ReservationConfirmed, transitionOpts{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !r.StockHeld || stockOf(t, w.ID) != 1 {
		t.Error("confirmar debe retener exactamente una unidad")
	}
	if r.PriceAtReservation != 150000 || r.WatchNameSnapshot != "Casio GA-2100-1A1" {
		t.Error("la reserva no debe cambiar si cambia el reloj")
	}

	// Seña sin monto: 400. Con monto: OK, y el stock no vuelve a descontarse.
	_, err = transitionReservation(ctx, r.ID, ReservationDepositPaid, transitionOpts{}, time.Now())
	wantStatus(t, err, http.StatusBadRequest)
	dep := int64(20000)
	r, err = transitionReservation(ctx, r.ID, ReservationDepositPaid, transitionOpts{DepositAmount: &dep, DepositMethod: DepositCash}, time.Now())
	if err != nil || r.DepositAmount != 20000 || stockOf(t, w.ID) != 1 {
		t.Fatalf("seña mal registrada: %v %+v", err, r)
	}

	// Completar: la unidad queda vendida (no vuelve al stock).
	r, err = transitionReservation(ctx, r.ID, ReservationCompleted, transitionOpts{}, time.Now())
	if err != nil || r.StockHeld || stockOf(t, w.ID) != 1 {
		t.Fatalf("completar no debe devolver stock: %v", err)
	}
	// Transición absurda desde estado final: 409.
	_, err = transitionReservation(ctx, r.ID, ReservationPending, transitionOpts{}, time.Now())
	wantStatus(t, err, http.StatusConflict)
}

func TestReservationWithoutStockIsRejected(t *testing.T) {
	setupTestDB(t)
	w := insertTestWatch(t, "LA670WA-1", 0, 40000)
	_, err := createReservation(context.Background(), ReservationInput{
		WatchID: w.ID.Hex(), CustomerName: "Ana", Phone: "351", Status: ReservationConfirmed,
	}, "test@admin", time.Now())
	wantStatus(t, err, http.StatusConflict)
	if stockOf(t, w.ID) != 0 {
		t.Error("el stock nunca debe quedar negativo")
	}
	if n, _ := reservationsCol().CountDocuments(context.Background(), bson.M{}); n != 0 {
		t.Error("no debería guardarse una reserva confirmada sin stock")
	}

	// Pendiente sí se puede registrar (lista de espera), pero no confirmar.
	r := newReservation(t, w, ReservationPending, nil)
	_, err = transitionReservation(context.Background(), r.ID, ReservationConfirmed, transitionOpts{}, time.Now())
	wantStatus(t, err, http.StatusConflict)
}

func TestCancelReleasesStockOnlyOnce(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	w := insertTestWatch(t, "MTP-V002D-1B", 1, 0)
	r := newReservation(t, w, ReservationConfirmed, nil)
	if stockOf(t, w.ID) != 0 {
		t.Fatal("la reserva confirmada debe retener la unidad")
	}

	// Dos cancelaciones simultáneas: solo una devuelve la unidad.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = transitionReservation(ctx, r.ID, ReservationCancelled, transitionOpts{}, time.Now())
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Errorf("exactamente una cancelación debería aplicarse, se aplicaron %d (%v)", ok, errs)
	}
	if got := stockOf(t, w.ID); got != 1 {
		t.Errorf("la cancelación debe devolver una sola unidad, stock=%d", got)
	}

	// Cancelar una pendiente no toca el stock.
	p := newReservation(t, w, ReservationPending, nil)
	if _, err := transitionReservation(ctx, p.ID, ReservationCancelled, transitionOpts{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if stockOf(t, w.ID) != 1 {
		t.Error("cancelar una pendiente no debe sumar stock")
	}
}

func TestLastUnitCannotBeReservedTwice(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	w := insertTestWatch(t, "A168WA-1W", 1, 50000)

	const n = 8
	pending := make([]*Reservation, n)
	for i := range pending {
		pending[i] = newReservation(t, w, ReservationPending, nil)
	}
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range pending {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = transitionReservation(ctx, pending[i].ID, ReservationConfirmed, transitionOpts{}, time.Now())
		}(i)
	}
	wg.Wait()

	confirmed := 0
	for _, err := range results {
		if err == nil {
			confirmed++
		} else {
			wantStatus(t, err, http.StatusConflict)
		}
	}
	if confirmed != 1 {
		t.Errorf("solo una reserva puede quedarse con la última unidad, se confirmaron %d", confirmed)
	}
	if got := stockOf(t, w.ID); got != 0 {
		t.Errorf("stock final esperado 0, obtuvo %d", got)
	}
}

func TestExpirationReleasesHeldStock(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	w := insertTestWatch(t, "LTP-V002D-7B", 2, 0)
	soon := time.Now().Add(time.Minute)
	held := newReservation(t, w, ReservationConfirmed, &soon)
	notHeld := newReservation(t, w, ReservationPending, &soon)
	noExpiry := newReservation(t, w, ReservationConfirmed, nil)
	if stockOf(t, w.ID) != 0 {
		t.Fatal("dos confirmadas deberían retener las dos unidades")
	}

	// Pasa el tiempo: se ejecuta el barrido como si fuera dentro de 2 minutos.
	n, err := expireDueReservations(ctx, time.Now().Add(2*time.Minute), nil)
	if err != nil || n != 2 {
		t.Fatalf("debían vencer 2 reservas, vencieron %d (%v)", n, err)
	}
	if got := stockOf(t, w.ID); got != 1 {
		t.Errorf("solo la reserva confirmada vencida devuelve stock, stock=%d", got)
	}
	for _, id := range []primitive.ObjectID{held.ID, notHeld.ID} {
		r, _ := findReservation(ctx, id)
		if r.Status != ReservationExpired || r.StockHeld {
			t.Errorf("reserva %s debería estar vencida y sin stock retenido: %+v", id.Hex(), r)
		}
	}
	// Un segundo barrido no devuelve nada dos veces.
	if n, _ := expireDueReservations(ctx, time.Now().Add(3*time.Minute), nil); n != 0 || stockOf(t, w.ID) != 1 {
		t.Error("el vencimiento debe ser idempotente")
	}
	if r, _ := findReservation(ctx, noExpiry.ID); r.Status != ReservationConfirmed {
		t.Error("una reserva sin vencimiento no debe vencer")
	}
}

func TestDuplicateModelRejectedByUniqueIndex(t *testing.T) {
	setupTestDB(t)
	insertTestWatch(t, "GA-2100-1A1", 1, 0)
	if err := ensureModelAvailable(context.Background(), modelKeyFor("ga 2100-1a1"), nil); err == nil {
		t.Error("la validación previa debería detectar el duplicado")
	}
	_, err := watchesCol().InsertOne(context.Background(), Watch{Name: "x", Model: "GA2100 1A1", ModelKey: modelKeyFor("GA2100 1A1")})
	if !mongo.IsDuplicateKeyError(err) {
		t.Errorf("el índice único debería rechazar el duplicado, err=%v", err)
	}
	wantStatus(t, duplicateOr(err), http.StatusConflict)
}

func TestAdjustStockNeverNegative(t *testing.T) {
	setupTestDB(t)
	w := insertTestWatch(t, "F-91W-1", 1, 0)
	_, err := adjustStock(context.Background(), w.ID, -2, MoveManualAdjustment, "", "test@admin")
	wantStatus(t, err, http.StatusConflict)
	if got, err := adjustStock(context.Background(), w.ID, -1, MoveManualAdjustment, "", "test@admin"); err != nil || got.StockQuantity != 0 || got.Availability != AvailabilityOutOfStock {
		t.Errorf("ajuste válido falló: %v %+v", err, got)
	}
}
