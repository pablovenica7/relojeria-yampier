package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Libro de movimientos de stock (stock_movements).
//
// Regla: cada cambio real de stockQuantity genera un movimiento, con el stock
// anterior y posterior tomados de la MISMA operación atómica que cambió el
// stock (FindOneAndUpdate), no de una lectura aparte.
//
// Sin transacciones (Mongo standalone), el movimiento se inserta después de
// que el cambio quedó firme. Si el proceso se cae justo en el medio, puede
// faltar una línea del libro (se ve como un salto entre stockAfter y el
// siguiente stockBefore); nunca se registra un movimiento que no ocurrió.
// Las compensaciones internas (retener y devolver en la misma operación por
// un conflicto) dejan el stock igual y no generan movimientos.
//
// Idempotencia: los movimientos de reservas llevan una clave única
// (tipo:reservaId). Una reserva solo puede retenerse, liberarse o venderse
// una vez en su vida (las transiciones no permiten volver atrás), así que un
// reintento nunca duplica la línea.

const systemActor = "sistema"

func movementKey(moveType string, reservationID primitive.ObjectID) string {
	return moveType + ":" + reservationID.Hex()
}

// recordMovement inserta el movimiento. Un duplicado (misma clave de
// idempotencia) se ignora. Otros errores se registran en el log: el cambio de
// stock ya ocurrió y no se revierte por no poder anotarlo.
func recordMovement(ctx context.Context, m StockMovement) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	if m.CreatedBy == "" {
		m.CreatedBy = systemActor
	}
	_, err := stockMovementsCol().InsertOne(ctx, m)
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		log.Printf("ATENCIÓN: no se pudo registrar el movimiento de stock %s del reloj %s: %v", m.Type, m.WatchID.Hex(), err)
	}
}

// movementFromWatch arma un movimiento a partir del documento del reloj
// DESPUÉS del cambio (lo devuelve FindOneAndUpdate con ReturnDocument After).
func movementFromWatch(after *Watch, moveType string, delta int, reservationID *primitive.ObjectID, reason, actor string) StockMovement {
	m := StockMovement{
		WatchID: after.ID, WatchNameSnapshot: after.Name, Type: moveType,
		QuantityDelta: delta, StockBefore: after.StockQuantity - delta, StockAfter: after.StockQuantity,
		ReservationID: reservationID, Reason: reason, CreatedBy: actor,
	}
	if reservationID != nil && (moveType == MoveReservationHold || moveType == MoveReservationRelease || moveType == MoveSale) {
		m.IdempotencyKey = movementKey(moveType, *reservationID)
	}
	return m
}

// GET /api/admin/stock-movements?watchId=&type=&from=&to=&page=&limit=  (Admin)
func listStockMovements(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	filter := bson.M{}
	if s := v.Get("watchId"); s != "" {
		id, err := primitive.ObjectIDFromHex(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Reloj inválido")
			return
		}
		filter["watchId"] = id
	}
	if t := v.Get("type"); t != "" {
		switch t {
		case MoveStockEntry, MoveManualAdjustment, MoveReservationHold, MoveReservationRelease, MoveSale, MoveCorrection:
			filter["type"] = t
		default:
			writeError(w, http.StatusBadRequest, "Tipo de movimiento inválido")
			return
		}
	}
	created, msg := dateRangeFilter(v.Get("from"), v.Get("to"))
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if created != nil {
		filter["createdAt"] = created
	}
	page, limit, msg := pageParams(v.Get("page"), v.Get("limit"), 50)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	total, err := stockMovementsCol().CountDocuments(ctx, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	cur, err := stockMovementsCol().Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page-1)*limit)).SetLimit(int64(limit)))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer cur.Close(ctx)
	items := []StockMovement{}
	if err := cur.All(ctx, &items); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newPaged(items, page, limit, total))
}

// dateRangeFilter traduce from/to (AAAA-MM-DD, hora local; "to" incluye el
// día completo) a un filtro de Mongo. Devuelve nil si no hay fechas.
func dateRangeFilter(from, to string) (bson.M, string) {
	f := bson.M{}
	for _, p := range []struct{ val, op string }{{from, "$gte"}, {to, "$lt"}} {
		if p.val == "" {
			continue
		}
		d, err := time.ParseInLocation("2006-01-02", p.val, time.Local)
		if err != nil {
			return nil, "Fecha inválida (usá AAAA-MM-DD)"
		}
		if p.op == "$lt" {
			d = d.AddDate(0, 0, 1)
		}
		f[p.op] = d
	}
	if len(f) == 0 {
		return nil, ""
	}
	return f, ""
}

// pageParams valida page/limit de los listados del Admin.
func pageParams(pageStr, limitStr string, defLimit int) (int, int, string) {
	page, limit := 1, defLimit
	if pageStr != "" {
		n, err := strconv.Atoi(pageStr)
		if err != nil || n < 1 || n > 100_000 {
			return 0, 0, "Página inválida"
		}
		page = n
	}
	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 1 || n > maxPageLimit {
			return 0, 0, "Límite inválido (1 a 100)"
		}
		limit = n
	}
	return page, limit, ""
}
