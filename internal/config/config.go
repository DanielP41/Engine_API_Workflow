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

	// Database - CORREGIDO para coincidir con main.go
	MongoURI      string
	MongoDatabase string

	// Redis - CORREGIDO para coincidir con main.go
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret string

	// External Services
	SlackWebhookURL string
	SlackBotToken   string

	// Backwards compatibility
	DatabaseURL string // Deprecated: usar MongoURI + MongoDatabase
	RedisURL    string // Deprecated: usar RedisHost + RedisPort
}

func Load() *Config {
	// Cargar archivo .env si existe
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Configuración principal
	cfg := &Config{
		ServerPort:  getEnv("PORT", "8081"),
		Environment: getEnv("ENV", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		JWTSecret:   getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-this-in-production"),
	}

	// Configuración MongoDB - NUEVO formato compatible con main.go
	cfg.MongoURI = getEnv("MONGODB_URI", "mongodb://admin:password123@localhost:27017/engine_workflow?authSource=admin")
	cfg.MongoDatabase = getEnv("MONGODB_DATABASE", "engine_workflow")

	// Configuración Redis - NUEVO formato compatible con main.go
	cfg.RedisHost = getEnv("REDIS_HOST", "localhost")
	cfg.RedisPort = getEnv("REDIS_PORT", "6379")
	cfg.RedisPassword = getEnv("REDIS_PASSWORD", "")
	cfg.RedisDB = getEnvAsInt("REDIS_DB", 0)

	// Configuración de servicios externos
	cfg.SlackWebhookURL = getEnv("SLACK_WEBHOOK_URL", "")
	cfg.SlackBotToken = getEnv("SLACK_BOT_TOKEN", "")

	// Compatibilidad hacia atrás (para otros archivos que puedan usar el formato anterior)
	cfg.DatabaseURL = cfg.MongoURI
	cfg.RedisURL = "redis://" + cfg.RedisHost + ":" + cfg.RedisPort

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
