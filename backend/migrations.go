package main

import (
	"context"
	"log"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// backfillLegacyData completa campos nuevos en documentos creados con
// versiones anteriores. Es ADITIVA e IDEMPOTENTE:
//   - solo agrega campos que faltan ($exists: false), nunca pisa valores;
//   - no borra campos viejos (price, stock, read quedan como estaban);
//   - correrla N veces da el mismo resultado.
//
// Reglas para relojes viejos:
//   - priceARS     <- price (float) redondeado a pesos enteros.
//   - stockQuantity <- stock: "sin_stock" -> 0, "ultima_unidad" -> 1,
//     "en_stock" -> 1, sin dato -> 0. Criterio conservador: el estado viejo
//     solo garantizaba "al menos una unidad", así que no se inventan unidades
//     que podrían reservarse sin existir. El Admin debe cargar la cantidad real.
//   - brandKey / modelKey <- derivados de brand / model.
//
// Consultas viejas: status <- "contacted" si read == true, si no "new".
func backfillLegacyData(ctx context.Context) {
	if err := backfillWatches(ctx); err != nil {
		log.Println("No se pudieron completar campos de relojes viejos:", err)
	}
	res, err := inquiriesCol().UpdateMany(ctx,
		bson.M{"status": bson.M{"$exists": false}},
		mongo.Pipeline{{{Key: "$set", Value: bson.M{
			"status": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$read", true}}, InquiryContacted, InquiryNew}},
		}}}},
	)
	if err != nil {
		log.Println("No se pudo completar el estado de consultas viejas:", err)
	} else if res.ModifiedCount > 0 {
		log.Printf("Consultas viejas con estado completado: %d", res.ModifiedCount)
	}
}

type legacyWatch struct {
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

// legacyStockQuantity traduce el estado de stock viejo a una cantidad.
func legacyStockQuantity(stock string) int {
	switch stock {
	case "ultima_unidad", "en_stock":
		return 1
	default:
		return 0
	}
}

func legacyPriceARS(price *float64) int64 {
	if price == nil || *price <= 0 || math.IsNaN(*price) || math.IsInf(*price, 0) {
		return 0
	}
	if *price > float64(maxPriceARS) {
		return maxPriceARS
	}
	return int64(math.Round(*price))
}

func backfillWatches(ctx context.Context) error {
	cur, err := watchesCol().Find(ctx, bson.M{"$or": bson.A{
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
		var w legacyWatch
		if err := cur.Decode(&w); err != nil {
			return err
		}
		set := bson.M{}
		// Cada campo se agrega solo si sigue faltando (condición en el filtro).
		filter := bson.M{"_id": w.ID}
		if w.PriceARS == nil {
			set["priceARS"] = legacyPriceARS(w.Price)
			filter["priceARS"] = bson.M{"$exists": false}
		}
		if w.StockQuantity == nil {
			set["stockQuantity"] = legacyStockQuantity(w.Stock)
			filter["stockQuantity"] = bson.M{"$exists": false}
		}
		if w.BrandKey == nil {
			set["brandKey"] = brandKeyFor(w.Brand)
			filter["brandKey"] = bson.M{"$exists": false}
		}
		if w.ModelKey == nil {
			if key := modelKeyFor(w.Model); key != "" {
				set["modelKey"] = key
				filter["modelKey"] = bson.M{"$exists": false}
			}
		}
		if len(set) == 0 {
			continue
		}
		set["updatedAt"] = time.Now()
		if _, err := watchesCol().UpdateOne(ctx, filter, bson.M{"$set": set}); err != nil {
			return err
		}
		n++
	}
	if n > 0 {
		log.Printf("Relojes viejos actualizados con campos nuevos (priceARS/stockQuantity/brandKey/modelKey): %d", n)
	}
	return cur.Err()
}
