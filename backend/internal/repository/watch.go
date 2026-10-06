package repository

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"relojeria-yampier/internal/domain"
)

// WatchRepository persiste el catálogo. Las operaciones de stock son
// atómicas: la condición y el cambio se evalúan en una sola operación de
// Mongo (nunca "leer, restar en Go, guardar").
type WatchRepository struct{ col *mongo.Collection }

func NewWatchRepository(col *mongo.Collection) *WatchRepository { return &WatchRepository{col: col} }

var catalogSorts = map[string]bson.D{
	domain.SortNewest:    {{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}},
	domain.SortNameAsc:   {{Key: "name", Value: 1}, {Key: "_id", Value: 1}},
	domain.SortNameDesc:  {{Key: "name", Value: -1}, {Key: "_id", Value: 1}},
	domain.SortPriceAsc:  {{Key: "priceARS", Value: 1}, {Key: "_id", Value: 1}},
	domain.SortPriceDesc: {{Key: "priceARS", Value: -1}, {Key: "_id", Value: 1}},
}

func scopeFilter(scope string) bson.M {
	switch scope {
	case domain.ScopeArchived:
		return bson.M{"archivedAt": bson.M{"$ne": nil}}
	case domain.ScopeAll:
		return bson.M{}
	default:
		// {archivedAt: null} también encuentra documentos viejos sin el campo.
		return bson.M{"archivedAt": nil}
	}
}

func availabilityFilter(a string) bson.M {
	switch a {
	case domain.AvailabilityInStock:
		return bson.M{"$gte": 2}
	case domain.AvailabilityLastUnit:
		return bson.M{"$eq": 1}
	case domain.AvailabilityOutOfStock:
		return bson.M{"$lte": 0}
	case domain.AvailabilityAvailable:
		return bson.M{"$gte": 1}
	}
	return nil
}

func catalogFilter(f domain.CatalogFilter) bson.M {
	q := scopeFilter(f.Scope)
	if f.BrandKey != "" {
		q["brandKey"] = f.BrandKey
	}
	if f.Gender != "" {
		q["gender"] = f.Gender
	}
	if af := availabilityFilter(f.Availability); af != nil {
		q["stockQuantity"] = af
	}
	if f.Search != "" {
		// Nombre/marca: texto literal sin distinguir mayúsculas. Modelo: se
		// compara la clave normalizada ("ga2100" encuentra "GA-2100-1A1").
		// QuoteMeta: el texto del usuario nunca se interpreta como regex.
		or := bson.A{
			bson.M{"name": bson.M{"$regex": regexp.QuoteMeta(f.Search), "$options": "i"}},
			bson.M{"brand": bson.M{"$regex": regexp.QuoteMeta(f.Search), "$options": "i"}},
		}
		if key := domain.ModelKeyFor(f.Search); key != "" {
			or = append(or, bson.M{"modelKey": bson.M{"$regex": regexp.QuoteMeta(key)}})
		}
		q["$or"] = or
	}
	return q
}

