package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PasswordReset es un pedido de restablecimiento. Solo se guarda el hash
// SHA-256 del token: con una copia de la base no se puede usar.
type PasswordReset struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	CustomerID primitive.ObjectID `bson:"customerId"`
	TokenHash  string             `bson:"tokenHash"`
	ExpiresAt  time.Time          `bson:"expiresAt"`
	UsedAt     *time.Time         `bson:"usedAt,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt"`
}
