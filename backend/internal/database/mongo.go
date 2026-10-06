// Package database maneja la conexión a MongoDB, los nombres de colecciones
// y las tareas de arranque (índices, backfill de datos viejos, seeds).
// Los repositories reciben la *mongo.Collection ya resuelta: no conocen la
// conexión ni leen configuración.
package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Nombres de colecciones (única definición).
const (
	CollWatches        = "watches"
	CollInquiries      = "inquiries"
	CollAdmins         = "admins"
	CollReservations   = "reservations"
	CollCustomers      = "customers"
	CollStockMovements = "stock_movements"
	CollPasswordResets = "password_resets"
	// app_meta guarda marcas internas (por ejemplo, si el seed demo ya corrió).
	CollMeta = "app_meta"
)

// DB agrupa el cliente y la base de la aplicación.
type DB struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// Connect abre la conexión y verifica con un ping (falla rápido si Mongo no responde).
func Connect(ctx context.Context, uri, dbName string) (*DB, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a MongoDB: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("MongoDB no responde: %w", err)
	}
	return &DB{Client: client, DB: client.Database(dbName)}, nil
}

// Collection devuelve una colección de la base de la aplicación.
func (d *DB) Collection(name string) *mongo.Collection { return d.DB.Collection(name) }

// Ping verifica que la base responda (healthcheck).
func (d *DB) Ping(ctx context.Context) error { return d.Client.Ping(ctx, nil) }

// Close cierra la conexión.
func (d *DB) Close(ctx context.Context) error { return d.Client.Disconnect(ctx) }
