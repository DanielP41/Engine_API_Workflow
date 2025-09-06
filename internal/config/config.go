package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	ServerPort  string
	Environment string
	LogLevel    string

	// Database
	MongoURI      string
	MongoDatabase string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// JWT - ✅ Configuración segura expandida
	JWTSecret     string
	JWTIssuer     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	JWTAudience   string
	JWTExpiresIn  string // Mantener para compatibilidad hacia atrás

	// Security - ✅ Nuevas configuraciones de seguridad
	TrustedProxies     []string
	CORSAllowedOrigins []string
	RateLimitRequests  int
	RateLimitWindow    time.Duration

	// External Services
	SlackWebhookURL string
	SlackBotToken   string

	// ✅ Feature flags
	EnableTokenBlacklist bool
	EnableRateLimit      bool
	EnableCORS           bool
}

// ✅ JWTConfig estructura específica para configuración JWT
type JWTConfig struct {
	Secret          string
	Issuer          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Audience        string
}

func Load() *Config {
	// Cargar archivo .env si existe
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// ✅ Parsear duraciones JWT de forma segura
	accessTTL := parseDuration("JWT_ACCESS_TTL", "15m")
	refreshTTL := parseDuration("JWT_REFRESH_TTL", "168h") // 7 días
	rateLimitWindow := parseDuration("RATE_LIMIT_WINDOW", "1m")

	// Configuración principal
	cfg := &Config{
		ServerPort:  getEnv("PORT", "8081"),
		Environment: getEnv("ENV", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// ✅ Configuración JWT segura
		JWTSecret:     getEnv("JWT_SECRET", generateDefaultJWTSecret()),
		JWTIssuer:     getEnv("JWT_ISSUER", "engine-api-workflow"),
		JWTAccessTTL:  accessTTL,
		JWTRefreshTTL: refreshTTL,
		JWTAudience:   getEnv("JWT_AUDIENCE", "engine-api"),
		JWTExpiresIn:  getEnv("JWT_EXPIRES_IN", "24h"), // Compatibilidad

		// ✅ Configuración de seguridad
		TrustedProxies:     parseStringSlice("TRUSTED_PROXIES", "127.0.0.1,::1"),
		CORSAllowedOrigins: parseStringSlice("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001"),
		RateLimitRequests:  getEnvAsInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:    rateLimitWindow,

		// ✅ Feature flags
		EnableTokenBlacklist: getEnvAsBool("ENABLE_TOKEN_BLACKLIST", true),
		EnableRateLimit:      getEnvAsBool("ENABLE_RATE_LIMIT", true),
		EnableCORS:           getEnvAsBool("ENABLE_CORS", true),
	}

	// Configuración MongoDB con validación
	cfg.MongoURI = getEnv("MONGODB_URI", getDefaultMongoURI(cfg.Environment))
	cfg.MongoDatabase = getEnv("MONGODB_DATABASE", "engine_workflow")

	// Configuración Redis con validación
	cfg.RedisHost = getEnv("REDIS_HOST", getDefaultRedisHost(cfg.Environment))
	cfg.RedisPort = getEnv("REDIS_PORT", "6379")
	cfg.RedisPassword = getEnv("REDIS_PASSWORD", "")
	cfg.RedisDB = getEnvAsInt("REDIS_DB", 0)

	// Configuración de servicios externos
	cfg.SlackWebhookURL = getEnv("SLACK_WEBHOOK_URL", "")
	cfg.SlackBotToken = getEnv("SLACK_BOT_TOKEN", "")

	// ✅ Validar configuración antes de continuar
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	return cfg
}

// ✅ Validate valida toda la configuración al cargar
func (c *Config) Validate() error {
	// Validar JWT
	if err := c.ValidateJWT(); err != nil {
		return fmt.Errorf("JWT validation failed: %w", err)
	}

	// Validar servidor
	if err := c.ValidateServer(); err != nil {
		return fmt.Errorf("Server validation failed: %w", err)
	}

	// Validar base de datos
	if err := c.ValidateDatabase(); err != nil {
		return fmt.Errorf("Database validation failed: %w", err)
	}

	// Validar Redis
	if err := c.ValidateRedis(); err != nil {
		return fmt.Errorf("Redis validation failed: %w", err)
	}

	return nil
}

// ✅ ValidateJWT valida específicamente la configuración JWT
func (c *Config) ValidateJWT() error {
	// Verificar longitud mínima del secret
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters, got %d", len(c.JWTSecret))
	}

	// Verificar que no sea el valor por defecto en producción
	if c.Environment == "production" {
		defaultSecrets := []string{
			"your-super-secret-jwt-key-change-this-in-production",
			"default-jwt-secret",
			"change-me",
			"secret",
		}

		for _, defaultSecret := range defaultSecrets {
			if c.JWTSecret == defaultSecret {
				return fmt.Errorf("JWT_SECRET cannot be default value in production environment")
			}
		}
	}

	// Verificar entropy del secret
	if entropy := calculateEntropy(c.JWTSecret); entropy < 3.5 {
		return fmt.Errorf("JWT_SECRET has insufficient entropy: %.2f (minimum: 3.5)", entropy)
	}

	// Validar TTLs
	if c.JWTAccessTTL > 1*time.Hour {
		return fmt.Errorf("JWT_ACCESS_TTL should not exceed 1 hour for security, got %v", c.JWTAccessTTL)
	}

	if c.JWTRefreshTTL > 30*24*time.Hour { // 30 días máximo
		return fmt.Errorf("JWT_REFRESH_TTL should not exceed 30 days for security, got %v", c.JWTRefreshTTL)
	}

	// Validar issuer y audience
	if c.JWTIssuer == "" {
		return fmt.Errorf("JWT_ISSUER cannot be empty")
	}

	if c.JWTAudience == "" {
		return fmt.Errorf("JWT_AUDIENCE cannot be empty")
	}

	return nil
}

