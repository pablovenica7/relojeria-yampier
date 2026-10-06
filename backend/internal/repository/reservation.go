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

// ReservationRepository persiste reservas. Todos los cambios de estado son
// condicionales al estado leído: dos operaciones simultáneas sobre la misma
// reserva no pueden aplicarse ambas.
type ReservationRepository struct{ col *mongo.Collection }

func NewReservationRepository(col *mongo.Collection) *ReservationRepository {
	return &ReservationRepository{col: col}
}

// Insert crea una reserva y completa su ID.
func (r *ReservationRepository) Insert(ctx context.Context, res *domain.Reservation) error {
	ins, err := r.col.InsertOne(ctx, res)
	if err != nil {
		return mapErr(err)
	}
	res.ID = ins.InsertedID.(primitive.ObjectID)
	return nil
}

// FindByID busca una reserva.
func (r *ReservationRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Reservation, error) {
	var res domain.Reservation
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&res); err != nil {
		return nil, mapErr(err)
	}
	return &res, nil
}

// FindForCustomer busca una reserva SOLO si pertenece al cliente.
func (r *ReservationRepository) FindForCustomer(ctx context.Context, id, customerID primitive.ObjectID) (*domain.Reservation, error) {
	var res domain.Reservation
	if err := r.col.FindOne(ctx, bson.M{"_id": id, "customerId": customerID}).Decode(&res); err != nil {
		return nil, mapErr(err)
	}
	return &res, nil
}

// ApplyStatusChange aplica el cambio solo si la reserva sigue en el estado
// (y con el stockHeld) leídos. Si cambió en paralelo devuelve ErrNotFound.
func (r *ReservationRepository) ApplyStatusChange(ctx context.Context, id primitive.ObjectID, c domain.StatusChange) (*domain.Reservation, error) {
	set := bson.M{"status": c.To, "stockHeld": c.StockHeld, "depositAmount": c.DepositAmount, "updatedAt": c.At}
	if c.DepositPaidAt != nil {
		set["depositMethod"] = c.DepositMethod
		set["depositPaidAt"] = *c.DepositPaidAt
	}
	switch c.To {
	case domain.ReservationConfirmed:
		set["confirmedAt"] = c.At
	case domain.ReservationCompleted:
		set["completedAt"] = c.At
	case domain.ReservationCancelled:
		set["cancelledAt"] = c.At
	case domain.ReservationExpired:
		set["expiredAt"] = c.At
	}
	var out domain.Reservation
	err := r.col.FindOneAndUpdate(ctx,
		bson.M{"_id": id, "status": c.FromStatus, "stockHeld": c.FromStockHeld},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&out)
	if err != nil {
		return nil, mapErr(err)
	}
	return &out, nil
}

// ExpireNextDue marca como vencida UNA reserva pendiente/confirmada cuyo
// vencimiento ya pasó y devuelve su estado ANTERIOR (para saber si retenía
// stock). Devuelve ErrNotFound cuando no queda ninguna. Es atómico: si una
// cancelación ocurre al mismo tiempo, solo una de las dos la encuentra activa.
func (r *ReservationRepository) ExpireNextDue(ctx context.Context, now time.Time, only *primitive.ObjectID) (*domain.Reservation, error) {
	filter := bson.M{
		"status":    bson.M{"$in": bson.A{domain.ReservationPending, domain.ReservationConfirmed}},
		"expiresAt": bson.M{"$ne": nil, "$lte": now},
	}
	if only != nil {
		filter["_id"] = *only
	}
	var before domain.Reservation
	err := r.col.FindOneAndUpdate(ctx, filter,
		bson.M{"$set": bson.M{"status": domain.ReservationExpired, "stockHeld": false, "updatedAt": now, "expiredAt": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.Before),
	).Decode(&before)
	if err != nil {
		return nil, mapErr(err)
	}
	return &before, nil
}

// UpdateDetails actualiza notas, seña y vencimiento, condicional al estado
// leído (si cambió en paralelo devuelve ErrNotFound).
func (r *ReservationRepository) UpdateDetails(ctx context.Context, id primitive.ObjectID, status string, c domain.DetailsChange) (*domain.Reservation, error) {
	set := bson.M{"updatedAt": c.At}
	if c.Notes != nil {
		set["notes"] = *c.Notes
	}
	if c.DepositAmount != nil {
		set["depositAmount"] = *c.DepositAmount
	}
	update := bson.M{"$set": set}
	if c.ClearExpiry {
		update["$unset"] = bson.M{"expiresAt": ""}
	} else if c.ExpiresAt != nil {
		set["expiresAt"] = *c.ExpiresAt
	}
	var out domain.Reservation
	err := r.col.FindOneAndUpdate(ctx, bson.M{"_id": id, "status": status}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if err != nil {
		return nil, mapErr(err)
	}
	return &out, nil
}

// List devuelve una página del listado del Admin (más recientes primero).
func (r *ReservationRepository) List(ctx context.Context, f domain.ReservationFilter, page, limit int) ([]domain.Reservation, int64, error) {
	filter := bson.M{}
	if f.Status != "" {
		filter["status"] = f.Status
	}
	if f.WatchID != nil {
		filter["watchId"] = *f.WatchID
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
	items := []domain.Reservation{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListByCustomer devuelve las reservas de un cliente (más recientes primero).
func (r *ReservationRepository) ListByCustomer(ctx context.Context, customerID primitive.ObjectID, limit int) ([]domain.Reservation, error) {
	cur, err := r.col.Find(ctx, bson.M{"customerId": customerID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	items := []domain.Reservation{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// CountForCustomer cuenta reservas de un cliente en los estados dados
// (opcionalmente de un reloj).
func (r *ReservationRepository) CountForCustomer(ctx context.Context, customerID primitive.ObjectID, watchID *primitive.ObjectID, statuses []string) (int64, error) {
	f := bson.M{"customerId": customerID, "status": bson.M{"$in": toBsonA(statuses)}}
	if watchID != nil {
		f["watchId"] = *watchID
	}
	return r.col.CountDocuments(ctx, f)
}

// CountForWatch cuenta reservas de un reloj (en los estados dados; vacío = todas).
func (r *ReservationRepository) CountForWatch(ctx context.Context, watchID primitive.ObjectID, statuses []string) (int64, error) {
	f := bson.M{"watchId": watchID}
	if len(statuses) > 0 {
		f["status"] = bson.M{"$in": toBsonA(statuses)}
	}
	return r.col.CountDocuments(ctx, f)
}

// CountByStatus cuenta reservas en un estado.
func (r *ReservationRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{"status": status})
}

// ListExpiringBetween lista reservas pendientes/confirmadas que vencen en el rango.
func (r *ReservationRepository) ListExpiringBetween(ctx context.Context, from, to time.Time, limit int) ([]domain.Reservation, error) {
	cur, err := r.col.Find(ctx, bson.M{
		"status":    bson.M{"$in": bson.A{domain.ReservationPending, domain.ReservationConfirmed}},
		"expiresAt": bson.M{"$gt": from, "$lte": to},
	}, options.Find().SetSort(bson.D{{Key: "expiresAt", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	items := []domain.Reservation{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}