// List devuelve una página del catálogo y el total.
func (r *WatchRepository) List(ctx context.Context, f domain.CatalogFilter) ([]domain.Watch, int64, error) {
	filter := catalogFilter(f)
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().
		SetSort(catalogSorts[f.Sort]).
		SetSkip(int64((f.Page - 1) * f.Limit)).
		SetLimit(int64(f.Limit))
	if strings.HasPrefix(f.Sort, "name_") {
		// Collation en español solo al ordenar por nombre: una collation
		// distinta a la del índice impediría usarlo en las igualdades.
		opts.SetCollation(&options.Collation{Locale: "es", Strength: 1})
	}
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	items := []domain.Watch{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Count cuenta relojes de un alcance con una disponibilidad (vacía = todas).
func (r *WatchRepository) Count(ctx context.Context, scope, availability string) (int64, error) {
	f := scopeFilter(scope)
	if af := availabilityFilter(availability); af != nil {
		f["stockQuantity"] = af
	}
	return r.col.CountDocuments(ctx, f)
}

// FindByID busca un reloj; si activeOnly, uno archivado cuenta como inexistente.
func (r *WatchRepository) FindByID(ctx context.Context, id primitive.ObjectID, activeOnly bool) (*domain.Watch, error) {
	f := bson.M{"_id": id}
	if activeOnly {
		f["archivedAt"] = nil
	}
	var w domain.Watch
	if err := r.col.FindOne(ctx, f).Decode(&w); err != nil {
		return nil, mapErr(err)
	}
	return &w, nil
}

// CountByModelKey cuenta relojes con ese modelo (excluyendo uno, si se indica).
func (r *WatchRepository) CountByModelKey(ctx context.Context, key string, exclude *primitive.ObjectID) (int64, error) {
	f := bson.M{"modelKey": key}
	if exclude != nil {
		f["_id"] = bson.M{"$ne": *exclude}
	}
	return r.col.CountDocuments(ctx, f)
}

// Insert crea un reloj y completa su ID.
func (r *WatchRepository) Insert(ctx context.Context, w *domain.Watch) error {
	res, err := r.col.InsertOne(ctx, w)
	if err != nil {
		return mapErr(err)
	}
	w.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

// UpdateDetails actualiza los datos editables (no el stock ni el archivado).
func (r *WatchRepository) UpdateDetails(ctx context.Context, id primitive.ObjectID, w *domain.Watch) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
		"brand": w.Brand, "brandKey": w.BrandKey, "name": w.Name,
		"model": w.Model, "modelKey": w.ModelKey, "gender": w.Gender,
		"description": w.Description, "image": w.Image, "specs": w.Specs,
		"priceARS": w.PriceARS, "updatedAt": w.UpdatedAt,
	}})
	if err != nil {
		return mapErr(err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetStock fija la cantidad de forma atómica y devuelve el reloj ANTES del
// cambio. Con base != nil, solo se aplica si el stock sigue siendo base
// (control de concurrencia optimista); si no, devuelve domain.ErrNotFound.
func (r *WatchRepository) SetStock(ctx context.Context, id primitive.ObjectID, base *int, qty int, at time.Time) (*domain.Watch, error) {
	f := bson.M{"_id": id}
	if base != nil {
		f["stockQuantity"] = *base
	}
	var before domain.Watch
	err := r.col.FindOneAndUpdate(ctx, f,
		bson.M{"$set": bson.M{"stockQuantity": qty, "updatedAt": at}},
		options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&before)
	if err != nil {
		return nil, mapErr(err)
	}
	return &before, nil
}

// AddStock suma delta de forma atómica y devuelve el reloj DESPUÉS del
// cambio. Con delta negativo, la condición {stockQuantity: {$gte: -delta}}
// impide quedar en negativo; con requireActive, solo relojes no archivados.
// Si la condición no se cumple (o el reloj no existe) devuelve ErrNotFound.
func (r *WatchRepository) AddStock(ctx context.Context, id primitive.ObjectID, delta int, requireActive bool, at time.Time) (*domain.Watch, error) {
	f := bson.M{"_id": id}
	if delta < 0 {
		f["stockQuantity"] = bson.M{"$gte": -delta}
	}
	if requireActive {
		f["archivedAt"] = nil
	}
	var after domain.Watch
	err := r.col.FindOneAndUpdate(ctx, f,
		bson.M{"$inc": bson.M{"stockQuantity": delta}, "$set": bson.M{"updatedAt": at}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&after)
	if err != nil {
		return nil, mapErr(err)
	}
	return &after, nil
}

// SetArchived archiva (at != nil) o restaura (at == nil) un reloj.
func (r *WatchRepository) SetArchived(ctx context.Context, id primitive.ObjectID, archivedAt *time.Time, now time.Time) error {
	update := bson.M{"$unset": bson.M{"archivedAt": ""}, "$set": bson.M{"updatedAt": now}}
	if archivedAt != nil {
		update = bson.M{"$set": bson.M{"archivedAt": *archivedAt, "updatedAt": now}}
	}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteArchived borra físicamente un reloj, solo si está archivado.
func (r *WatchRepository) DeleteArchived(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id, "archivedAt": bson.M{"$ne": nil}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
