package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// CacheConfiguration configuración específica del caché
type CacheConfiguration struct {
	// Configuración básica
	Enabled          bool          `env:"CACHE_ENABLED" envDefault:"true"`
	DefaultTTL       time.Duration `env:"CACHE_DEFAULT_TTL" envDefault:"5m"`
	CleanupInterval  time.Duration `env:"CACHE_CLEANUP_INTERVAL" envDefault:"1h"`
	MaxMemory        string        `env:"CACHE_MAX_MEMORY" envDefault:"100MB"`
	CompressionLevel int           `env:"CACHE_COMPRESSION_LEVEL" envDefault:"1"`
	Serializer       string        `env:"CACHE_SERIALIZER" envDefault:"json"`

	// Configuración de TTL por dominio
	DashboardTTL time.Duration `env:"CACHE_DASHBOARD_TTL" envDefault:"30s"`
	WorkflowTTL  time.Duration `env:"CACHE_WORKFLOW_TTL" envDefault:"5m"`
	UserTTL      time.Duration `env:"CACHE_USER_TTL" envDefault:"15m"`
	MetricsTTL   time.Duration `env:"CACHE_METRICS_TTL" envDefault:"1m"`
	QueueTTL     time.Duration `env:"CACHE_QUEUE_TTL" envDefault:"10s"`
	SystemTTL    time.Duration `env:"CACHE_SYSTEM_TTL" envDefault:"5m"`
	AuthTTL      time.Duration `env:"CACHE_AUTH_TTL" envDefault:"10m"`

	// Configuración avanzada
	EnableCompression  bool `env:"CACHE_ENABLE_COMPRESSION" envDefault:"false"`
	EnableMetrics      bool `env:"CACHE_ENABLE_METRICS" envDefault:"true"`
	EnableDistribution bool `env:"CACHE_ENABLE_DISTRIBUTION" envDefault:"false"`
	EnablePersistence  bool `env:"CACHE_ENABLE_PERSISTENCE" envDefault:"true"`
	EnableWarmup       bool `env:"CACHE_ENABLE_WARMUP" envDefault:"true"`

	// Configuración de warmup
	WarmupEnabled     bool          `env:"CACHE_WARMUP_ENABLED" envDefault:"true"`
	WarmupTimeout     time.Duration `env:"CACHE_WARMUP_TIMEOUT" envDefault:"30s"`
	WarmupConcurrency int           `env:"CACHE_WARMUP_CONCURRENCY" envDefault:"5"`

	// Configuración de invalidación
	InvalidationEnabled bool          `env:"CACHE_INVALIDATION_ENABLED" envDefault:"true"`
	InvalidationBuffer  int           `env:"CACHE_INVALIDATION_BUFFER" envDefault:"1000"`
	InvalidationTimeout time.Duration `env:"CACHE_INVALIDATION_TIMEOUT" envDefault:"5s"`

	// Configuración de backup del caché
	BackupEnabled  bool          `env:"CACHE_BACKUP_ENABLED" envDefault:"false"`
	BackupInterval time.Duration `env:"CACHE_BACKUP_INTERVAL" envDefault:"1h"`
	BackupPath     string        `env:"CACHE_BACKUP_PATH" envDefault:"./cache_backups"`
}

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

	// JWT - Configuración segura expandida
	JWTSecret     string
	JWTIssuer     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	JWTAudience   string
	JWTExpiresIn  string // Mantener para compatibilidad hacia atrás

	// Security - Nuevas configuraciones de seguridad
	TrustedProxies     []string
	CORSAllowedOrigins []string
	RateLimitRequests  int
	RateLimitWindow    time.Duration

	// CORS Configuration Expanded
	CORSAllowedMethods    []string
	CORSAllowedHeaders    []string
	CORSExposedHeaders    []string
	CORSAllowCredentials  bool
	CORSMaxAge            int
	CORSPreflightContinue bool

	// External Services
	SlackWebhookURL string
	SlackBotToken   string

	// Feature flags
	EnableTokenBlacklist bool
	EnableRateLimit      bool
	EnableCORS           bool

	// Web Interface Settings
	EnableWebInterface bool
	StaticFilesPath    string
	TemplatesPath      string

	// CACHE CONFIGURATION - AGREGADO
	Cache CacheConfiguration `mapstructure:"cache"`

	// BACKUP CONFIGURATION - AGREGADO
	BackupEnabled       bool          `json:"backup_enabled"`
	BackupInterval      time.Duration `json:"backup_interval"`
	BackupRetentionDays int           `json:"backup_retention_days"`
	BackupStorageType   string        `json:"backup_storage_type"`
	BackupStoragePath   string        `json:"backup_storage_path"`

	// Configuración avanzada de backup
	BackupCompressionEnabled  bool   `json:"backup_compression_enabled"`
	BackupCompressionLevel    int    `json:"backup_compression_level"`
	BackupValidationEnabled   bool   `json:"backup_validation_enabled"`
	BackupNotifyOnSuccess     bool   `json:"backup_notify_on_success"`
	BackupNotifyOnFailure     bool   `json:"backup_notify_on_failure"`
	BackupNotificationEmail   string `json:"backup_notification_email"`
	BackupNotificationWebhook string `json:"backup_notification_webhook"`

	// Configuración de almacenamiento remoto
	BackupRemoteEnabled bool   `json:"backup_remote_enabled"`
	BackupS3Bucket      string `json:"backup_s3_bucket"`
	BackupS3Region      string `json:"backup_s3_region"`
	BackupS3AccessKey   string `json:"backup_s3_access_key"`
	BackupS3SecretKey   string `json:"backup_s3_secret_key"`
}

