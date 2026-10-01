package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const dbTimeout = 8 * time.Second

func parseObjectID(r *http.Request, name string) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(r.PathValue(name))
	return id, err == nil
}

// listWatches ejecuta un catalogQuery y devuelve la página pedida.
func listWatches(ctx context.Context, q catalogQuery) (paged[Watch], error) {
	filter := q.filter()
	total, err := watchesCol().CountDocuments(ctx, filter)
	if err != nil {
		return paged[Watch]{}, err
	}
	cur, err := watchesCol().Find(ctx, filter, q.findOptions())
	if err != nil {
		return paged[Watch]{}, err
	}
	defer cur.Close(ctx)
	items := []Watch{}
	if err := cur.All(ctx, &items); err != nil {
		return paged[Watch]{}, err
	}
	for i := range items {
		normalizeWatch(&items[i])
	}
	return newPaged(items, q.Page, q.Limit, total), nil
}

// GET /api/watches?search=&brand=&gender=&availability=&sort=&page=&limit=  (público)
// Solo devuelve relojes activos (no archivados).
func getWatches(w http.ResponseWriter, r *http.Request) {
	q, msg := parseCatalogQuery(r.URL.Query(), false)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	out, err := listWatches(ctx, q)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func findWatch(ctx context.Context, filter bson.M) (*Watch, error) {
	var watch Watch
	if err := watchesCol().FindOne(ctx, filter).Decode(&watch); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errCode(http.StatusNotFound, CodeWatchNotFound, "Reloj no encontrado")
		}
		return nil, err
	}
	normalizeWatch(&watch)
	return &watch, nil
}

// GET /api/watches/{id}  (público) — un reloj archivado responde 404.
func getWatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reloj no encontrado")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	watch, err := findWatch(ctx, bson.M{"_id": id, "archivedAt": nil})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

type watchStats struct {
	Total      int64 `json:"total"`
	OutOfStock int64 `json:"outOfStock"`
	LastUnit   int64 `json:"lastUnit"`
}

