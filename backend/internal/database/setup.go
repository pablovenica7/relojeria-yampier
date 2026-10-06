package database

import (
	"context"
	"log/slog"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"relojeria-yampier/internal/auth"
	"relojeria-yampier/internal/domain"
)

// Tareas de arranque. Todas son NO destructivas: crean lo que falta y nunca
// borran ni pisan datos existentes.

// EnsureIndexes crea los índices necesarios si no existen. No se indexan
// campos de baja selectividad (archivedAt, stockQuantity): con un catálogo
// del tamaño de una relojería no aportan y encarecen las escrituras.
func (d *DB) EnsureIndexes(ctx context.Context, log *slog.Logger) {
	create := func(coll, name string, model mongo.IndexModel) {
		if model.Options == nil {
			model.Options = options.Index()
		}
		model.Options.SetName(name)
		if _, err := d.Collection(coll).Indexes().CreateOne(ctx, model); err != nil {
			log.Warn("no se pudo asegurar un índice", "collection", coll, "index", name, "error", err)
		}
	}
	key := func(k string, v int) bson.D { return bson.D{{Key: k, Value: v}} }

	create(CollAdmins, "email_1", mongo.IndexModel{Keys: key("email", 1), Options: options.Index().SetUnique(true)})

	create(CollWatches, "gender_1", mongo.IndexModel{Keys: key("gender", 1)})
	create(CollWatches, "brandKey_1", mongo.IndexModel{Keys: key("brandKey", 1)})
	d.ensureUniqueModelIndex(ctx, log)
	// "brand_1" lo creó una versión anterior; quedó reemplazado por
	// brandKey_1. Borrar un índice no toca datos.
	if _, err := d.Collection(CollWatches).Indexes().DropOne(ctx, "brand_1"); err == nil {
		log.Info("índice obsoleto watches.brand_1 eliminado (reemplazado por brandKey_1)")
	}

	create(CollInquiries, "createdAt_-1", mongo.IndexModel{Keys: key("createdAt", -1)})
	create(CollInquiries, "status_1_createdAt_-1", mongo.IndexModel{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}})
	create(CollInquiries, "watchId_1", mongo.IndexModel{Keys: key("watchId", 1), Options: options.Index().SetSparse(true)})

	create(CollReservations, "createdAt_-1", mongo.IndexModel{Keys: key("createdAt", -1)})
	// Cubre el filtro por estado y el barrido de vencimientos.
	create(CollReservations, "status_1_expiresAt_1", mongo.IndexModel{Keys: bson.D{{Key: "status", Value: 1}, {Key: "expiresAt", Value: 1}}})
	create(CollReservations, "watchId_1_createdAt_-1", mongo.IndexModel{Keys: bson.D{{Key: "watchId", Value: 1}, {Key: "createdAt", Value: -1}}})
	create(CollReservations, "customerId_1_createdAt_-1", mongo.IndexModel{
		Keys: bson.D{{Key: "customerId", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetSparse(true),
	})

	create(CollCustomers, "email_1", mongo.IndexModel{Keys: key("email", 1), Options: options.Index().SetUnique(true)})

	create(CollStockMovements, "watchId_1_createdAt_-1", mongo.IndexModel{Keys: bson.D{{Key: "watchId", Value: 1}, {Key: "createdAt", Value: -1}}})
	create(CollStockMovements, "createdAt_-1", mongo.IndexModel{Keys: key("createdAt", -1)})
	create(CollStockMovements, "idempotencyKey_unique", mongo.IndexModel{
		Keys:    key("idempotencyKey", 1),
		Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"idempotencyKey": bson.M{"$type": "string"}}),
	})

	// password_resets: Mongo borra solos los pedidos un día después de vencidos
	// (credenciales temporales, no datos de negocio).
	create(CollPasswordResets, "tokenHash_unique", mongo.IndexModel{Keys: key("tokenHash", 1), Options: options.Index().SetUnique(true)})
	create(CollPasswordResets, "customerId_1_createdAt_-1", mongo.IndexModel{Keys: bson.D{{Key: "customerId", Value: 1}, {Key: "createdAt", Value: -1}}})
	create(CollPasswordResets, "expiresAt_ttl", mongo.IndexModel{Keys: key("expiresAt", 1), Options: options.Index().SetExpireAfterSeconds(86400)})

	log.Info("índices de MongoDB verificados")
}