// JWTConfig estructura específica para configuración JWT
type JWTConfig struct {
	Secret          string
	Issuer          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Audience        string
}

// CORSConfig estructura específica para configuración CORS
type CORSConfig struct {
	AllowedOrigins    []string
	AllowedMethods    []string
	AllowedHeaders    []string
	ExposedHeaders    []string
	AllowCredentials  bool
	MaxAge            int
	PreflightContinue bool
}

// BackupConfig estructura específica para configuración de backup
type BackupConfig struct {
	Enabled             bool          `json:"enabled"`
	Interval            time.Duration `json:"interval"`
	RetentionDays       int           `json:"retention_days"`
	StorageType         string        `json:"storage_type"`
	StoragePath         string        `json:"storage_path"`
	CompressionEnabled  bool          `json:"compression_enabled"`
	CompressionLevel    int           `json:"compression_level"`
	ValidationEnabled   bool          `json:"validation_enabled"`
	NotifyOnSuccess     bool          `json:"notify_on_success"`
	NotifyOnFailure     bool          `json:"notify_on_failure"`
	NotificationEmail   string        `json:"notification_email"`
	NotificationWebhook string        `json:"notification_webhook"`
	RemoteEnabled       bool          `json:"remote_enabled"`
	S3Bucket            string        `json:"s3_bucket"`
	S3Region            string        `json:"s3_region"`
	S3AccessKey         string        `json:"s3_access_key"`
	S3SecretKey         string        `json:"s3_secret_key"`
}

