package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"relojeria-yampier/internal/domain"
)

// PasswordResetRepository persiste pedidos de recuperación (solo hashes).
type PasswordResetRepository struct{ col *mongo.Collection }

func NewPasswordResetRepository(col *mongo.Collection) *PasswordResetRepository {
	return &PasswordResetRepository{col: col}
}

// CountRecentUnused cuenta pedidos sin usar creados después de since.
func (r *PasswordResetRepository) CountRecentUnused(ctx context.Context, customerID primitive.ObjectID, since time.Time) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{"customerId": customerID, "usedAt": nil, "createdAt": bson.M{"$gt": since}})
}

// InvalidateAll marca como usados todos los pedidos pendientes del cliente.
func (r *PasswordResetRepository) InvalidateAll(ctx context.Context, customerID primitive.ObjectID, at time.Time) error {
	_, err := r.col.UpdateMany(ctx, bson.M{"customerId": customerID, "usedAt": nil}, bson.M{"$set": bson.M{"usedAt": at}})
	return err
}

// Insert guarda un pedido.
func (r *PasswordResetRepository) Insert(ctx context.Context, p *domain.PasswordReset) error {
	res, err := r.col.InsertOne(ctx, p)
	if err != nil {
		return mapErr(err)
	}
	p.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

// Consume marca el pedido como usado de forma atómica, solo si existe, no se
// usó y no venció. Dos usos simultáneos no pueden ganar ambos.
func (r *PasswordResetRepository) Consume(ctx context.Context, tokenHash string, now time.Time) (*domain.PasswordReset, error) {
	var p domain.PasswordReset
	err := r.col.FindOneAndUpdate(ctx,
		bson.M{"tokenHash": tokenHash, "usedAt": nil, "expiresAt": bson.M{"$gt": now}},
		bson.M{"$set": bson.M{"usedAt": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&p)
	if err != nil {
		return nil, mapErr(err)
	}
	return &p, nil
}