// ✅ ValidateServer valida la configuración del servidor
func (c *Config) ValidateServer() error {
	// Validar puerto
	if port, err := strconv.Atoi(c.ServerPort); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid server port: %s", c.ServerPort)
	}

	// Validar environment
	validEnvs := []string{"development", "staging", "production", "test"}
	isValidEnv := false
	for _, env := range validEnvs {
		if c.Environment == env {
			isValidEnv = true
			break
		}
	}
	if !isValidEnv {
		return fmt.Errorf("invalid environment: %s (valid: %v)", c.Environment, validEnvs)
	}

	// Validar log level
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	isValidLogLevel := false
	for _, level := range validLogLevels {
		if c.LogLevel == level {
			isValidLogLevel = true
			break
		}
	}
	if !isValidLogLevel {
		return fmt.Errorf("invalid log level: %s (valid: %v)", c.LogLevel, validLogLevels)
	}

	return nil
}

// ✅ ValidateDatabase valida la configuración de MongoDB
func (c *Config) ValidateDatabase() error {
	if c.MongoURI == "" {
		return fmt.Errorf("MONGODB_URI cannot be empty")
	}

	if c.MongoDatabase == "" {
		return fmt.Errorf("MONGODB_DATABASE cannot be empty")
	}

	// Validar que no contenga credenciales por defecto en producción
	if c.Environment == "production" {
		if strings.Contains(c.MongoURI, "password123") {
			return fmt.Errorf("MongoDB URI contains default password in production")
		}
	}

	return nil
}

// ✅ ValidateRedis valida la configuración de Redis
func (c *Config) ValidateRedis() error {
	if c.RedisHost == "" {
		return fmt.Errorf("REDIS_HOST cannot be empty")
	}

	if port, err := strconv.Atoi(c.RedisPort); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid Redis port: %s", c.RedisPort)
	}

	if c.RedisDB < 0 || c.RedisDB > 15 {
		return fmt.Errorf("invalid Redis DB number: %d (valid: 0-15)", c.RedisDB)
	}

	return nil
}

// ✅ GetJWTConfig retorna configuración específica para JWT
func (c *Config) GetJWTConfig() JWTConfig {
	return JWTConfig{
		Secret:          c.JWTSecret,
		Issuer:          c.JWTIssuer,
		AccessTokenTTL:  c.JWTAccessTTL,
		RefreshTokenTTL: c.JWTRefreshTTL,
		Audience:        c.JWTAudience,
	}
}

