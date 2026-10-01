package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ensureIndexes crea los índices necesarios si no existen. CreateOne/CreateMany
// no son destructivos: si el índice ya existe con la misma definición, Mongo
// no hace nada; si existiera con una definición distinta, devuelve error (no
// lo borra ni lo reemplaza solo).
//
// No se indexan campos de baja selectividad (archivedAt, stockQuantity): con
// un catálogo del tamaño de una relojería, Mongo los filtra igual de rápido
// sin índice, y cada índice extra encarece las escrituras.
func ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	create := func(col *mongo.Collection, name string, model mongo.IndexModel) {
		model.Options = optionsOrNew(model.Options).SetName(name)
		if _, err := col.Indexes().CreateOne(ctx, model); err != nil {
			log.Printf("No se pudo asegurar el índice %s.%s: %v", col.Name(), name, err)
		}
	}

	create(adminsCol(), "email_1", mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true),
	})

	// watches
	create(watchesCol(), "gender_1", mongo.IndexModel{Keys: bson.D{{Key: "gender", Value: 1}}})
	create(watchesCol(), "brandKey_1", mongo.IndexModel{Keys: bson.D{{Key: "brandKey", Value: 1}}})
	ensureUniqueModelIndex(ctx)
	// "brand_1" lo creó una versión anterior para el filtro por regex; quedó
	// reemplazado por brandKey_1. Borrar un índice no toca datos.
	if _, err := watchesCol().Indexes().DropOne(ctx, "brand_1"); err == nil {
		log.Println("Índice obsoleto watches.brand_1 eliminado (reemplazado por brandKey_1)")
	}

	// inquiries
	create(inquiriesCol(), "createdAt_-1", mongo.IndexModel{Keys: bson.D{{Key: "createdAt", Value: -1}}})
	create(inquiriesCol(), "status_1_createdAt_-1", mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}},
	})
	create(inquiriesCol(), "watchId_1", mongo.IndexModel{
		Keys: bson.D{{Key: "watchId", Value: 1}}, Options: options.Index().SetSparse(true),
	})

	// reservations
	create(reservationsCol(), "createdAt_-1", mongo.IndexModel{Keys: bson.D{{Key: "createdAt", Value: -1}}})
	// Cubre el filtro por estado y el barrido de vencimientos (status + expiresAt).
	create(reservationsCol(), "status_1_expiresAt_1", mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "expiresAt", Value: 1}},
	})
	create(reservationsCol(), "watchId_1_createdAt_-1", mongo.IndexModel{
		Keys: bson.D{{Key: "watchId", Value: 1}, {Key: "createdAt", Value: -1}},
	})

	create(reservationsCol(), "customerId_1_createdAt_-1", mongo.IndexModel{
		Keys: bson.D{{Key: "customerId", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetSparse(true),
	})

	// customers: email único (ya normalizado a minúsculas al guardar).
	create(customersCol(), "email_1", mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true),
	})

	// stock_movements: historial por reloj y global; clave de idempotencia
	// única solo donde existe (movimientos de reservas).
	create(stockMovementsCol(), "watchId_1_createdAt_-1", mongo.IndexModel{
		Keys: bson.D{{Key: "watchId", Value: 1}, {Key: "createdAt", Value: -1}},
	})
	create(stockMovementsCol(), "createdAt_-1", mongo.IndexModel{Keys: bson.D{{Key: "createdAt", Value: -1}}})
	create(stockMovementsCol(), "idempotencyKey_unique", mongo.IndexModel{
		Keys: bson.D{{Key: "idempotencyKey", Value: 1}},
		Options: options.Index().SetUnique(true).
			SetPartialFilterExpression(bson.M{"idempotencyKey": bson.M{"$type": "string"}}),
	})

	// password_resets: búsqueda por hash; Mongo borra solo los pedidos un día
	// después de vencidos (son credenciales temporales, no datos de negocio).
	create(passwordResetsCol(), "tokenHash_unique", mongo.IndexModel{
		Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetUnique(true),
	})
	create(passwordResetsCol(), "customerId_1_createdAt_-1", mongo.IndexModel{
		Keys: bson.D{{Key: "customerId", Value: 1}, {Key: "createdAt", Value: -1}},
	})
	create(passwordResetsCol(), "expiresAt_ttl", mongo.IndexModel{
		Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(86400),
	})

	log.Println("Índices de MongoDB verificados")
}

func optionsOrNew(o *options.IndexOptions) *options.IndexOptions {
	if o == nil {
		return options.Index()
	}
	return o
}

// ensureUniqueModelIndex crea el índice único de modelo SOLO si no hay
// duplicados. Si los hay, no borra ni modifica nada: los informa en el log
// para corregirlos desde el Admin, y la app sigue arrancando (la validación
// previa al guardar evita nuevos duplicados igual).
func ensureUniqueModelIndex(ctx context.Context) {
	dups, err := findDuplicateModelKeys(ctx)
	if err != nil {
		log.Println("No se pudo verificar modelos duplicados:", err)
		return
	}
	if len(dups) > 0 {
		log.Printf("ATENCIÓN: hay modelos duplicados en el catálogo %v. No se crea el índice único "+
			"watches.modelKey hasta corregirlos desde el Admin (no se borró nada).", dups)
		return
	}
	_, err = watchesCol().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "modelKey", Value: 1}},
		// Parcial: los relojes viejos sin modelo no tienen modelKey y no chocan entre sí.
		Options: options.Index().SetName("modelKey_unique").SetUnique(true).
			SetPartialFilterExpression(bson.M{"modelKey": bson.M{"$type": "string"}}),
	})
	if err != nil {
		log.Println("No se pudo asegurar el índice único watches.modelKey:", err)
	}
}

func findDuplicateModelKeys(ctx context.Context) ([]string, error) {
	cur, err := watchesCol().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"modelKey": bson.M{"$type": "string"}}}},
		{{Key: "$group", Value: bson.M{"_id": "$modelKey", "n": bson.M{"$sum": 1}}}},
		{{Key: "$match", Value: bson.M{"n": bson.M{"$gt": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID string `bson:"_id"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out, nil
}