func Load() *Config {
	// Cargar archivo .env si existe
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Parsear duraciones JWT de forma segura
	accessTTL := parseDuration("JWT_ACCESS_TTL", "15m")
	refreshTTL := parseDuration("JWT_REFRESH_TTL", "168h") // 7 días
	rateLimitWindow := parseDuration("RATE_LIMIT_WINDOW", "1m")

	// Parsear duraciones de backup
	backupInterval := parseDuration("BACKUP_INTERVAL", "24h")

	// Parsear duraciones de caché
	cacheDefaultTTL := parseDuration("CACHE_DEFAULT_TTL", "5m")
	cacheCleanupInterval := parseDuration("CACHE_CLEANUP_INTERVAL", "1h")
	cacheDashboardTTL := parseDuration("CACHE_DASHBOARD_TTL", "30s")
	cacheWorkflowTTL := parseDuration("CACHE_WORKFLOW_TTL", "5m")
	cacheUserTTL := parseDuration("CACHE_USER_TTL", "15m")
	cacheMetricsTTL := parseDuration("CACHE_METRICS_TTL", "1m")
	cacheQueueTTL := parseDuration("CACHE_QUEUE_TTL", "10s")
	cacheSystemTTL := parseDuration("CACHE_SYSTEM_TTL", "5m")
	cacheAuthTTL := parseDuration("CACHE_AUTH_TTL", "10m")
	cacheWarmupTimeout := parseDuration("CACHE_WARMUP_TIMEOUT", "30s")
	cacheInvalidationTimeout := parseDuration("CACHE_INVALIDATION_TIMEOUT", "5s")
	cacheBackupInterval := parseDuration("CACHE_BACKUP_INTERVAL", "1h")

	// Configuración principal
	cfg := &Config{
		ServerPort:  getEnv("PORT", "8081"),
		Environment: getEnv("ENVIRONMENT", "development"), // 🔧 CORREGIDO: de ENV a ENVIRONMENT
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// Configuración JWT segura
		JWTSecret:     getEnv("JWT_SECRET", generateDefaultJWTSecret()),
		JWTIssuer:     getEnv("JWT_ISSUER", "engine-api-workflow"),
		JWTAccessTTL:  accessTTL,
		JWTRefreshTTL: refreshTTL,
		JWTAudience:   getEnv("JWT_AUDIENCE", "engine-api"),
		JWTExpiresIn:  getEnv("JWT_EXPIRES_IN", "24h"), // Compatibilidad

		// Configuración de seguridad
		TrustedProxies:    parseStringSlice("TRUSTED_PROXIES", "127.0.0.1,::1"),
		RateLimitRequests: getEnvAsInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:   rateLimitWindow,

		// CORS Configuration Completa
		CORSAllowedOrigins:    parseStringSlice("CORS_ALLOWED_ORIGINS", getDefaultCORSOrigins()),
		CORSAllowedMethods:    parseStringSlice("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,OPTIONS,PATCH"),
		CORSAllowedHeaders:    parseStringSlice("CORS_ALLOWED_HEADERS", getDefaultCORSHeaders()),
		CORSExposedHeaders:    parseStringSlice("CORS_EXPOSED_HEADERS", "X-Total-Count,X-Request-ID"),
		CORSAllowCredentials:  getEnvAsBool("CORS_ALLOW_CREDENTIALS", true), // 🔧 TRUE para cookies
		CORSMaxAge:            getEnvAsInt("CORS_MAX_AGE", 3600),
		CORSPreflightContinue: getEnvAsBool("CORS_PREFLIGHT_CONTINUE", false),

		// Feature flags
		EnableTokenBlacklist: getEnvAsBool("ENABLE_TOKEN_BLACKLIST", true),
		EnableRateLimit:      getEnvAsBool("ENABLE_RATE_LIMIT", true),
		EnableCORS:           getEnvAsBool("ENABLE_CORS", true),

		// Web Interface Settings
		EnableWebInterface: getEnvAsBool("ENABLE_WEB_INTERFACE", true),
		StaticFilesPath:    getEnv("STATIC_FILES_PATH", "./web/static"),
		TemplatesPath:      getEnv("TEMPLATES_PATH", "./web/templates"),

		// CACHE CONFIGURATION - AGREGADO
		Cache: CacheConfiguration{
			Enabled:          getEnvAsBool("CACHE_ENABLED", true),
			DefaultTTL:       cacheDefaultTTL,
			CleanupInterval:  cacheCleanupInterval,
			MaxMemory:        getEnv("CACHE_MAX_MEMORY", "100MB"),
			CompressionLevel: getEnvAsInt("CACHE_COMPRESSION_LEVEL", 1),
			Serializer:       getEnv("CACHE_SERIALIZER", "json"),

			// TTL por dominio
			DashboardTTL: cacheDashboardTTL,
			WorkflowTTL:  cacheWorkflowTTL,
			UserTTL:      cacheUserTTL,
			MetricsTTL:   cacheMetricsTTL,
			QueueTTL:     cacheQueueTTL,
			SystemTTL:    cacheSystemTTL,
			AuthTTL:      cacheAuthTTL,

			// Configuración avanzada
			EnableCompression:  getEnvAsBool("CACHE_ENABLE_COMPRESSION", false),
			EnableMetrics:      getEnvAsBool("CACHE_ENABLE_METRICS", true),
			EnableDistribution: getEnvAsBool("CACHE_ENABLE_DISTRIBUTION", false),
			EnablePersistence:  getEnvAsBool("CACHE_ENABLE_PERSISTENCE", true),
			EnableWarmup:       getEnvAsBool("CACHE_ENABLE_WARMUP", true),

			// Configuración de warmup
			WarmupEnabled:     getEnvAsBool("CACHE_WARMUP_ENABLED", true),
			WarmupTimeout:     cacheWarmupTimeout,
			WarmupConcurrency: getEnvAsInt("CACHE_WARMUP_CONCURRENCY", 5),

			// Configuración de invalidación
			InvalidationEnabled: getEnvAsBool("CACHE_INVALIDATION_ENABLED", true),
			InvalidationBuffer:  getEnvAsInt("CACHE_INVALIDATION_BUFFER", 1000),
			InvalidationTimeout: cacheInvalidationTimeout,

			// Configuración de backup del caché
			BackupEnabled:  getEnvAsBool("CACHE_BACKUP_ENABLED", false),
			BackupInterval: cacheBackupInterval,
			BackupPath:     getEnv("CACHE_BACKUP_PATH", "./cache_backups"),
		},

		// BACKUP CONFIGURATION - NUEVOS CAMPOS
		BackupEnabled:       getEnvAsBool("BACKUP_ENABLED", false),
		BackupInterval:      backupInterval,
		BackupRetentionDays: getEnvAsInt("BACKUP_RETENTION_DAYS", 30),
		BackupStorageType:   getEnv("BACKUP_STORAGE_TYPE", "local"),
		BackupStoragePath:   getEnv("BACKUP_STORAGE_PATH", "./backups"),

		// Configuración avanzada de backup
		BackupCompressionEnabled:  getEnvAsBool("BACKUP_COMPRESSION_ENABLED", true),
		BackupCompressionLevel:    getEnvAsInt("BACKUP_COMPRESSION_LEVEL", 6),
		BackupValidationEnabled:   getEnvAsBool("BACKUP_VALIDATION_ENABLED", true),
		BackupNotifyOnSuccess:     getEnvAsBool("BACKUP_NOTIFY_ON_SUCCESS", false),
		BackupNotifyOnFailure:     getEnvAsBool("BACKUP_NOTIFY_ON_FAILURE", true),
		BackupNotificationEmail:   getEnv("BACKUP_NOTIFICATION_EMAIL", ""),
		BackupNotificationWebhook: getEnv("BACKUP_NOTIFICATION_WEBHOOK", ""),

		// Configuración de almacenamiento remoto
		BackupRemoteEnabled: getEnvAsBool("BACKUP_REMOTE_ENABLED", false),
		BackupS3Bucket:      getEnv("BACKUP_S3_BUCKET", ""),
		BackupS3Region:      getEnv("BACKUP_S3_REGION", "us-east-1"),
		BackupS3AccessKey:   getEnv("BACKUP_S3_ACCESS_KEY", ""),
		BackupS3SecretKey:   getEnv("BACKUP_S3_SECRET_KEY", ""),
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

	// Validar configuración antes de continuar
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	return cfg
}

// Validate valida toda la configuración al cargar
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

	// Validar CORS
	if err := c.ValidateCORS(); err != nil {
		return fmt.Errorf("CORS validation failed: %w", err)
	}

	// Validar Backup
	if err := c.ValidateBackup(); err != nil {
		return fmt.Errorf("Backup validation failed: %w", err)
	}

	return nil
}

