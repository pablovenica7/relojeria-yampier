package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"relojeria-yampier/internal/domain"
)

// StockMovementRepository persiste el libro de inventario.
type StockMovementRepository struct{ col *mongo.Collection }

func NewStockMovementRepository(col *mongo.Collection) *StockMovementRepository {
	return &StockMovementRepository{col: col}
}

// Insert guarda un movimiento. Una clave de idempotencia repetida devuelve
// domain.ErrDuplicate (el movimiento ya estaba registrado).
func (r *StockMovementRepository) Insert(ctx context.Context, m *domain.StockMovement) error {
	_, err := r.col.InsertOne(ctx, m)
	return mapErr(err)
}

// List devuelve una página del historial (más recientes primero).
func (r *StockMovementRepository) List(ctx context.Context, f domain.MovementFilter, page, limit int) ([]domain.StockMovement, int64, error) {
	filter := bson.M{}
	if f.WatchID != nil {
		filter["watchId"] = *f.WatchID
	}
	if f.Type != "" {
		filter["type"] = f.Type
	}
	if dr := dateRange(f.CreatedFrom, f.CreatedTo); dr != nil {
		filter["createdAt"] = dr
	}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cur, err := r.col.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page-1)*limit)).SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	items := []domain.StockMovement{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
