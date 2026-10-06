package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"relojeria-yampier/internal/domain"
)

// InquiryRepository persiste consultas.
type InquiryRepository struct{ col *mongo.Collection }

func NewInquiryRepository(col *mongo.Collection) *InquiryRepository {
	return &InquiryRepository{col: col}
}

// Insert guarda una consulta.
func (r *InquiryRepository) Insert(ctx context.Context, q *domain.Inquiry) error {
	res, err := r.col.InsertOne(ctx, q)
	if err != nil {
		return mapErr(err)
	}
	q.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

// List devuelve consultas (más recientes primero), filtradas por estado y reloj.
func (r *InquiryRepository) List(ctx context.Context, status string, watchID *primitive.ObjectID, limit int) ([]domain.Inquiry, error) {
	f := bson.M{}
	if status != "" {
		f["status"] = status
	}
	if watchID != nil {
		f["watchId"] = *watchID
	}
	cur, err := r.col.Find(ctx, f, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := []domain.Inquiry{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update aplica estado y/o notas. contactedAt / closedAt se fijan solo la
// primera vez que la consulta llega a ese estado ($ifNull en un pipeline).
// En un pipeline, un string que empieza con "$" se interpretaría como campo
// (ej: una nota "$100 de seña"): todo valor del usuario va en $literal.
func (r *InquiryRepository) Update(ctx context.Context, id primitive.ObjectID, c domain.InquiryChange) (*domain.Inquiry, error) {
	set := bson.M{"updatedAt": bson.M{"$literal": c.At}}
	if c.Status != nil {
		set["status"] = bson.M{"$literal": *c.Status}
	}
	if c.AdminNotes != nil {
		set["adminNotes"] = bson.M{"$literal": *c.AdminNotes}
	}
	pipeline := bson.A{bson.M{"$set": set}}
	if c.Status != nil && *c.Status == domain.InquiryContacted {
		pipeline = append(pipeline, bson.M{"$set": bson.M{"contactedAt": bson.M{"$ifNull": bson.A{"$contactedAt", c.At}}}})
	}
	if c.Status != nil && *c.Status == domain.InquiryClosed {
		pipeline = append(pipeline, bson.M{"$set": bson.M{"closedAt": bson.M{"$ifNull": bson.A{"$closedAt", c.At}}}})
	}
	var out domain.Inquiry
	err := r.col.FindOneAndUpdate(ctx, bson.M{"_id": id}, pipeline,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if err != nil {
		return nil, mapErr(err)
	}
	return &out, nil
}

// Delete borra una consulta.
func (r *InquiryRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CountByStatus cuenta consultas en un estado.
func (r *InquiryRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{"status": status})
}