// ValidateJWT valida específicamente la configuración JWT
func (c *Config) ValidateJWT() error {
	// Verificar longitud mínima del secret
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters, got %d", len(c.JWTSecret))
	}

	// Verificar que no sea el valor por defecto en producción
	if c.Environment == "production" {
		defaultSecrets := []string{
			"your-super-secret-jwt-key-change-this-in-production",
			"your-super-secret-jwt-key-change-this-in-production-make-it-longer-and-more-secure",
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

// ValidateServer valida la configuración del servidor
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

// ValidateDatabase valida la configuración de MongoDB
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

// ValidateRedis valida la configuración de Redis
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

// ValidateCORS valida la configuración de CORS
func (c *Config) ValidateCORS() error {
	if !c.EnableCORS {
		return nil // CORS deshabilitado, no validar
	}

	// Validar orígenes
	for _, origin := range c.CORSAllowedOrigins {
		if origin == "*" {
			// Wildcard permitido pero advertir en producción
			if c.IsProduction() {
				log.Printf("Warning: CORS wildcard (*) origin detected in production environment")
			}
			continue
		}

		// Validar formato de URL
		if _, err := url.Parse(origin); err != nil {
			return fmt.Errorf("invalid CORS origin format: %s", origin)
		}
	}

	// Validar métodos HTTP
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "HEAD"}
	for _, method := range c.CORSAllowedMethods {
		isValid := false
		for _, validMethod := range validMethods {
			if method == validMethod {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid CORS method: %s (valid: %v)", method, validMethods)
		}
	}

	// Advertir sobre configuraciones inseguras en producción
	if c.IsProduction() {
		if c.CORSAllowCredentials && len(c.CORSAllowedOrigins) > 0 {
			for _, origin := range c.CORSAllowedOrigins {
				if origin == "*" {
					return fmt.Errorf("CORS credentials cannot be true when origin is wildcard in production")
				}
			}
		}
	}

	return nil
}

// ValidateBackup valida la configuración de backup
func (c *Config) ValidateBackup() error {
	if !c.BackupEnabled {
		return nil // Backup deshabilitado, no validar
	}

	// Validar intervalo de backup
	if c.BackupInterval < 1*time.Hour {
		return fmt.Errorf("BACKUP_INTERVAL must be at least 1 hour, got %v", c.BackupInterval)
	}

	if c.BackupInterval > 30*24*time.Hour { // 30 días máximo
		return fmt.Errorf("BACKUP_INTERVAL should not exceed 30 days, got %v", c.BackupInterval)
	}

	// Validar días de retención
	if c.BackupRetentionDays < 1 {
		return fmt.Errorf("BACKUP_RETENTION_DAYS must be at least 1, got %d", c.BackupRetentionDays)
	}

	if c.BackupRetentionDays > 365*2 { // 2 años máximo
		return fmt.Errorf("BACKUP_RETENTION_DAYS should not exceed 730 days, got %d", c.BackupRetentionDays)
	}

	// Validar tipo de almacenamiento
	validStorageTypes := []string{"local", "s3", "gcs", "azure"}
	isValidStorage := false
	for _, storageType := range validStorageTypes {
		if c.BackupStorageType == storageType {
			isValidStorage = true
			break
		}
	}
	if !isValidStorage {
		return fmt.Errorf("invalid BACKUP_STORAGE_TYPE: %s (valid: %v)", c.BackupStorageType, validStorageTypes)
	}

	// Validar ruta de almacenamiento local
	if c.BackupStorageType == "local" {
		if c.BackupStoragePath == "" {
			return fmt.Errorf("BACKUP_STORAGE_PATH cannot be empty for local storage")
		}

		// Verificar que la ruta no sea un directorio del sistema crítico
		dangerousPaths := []string{"/", "/bin", "/boot", "/dev", "/etc", "/lib", "/proc", "/root", "/sbin", "/sys", "/usr", "/var"}
		for _, dangerousPath := range dangerousPaths {
			if strings.HasPrefix(c.BackupStoragePath, dangerousPath) {
				return fmt.Errorf("BACKUP_STORAGE_PATH cannot be in system directory: %s", c.BackupStoragePath)
			}
		}
	}

	// Validar configuración de S3 si está habilitado
	if c.BackupStorageType == "s3" || c.BackupRemoteEnabled {
		if c.BackupS3Bucket == "" {
			return fmt.Errorf("BACKUP_S3_BUCKET is required for S3 storage")
		}
		if c.BackupS3AccessKey == "" {
			return fmt.Errorf("BACKUP_S3_ACCESS_KEY is required for S3 storage")
		}
		if c.BackupS3SecretKey == "" {
			return fmt.Errorf("BACKUP_S3_SECRET_KEY is required for S3 storage")
		}
	}

	// Validar nivel de compresión
	if c.BackupCompressionLevel < 1 || c.BackupCompressionLevel > 9 {
		return fmt.Errorf("BACKUP_COMPRESSION_LEVEL must be between 1 and 9, got %d", c.BackupCompressionLevel)
	}

	// Validar configuración de notificaciones
	if c.BackupNotifyOnSuccess || c.BackupNotifyOnFailure {
		if c.BackupNotificationEmail == "" && c.BackupNotificationWebhook == "" {
			return fmt.Errorf("BACKUP_NOTIFICATION_EMAIL or BACKUP_NOTIFICATION_WEBHOOK required when notifications are enabled")
		}
	}

	// Advertir sobre configuraciones en producción
	if c.IsProduction() {
		if c.BackupStorageType == "local" {
			log.Printf("Warning: Using local backup storage in production. Consider using remote storage for better reliability.")
		}

		if !c.BackupValidationEnabled {
			log.Printf("Warning: Backup validation is disabled in production. This is not recommended.")
		}

		if c.BackupRetentionDays < 7 {
			log.Printf("Warning: Backup retention is less than 7 days in production. Consider increasing for better recovery options.")
		}
	}

	return nil
}

// GetJWTConfig retorna configuración específica para JWT
func (c *Config) GetJWTConfig() JWTConfig {
	return JWTConfig{
		Secret:          c.JWTSecret,
		Issuer:          c.JWTIssuer,
		AccessTokenTTL:  c.JWTAccessTTL,
		RefreshTokenTTL: c.JWTRefreshTTL,
		Audience:        c.JWTAudience,
	}
}

// GetCORSConfig retorna configuración específica para CORS
func (c *Config) GetCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:    c.GetCORSOrigins(),
		AllowedMethods:    c.CORSAllowedMethods,
		AllowedHeaders:    c.CORSAllowedHeaders,
		ExposedHeaders:    c.CORSExposedHeaders,
		AllowCredentials:  c.CORSAllowCredentials,
		MaxAge:            c.CORSMaxAge,
		PreflightContinue: c.CORSPreflightContinue,
	}
}