// ensureUniqueModelIndex crea el índice único de modelo SOLO si no hay
// duplicados; si los hay, los informa y no borra nada.
func (d *DB) ensureUniqueModelIndex(ctx context.Context, log *slog.Logger) {
	cur, err := d.Collection(CollWatches).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"modelKey": bson.M{"$type": "string"}}}},
		{{Key: "$group", Value: bson.M{"_id": "$modelKey", "n": bson.M{"$sum": 1}}}},
		{{Key: "$match", Value: bson.M{"n": bson.M{"$gt": 1}}}},
	})
	if err != nil {
		log.Warn("no se pudo verificar modelos duplicados", "error", err)
		return
	}
	var dups []struct {
		ID string `bson:"_id"`
	}
	if err := cur.All(ctx, &dups); err != nil {
		log.Warn("no se pudo verificar modelos duplicados", "error", err)
		return
	}
	if len(dups) > 0 {
		log.Warn("ATENCIÓN: hay modelos duplicados; no se crea el índice único watches.modelKey hasta corregirlos (no se borró nada)", "duplicates", dups)
		return
	}
	_, err = d.Collection(CollWatches).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "modelKey", Value: 1}},
		// Parcial: relojes viejos sin modelo no tienen modelKey y no chocan.
		Options: options.Index().SetName("modelKey_unique").SetUnique(true).
			SetPartialFilterExpression(bson.M{"modelKey": bson.M{"$type": "string"}}),
	})
	if err != nil {
		log.Warn("no se pudo asegurar el índice único watches.modelKey", "error", err)
	}
}

// ---------- Backfill de documentos viejos ----------

// LegacyStockQuantity traduce el estado de stock viejo a cantidad. Criterio
// conservador: "en_stock" solo garantizaba al menos una unidad.
func LegacyStockQuantity(stock string) int {
	switch stock {
	case "ultima_unidad", "en_stock":
		return 1
	default:
		return 0
	}
}

// LegacyPriceARS convierte el precio float viejo a pesos enteros.
func LegacyPriceARS(price *float64) int64 {
	if price == nil || *price <= 0 || math.IsNaN(*price) || math.IsInf(*price, 0) {
		return 0
	}
	if *price > float64(domain.MaxPriceARS) {
		return domain.MaxPriceARS
	}
	return int64(math.Round(*price))
}

// BackfillLegacyData completa campos nuevos en documentos de versiones
// anteriores. ADITIVA e IDEMPOTENTE: solo agrega campos faltantes
// ($exists: false), no borra price/stock/read, y correrla N veces da igual.
func (d *DB) BackfillLegacyData(ctx context.Context, log *slog.Logger) {
	if err := d.backfillWatches(ctx, log); err != nil {
		log.Warn("no se pudieron completar campos de relojes viejos", "error", err)
	}
	res, err := d.Collection(CollInquiries).UpdateMany(ctx,
		bson.M{"status": bson.M{"$exists": false}},
		mongo.Pipeline{{{Key: "$set", Value: bson.M{
			"status": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$read", true}}, domain.InquiryContacted, domain.InquiryNew}},
		}}}},
	)
	if err != nil {
		log.Warn("no se pudo completar el estado de consultas viejas", "error", err)
	} else if res.ModifiedCount > 0 {
		log.Info("consultas viejas con estado completado", "count", res.ModifiedCount)
	}
}

