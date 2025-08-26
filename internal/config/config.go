package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
}

func Load() *Config {
	// Cargar archivo .env si existe
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Construir URL de MongoDB completa
	mongoURI := getEnv("MONGODB_URI", "mongodb://localhost:27017")
	mongoDatabase := getEnv("MONGODB_DATABASE", "engine_workflow")
	fullMongoURI := mongoURI + "/" + mongoDatabase

	// Construir URL de Redis
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisURL := "redis://" + redisHost + ":" + redisPort

	return &Config{
		Port:        getEnv("SERVER_PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		DatabaseURL: fullMongoURI,
		RedisURL:    redisURL,
		JWTSecret:   getEnv("JWT_SECRET", "default-secret-key"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
