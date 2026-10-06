package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"relojeria-yampier/internal/config"
	"relojeria-yampier/internal/database"
	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
)

// Tests de INTEGRACIÓN: router HTTP real + MongoDB real, en una base
// temporal (yampier_test_<id>) que se borra al terminar. Nunca tocan la base
// real. Si no hay Mongo (MONGO_TEST_URI o localhost:27017) se saltean: los
// tests unitarios de service/domain corren igual sin Docker.

type testApp struct {
	t   *testing.T
	app *App
	db  *database.DB
	h   http.Handler
}

func newTestApp(t *testing.T) *testApp {
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
	db := &database.DB{Client: client, DB: client.Database("yampier_test_" + primitive.NewObjectID().Hex())}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.DB.Drop(c)
		_ = client.Disconnect(c)
	})
	cfg := config.Config{JWTSecret: "clave-de-test-api", AllowedOrigin: "http://localhost:5173",
		UploadsDir: t.TempDir(), PublicURL: "http://localhost:5173"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := New(cfg, db, log)
	a.Prepare(context.Background()) // índices (incluidos los únicos)
	return &testApp{t: t, app: a, db: db, h: a.Handler()}
}

func (ta *testApp) do(method, path, token string, body any, out any) int {
	ta.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	ta.h.ServeHTTP(rec, req)
	if out != nil {
		_ = json.Unmarshal(rec.Body.Bytes(), out)
	}
	return rec.Code
}

func (ta *testApp) col(name string) *mongo.Collection { return ta.db.Collection(name) }

// adminToken crea un admin en la base de prueba y devuelve su token.
func (ta *testApp) adminToken() string {
	ta.t.Helper()
	_, _ = ta.col(database.CollAdmins).InsertOne(context.Background(), domain.AdminUser{Email: "admin@example.com", CreatedAt: time.Now()})
	tok, err := ta.app.Tokens.IssueAdmin("admin@example.com", 0)
	if err != nil {
		ta.t.Fatal(err)
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

type authResp struct {
	Token    string                `json:"token"`
	Customer *dto.CustomerResponse `json:"customer"`
}

func (ta *testApp) register(email string) (string, *dto.CustomerResponse) {
	ta.t.Helper()
	var res authResp
	if code := ta.do("POST", "/api/auth/register", "", registerBody(email), &res); code != http.StatusCreated {
		ta.t.Fatalf("registro falló: %d", code)
	}
	return res.Token, res.Customer
}

func (ta *testApp) insertWatch(model string, qty int, price int64) *domain.Watch {
	ta.t.Helper()
	now := time.Now()
	w := &domain.Watch{Brand: "Casio", BrandKey: "casio", Name: "Casio " + model, Model: model, ModelKey: domain.ModelKeyFor(model),
		Gender: "hombre", PriceARS: price, StockQuantity: qty, CreatedAt: now, UpdatedAt: now}
	res, err := ta.col(database.CollWatches).InsertOne(context.Background(), w)
	if err != nil {
		ta.t.Fatalf("no se pudo insertar el reloj de prueba: %v", err)
	}
	w.ID = res.InsertedID.(primitive.ObjectID)
	return w
}

func (ta *testApp) stockOf(id primitive.ObjectID) int {
	ta.t.Helper()
	var w domain.Watch
	if err := ta.col(database.CollWatches).FindOne(context.Background(), bson.M{"_id": id}).Decode(&w); err != nil {
		ta.t.Fatal(err)
	}
	return w.StockQuantity
}

func (ta *testApp) reservation(id primitive.ObjectID) domain.Reservation {
	ta.t.Helper()
	var r domain.Reservation
	if err := ta.col(database.CollReservations).FindOne(context.Background(), bson.M{"_id": id}).Decode(&r); err != nil {
		ta.t.Fatal(err)
	}
	return r
}

func (ta *testApp) movements(watchID primitive.ObjectID) []domain.StockMovement {
	ta.t.Helper()
	cur, err := ta.col(database.CollStockMovements).Find(context.Background(), bson.M{"watchId": watchID})
	if err != nil {
		ta.t.Fatal(err)
	}
	var out []domain.StockMovement
	_ = cur.All(context.Background(), &out)
	return out
}

// newReservation crea una reserva desde el Admin (pendiente o confirmada).
func (ta *testApp) newReservation(w *domain.Watch, status string, expires *time.Time) *domain.Reservation {
	ta.t.Helper()
	r, err := ta.app.Services.Reservations.CreateByAdmin(context.Background(), dto.AdminReservationRequest{
		WatchID: w.ID.Hex(), CustomerName: "Cliente", Phone: "351", Status: status, ExpiresAt: expires,
	}, "admin@example.com")
	if err != nil {
		ta.t.Fatalf("no se pudo crear la reserva: %v", err)
	}
	return r
}

// backdate mueve campos de fecha al pasado (simula el paso del tiempo).
func (ta *testApp) backdate(coll string, filter bson.M, set bson.M) {
	ta.t.Helper()
	if _, err := ta.col(coll).UpdateMany(context.Background(), filter, bson.M{"$set": set}); err != nil {
		ta.t.Fatal(err)
	}
}

func findMove(ms []domain.StockMovement, typ string) *domain.StockMovement {
	for i := range ms {
		if ms[i].Type == typ {
			return &ms[i]
		}
	}
	return nil
}