func (d *DB) backfillWatches(ctx context.Context, log *slog.Logger) error {
	col := d.Collection(CollWatches)
	cur, err := col.Find(ctx, bson.M{"$or": bson.A{
		bson.M{"priceARS": bson.M{"$exists": false}},
		bson.M{"stockQuantity": bson.M{"$exists": false}},
		bson.M{"brandKey": bson.M{"$exists": false}},
		bson.M{"modelKey": bson.M{"$exists": false}, "model": bson.M{"$nin": bson.A{"", nil}}},
	}})
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	n := 0
	for cur.Next(ctx) {
		var w struct {
			ID            interface{} `bson:"_id"`
			Brand         string      `bson:"brand"`
			Model         string      `bson:"model"`
			Price         *float64    `bson:"price"`
			Stock         string      `bson:"stock"`
			PriceARS      *int64      `bson:"priceARS"`
			StockQuantity *int        `bson:"stockQuantity"`
			BrandKey      *string     `bson:"brandKey"`
			ModelKey      *string     `bson:"modelKey"`
		}
		if err := cur.Decode(&w); err != nil {
			return err
		}
		set, filter := bson.M{}, bson.M{"_id": w.ID}
		if w.PriceARS == nil {
			set["priceARS"] = LegacyPriceARS(w.Price)
			filter["priceARS"] = bson.M{"$exists": false}
		}
		if w.StockQuantity == nil {
			set["stockQuantity"] = LegacyStockQuantity(w.Stock)
			filter["stockQuantity"] = bson.M{"$exists": false}
		}
		if w.BrandKey == nil {
			set["brandKey"] = domain.BrandKeyFor(w.Brand)
			filter["brandKey"] = bson.M{"$exists": false}
		}
		if w.ModelKey == nil {
			if k := domain.ModelKeyFor(w.Model); k != "" {
				set["modelKey"] = k
				filter["modelKey"] = bson.M{"$exists": false}
			}
		}
		if len(set) == 0 {
			continue
		}
		set["updatedAt"] = time.Now()
		if _, err := col.UpdateOne(ctx, filter, bson.M{"$set": set}); err != nil {
			return err
		}
		n++
	}
	if n > 0 {
		log.Info("relojes viejos actualizados con campos nuevos", "count", n)
	}
	return cur.Err()
}

// ---------- Seeds ----------

// SeedAdmin crea el primer admin (ADMIN_EMAIL/ADMIN_PASSWORD) si no existe.
func (d *DB) SeedAdmin(ctx context.Context, email, password string, log *slog.Logger) {
	if email == "" || password == "" {
		return
	}
	col := d.Collection(CollAdmins)
	n, err := col.CountDocuments(ctx, bson.M{"email": email})
	if err != nil || n > 0 {
		if err != nil {
			log.Warn("no se pudo verificar el usuario admin", "error", err)
		}
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Warn("no se pudo generar el hash del admin", "error", err)
		return
	}
	if _, err := col.InsertOne(ctx, domain.AdminUser{Email: email, PasswordHash: hash, CreatedAt: time.Now()}); err != nil {
		log.Warn("no se pudo crear el admin inicial", "error", err)
		return
	}
	log.Info("usuario admin inicial creado", "email", email)
}

const demoSeedMarker = "seed_demo_casio_v1"

// SeedDemoWatches carga los relojes Casio de demostración UNA sola vez por
// base (deja una marca en app_meta: aunque se archiven o borren, reiniciar no
// los vuelve a insertar). Nunca borra ni modifica documentos existentes. La
// configuración ya garantiza que no corre en producción.
func (d *DB) SeedDemoWatches(ctx context.Context, log *slog.Logger) {
	meta := d.Collection(CollMeta)
	// Reclamar la marca primero (insert con _id fijo = atómico): si dos
	// instancias arrancan a la vez, solo una carga los datos.
	_, err := meta.InsertOne(ctx, bson.M{"_id": demoSeedMarker, "createdAt": time.Now()})
	if mongo.IsDuplicateKeyError(err) {
		return
	}
	if err != nil {
		log.Warn("no se pudo registrar la carga demo", "error", err)
		return
	}
	watches := d.Collection(CollWatches)
	n, err := watches.CountDocuments(ctx, bson.M{"brandKey": "casio"})
	if err != nil {
		log.Warn("no se pudo verificar el catálogo", "error", err)
		return
	}
	if n > 0 {
		log.Info("ya hay relojes Casio en el catálogo: se omite la carga demo")
		return
	}
	sample := DemoCasioWatches(time.Now())
	docs := make([]interface{}, len(sample))
	for i := range sample {
		docs[i] = sample[i]
	}
	if _, err := watches.InsertMany(ctx, docs); err != nil {
		log.Warn("no se pudo cargar el catálogo de ejemplo", "error", err)
		_, _ = meta.DeleteOne(ctx, bson.M{"_id": demoSeedMarker}) // reintentar en el próximo arranque
		return
	}
	log.Info("catálogo Casio de ejemplo cargado", "count", len(docs))
}
