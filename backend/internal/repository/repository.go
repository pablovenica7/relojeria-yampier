// Package repository contiene la persistencia en MongoDB. Cada repository
// recibe su colección por constructor y solo traduce entre entidades de
// dominio y documentos: no valida reglas de negocio ni conoce HTTP.
//
// Errores: "no encontrado" se devuelve como domain.ErrNotFound y una clave
// única repetida como domain.ErrDuplicate; el service decide qué significan.
//
// Los métodos reciben el context.Context del request (cancelación y
// deadlines llegan hasta Mongo); no se crea context.Background() acá.
package repository

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"relojeria-yampier/internal/domain"
)

// mapErr traduce errores del driver a errores de dominio.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return domain.ErrNotFound
	case mongo.IsDuplicateKeyError(err):
		return domain.ErrDuplicate
	}
	return err
}

// dateRange arma un filtro {$gte: from, $lt: to}; nil si no hay fechas.
func dateRange(from, to *time.Time) bson.M {
	f := bson.M{}
	if from != nil {
		f["$gte"] = *from
	}
	if to != nil {
		f["$lt"] = *to
	}
	if len(f) == 0 {
		return nil
	}
	return f
}

func toBsonA(values []string) bson.A {
	out := bson.A{}
	for _, v := range values {
		out = append(out, v)
	}
	return out
}