// GetBackupConfig retorna configuración específica para backup
func (c *Config) GetBackupConfig() BackupConfig {
	return BackupConfig{
		Enabled:             c.BackupEnabled,
		Interval:            c.BackupInterval,
		RetentionDays:       c.BackupRetentionDays,
		StorageType:         c.BackupStorageType,
		StoragePath:         c.BackupStoragePath,
		CompressionEnabled:  c.BackupCompressionEnabled,
		CompressionLevel:    c.BackupCompressionLevel,
		ValidationEnabled:   c.BackupValidationEnabled,
		NotifyOnSuccess:     c.BackupNotifyOnSuccess,
		NotifyOnFailure:     c.BackupNotifyOnFailure,
		NotificationEmail:   c.BackupNotificationEmail,
		NotificationWebhook: c.BackupNotificationWebhook,
		RemoteEnabled:       c.BackupRemoteEnabled,
		S3Bucket:            c.BackupS3Bucket,
		S3Region:            c.BackupS3Region,
		S3AccessKey:         c.BackupS3AccessKey,
		S3SecretKey:         c.BackupS3SecretKey,
	}
}

// IsProduction retorna si está en ambiente de producción
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment retorna si está en ambiente de desarrollo
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// MEJORAss: GetCORSOrigins retorna los orígenes CORS permitidos con lógica específica para tu frontend
func (c *Config) GetCORSOrigins() []string {
	// Siempre incluir el origen del servidor para el frontend web
	baseOrigins := []string{
		fmt.Sprintf("http://localhost:%s", c.ServerPort),
		fmt.Sprintf("http://127.0.0.1:%s", c.ServerPort),
	}

	if c.IsProduction() {
		// En producción, usar solo los configurados + servidor
		return removeDuplicates(append(baseOrigins, c.CORSAllowedOrigins...))
	}

	// En desarrollo, agregar orígenes comunes de desarrollo
	devOrigins := []string{
		"http://localhost:3000", // React
		"http://localhost:3001", // React alternativo
		"http://localhost:8080", // Vue
		"http://localhost:4200", // Angular
		"http://127.0.0.1:3000",
		"http://127.0.0.1:3001",
	}

	allOrigins := append(baseOrigins, c.CORSAllowedOrigins...)
	allOrigins = append(allOrigins, devOrigins...)

	return removeDuplicates(allOrigins)
}

