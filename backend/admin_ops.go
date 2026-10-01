package main

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---------- Dashboard ----------

type expiringReservation struct {
	ID           string    `json:"id"`
	WatchName    string    `json:"watchName"`
	CustomerName string    `json:"customerName"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// GET /api/admin/dashboard  (Admin) — números para la operación diaria.
func getDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	now := time.Now()
	// Vencimiento al día antes de contar.
	if _, err := expireDueReservations(ctx, now, nil); err != nil {
		writeErr(w, r, err)
		return
	}

	counts := map[string]bson.M{
		"pendingReservations":     {"status": ReservationPending},
		"confirmedReservations":   {"status": ReservationConfirmed},
		"depositPaidReservations": {"status": ReservationDepositPaid},
	}
	out := map[string]any{}
	for key, f := range counts {
		n, err := reservationsCol().CountDocuments(ctx, f)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		out[key] = n
	}
	watchCounts := map[string]bson.M{
		"outOfStock": {"archivedAt": nil, "stockQuantity": bson.M{"$lte": 0}},
		"lastUnit":   {"archivedAt": nil, "stockQuantity": 1},
	}
	for key, f := range watchCounts {
		n, err := watchesCol().CountDocuments(ctx, f)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		out[key] = n
	}
	n, err := inquiriesCol().CountDocuments(ctx, bson.M{"status": InquiryNew})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out["newInquiries"] = n

	// Reservas que vencen en las próximas 48 h (para actuar antes).
	cur, err := reservationsCol().Find(ctx, bson.M{
		"status":    bson.M{"$in": bson.A{ReservationPending, ReservationConfirmed}},
		"expiresAt": bson.M{"$gt": now, "$lte": now.Add(48 * time.Hour)},
	}, options.Find().SetSort(bson.D{{Key: "expiresAt", Value: 1}}).SetLimit(10))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer cur.Close(ctx)
	var items []Reservation
	if err := cur.All(ctx, &items); err != nil {
		writeErr(w, r, err)
		return
	}
	expiring := make([]expiringReservation, 0, len(items))
	for _, it := range items {
		expiring = append(expiring, expiringReservation{
			ID: it.ID.Hex(), WatchName: it.WatchNameSnapshot, CustomerName: it.CustomerName,
			Status: it.Status, ExpiresAt: *it.ExpiresAt,
		})
	}
	out["expiringSoon"] = expiring
	writeJSON(w, http.StatusOK, out)
}

// ---------- Clientes (Admin) ----------

// adminCustomerView agrega a los datos públicos del cliente el estado de la
// cuenta (que no se expone al propio cliente). No se embebe Customer porque
// su MarshalJSON se promovería y descartaría estos campos.
func adminCustomerView(c Customer) map[string]any {
	b, _ := json.Marshal(c)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	out["active"] = c.Active
	if c.DeactivatedAt != nil {
		out["deactivatedAt"] = c.DeactivatedAt
	}
	return out
}

// GET /api/admin/customers?search=&status=active|inactive|all&page=&limit=
func listCustomers(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	filter := bson.M{}
	switch v.Get("status") {
	case "", "all":
	case "active":
		filter["active"] = true
	case "inactive":
		filter["active"] = false
	default:
		writeError(w, http.StatusBadRequest, "Estado inválido")
		return
	}
	if s := strings.TrimSpace(v.Get("search")); s != "" {
		if len(s) > 80 {
			writeError(w, http.StatusBadRequest, "La búsqueda es demasiado larga")
			return
		}
		q := bson.M{"$regex": regexp.QuoteMeta(s), "$options": "i"}
		filter["$or"] = bson.A{
			bson.M{"firstName": q}, bson.M{"lastName": q}, bson.M{"email": q},
			bson.M{"documentNumber": bson.M{"$regex": regexp.QuoteMeta(digitsOrSelf(s))}},
		}
	}
	page, limit, msg := pageParams(v.Get("page"), v.Get("limit"), 25)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	total, err := customersCol().CountDocuments(ctx, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	cur, err := customersCol().Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetSkip(int64((page-1)*limit)).SetLimit(int64(limit)))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer cur.Close(ctx)
	var custs []Customer
	if err := cur.All(ctx, &custs); err != nil {
		writeErr(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(custs))
	for _, c := range custs {
		items = append(items, adminCustomerView(c))
	}
	writeJSON(w, http.StatusOK, newPaged(items, page, limit, total))
}

func digitsOrSelf(s string) string {
	if d := digitsOnly(s); d != "" {
		return d
	}
	return s
}

// setCustomerActive desactiva (soft delete) o reactiva una cuenta. Nunca se
// borra: sus reservas históricas se conservan. Desactivar impide nuevos
// logins y, al subir tokenVersion, cierra todas sus sesiones.
func setCustomerActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Cliente no encontrado")
		return
	}
	now := time.Now()
	update := bson.M{"$set": bson.M{"active": true, "updatedAt": now}, "$unset": bson.M{"deactivatedAt": ""}}
	if !active {
		update = bson.M{"$set": bson.M{"active": false, "deactivatedAt": now, "updatedAt": now},
			"$inc": bson.M{"tokenVersion": 1}}
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	var c Customer
	err := customersCol().FindOneAndUpdate(ctx, bson.M{"_id": id, "active": !active}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&c)
	if err != nil {
		// No existe, o ya estaba en el estado pedido.
		n, _ := customersCol().CountDocuments(ctx, bson.M{"_id": id})
		if n == 0 {
			writeError(w, http.StatusNotFound, "Cliente no encontrado")
			return
		}
		writeError(w, http.StatusConflict, "La cuenta ya estaba en ese estado")
		return
	}
	writeJSON(w, http.StatusOK, adminCustomerView(c))
}

// PATCH /api/admin/customers/{id}/deactivate y /reactivate  (Admin)
func deactivateCustomer(w http.ResponseWriter, r *http.Request) { setCustomerActive(w, r, false) }
func reactivateCustomer(w http.ResponseWriter, r *http.Request) { setCustomerActive(w, r, true) }
