package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	mongoClient *mongo.Client
	database    *mongo.Database
)

func watchesCol() *mongo.Collection        { return database.Collection("watches") }
func inquiriesCol() *mongo.Collection      { return database.Collection("inquiries") }
func adminsCol() *mongo.Collection         { return database.Collection("admins") }
func reservationsCol() *mongo.Collection   { return database.Collection("reservations") }
func stockMovementsCol() *mongo.Collection { return database.Collection("stock_movements") }
func passwordResetsCol() *mongo.Collection { return database.Collection("password_resets") }
func customersCol() *mongo.Collection      { return database.Collection("customers") }

// metaCol guarda marcas internas de la app (por ejemplo, si el seed demo ya
// se ejecutó alguna vez), para que ciertas acciones sean de una sola vez.
func metaCol() *mongo.Collection { return database.Collection("app_meta") }

// connectMongo abre la conexión con MongoDB usando MONGO_URI y MONGO_DB.
func connectMongo() {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	dbName := os.Getenv("MONGO_DB")
	if dbName == "" {
		dbName = "relojeria_yampier"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("No se pudo conectar a MongoDB: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB no responde: %v", err)
	}

	mongoClient = client
	database = client.Database(dbName)
	log.Println("Conectado a MongoDB:", dbName)
}