// Funciones helper para valores por defecto

func getDefaultCORSOrigins() string {
	return "http://localhost:8081,http://127.0.0.1:8081,http://localhost:3000"
}

func getDefaultCORSHeaders() string {
	return "Accept,Authorization,Content-Type,X-CSRF-Token,X-Requested-With,Cache-Control,X-File-Name"
}

// Helper functions

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

// getEnvAsBool parsea variables de entorno como boolean
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

// parseDuration parsea duraciones de forma segura
func parseDuration(envKey, defaultValue string) time.Duration {
	value := getEnv(envKey, defaultValue)
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("Warning: Invalid duration for %s (%s), using default: %s", envKey, value, defaultValue)
		duration, _ = time.ParseDuration(defaultValue)
	}
	return duration
}

// parseStringSlice parsea una lista separada por comas
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

// calculateEntropy calcula la entropía de una cadena
func calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Contar frecuencia de cada carácter
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

// generateDefaultJWTSecret genera un secret por defecto seguro
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

// getDefaultMongoURI retorna URI por defecto según el ambiente
func getDefaultMongoURI(environment string) string {
	switch environment {
	case "production":
		return "mongodb://localhost:27017/engine_workflow" // Sin credenciales por defecto
	case "development":
		return "mongodb://admin:password123@localhost:27018/engine_workflow?authSource=admin" // 🔧 PUERTO 27018
	default:
		return "mongodb://localhost:27017/engine_workflow"
	}
}

