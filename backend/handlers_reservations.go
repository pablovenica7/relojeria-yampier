package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GET /api/admin/reservations?status=&watchId=&from=YYYY-MM-DD&to=YYYY-MM-DD&page=&limit=  (protegido)
func listReservations(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	filter := bson.M{}
	if s := v.Get("status"); s != "" {
		if !validReservationStatus(s) {
			writeError(w, http.StatusBadRequest, "Estado inválido")
			return
		}
		filter["status"] = s
	}
	if s := v.Get("watchId"); s != "" {
		id, err := primitive.ObjectIDFromHex(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Reloj inválido")
			return
		}
		filter["watchId"] = id
	}
	created := bson.M{}
	for param, op := range map[string]string{"from": "$gte", "to": "$lt"} {
		s := v.Get(param)
		if s == "" {
			continue
		}
		d, err := time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Fecha inválida (usá AAAA-MM-DD)")
			return
		}
		if param == "to" {
			d = d.AddDate(0, 0, 1) // "hasta" incluye el día completo
		}
		created[op] = d
	}
	if len(created) > 0 {
		filter["createdAt"] = created
	}
	page, limit := 1, 50
	if s := v.Get("page"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "Página inválida")
			return
		}
		page = n
	}
	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxPageLimit {
			writeError(w, http.StatusBadRequest, "Límite inválido")
			return
		}
		limit = n
	}

	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	// Vencimiento "en el momento": el Admin siempre ve estados al día.
	if _, err := expireDueReservations(ctx, time.Now(), nil); err != nil {
		writeErr(w, r, err)
		return
	}
	total, err := reservationsCol().CountDocuments(ctx, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page - 1) * limit)).SetLimit(int64(limit))
	cur, err := reservationsCol().Find(ctx, filter, opts)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer cur.Close(ctx)
	items := []Reservation{}
	if err := cur.All(ctx, &items); err != nil {
		writeErr(w, r, err)
		return
	}
	out, err := withCustomers(ctx, items)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newPaged(out, page, limit, total))
}

