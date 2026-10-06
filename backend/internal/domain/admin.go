package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdminUser es un usuario del panel de administración.
type AdminUser struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Email        string             `bson:"email"`
	PasswordHash string             `bson:"passwordHash"`
	TokenVersion int                `bson:"tokenVersion"` // ver Customer.TokenVersion
	CreatedAt    time.Time          `bson:"createdAt"`
}
