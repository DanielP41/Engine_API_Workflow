package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Global MongoDB client
var MongoClient *mongo.Client
var MongoDB *mongo.Database

// ConnectMongoDB establece conexión con MongoDB
func ConnectMongoDB(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Configurar opciones del cliente
	clientOptions := options.Client().ApplyURI(uri)

	// Conectar
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Verificar la conexión
	if err = client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	// Asignar cliente global
	MongoClient = client
	MongoDB = client.Database("engine_workflow")

	return client, nil
}

// NewMongoConnection compatible con main.go - AGREGADO
func NewMongoConnection(uri, dbName string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Configurar opciones del cliente
	clientOptions := options.Client().ApplyURI(uri)

	// Conectar
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Verificar la conexión
	if err = client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	// Asignar cliente global
	MongoClient = client
	MongoDB = client.Database(dbName) // USAR el dbName específico del parámetro

	return client, nil
}

// DisconnectMongoDB cierra la conexión con MongoDB
func DisconnectMongoDB(client *mongo.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.Disconnect(ctx)
}

// GetCollection retorna una colección específica
func GetCollection(collectionName string) *mongo.Collection {
	return MongoDB.Collection(collectionName)
}
