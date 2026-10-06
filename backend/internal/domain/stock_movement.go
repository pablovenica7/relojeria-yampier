package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Tipos de movimiento de stock.
const (
	MoveStockEntry         = "stock_entry"         // ingreso de mercadería
	MoveManualAdjustment   = "manual_adjustment"   // ajuste desde el Admin
	MoveReservationHold    = "reservation_hold"    // reserva confirmada: -1
	MoveReservationRelease = "reservation_release" // reserva cancelada/vencida: +1
	MoveSale               = "sale"                // venta completada (delta 0: la unidad ya estaba retenida)
	MoveCorrection         = "correction"          // corrección de inventario
)

// ValidManualMoveType: tipos que el Admin puede elegir a mano.
func ValidManualMoveType(t string) bool {
	return t == MoveStockEntry || t == MoveManualAdjustment || t == MoveCorrection
}

// ValidMoveType indica si el tipo existe.
func ValidMoveType(t string) bool {
	return ValidManualMoveType(t) || t == MoveReservationHold || t == MoveReservationRelease || t == MoveSale
}

// SystemActor identifica cambios hechos por el sistema (vencimientos).
const SystemActor = "sistema"

// StockMovement es una línea del libro de inventario. Cada cambio de
// stockQuantity genera exactamente un movimiento (ver service.StockService).
type StockMovement struct {
	ID                primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	WatchID           primitive.ObjectID  `bson:"watchId" json:"watchId"`
	WatchNameSnapshot string              `bson:"watchNameSnapshot" json:"watchName"`
	Type              string              `bson:"type" json:"type"`
	QuantityDelta     int                 `bson:"quantityDelta" json:"quantityDelta"`
	StockBefore       int                 `bson:"stockBefore" json:"stockBefore"`
	StockAfter        int                 `bson:"stockAfter" json:"stockAfter"`
	ReservationID     *primitive.ObjectID `bson:"reservationId,omitempty" json:"reservationId,omitempty"`
	Reason            string              `bson:"reason,omitempty" json:"reason,omitempty"`
	CreatedBy         string              `bson:"createdBy" json:"createdBy"` // email del admin o "sistema"
	// IdempotencyKey evita duplicar el movimiento de una reserva ante
	// reintentos (índice único parcial). Vacío en ajustes manuales.
	IdempotencyKey string    `bson:"idempotencyKey,omitempty" json:"-"`
	CreatedAt      time.Time `bson:"createdAt" json:"createdAt"`
}

// MovementFilter filtra el historial de stock.
type MovementFilter struct {
	WatchID     *primitive.ObjectID
	Type        string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}
