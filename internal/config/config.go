package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	ServerPort  string
	Environment string
	LogLevel    string

	MongoURI      string
	MongoDatabase string

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret    string
	JWTExpiresIn string

	// External Services
	SlackWebhookURL string
	SlackBotToken   string
}

func Load() *Config {
	// Cargar archivo .env si existe
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Configuración principal
	cfg := &Config{
		ServerPort:   getEnv("PORT", "8081"), // CORREGIDO: usar 8081.-
		Environment:  getEnv("ENV", "development"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		JWTSecret:    getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-this-in-production"),
		JWTExpiresIn: getEnv("JWT_EXPIRES_IN", "24h"),
	}

	// Configuración MongoDB - CORREGIDO para consistencia y Docker
	cfg.MongoURI = getEnv("MONGODB_URI", "mongodb://admin:password123@mongodb:27017/engine_workflow?authSource=admin")
	cfg.MongoDatabase = getEnv("MONGODB_DATABASE", "engine_workflow")

	// Configuración Redis - CORREGIDO para consistencia
	cfg.RedisHost = getEnv("REDIS_HOST", "localhost")
	cfg.RedisPort = getEnv("REDIS_PORT", "6379")
	cfg.RedisPassword = getEnv("REDIS_PASSWORD", "")
	cfg.RedisDB = getEnvAsInt("REDIS_DB", 0)

	// Configuración de servicios externos
	cfg.SlackWebhookURL = getEnv("SLACK_WEBHOOK_URL", "")
	cfg.SlackBotToken = getEnv("SLACK_BOT_TOKEN", "")

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}
