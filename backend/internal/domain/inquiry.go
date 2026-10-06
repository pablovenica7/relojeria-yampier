package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Estados de una consulta.
const (
	InquiryNew       = "new"
	InquiryContacted = "contacted"
	InquiryClosed    = "closed"
)

// ValidInquiryStatus indica si el estado existe.
func ValidInquiryStatus(s string) bool {
	return s == InquiryNew || s == InquiryContacted || s == InquiryClosed
}

// Inquiry es una consulta enviada desde el sitio. Una consulta solo expresa
// interés: nunca modifica el stock.
type Inquiry struct {
	ID                 primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name               string              `bson:"name" json:"name"`
	Email              string              `bson:"email" json:"email"`
	Phone              string              `bson:"phone" json:"phone"`
	Message            string              `bson:"message" json:"message"`
	Status             string              `bson:"status" json:"status"`
	AdminNotes         string              `bson:"adminNotes" json:"adminNotes"`
	WatchID            *primitive.ObjectID `bson:"watchId,omitempty" json:"watchId,omitempty"`
	WatchNameSnapshot  string              `bson:"watchNameSnapshot,omitempty" json:"watchNameSnapshot,omitempty"`
	WatchModelSnapshot string              `bson:"watchModelSnapshot,omitempty" json:"watchModelSnapshot,omitempty"`
	ContactedAt        *time.Time          `bson:"contactedAt,omitempty" json:"contactedAt,omitempty"`
	ClosedAt           *time.Time          `bson:"closedAt,omitempty" json:"closedAt,omitempty"`
	CreatedAt          time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

// InquiryChange son los campos editables de una consulta (desde el Admin).
type InquiryChange struct {
	Status     *string
	AdminNotes *string
	At         time.Time
}
