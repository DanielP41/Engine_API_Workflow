package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig configuración SMTP para envío de emails
type SMTPConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Host        string `json:"host" yaml:"host"`
	Port        int    `json:"port" yaml:"port"`
	Username    string `json:"username" yaml:"username"`
	Password    string `json:"password" yaml:"password"`
	FromEmail   string `json:"from_email" yaml:"from_email"`
	FromName    string `json:"from_name" yaml:"from_name"`
	UseTLS      bool   `json:"use_tls" yaml:"use_tls"`
	UseStartTLS bool   `json:"use_starttls" yaml:"use_starttls"`
	SkipVerify  bool   `json:"skip_verify" yaml:"skip_verify"`

	// Configuración avanzada
	ConnTimeout time.Duration `json:"conn_timeout" yaml:"conn_timeout"`
	SendTimeout time.Duration `json:"send_timeout" yaml:"send_timeout"`
	MaxRetries  int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay  time.Duration `json:"retry_delay" yaml:"retry_delay"`
	RateLimit   int           `json:"rate_limit" yaml:"rate_limit"`   // emails por minuto
	BurstLimit  int           `json:"burst_limit" yaml:"burst_limit"` // ráfaga máxima
}

// NotificationConfig configuración del sistema de notificaciones
type NotificationConfig struct {
	// Configuración general
	Enabled         bool `json:"enabled" yaml:"enabled"`
	EnableWorkers   bool `json:"enable_workers" yaml:"enable_workers"`
	EnableScheduled bool `json:"enable_scheduled" yaml:"enable_scheduled"`
	EnableRetries   bool `json:"enable_retries" yaml:"enable_retries"`
	EnableCleanup   bool `json:"enable_cleanup" yaml:"enable_cleanup"`

	// Configuración de procesamiento
	MaxConcurrentJobs  int           `json:"max_concurrent_jobs" yaml:"max_concurrent_jobs"`
	ProcessingInterval time.Duration `json:"processing_interval" yaml:"processing_interval"`
	RetryInterval      time.Duration `json:"retry_interval" yaml:"retry_interval"`
	CleanupInterval    time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`

	// Configuración de reintentos
	DefaultMaxAttempts int           `json:"default_max_attempts" yaml:"default_max_attempts"`
	InitialRetryDelay  time.Duration `json:"initial_retry_delay" yaml:"initial_retry_delay"`
	MaxRetryDelay      time.Duration `json:"max_retry_delay" yaml:"max_retry_delay"`
	RetryBackoffFactor float64       `json:"retry_backoff_factor" yaml:"retry_backoff_factor"`

	// Configuración de limpieza
	CleanupAfterDays int `json:"cleanup_after_days" yaml:"cleanup_after_days"`

	// Configuración de colas Redis
	RedisQueuePrefix string `json:"redis_queue_prefix" yaml:"redis_queue_prefix"`

	// Rate limiting
	RateLimits NotificationRateLimits `json:"rate_limits" yaml:"rate_limits"`
}

// NotificationRateLimits configuración de rate limiting para notificaciones
type NotificationRateLimits struct {
	EmailSending RateLimitRule `json:"email_sending" yaml:"email_sending"`
	TemplateOps  RateLimitRule `json:"template_ops" yaml:"template_ops"`
	Management   RateLimitRule `json:"management" yaml:"management"`
	AdminOps     RateLimitRule `json:"admin_ops" yaml:"admin_ops"`
}

// RateLimitRule regla de rate limiting
type RateLimitRule struct {
	Limit  int `json:"limit" yaml:"limit"`
	Window int `json:"window" yaml:"window"` // segundos
}

// TemplateConfig configuración de templates
type TemplateConfig struct {
	CreateDefaultTemplates bool     `json:"create_default_templates" yaml:"create_default_templates"`
	DefaultLanguage        string   `json:"default_language" yaml:"default_language"`
	AllowedLanguages       []string `json:"allowed_languages" yaml:"allowed_languages"`
	EnableVersioning       bool     `json:"enable_versioning" yaml:"enable_versioning"`
	MaxTemplateSize        int      `json:"max_template_size" yaml:"max_template_size"` // bytes
}

// loadSMTPConfig carga la configuración SMTP desde variables de entorno
func loadSMTPConfig() SMTPConfig {
	config := SMTPConfig{
		Enabled:     getNotificationBoolEnv("SMTP_ENABLED", true),
		Host:        getNotificationEnv("SMTP_HOST", "smtp.gmail.com"),
		Port:        getNotificationIntEnv("SMTP_PORT", 587),
		Username:    getNotificationEnv("SMTP_USERNAME", ""),
		Password:    getNotificationEnv("SMTP_PASSWORD", ""),
		FromEmail:   getNotificationEnv("SMTP_FROM_EMAIL", ""),
		FromName:    getNotificationEnv("SMTP_FROM_NAME", "Engine API Workflow"),
		UseTLS:      getNotificationBoolEnv("SMTP_USE_TLS", true),
		UseStartTLS: getNotificationBoolEnv("SMTP_USE_STARTTLS", true),
		SkipVerify:  getNotificationBoolEnv("SMTP_SKIP_VERIFY", false),

		// Configuración avanzada con valores por defecto
		ConnTimeout: getNotificationDurationEnv("SMTP_CONN_TIMEOUT", 10*time.Second),
		SendTimeout: getNotificationDurationEnv("SMTP_SEND_TIMEOUT", 30*time.Second),
		MaxRetries:  getNotificationIntEnv("SMTP_MAX_RETRIES", 3),
		RetryDelay:  getNotificationDurationEnv("SMTP_RETRY_DELAY", 5*time.Second),
		RateLimit:   getNotificationIntEnv("SMTP_RATE_LIMIT", 60), // 60 emails por minuto
		BurstLimit:  getNotificationIntEnv("SMTP_BURST_LIMIT", 10),
	}

	// Si FromEmail no está configurado, usar Username
	if config.FromEmail == "" {
		config.FromEmail = config.Username
	}

	return config
}

// loadNotificationConfig carga la configuración de notificaciones
func loadNotificationConfig() NotificationConfig {
	return NotificationConfig{
		Enabled:         getNotificationBoolEnv("NOTIFICATIONS_ENABLED", true),
		EnableWorkers:   getNotificationBoolEnv("NOTIFICATIONS_ENABLE_WORKERS", true),
		EnableScheduled: getNotificationBoolEnv("NOTIFICATIONS_ENABLE_SCHEDULED", true),
		EnableRetries:   getNotificationBoolEnv("NOTIFICATIONS_ENABLE_RETRIES", true),
		EnableCleanup:   getNotificationBoolEnv("NOTIFICATIONS_ENABLE_CLEANUP", true),

		MaxConcurrentJobs:  getNotificationIntEnv("NOTIFICATIONS_MAX_CONCURRENT_JOBS", 5),
		ProcessingInterval: getNotificationDurationEnv("NOTIFICATIONS_PROCESSING_INTERVAL", 10*time.Second),
		RetryInterval:      getNotificationDurationEnv("NOTIFICATIONS_RETRY_INTERVAL", 30*time.Second),
		CleanupInterval:    getNotificationDurationEnv("NOTIFICATIONS_CLEANUP_INTERVAL", 5*time.Minute),

		DefaultMaxAttempts: getNotificationIntEnv("NOTIFICATIONS_DEFAULT_MAX_ATTEMPTS", 3),
		InitialRetryDelay:  getNotificationDurationEnv("NOTIFICATIONS_INITIAL_RETRY_DELAY", 30*time.Second),
		MaxRetryDelay:      getNotificationDurationEnv("NOTIFICATIONS_MAX_RETRY_DELAY", 5*time.Minute),
		RetryBackoffFactor: getNotificationFloatEnv("NOTIFICATIONS_RETRY_BACKOFF_FACTOR", 2.0),

		CleanupAfterDays: getNotificationIntEnv("NOTIFICATIONS_CLEANUP_AFTER_DAYS", 90),
		RedisQueuePrefix: getNotificationEnv("NOTIFICATIONS_REDIS_QUEUE_PREFIX", "notifications:"),

		RateLimits: loadNotificationRateLimits(),
	}
}

// loadNotificationRateLimits carga la configuración de rate limiting
func loadNotificationRateLimits() NotificationRateLimits {
	return NotificationRateLimits{
		EmailSending: RateLimitRule{
			Limit:  getNotificationIntEnv("NOTIFICATIONS_RATE_LIMIT_EMAIL_SENDING_LIMIT", 10),
			Window: getNotificationIntEnv("NOTIFICATIONS_RATE_LIMIT_EMAIL_SENDING_WINDOW", 60),
		},
		TemplateOps: RateLimitRule{
			Limit:  getNotificationIntEnv("NOTIFICATIONS_RATE_LIMIT_TEMPLATE_OPS_LIMIT", 20),
			Window: getNotificationIntEnv("NOTIFICATIONS_RATE_LIMIT_TEMPLATE_OPS_WINDOW", 60),
		},
		Management: RateLimitRule{
			Limit:  getNotificationIntEnv("NOTIFICATIONS_RATE_LIMIT_MANAGEMENT_LIMIT", 50),
			Window: getNotificationIntEnv("NOTIFICATIONS_RATE_LIMIT_MANAGEMENT_WINDOW", 60),
		},
		AdminOps: RateLimitRule{
			Limit:  getNotificationIntEnv("NOTIFICATIONS_RATE_LIMIT_ADMIN_OPS_LIMIT", 5),
			Window: getNotificationIntEnv("NOTIFICATIONS_RATE_LIMIT_ADMIN_OPS_WINDOW", 300),
		},
	}
}

// loadTemplateConfig carga la configuración de templates
func loadTemplateConfig() TemplateConfig {
	return TemplateConfig{
		CreateDefaultTemplates: getNotificationBoolEnv("TEMPLATES_CREATE_DEFAULT", true),
		DefaultLanguage:        getNotificationEnv("TEMPLATES_DEFAULT_LANGUAGE", "en"),
		AllowedLanguages:       getNotificationStringSliceEnv("TEMPLATES_ALLOWED_LANGUAGES", []string{"en", "es"}),
		EnableVersioning:       getNotificationBoolEnv("TEMPLATES_ENABLE_VERSIONING", true),
		MaxTemplateSize:        getNotificationIntEnv("TEMPLATES_MAX_SIZE", 1024*1024), // 1MB
	}
}

// ValidateSMTPConfig valida la configuración SMTP
func (c *SMTPConfig) ValidateSMTPConfig() error {
	if !c.Enabled {
		return nil // Si está deshabilitado, no validar
	}

	if c.Host == "" {
		return fmt.Errorf("SMTP_HOST is required when SMTP is enabled")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("SMTP_PORT must be between 1 and 65535")
	}

	if c.Username == "" {
		return fmt.Errorf("SMTP_USERNAME is required when SMTP is enabled")
	}

	if c.Password == "" {
		return fmt.Errorf("SMTP_PASSWORD is required when SMTP is enabled")
	}

	if c.FromEmail == "" {
		return fmt.Errorf("SMTP_FROM_EMAIL is required when SMTP is enabled")
	}

	if c.RateLimit <= 0 {
		return fmt.Errorf("SMTP_RATE_LIMIT must be greater than 0")
	}

	if c.BurstLimit <= 0 {
		return fmt.Errorf("SMTP_BURST_LIMIT must be greater than 0")
	}

	return nil
}

// ValidateNotificationConfig valida la configuración de notificaciones
func (c *NotificationConfig) ValidateNotificationConfig() error {
	if !c.Enabled {
		return nil
	}

	if c.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("NOTIFICATIONS_MAX_CONCURRENT_JOBS must be greater than 0")
	}

	if c.ProcessingInterval <= 0 {
		return fmt.Errorf("NOTIFICATIONS_PROCESSING_INTERVAL must be greater than 0")
	}

	if c.DefaultMaxAttempts <= 0 {
		return fmt.Errorf("NOTIFICATIONS_DEFAULT_MAX_ATTEMPTS must be greater than 0")
	}

	if c.RetryBackoffFactor <= 1.0 {
		return fmt.Errorf("NOTIFICATIONS_RETRY_BACKOFF_FACTOR must be greater than 1.0")
	}

	if c.CleanupAfterDays <= 0 {
		return fmt.Errorf("NOTIFICATIONS_CLEANUP_AFTER_DAYS must be greater than 0")
	}

	// Validar rate limits
	if err := c.RateLimits.Validate(); err != nil {
		return fmt.Errorf("rate limits validation failed: %w", err)
	}

	return nil
}

// Validate valida las reglas de rate limiting
func (r *NotificationRateLimits) Validate() error {
	rules := map[string]RateLimitRule{
		"email_sending": r.EmailSending,
		"template_ops":  r.TemplateOps,
		"management":    r.Management,
		"admin_ops":     r.AdminOps,
	}

	for name, rule := range rules {
		if rule.Limit <= 0 {
			return fmt.Errorf("%s limit must be greater than 0", name)
		}
		if rule.Window <= 0 {
			return fmt.Errorf("%s window must be greater than 0", name)
		}
	}

	return nil
}

// ValidateTemplateConfig valida la configuración de templates
func (c *TemplateConfig) ValidateTemplateConfig() error {
	if c.DefaultLanguage == "" {
		return fmt.Errorf("TEMPLATES_DEFAULT_LANGUAGE cannot be empty")
	}

	if len(c.AllowedLanguages) == 0 {
		return fmt.Errorf("TEMPLATES_ALLOWED_LANGUAGES cannot be empty")
	}

	// Verificar que el idioma por defecto esté en la lista de permitidos
	defaultFound := false
	for _, lang := range c.AllowedLanguages {
		if lang == c.DefaultLanguage {
			defaultFound = true
			break
		}
	}

	if !defaultFound {
		return fmt.Errorf("TEMPLATES_DEFAULT_LANGUAGE '%s' must be in TEMPLATES_ALLOWED_LANGUAGES", c.DefaultLanguage)
	}

	if c.MaxTemplateSize <= 0 {
		return fmt.Errorf("TEMPLATES_MAX_SIZE must be greater than 0")
	}

	return nil
}

// Helper functions para variables de entorno - Renombradas para evitar conflictos

func getNotificationEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getNotificationIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getNotificationBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getNotificationFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getNotificationDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getNotificationStringSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Asumimos que están separados por comas
		parts := make([]string, 0)
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return defaultValue
}

// GetSMTPConfigForEnvironment obtiene configuración SMTP según el entorno
func GetSMTPConfigForEnvironment(environment string) SMTPConfig {
	config := loadSMTPConfig()

	switch environment {
	case "production":
		// Configuración más estricta para producción
		config.SkipVerify = false
		config.RateLimit = 30 // Más conservador
		config.BurstLimit = 5
		config.MaxRetries = 5 // Más reintentos en producción
	case "development":
		// Configuración más permisiva para desarrollo
		config.RateLimit = 100
		config.BurstLimit = 20
		config.SkipVerify = true // Permitir certificados auto-firmados en dev
	case "testing":
		// Configuración para tests
		config.Enabled = false // Deshabilitado por defecto en tests
		config.RateLimit = 1000
		config.BurstLimit = 100
	}

	return config
}

// GetNotificationConfigForEnvironment obtiene configuración de notificaciones según el entorno
func GetNotificationConfigForEnvironment(environment string) NotificationConfig {
	config := loadNotificationConfig()

	switch environment {
	case "production":
		// Configuración para producción
		config.MaxConcurrentJobs = 10
		config.ProcessingInterval = 5 * time.Second
		config.CleanupAfterDays = 180 // Retener más tiempo en producción

		// Rate limits más estrictos
		config.RateLimits.EmailSending.Limit = 5
		config.RateLimits.AdminOps.Limit = 3
		config.RateLimits.AdminOps.Window = 600

	case "development":
		// Configuración para desarrollo
		config.MaxConcurrentJobs = 2
		config.ProcessingInterval = 30 * time.Second
		config.CleanupAfterDays = 7 // Limpiar más frecuentemente en dev

		// Rate limits más permisivos
		config.RateLimits.EmailSending.Limit = 50
		config.RateLimits.TemplateOps.Limit = 100
		config.RateLimits.Management.Limit = 200

	case "testing":
		// Configuración para tests
		config.EnableWorkers = false // Deshabilitar workers en tests
		config.ProcessingInterval = 100 * time.Millisecond
		config.RetryInterval = 100 * time.Millisecond
		config.CleanupInterval = 1 * time.Second

		// Rate limits muy permisivos para tests
		config.RateLimits.EmailSending.Limit = 1000
		config.RateLimits.TemplateOps.Limit = 1000
		config.RateLimits.Management.Limit = 1000
		config.RateLimits.AdminOps.Limit = 1000
	}

	return config
}

// Ejemplo de variables de entorno para .env.example:
/*
# SMTP Configuration
SMTP_ENABLED=true
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM_EMAIL=noreply@yourdomain.com
SMTP_FROM_NAME="Your App Name"
SMTP_USE_TLS=true
SMTP_USE_STARTTLS=true
SMTP_SKIP_VERIFY=false
SMTP_RATE_LIMIT=60
SMTP_BURST_LIMIT=10

# Notification System Configuration
NOTIFICATIONS_ENABLED=true
NOTIFICATIONS_ENABLE_WORKERS=true
NOTIFICATIONS_ENABLE_SCHEDULED=true
NOTIFICATIONS_ENABLE_RETRIES=true
NOTIFICATIONS_ENABLE_CLEANUP=true
NOTIFICATIONS_MAX_CONCURRENT_JOBS=5
NOTIFICATIONS_PROCESSING_INTERVAL=10s
NOTIFICATIONS_RETRY_INTERVAL=30s
NOTIFICATIONS_CLEANUP_INTERVAL=5m
NOTIFICATIONS_DEFAULT_MAX_ATTEMPTS=3
NOTIFICATIONS_CLEANUP_AFTER_DAYS=90

# Rate Limiting
NOTIFICATIONS_RATE_LIMIT_EMAIL_SENDING_LIMIT=10
NOTIFICATIONS_RATE_LIMIT_EMAIL_SENDING_WINDOW=60
NOTIFICATIONS_RATE_LIMIT_TEMPLATE_OPS_LIMIT=20
NOTIFICATIONS_RATE_LIMIT_TEMPLATE_OPS_WINDOW=60
NOTIFICATIONS_RATE_LIMIT_MANAGEMENT_LIMIT=50
NOTIFICATIONS_RATE_LIMIT_MANAGEMENT_WINDOW=60
NOTIFICATIONS_RATE_LIMIT_ADMIN_OPS_LIMIT=5
NOTIFICATIONS_RATE_LIMIT_ADMIN_OPS_WINDOW=300

# Template Configuration
TEMPLATES_CREATE_DEFAULT=true
TEMPLATES_DEFAULT_LANGUAGE=en
TEMPLATES_ALLOWED_LANGUAGES=en,es
TEMPLATES_ENABLE_VERSIONING=true
TEMPLATES_MAX_SIZE=1048576
*/