// POST /api/admin/reservations  (protegido)
func postReservation(w http.ResponseWriter, r *http.Request) {
	var in ReservationInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos (montos enteros y fechas en formato ISO)")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	res, err := createReservation(ctx, in, adminEmail(r), time.Now())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

// PATCH /api/admin/reservations/{id}/status  (protegido) — body {"status", "depositAmount"?}
func patchReservationStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reserva no encontrada")
		return
	}
	var in struct {
		Status        string `json:"status"`
		DepositAmount *int64 `json:"depositAmount"`
		DepositMethod string `json:"depositMethod"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	res, err := transitionReservation(ctx, id, strings.TrimSpace(in.Status),
		transitionOpts{DepositAmount: in.DepositAmount, DepositMethod: strings.TrimSpace(in.DepositMethod), Actor: adminEmail(r)}, time.Now())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// PATCH /api/admin/reservations/{id}  (protegido) — notas, seña acordada y
// vencimiento. El estado se cambia solo por /status (con sus reglas de stock).
func patchReservation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reserva no encontrada")
		return
	}
	var in struct {
		Notes         *string    `json:"notes"`
		DepositAmount *int64     `json:"depositAmount"`
		ExpiresAt     *time.Time `json:"expiresAt"`
		ClearExpiry   bool       `json:"clearExpiry"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	now := time.Now()
	if _, err := expireDueReservations(ctx, now, &id); err != nil {
		writeErr(w, r, err)
		return
	}
	current, err := findReservation(ctx, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}

	set := bson.M{"updatedAt": now}
	unset := bson.M{}
	if in.Notes != nil {
		notes := strings.TrimSpace(*in.Notes)
		if len(notes) > reservationNotes {
			writeError(w, http.StatusBadRequest, "Las notas son demasiado largas (máximo 2000 caracteres)")
			return
		}
		set["notes"] = notes
	}
	open := current.Status == ReservationPending || current.Status == ReservationConfirmed || current.Status == ReservationDepositPaid
	if in.DepositAmount != nil {
		d := *in.DepositAmount
		if !open {
			writeError(w, http.StatusConflict, "La reserva está cerrada: solo se pueden editar las notas")
			return
		}
		if d < 0 || d > maxPriceARS || (current.PriceAtReservation > 0 && d > current.PriceAtReservation) {
			writeError(w, http.StatusBadRequest, "La seña es inválida o supera el precio")
			return
		}
		if current.Status == ReservationDepositPaid && d == 0 {
			writeError(w, http.StatusBadRequest, "Una reserva con seña paga debe tener un monto de seña")
			return
		}
		set["depositAmount"] = d
	}
	if in.ExpiresAt != nil || in.ClearExpiry {
		if !expirable(current.Status) {
			writeError(w, http.StatusConflict, "Solo las reservas pendientes o confirmadas tienen vencimiento")
			return
		}
		if in.ClearExpiry {
			unset["expiresAt"] = ""
		} else {
			if msg := validateExpiry(in.ExpiresAt, now); msg != "" {
				writeError(w, http.StatusBadRequest, msg)
				return
			}
			set["expiresAt"] = *in.ExpiresAt
		}
	}
	update := bson.M{"$set": set}
	if len(unset) > 0 {
		update["$unset"] = unset
	}
	// Condicional al estado leído: si cambió en paralelo, 409.
	var out Reservation
	err = reservationsCol().FindOneAndUpdate(ctx, bson.M{"_id": id, "status": current.Status}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if err != nil {
		writeError(w, http.StatusConflict, "La reserva fue modificada al mismo tiempo. Recargá y volvé a intentar.")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// adminReservation agrega a la reserva los datos completos del cliente
// (documento, condición fiscal, domicilio). SOLO se usa en endpoints Admin.
type adminReservation struct {
	Reservation
	Customer *Customer `json:"customer,omitempty"`
}

func withCustomers(ctx context.Context, items []Reservation) ([]adminReservation, error) {
	ids := bson.A{}
	for _, r := range items {
		if r.CustomerID != nil {
			ids = append(ids, *r.CustomerID)
		}
	}
	byID := map[primitive.ObjectID]*Customer{}
	if len(ids) > 0 {
		cur, err := customersCol().Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
		if err != nil {
			return nil, err
		}
		var custs []Customer
		if err := cur.All(ctx, &custs); err != nil {
			return nil, err
		}
		for i := range custs {
			byID[custs[i].ID] = &custs[i]
		}
	}
	out := make([]adminReservation, 0, len(items))
	for _, r := range items {
		ar := adminReservation{Reservation: r}
		if r.CustomerID != nil {
			ar.Customer = byID[*r.CustomerID]
		}
		out = append(out, ar)
	}
	return out, nil
}

// ---------- Endpoints del cliente (requireCustomer) ----------

// POST /api/reservations — solicita una reserva (queda pendiente).
func postCustomerReservation(w http.ResponseWriter, r *http.Request) {
	var in CustomerReservationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	res, err := createCustomerReservation(ctx, currentCustomer(r), in, time.Now())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, customerView(*res))
}

// GET /api/reservations — solo las reservas del cliente autenticado.
func getCustomerReservations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	items, err := listCustomerReservations(ctx, currentCustomer(r).ID, time.Now())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GET /api/reservations/{id} — 404 si no existe O si es de otro cliente
// (no se distingue, para no revelar reservas ajenas).
func getCustomerReservation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reserva no encontrada")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	var res Reservation
	err := reservationsCol().FindOne(ctx, bson.M{"_id": id, "customerId": currentCustomer(r).ID}).Decode(&res)
	if err != nil {
		writeError(w, http.StatusNotFound, "Reserva no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, customerView(res))
}

// POST /api/reservations/{id}/cancel — el cliente solo puede cancelar sus
// reservas PENDIENTES. Las confirmadas o con seña se cancelan hablando con el
// local (puede haber una seña de por medio).
func postCustomerCancel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reserva no encontrada")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	cid := currentCustomer(r).ID
	res, err := transitionReservation(ctx, id, ReservationCancelled, transitionOpts{
		CustomerID: &cid, AllowedFrom: []string{ReservationPending}, Actor: "cliente",
	}, time.Now())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, customerView(*res))
}