// GET /api/admin/watches  (protegido) — igual que el público pero permite
// status=active|archived|all y agrega contadores del alcance elegido.
func adminListWatches(w http.ResponseWriter, r *http.Request) {
	q, msg := parseCatalogQuery(r.URL.Query(), true)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	out, err := listWatches(ctx, q)
	if err != nil {
		writeErr(w, r, err)
		return
	}

	var stats watchStats
	scope := q.scopeFilter()
	counts := []struct {
		dst   *int64
		extra bson.M
	}{
		{&stats.Total, nil},
		{&stats.OutOfStock, availabilityFilter(AvailabilityOutOfStock)},
		{&stats.LastUnit, availabilityFilter(AvailabilityLastUnit)},
	}
	for _, c := range counts {
		f := bson.M{}
		for k, v := range scope {
			f[k] = v
		}
		if c.extra != nil {
			f["stockQuantity"] = c.extra
		}
		if *c.dst, err = watchesCol().CountDocuments(ctx, f); err != nil {
			writeErr(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, struct {
		paged[Watch]
		Stats watchStats `json:"stats"`
	}{out, stats})
}

// GET /api/admin/watches/{id}  (protegido) — incluye archivados.
func adminGetWatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reloj no encontrado")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	watch, err := findWatch(ctx, bson.M{"_id": id})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

// ensureModelAvailable devuelve 409 si otro reloj ya usa ese modelo. El
// índice único sobre modelKey es la garantía real ante altas simultáneas;
// esta verificación previa solo da un mensaje más claro.
func ensureModelAvailable(ctx context.Context, modelKey string, exclude *primitive.ObjectID) error {
	filter := bson.M{"modelKey": modelKey}
	if exclude != nil {
		filter["_id"] = bson.M{"$ne": *exclude}
	}
	n, err := watchesCol().CountDocuments(ctx, filter)
	if err != nil {
		return err
	}
	if n > 0 {
		return errCode(http.StatusConflict, CodeDuplicateModel, "Ya existe un reloj con ese modelo")
	}
	return nil
}

func decodeWatchInput(w http.ResponseWriter, r *http.Request) (WatchInput, bool) {
	var in WatchInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos (revisá que precio y stock sean números enteros)")
		return in, false
	}
	if msg := validateWatchInput(&in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return in, false
	}
	return in, true
}

func duplicateOr(err error) error {
	if mongo.IsDuplicateKeyError(err) {
		return errCode(http.StatusConflict, CodeDuplicateModel, "Ya existe un reloj con ese modelo")
	}
	return err
}

// POST /api/admin/watches  (protegido)
func createWatch(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeWatchInput(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()

	key := modelKeyFor(in.Model)
	if err := ensureModelAvailable(ctx, key, nil); err != nil {
		writeErr(w, r, err)
		return
	}
	now := time.Now()
	watch := Watch{
		Brand: in.Brand, BrandKey: brandKeyFor(in.Brand), Name: in.Name,
		Model: in.Model, ModelKey: key, Gender: in.Gender,
		Description: in.Description, Image: in.Image, Specs: in.Specs,
		PriceARS: in.PriceARS, StockQuantity: *in.StockQuantity,
		CreatedAt: now, UpdatedAt: now,
	}
	res, err := watchesCol().InsertOne(ctx, watch)
	if err != nil {
		writeErr(w, r, duplicateOr(err))
		return
	}
	watch.ID = res.InsertedID.(primitive.ObjectID)
	if watch.StockQuantity > 0 {
		recordMovement(ctx, movementFromWatch(&watch, MoveStockEntry, watch.StockQuantity, nil, "Alta del reloj", adminEmail(r)))
	}
	normalizeWatch(&watch)
	writeJSON(w, http.StatusCreated, watch)
}

// PUT /api/admin/watches/{id}  (protegido)
func updateWatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reloj no encontrado")
		return
	}
	in, ok := decodeWatchInput(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()

	key := modelKeyFor(in.Model)
	if err := ensureModelAvailable(ctx, key, &id); err != nil {
		writeErr(w, r, err)
		return
	}

	set := bson.M{
		"brand": in.Brand, "brandKey": brandKeyFor(in.Brand), "name": in.Name,
		"model": in.Model, "modelKey": key, "gender": in.Gender,
		"description": in.Description, "image": in.Image, "specs": in.Specs,
		"priceARS": in.PriceARS, "updatedAt": time.Now(),
	}

	// 1) Stock: operación atómica propia, para conocer el valor anterior y
	//    anotar el movimiento. Si base == nueva cantidad, no se toca (así un
	//    guardado del formulario no deshace una reserva confirmada mientras se
	//    editaba). Con base, solo se pisa si nadie lo cambió mientras tanto.
	if in.StockQuantityBase == nil || *in.StockQuantityBase != *in.StockQuantity {
		filter := bson.M{"_id": id}
		if in.StockQuantityBase != nil {
			filter["stockQuantity"] = *in.StockQuantityBase
		}
		var before Watch
		err := watchesCol().FindOneAndUpdate(ctx, filter,
			bson.M{"$set": bson.M{"stockQuantity": *in.StockQuantity, "updatedAt": time.Now()}},
			options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&before)
		if errors.Is(err, mongo.ErrNoDocuments) {
			if _, ferr := findWatch(ctx, bson.M{"_id": id}); ferr == nil {
				writeAPIError(w, &apiError{Status: http.StatusConflict, Code: CodeConcurrentUpdate,
					Message: "El stock cambió mientras editabas (por ejemplo, por una reserva). Recargá el reloj y volvé a intentar."})
				return
			}
			writeAPIError(w, &apiError{Status: http.StatusNotFound, Code: CodeWatchNotFound, Message: "Reloj no encontrado"})
			return
		}
		if err != nil {
			writeErr(w, r, err)
			return
		}
		if delta := *in.StockQuantity - before.StockQuantity; delta != 0 {
			after := before
			after.StockQuantity = *in.StockQuantity
			recordMovement(ctx, movementFromWatch(&after, in.StockMoveType, delta, nil, in.StockMoveReason, adminEmail(r)))
		}
	}

	// 2) Resto de los datos.
	res, err := watchesCol().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	if err != nil {
		writeErr(w, r, duplicateOr(err))
		return
	}
	if res.MatchedCount == 0 {
		writeAPIError(w, &apiError{Status: http.StatusNotFound, Code: CodeWatchNotFound, Message: "Reloj no encontrado"})
		return
	}
	watch, err := findWatch(ctx, bson.M{"_id": id})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

// adjustStock suma delta al stock de forma atómica, sin permitir negativos.
// El filtro {stockQuantity: {$gte: -delta}} y el $inc se evalúan en una sola
// operación de Mongo: no hay ventana entre "leer" y "escribir".
func adjustStock(ctx context.Context, id primitive.ObjectID, delta int, moveType, reason, actor string) (*Watch, error) {
	filter := bson.M{"_id": id}
	if delta < 0 {
		filter["stockQuantity"] = bson.M{"$gte": -delta}
	}
	var watch Watch
	err := watchesCol().FindOneAndUpdate(ctx, filter,
		bson.M{"$inc": bson.M{"stockQuantity": delta}, "$set": bson.M{"updatedAt": time.Now()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&watch)
	if errors.Is(err, mongo.ErrNoDocuments) {
		if _, ferr := findWatch(ctx, bson.M{"_id": id}); ferr != nil {
			return nil, ferr
		}
		return nil, errConflict("El stock no puede quedar negativo")
	}
	if err != nil {
		return nil, err
	}
	recordMovement(ctx, movementFromWatch(&watch, moveType, delta, nil, reason, actor))
	normalizeWatch(&watch)
	return &watch, nil
}

// PATCH /api/admin/watches/{id}/stock  (protegido) — body {"delta": 1 | -1 | n}
func patchWatchStock(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reloj no encontrado")
		return
	}
	var in struct {
		Delta  int    `json:"delta"`
		Type   string `json:"type"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil || in.Delta == 0 || in.Delta > maxStockQuantity || in.Delta < -maxStockQuantity {
		writeError(w, http.StatusBadRequest, "Indicá un ajuste de stock entero distinto de cero")
		return
	}
	if in.Type == "" {
		in.Type = MoveManualAdjustment
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validManualMoveType(in.Type) || len(in.Reason) > 300 {
		writeError(w, http.StatusBadRequest, "Tipo de movimiento o motivo inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	watch, err := adjustStock(ctx, id, in.Delta, in.Type, in.Reason, adminEmail(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

func setArchived(w http.ResponseWriter, r *http.Request, archived bool) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reloj no encontrado")
		return
	}
	// Archivar un reloj con reservas confirmadas o con seña no se bloquea
	// (el negocio puede necesitarlo), pero exige confirmación explícita:
	// sin ?confirm=true responde 409 ACTIVE_RESERVATIONS con la cantidad.
	if archived && r.URL.Query().Get("confirm") != "true" {
		n, err := countActiveReservations(r.Context(), id)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		if n > 0 {
			writeAPIError(w, &apiError{Status: http.StatusConflict, Code: CodeActiveReservations,
				Message: "Este reloj tiene reservas activas. Confirmá si querés archivarlo igualmente.",
				Extra:   map[string]any{"activeReservations": n}})
			return
		}
	}
	now := time.Now()
	update := bson.M{"$unset": bson.M{"archivedAt": ""}, "$set": bson.M{"updatedAt": now}}
	if archived {
		update = bson.M{"$set": bson.M{"archivedAt": now, "updatedAt": now}}
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	res, err := watchesCol().UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if res.MatchedCount == 0 {
		writeError(w, http.StatusNotFound, "Reloj no encontrado")
		return
	}
	watch, err := findWatch(ctx, bson.M{"_id": id})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, watch)
}

// PATCH /api/admin/watches/{id}/archive y /restore  (protegido)
func archiveWatch(w http.ResponseWriter, r *http.Request) { setArchived(w, r, true) }
func restoreWatch(w http.ResponseWriter, r *http.Request) { setArchived(w, r, false) }

// DELETE /api/admin/watches/{id}  (protegido) — borrado físico EXCEPCIONAL.
// El flujo normal es archivar. Solo se permite borrar un reloj ya archivado
// y sin reservas asociadas (las consultas conservan su propio snapshot).
func deleteWatch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Reloj no encontrado")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()

	watch, err := findWatch(ctx, bson.M{"_id": id})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if watch.ArchivedAt == nil {
		writeError(w, http.StatusConflict, "Archivá el reloj antes de eliminarlo definitivamente")
		return
	}
	n, err := reservationsCol().CountDocuments(ctx, bson.M{"watchId": id})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if n > 0 {
		writeError(w, http.StatusConflict, "El reloj tiene reservas asociadas: se mantiene archivado")
		return
	}
	res, err := watchesCol().DeleteOne(ctx, bson.M{"_id": id, "archivedAt": bson.M{"$ne": nil}})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if res.DeletedCount == 0 {
		writeError(w, http.StatusNotFound, "Reloj no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// countActiveReservations cuenta reservas que retienen una unidad del reloj.
func countActiveReservations(ctx context.Context, watchID primitive.ObjectID) (int64, error) {
	c, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	return reservationsCol().CountDocuments(c, bson.M{"watchId": watchID,
		"status": bson.M{"$in": bson.A{ReservationConfirmed, ReservationDepositPaid}}})
}
