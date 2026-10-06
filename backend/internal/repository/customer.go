package repository

import (
	"context"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"relojeria-yampier/internal/domain"
)

// CustomerRepository persiste cuentas de clientes.
type CustomerRepository struct{ col *mongo.Collection }

func NewCustomerRepository(col *mongo.Collection) *CustomerRepository {
	return &CustomerRepository{col: col}
}

// Insert crea un cliente (email repetido → domain.ErrDuplicate).
func (r *CustomerRepository) Insert(ctx context.Context, c *domain.Customer) error {
	res, err := r.col.InsertOne(ctx, c)
	if err != nil {
		return mapErr(err)
	}
	c.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *CustomerRepository) findOne(ctx context.Context, f bson.M) (*domain.Customer, error) {
	var c domain.Customer
	if err := r.col.FindOne(ctx, f).Decode(&c); err != nil {
		return nil, mapErr(err)
	}
	return &c, nil
}

// FindByID busca un cliente (activo o no).
func (r *CustomerRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Customer, error) {
	return r.findOne(ctx, bson.M{"_id": id})
}

// FindActiveByID busca un cliente activo.
func (r *CustomerRepository) FindActiveByID(ctx context.Context, id primitive.ObjectID) (*domain.Customer, error) {
	return r.findOne(ctx, bson.M{"_id": id, "active": true})
}

// FindByEmail busca por email (ya normalizado).
func (r *CustomerRepository) FindByEmail(ctx context.Context, email string) (*domain.Customer, error) {
	return r.findOne(ctx, bson.M{"email": email})
}

// EmailTakenByOther indica si otro cliente usa ese email.
func (r *CustomerRepository) EmailTakenByOther(ctx context.Context, email string, self primitive.ObjectID) (bool, error) {
	n, err := r.col.CountDocuments(ctx, bson.M{"email": email, "_id": bson.M{"$ne": self}})
	return n > 0, err
}

// UpdateProfile guarda los datos editables del perfil.
func (r *CustomerRepository) UpdateProfile(ctx context.Context, id primitive.ObjectID, c domain.ProfileChange) (*domain.Customer, error) {
	set := bson.M{
		"firstName": c.FirstName, "lastName": c.LastName, "email": c.Email, "phone": c.Phone,
		"documentType": c.DocumentType, "documentNumber": c.DocumentNumber,
		"taxCondition": c.TaxCondition, "taxIdType": c.TaxIDType, "taxId": c.TaxID,
		"marketingConsent": c.MarketingConsent, "updatedAt": c.At,
	}
	update := bson.M{"$set": set}
	if c.BillingAddress != nil {
		set["billingAddress"] = c.BillingAddress
	} else {
		update["$unset"] = bson.M{"billingAddress": ""}
	}
	if c.MarketingConsentChanged {
		set["marketingConsentAt"] = c.At
	}
	var out domain.Customer
	err := r.col.FindOneAndUpdate(ctx, bson.M{"_id": id}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if err != nil {
		return nil, mapErr(err)
	}
	return &out, nil
}

// ReplacePassword cambia el hash y sube tokenVersion (cierra todas las
// sesiones). Con expectedVersion != nil, solo si la versión no cambió (si
// cambió, ErrNotFound). onlyActive limita a cuentas activas.
func (r *CustomerRepository) ReplacePassword(ctx context.Context, id primitive.ObjectID, expectedVersion *int, onlyActive bool, hash string, at time.Time) (*domain.Customer, error) {
	f := bson.M{"_id": id}
	if expectedVersion != nil {
		f["tokenVersion"] = *expectedVersion
	}
	if onlyActive {
		f["active"] = true
	}
	var out domain.Customer
	err := r.col.FindOneAndUpdate(ctx, f, bson.M{
		"$set": bson.M{"passwordHash": hash, "updatedAt": at},
		"$inc": bson.M{"tokenVersion": 1},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if err != nil {
		return nil, mapErr(err)
	}
	return &out, nil
}

// SetActive desactiva (sube tokenVersion) o reactiva una cuenta, solo si
// estaba en el estado contrario. Si ya estaba así o no existe: ErrNotFound.
func (r *CustomerRepository) SetActive(ctx context.Context, id primitive.ObjectID, active bool, at time.Time) (*domain.Customer, error) {
	update := bson.M{"$set": bson.M{"active": true, "updatedAt": at}, "$unset": bson.M{"deactivatedAt": ""}}
	if !active {
		update = bson.M{"$set": bson.M{"active": false, "deactivatedAt": at, "updatedAt": at},
			"$inc": bson.M{"tokenVersion": 1}}
	}
	var out domain.Customer
	err := r.col.FindOneAndUpdate(ctx, bson.M{"_id": id, "active": !active}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if err != nil {
		return nil, mapErr(err)
	}
	return &out, nil
}

// FindByIDs busca varios clientes por ID.
func (r *CustomerRepository) FindByIDs(ctx context.Context, ids []primitive.ObjectID) ([]domain.Customer, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cur, err := r.col.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Customer
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// List devuelve una página de clientes (más recientes primero).
func (r *CustomerRepository) List(ctx context.Context, f domain.CustomerFilter, page, limit int) ([]domain.Customer, int64, error) {
	filter := bson.M{}
	if f.Active != nil {
		filter["active"] = *f.Active
	}
	if f.Search != "" {
		q := bson.M{"$regex": regexp.QuoteMeta(f.Search), "$options": "i"}
		docSearch := f.Search
		if d := domain.DigitsOnly(f.Search); d != "" {
			docSearch = d
		}
		filter["$or"] = bson.A{
			bson.M{"firstName": q}, bson.M{"lastName": q}, bson.M{"email": q},
			bson.M{"documentNumber": bson.M{"$regex": regexp.QuoteMeta(docSearch)}},
		}
	}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cur, err := r.col.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetSkip(int64((page-1)*limit)).SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []domain.Customer
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