// ✅ IsProduction retorna si está en ambiente de producción
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// ✅ IsDevelopment retorna si está en ambiente de desarrollo
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// ✅ GetCORSOrigins retorna los orígenes CORS permitidos
func (c *Config) GetCORSOrigins() []string {
	if c.IsProduction() {
		// En producción, ser más restrictivo
		return c.CORSAllowedOrigins
	}
	// En desarrollo, agregar orígenes comunes
	origins := append(c.CORSAllowedOrigins,
		"http://localhost:3000",
		"http://localhost:3001",
		"http://127.0.0.1:3000",
	)
	return removeDuplicates(origins)
}

// ✅ Helper functions

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

// ✅ getEnvAsBool parsea variables de entorno como boolean
func getEnvAsBool(name string, defaultVal bool) bool {
	valueStr := getEnv(name, "")
	if valueStr == "" {
		return defaultVal
	}

	switch strings.ToLower(valueStr) {
	case "true", "1", "yes", "on", "enable", "enabled":
		return true
	case "false", "0", "no", "off", "disable", "disabled":
		return false
	default:
		return defaultVal
	}
}

// ✅ parseDuration parsea duraciones de forma segura
func parseDuration(envKey, defaultValue string) time.Duration {
	value := getEnv(envKey, defaultValue)
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("Warning: Invalid duration for %s (%s), using default: %s", envKey, value, defaultValue)
		duration, _ = time.ParseDuration(defaultValue)
	}
	return duration
}

// ✅ parseStringSlice parsea una lista separada por comas
func parseStringSlice(envKey, defaultValue string) []string {
	value := getEnv(envKey, defaultValue)
	if value == "" {
		return []string{}
	}

	parts := strings.Split(value, ",")
	var result []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ✅ calculateEntropy calcula la entropía de una cadena
func calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Contar frecuencia de caracteres
	freq := make(map[rune]float64)
	for _, char := range s {
		freq[char]++
	}

	// Calcular entropía usando fórmula de Shannon
	entropy := 0.0
	length := float64(len(s))

	for _, count := range freq {
		p := count / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}

// ✅ generateDefaultJWTSecret genera un secret por defecto seguro
func generateDefaultJWTSecret() string {
	// Solo para desarrollo - en producción debe ser configurado explícitamente
	log.Println("Warning: Using auto-generated JWT secret. Set JWT_SECRET environment variable in production.")

	bytes := make([]byte, 64)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback si no se puede generar random
		return "DEVELOPMENT_ONLY_CHANGE_IN_PRODUCTION_" + fmt.Sprintf("%d", time.Now().Unix())
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// ✅ getDefaultMongoURI retorna URI por defecto según el ambiente
func getDefaultMongoURI(environment string) string {
	switch environment {
	case "production":
		return "mongodb://localhost:27017/engine_workflow" // Sin credenciales por defecto
	case "development":
		return "mongodb://admin:password123@localhost:27017/engine_workflow?authSource=admin"
	default:
		return "mongodb://localhost:27017/engine_workflow"
	}
}

// ✅ getDefaultRedisHost retorna host por defecto según el ambiente
func getDefaultRedisHost(environment string) string {
	switch environment {
	case "development":
		return "localhost" // Para desarrollo local
	case "production":
		return "redis" // Para Docker/Kubernetes
	default:
		return "localhost"
	}
}

// ✅ removeDuplicates remueve elementos duplicados de un slice
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// ✅ LogConfig registra la configuración actual (sin secrets)
func (c *Config) LogConfig() {
	log.Printf("Configuration loaded:")
	log.Printf("  Environment: %s", c.Environment)
	log.Printf("  Server Port: %s", c.ServerPort)
	log.Printf("  Log Level: %s", c.LogLevel)
	log.Printf("  MongoDB Database: %s", c.MongoDatabase)
	log.Printf("  Redis Host: %s:%s", c.RedisHost, c.RedisPort)
	log.Printf("  JWT Issuer: %s", c.JWTIssuer)
	log.Printf("  JWT Access TTL: %v", c.JWTAccessTTL)
	log.Printf("  JWT Refresh TTL: %v", c.JWTRefreshTTL)
	log.Printf("  Features: BlackList=%v, RateLimit=%v, CORS=%v",
		c.EnableTokenBlacklist, c.EnableRateLimit, c.EnableCORS)

	// ✅ Nunca logear secrets
	secretLength := len(c.JWTSecret)
	log.Printf("  JWT Secret: [%d characters] %s***", secretLength, c.JWTSecret[:4])
}
