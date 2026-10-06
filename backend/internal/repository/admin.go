package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"relojeria-yampier/internal/domain"
)

// AdminRepository persiste usuarios del panel Admin.
type AdminRepository struct{ col *mongo.Collection }

func NewAdminRepository(col *mongo.Collection) *AdminRepository { return &AdminRepository{col: col} }

// FindByEmail busca un admin por email (ya normalizado).
func (r *AdminRepository) FindByEmail(ctx context.Context, email string) (*domain.AdminUser, error) {
	var a domain.AdminUser
	if err := r.col.FindOne(ctx, bson.M{"email": email}).Decode(&a); err != nil {
		return nil, mapErr(err)
	}
	return &a, nil
}

// Insert crea un admin.
func (r *AdminRepository) Insert(ctx context.Context, a *domain.AdminUser) error {
	res, err := r.col.InsertOne(ctx, a)
	if err != nil {
		return mapErr(err)
	}
	a.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}
