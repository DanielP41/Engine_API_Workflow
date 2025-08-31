package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Global Redis client
var RedisClient *redis.Client

// ConnectRedis establece conexión con Redis
func ConnectRedis(redisURL string) (*redis.Client, error) {
	// Parsear opciones desde URL
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		// Fallback para configuración simple
		opt = &redis.Options{
			Addr:     "localhost:6379",
			Password: "", // sin password por defecto
			DB:       0,  // base de datos por defecto
		}
	}

	// Crear cliente
	client := redis.NewClient(opt)

	// Verificar conexión
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	// Asignar cliente global
	RedisClient = client

	return client, nil
}

// NewRedisConnection compatible con main.go - AGREGADO
func NewRedisConnection(host, port, password string, db int) (*redis.Client, error) {
	// Construir URL de Redis
	var redisURL string
	if password != "" {
		redisURL = fmt.Sprintf("redis://:%s@%s:%s/%d", password, host, port, db)
	} else {
		redisURL = fmt.Sprintf("redis://%s:%s/%d", host, port, db)
	}

	return ConnectRedis(redisURL)
}

// GetRedisClient retorna el cliente Redis global
func GetRedisClient() *redis.Client {
	return RedisClient
}