// getDefaultRedisHost retorna host por defecto según el ambiente
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

// removeDuplicates remueve elementos duplicados de un slice
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

// 🔧 MEJORADO: LogConfig registra la configuración actual (sin secrets)
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
	log.Printf("  Features: BlackList=%v, RateLimit=%v, CORS=%v, WebUI=%v",
		c.EnableTokenBlacklist, c.EnableRateLimit, c.EnableCORS, c.EnableWebInterface)

	//  CORS Configuration Log
	if c.EnableCORS {
		log.Printf("  CORS Origins: %v", c.GetCORSOrigins())
		log.Printf("  CORS Allow Credentials: %v", c.CORSAllowCredentials)
		log.Printf("  CORS Methods: %v", c.CORSAllowedMethods)
	}

	//  Cache Configuration Log
	if c.Cache.Enabled {
		log.Printf("  Cache Enabled: %v", c.Cache.Enabled)
		log.Printf("  Cache Default TTL: %v", c.Cache.DefaultTTL)
		log.Printf("  Cache Max Memory: %s", c.Cache.MaxMemory)
		log.Printf("  Cache Compression: %v (level %d)", c.Cache.EnableCompression, c.Cache.CompressionLevel)
		log.Printf("  Cache Warmup: %v", c.Cache.EnableWarmup)
	} else {
		log.Printf("  Cache: Disabled")
	}

	//  Backup Configuration Log
	if c.BackupEnabled {
		log.Printf("  Backup Enabled: %v", c.BackupEnabled)
		log.Printf("  Backup Interval: %v", c.BackupInterval)
		log.Printf("  Backup Retention: %d days", c.BackupRetentionDays)
		log.Printf("  Backup Storage: %s (%s)", c.BackupStorageType, c.BackupStoragePath)
		log.Printf("  Backup Compression: %v (level %d)", c.BackupCompressionEnabled, c.BackupCompressionLevel)
		log.Printf("  Backup Validation: %v", c.BackupValidationEnabled)
		log.Printf("  Backup Notifications: Success=%v, Failure=%v", c.BackupNotifyOnSuccess, c.BackupNotifyOnFailure)
		if c.BackupRemoteEnabled {
			log.Printf("  Backup Remote Storage: S3 Bucket=%s, Region=%s", c.BackupS3Bucket, c.BackupS3Region)
		}
	} else {
		log.Printf("  Backup: Disabled")
	}

	// Nunca logear secrets
	secretLength := len(c.JWTSecret)
	if secretLength >= 4 {
		log.Printf("  JWT Secret: [%d characters] %s***", secretLength, c.JWTSecret[:4])
	} else {
		log.Printf("  JWT Secret: [%d characters] ***", secretLength)
	}

	// Backup secrets logging (solo longitud)
	if c.BackupS3SecretKey != "" {
		log.Printf("  Backup S3 Secret: [%d characters] ***", len(c.BackupS3SecretKey))
	}
}
