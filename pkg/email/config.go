package email

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"
)

// ================================
// CONSTANTES Y CONFIGURACIÓN
// ================================

const (
	// Timeouts por defecto
	DefaultConnectTimeout = 30 * time.Second
	DefaultSendTimeout    = 60 * time.Second
	DefaultIdleTimeout    = 5 * time.Minute

	// Límites por defecto
	DefaultMaxConnections = 10
	DefaultMaxMessageSize = 25 * 1024 * 1024 // 25MB
	DefaultRateLimit      = 100              // emails por minuto
	DefaultRetryAttempts  = 3
	DefaultRetryDelay     = 5 * time.Second

	// Configuración SMTP por defecto
	DefaultSMTPPort = 587
	DefaultSMTPTLS  = true

	// Headers y configuración
	DefaultUserAgent = "Engine-API-Workflow/1.0"
	DefaultCharset   = "UTF-8"
)

// Proveedores SMTP conocidos
var KnownProviders = map[string]ProviderConfig{
	"gmail": {
		Host:         "smtp.gmail.com",
		Port:         587,
		RequiresTLS:  true,
		RequiresAuth: true,
		Name:         "Gmail",
	},
	"outlook": {
		Host:         "smtp-mail.outlook.com",
		Port:         587,
		RequiresTLS:  true,
		RequiresAuth: true,
		Name:         "Outlook/Hotmail",
	},
	"yahoo": {
		Host:         "smtp.mail.yahoo.com",
		Port:         587,
		RequiresTLS:  true,
		RequiresAuth: true,
		Name:         "Yahoo Mail",
	},
	"sendgrid": {
		Host:         "smtp.sendgrid.net",
		Port:         587,
		RequiresTLS:  true,
		RequiresAuth: true,
		Name:         "SendGrid",
	},
	"mailgun": {
		Host:         "smtp.mailgun.org",
		Port:         587,
		RequiresTLS:  true,
		RequiresAuth: true,
		Name:         "Mailgun",
	},
}

// ================================
// ESTRUCTURAS DE CONFIGURACIÓN
// ================================

// Config configuración principal del servicio de email
type Config struct {
	// Configuración SMTP
	SMTP SMTPConfig `json:"smtp" yaml:"smtp" mapstructure:"smtp"`

	// Configuración de seguridad
	Security SecurityConfig `json:"security" yaml:"security" mapstructure:"security"`

	// Configuración de rendimiento
	Performance PerformanceConfig `json:"performance" yaml:"performance" mapstructure:"performance"`

	// Configuración de reintentos
	Retry RetryConfig `json:"retry" yaml:"retry" mapstructure:"retry"`

	// Configuración de templates
	Templates TemplateConfig `json:"templates" yaml:"templates" mapstructure:"templates"`

	// Configuración de logging
	Logging LoggingConfig `json:"logging" yaml:"logging" mapstructure:"logging"`

	// Headers personalizados
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty" mapstructure:"headers"`

	// Configuración de desarrollo
	Development DevelopmentConfig `json:"development" yaml:"development" mapstructure:"development"`
}

// SMTPConfig configuración del servidor SMTP
type SMTPConfig struct {
	// Conexión
	Host string `json:"host" yaml:"host" mapstructure:"host" validate:"required"`
	Port int    `json:"port" yaml:"port" mapstructure:"port" validate:"min=1,max=65535"`

	// Autenticación
	Username string `json:"username" yaml:"username" mapstructure:"username"`
	Password string `json:"password" yaml:"password" mapstructure:"password"`

	// TLS/SSL
	EnableTLS       bool        `json:"enable_tls" yaml:"enable_tls" mapstructure:"enable_tls"`
	TLSConfig       *tls.Config `json:"-" yaml:"-" mapstructure:"-"`
	TLSSkipVerify   bool        `json:"tls_skip_verify" yaml:"tls_skip_verify" mapstructure:"tls_skip_verify"`
	RequireSTARTTLS bool        `json:"require_starttls" yaml:"require_starttls" mapstructure:"require_starttls"`

	// Configuración avanzada
	AuthMethod AuthMethod `json:"auth_method" yaml:"auth_method" mapstructure:"auth_method"`
	LocalName  string     `json:"local_name,omitempty" yaml:"local_name,omitempty" mapstructure:"local_name"`
	KeepAlive  bool       `json:"keep_alive" yaml:"keep_alive" mapstructure:"keep_alive"`

	// Timeouts
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout" mapstructure:"connect_timeout"`
	SendTimeout    time.Duration `json:"send_timeout" yaml:"send_timeout" mapstructure:"send_timeout"`
	IdleTimeout    time.Duration `json:"idle_timeout" yaml:"idle_timeout" mapstructure:"idle_timeout"`

	// Configuración del remitente por defecto
	DefaultFrom SenderConfig `json:"default_from" yaml:"default_from" mapstructure:"default_from"`
}

// SenderConfig configuración del remitente
type SenderConfig struct {
	Email string `json:"email" yaml:"email" mapstructure:"email" validate:"required,email"`
	Name  string `json:"name,omitempty" yaml:"name,omitempty" mapstructure:"name"`
}

// SecurityConfig configuración de seguridad
type SecurityConfig struct {
	// Validación de destinatarios
	AllowedDomains  []string `json:"allowed_domains,omitempty" yaml:"allowed_domains,omitempty" mapstructure:"allowed_domains"`
	BlockedDomains  []string `json:"blocked_domains,omitempty" yaml:"blocked_domains,omitempty" mapstructure:"blocked_domains"`
	ValidateEmails  bool     `json:"validate_emails" yaml:"validate_emails" mapstructure:"validate_emails"`
	ValidateDomains bool     `json:"validate_domains" yaml:"validate_domains" mapstructure:"validate_domains"`

	// Límites de contenido
	MaxRecipients     int   `json:"max_recipients" yaml:"max_recipients" mapstructure:"max_recipients"`
	MaxMessageSize    int64 `json:"max_message_size" yaml:"max_message_size" mapstructure:"max_message_size"`
	MaxAttachmentSize int64 `json:"max_attachment_size" yaml:"max_attachment_size" mapstructure:"max_attachment_size"`
	MaxAttachments    int   `json:"max_attachments" yaml:"max_attachments" mapstructure:"max_attachments"`

	// Filtros de contenido
	BlockedWords    []string `json:"blocked_words,omitempty" yaml:"blocked_words,omitempty" mapstructure:"blocked_words"`
	RequiredHeaders []string `json:"required_headers,omitempty" yaml:"required_headers,omitempty" mapstructure:"required_headers"`

	// Cifrado
	EncryptionEnabled bool   `json:"encryption_enabled" yaml:"encryption_enabled" mapstructure:"encryption_enabled"`
	SigningEnabled    bool   `json:"signing_enabled" yaml:"signing_enabled" mapstructure:"signing_enabled"`
	KeyPath           string `json:"key_path,omitempty" yaml:"key_path,omitempty" mapstructure:"key_path"`
	CertPath          string `json:"cert_path,omitempty" yaml:"cert_path,omitempty" mapstructure:"cert_path"`
}

// PerformanceConfig configuración de rendimiento
type PerformanceConfig struct {
	// Pool de conexiones
	MaxConnections    int           `json:"max_connections" yaml:"max_connections" mapstructure:"max_connections"`
	MaxIdleConns      int           `json:"max_idle_conns" yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout" mapstructure:"connection_timeout"`

	// Rate limiting
	RateLimit       int           `json:"rate_limit" yaml:"rate_limit" mapstructure:"rate_limit"` // emails por minuto
	BurstLimit      int           `json:"burst_limit" yaml:"burst_limit" mapstructure:"burst_limit"`
	RateLimitWindow time.Duration `json:"rate_limit_window" yaml:"rate_limit_window" mapstructure:"rate_limit_window"`

	// Batch processing
	BatchSize         int           `json:"batch_size" yaml:"batch_size" mapstructure:"batch_size"`
	BatchTimeout      time.Duration `json:"batch_timeout" yaml:"batch_timeout" mapstructure:"batch_timeout"`
	ProcessingWorkers int           `json:"processing_workers" yaml:"processing_workers" mapstructure:"processing_workers"`

	// Caching
	TemplateCacheSize  int           `json:"template_cache_size" yaml:"template_cache_size" mapstructure:"template_cache_size"`
	TemplateCacheTTL   time.Duration `json:"template_cache_ttl" yaml:"template_cache_ttl" mapstructure:"template_cache_ttl"`
	ConnectionCacheTTL time.Duration `json:"connection_cache_ttl" yaml:"connection_cache_ttl" mapstructure:"connection_cache_ttl"`
}

// RetryConfig configuración de reintentos
type RetryConfig struct {
	MaxAttempts     int           `json:"max_attempts" yaml:"max_attempts" mapstructure:"max_attempts"`
	InitialDelay    time.Duration `json:"initial_delay" yaml:"initial_delay" mapstructure:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay" yaml:"max_delay" mapstructure:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor" yaml:"backoff_factor" mapstructure:"backoff_factor"`
	JitterEnabled   bool          `json:"jitter_enabled" yaml:"jitter_enabled" mapstructure:"jitter_enabled"`
	RetryableErrors []string      `json:"retryable_errors,omitempty" yaml:"retryable_errors,omitempty" mapstructure:"retryable_errors"`
}

// TemplateConfig configuración de templates
type TemplateConfig struct {
	CacheEnabled      bool          `json:"cache_enabled" yaml:"cache_enabled" mapstructure:"cache_enabled"`
	CacheSize         int           `json:"cache_size" yaml:"cache_size" mapstructure:"cache_size"`
	CacheTTL          time.Duration `json:"cache_ttl" yaml:"cache_ttl" mapstructure:"cache_ttl"`
	DefaultLanguage   string        `json:"default_language" yaml:"default_language" mapstructure:"default_language"`
	FallbackLanguage  string        `json:"fallback_language" yaml:"fallback_language" mapstructure:"fallback_language"`
	ValidateVariables bool          `json:"validate_variables" yaml:"validate_variables" mapstructure:"validate_variables"`
}

// LoggingConfig configuración de logging
type LoggingConfig struct {
	Enabled         bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Level           string   `json:"level" yaml:"level" mapstructure:"level"` // debug, info, warn, error
	LogHeaders      bool     `json:"log_headers" yaml:"log_headers" mapstructure:"log_headers"`
	LogBody         bool     `json:"log_body" yaml:"log_body" mapstructure:"log_body"`
	SensitiveFields []string `json:"sensitive_fields,omitempty" yaml:"sensitive_fields,omitempty" mapstructure:"sensitive_fields"`
	MaxBodySize     int      `json:"max_body_size" yaml:"max_body_size" mapstructure:"max_body_size"`
}

// DevelopmentConfig configuración para desarrollo
type DevelopmentConfig struct {
	MockMode          bool     `json:"mock_mode" yaml:"mock_mode" mapstructure:"mock_mode"`
	LogToConsole      bool     `json:"log_to_console" yaml:"log_to_console" mapstructure:"log_to_console"`
	SaveToFile        bool     `json:"save_to_file" yaml:"save_to_file" mapstructure:"save_to_file"`
	FileStoragePath   string   `json:"file_storage_path,omitempty" yaml:"file_storage_path,omitempty" mapstructure:"file_storage_path"`
	AllowedTestEmails []string `json:"allowed_test_emails,omitempty" yaml:"allowed_test_emails,omitempty" mapstructure:"allowed_test_emails"`
}

// ProviderConfig configuración de proveedor conocido
type ProviderConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	RequiresTLS  bool   `json:"requires_tls"`
	RequiresAuth bool   `json:"requires_auth"`
	Name         string `json:"name"`
}

// AuthMethod métodos de autenticación SMTP
type AuthMethod string

const (
	AuthMethodPlain    AuthMethod = "plain"
	AuthMethodLogin    AuthMethod = "login"
	AuthMethodCRAMMD5  AuthMethod = "cram-md5"
	AuthMethodExternal AuthMethod = "external"
	AuthMethodAuto     AuthMethod = "auto"
)

// ================================
// FUNCIONES DE CONFIGURACIÓN
// ================================

// NewDefaultConfig crea una configuración por defecto
func NewDefaultConfig() *Config {
	return &Config{
		SMTP: SMTPConfig{
			Port:            DefaultSMTPPort,
			EnableTLS:       DefaultSMTPTLS,
			TLSSkipVerify:   false,
			RequireSTARTTLS: true,
			AuthMethod:      AuthMethodAuto,
			KeepAlive:       true,
			ConnectTimeout:  DefaultConnectTimeout,
			SendTimeout:     DefaultSendTimeout,
			IdleTimeout:     DefaultIdleTimeout,
		},
		Security: SecurityConfig{
			ValidateEmails:    true,
			ValidateDomains:   true,
			MaxRecipients:     100,
			MaxMessageSize:    DefaultMaxMessageSize,
			MaxAttachmentSize: DefaultMaxMessageSize,
			MaxAttachments:    10,
		},
		Performance: PerformanceConfig{
			MaxConnections:     DefaultMaxConnections,
			MaxIdleConns:       5,
			ConnectionTimeout:  DefaultConnectTimeout,
			RateLimit:          DefaultRateLimit,
			BurstLimit:         10,
			RateLimitWindow:    time.Minute,
			BatchSize:          50,
			BatchTimeout:       5 * time.Second,
			ProcessingWorkers:  3,
			TemplateCacheSize:  100,
			TemplateCacheTTL:   time.Hour,
			ConnectionCacheTTL: 30 * time.Minute,
		},
		Retry: RetryConfig{
			MaxAttempts:     DefaultRetryAttempts,
			InitialDelay:    DefaultRetryDelay,
			MaxDelay:        time.Hour,
			BackoffFactor:   2.0,
			JitterEnabled:   true,
			RetryableErrors: []string{"timeout", "connection", "temporary"},
		},
		Templates: TemplateConfig{
			CacheEnabled:      true,
			CacheSize:         100,
			CacheTTL:          time.Hour,
			DefaultLanguage:   "en",
			FallbackLanguage:  "en",
			ValidateVariables: true,
		},
		Logging: LoggingConfig{
			Enabled:         true,
			Level:           "info",
			LogHeaders:      true,
			LogBody:         false,
			SensitiveFields: []string{"password", "token", "key"},
			MaxBodySize:     1024,
		},
		Development: DevelopmentConfig{
			MockMode:        false,
			LogToConsole:    true,
			SaveToFile:      false,
			FileStoragePath: "/tmp/emails",
		},
		Headers: map[string]string{
			"User-Agent": DefaultUserAgent,
			"X-Mailer":   DefaultUserAgent,
		},
	}
}

// NewConfigFromProvider crea configuración basada en un proveedor conocido
func NewConfigFromProvider(provider, username, password string) (*Config, error) {
	providerConfig, exists := KnownProviders[strings.ToLower(provider)]
	if !exists {
		return nil, fmt.Errorf("unknown email provider: %s", provider)
	}

	config := NewDefaultConfig()
	config.SMTP.Host = providerConfig.Host
	config.SMTP.Port = providerConfig.Port
	config.SMTP.Username = username
	config.SMTP.Password = password
	config.SMTP.EnableTLS = providerConfig.RequiresTLS
	config.SMTP.RequireSTARTTLS = providerConfig.RequiresTLS

	return config, nil
}

// ================================
// MÉTODOS DE VALIDACIÓN
// ================================

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Validate valida toda la configuración
func (c *Config) Validate() error {
	if err := c.SMTP.Validate(); err != nil {
		return fmt.Errorf("SMTP config error: %w", err)
	}

	if err := c.Security.Validate(); err != nil {
		return fmt.Errorf("security config error: %w", err)
	}

	if err := c.Performance.Validate(); err != nil {
		return fmt.Errorf("performance config error: %w", err)
	}

	if err := c.Retry.Validate(); err != nil {
		return fmt.Errorf("retry config error: %w", err)
	}

	return nil
}

// Validate valida la configuración SMTP
func (s *SMTPConfig) Validate() error {
	if strings.TrimSpace(s.Host) == "" {
		return fmt.Errorf("SMTP host is required")
	}

	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}

	if s.Username != "" && s.Password == "" {
		return fmt.Errorf("SMTP password is required when username is provided")
	}

	if s.DefaultFrom.Email != "" && !emailRegex.MatchString(s.DefaultFrom.Email) {
		return fmt.Errorf("invalid default from email: %s", s.DefaultFrom.Email)
	}

	if s.ConnectTimeout <= 0 {
		s.ConnectTimeout = DefaultConnectTimeout
	}

	if s.SendTimeout <= 0 {
		s.SendTimeout = DefaultSendTimeout
	}

	if s.IdleTimeout <= 0 {
		s.IdleTimeout = DefaultIdleTimeout
	}

	return nil
}

// Validate valida la configuración de seguridad
func (s *SecurityConfig) Validate() error {
	if s.MaxRecipients <= 0 {
		s.MaxRecipients = 100
	}

	if s.MaxMessageSize <= 0 {
		s.MaxMessageSize = DefaultMaxMessageSize
	}

	if s.MaxAttachmentSize <= 0 {
		s.MaxAttachmentSize = DefaultMaxMessageSize
	}

	if s.MaxAttachments < 0 {
		s.MaxAttachments = 10
	}

	// Validar dominios permitidos
	for _, domain := range s.AllowedDomains {
		if !isValidDomain(domain) {
			return fmt.Errorf("invalid allowed domain: %s", domain)
		}
	}

	// Validar dominios bloqueados
	for _, domain := range s.BlockedDomains {
		if !isValidDomain(domain) {
			return fmt.Errorf("invalid blocked domain: %s", domain)
		}
	}

	return nil
}

// Validate valida la configuración de rendimiento
func (p *PerformanceConfig) Validate() error {
	if p.MaxConnections <= 0 {
		p.MaxConnections = DefaultMaxConnections
	}

	if p.MaxIdleConns <= 0 {
		p.MaxIdleConns = p.MaxConnections / 2
	}

	if p.MaxIdleConns > p.MaxConnections {
		p.MaxIdleConns = p.MaxConnections
	}

	if p.RateLimit <= 0 {
		p.RateLimit = DefaultRateLimit
	}

	if p.BurstLimit <= 0 {
		p.BurstLimit = p.RateLimit / 10
	}

	if p.ProcessingWorkers <= 0 {
		p.ProcessingWorkers = 3
	}

	return nil
}

// Validate valida la configuración de reintentos
func (r *RetryConfig) Validate() error {
	if r.MaxAttempts <= 0 {
		r.MaxAttempts = DefaultRetryAttempts
	}

	if r.InitialDelay <= 0 {
		r.InitialDelay = DefaultRetryDelay
	}

	if r.MaxDelay <= 0 {
		r.MaxDelay = time.Hour
	}

	if r.BackoffFactor <= 1.0 {
		r.BackoffFactor = 2.0
	}

	return nil
}

// ================================
// MÉTODOS DE UTILIDAD
// ================================

// GetSMTPAuth obtiene la configuración de autenticación SMTP
func (s *SMTPConfig) GetSMTPAuth() smtp.Auth {
	if s.Username == "" || s.Password == "" {
		return nil
	}

	switch s.AuthMethod {
	case AuthMethodPlain, AuthMethodAuto:
		return smtp.PlainAuth("", s.Username, s.Password, s.Host)
	case AuthMethodCRAMMD5:
		return smtp.CRAMMD5Auth(s.Username, s.Password)
	default:
		return smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}
}

// GetAddress devuelve la dirección completa del servidor SMTP
func (s *SMTPConfig) GetAddress() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// GetTLSConfig devuelve la configuración TLS
func (s *SMTPConfig) GetTLSConfig() *tls.Config {
	if s.TLSConfig != nil {
		return s.TLSConfig
	}

	return &tls.Config{
		ServerName:         s.Host,
		InsecureSkipVerify: s.TLSSkipVerify,
	}
}

// IsEmailAllowed verifica si un email está permitido según la configuración de seguridad
func (s *SecurityConfig) IsEmailAllowed(email string) bool {
	if !s.ValidateEmails {
		return true
	}

	// Validar formato
	if !emailRegex.MatchString(email) {
		return false
	}

	domain := extractDomain(email)

	// Verificar dominios bloqueados
	for _, blocked := range s.BlockedDomains {
		if strings.EqualFold(domain, blocked) {
			return false
		}
	}

	// Verificar dominios permitidos (si se especifican)
	if len(s.AllowedDomains) > 0 {
		for _, allowed := range s.AllowedDomains {
			if strings.EqualFold(domain, allowed) {
				return true
			}
		}
		return false
	}

	return true
}

// TestConnection prueba la conexión SMTP
func (s *SMTPConfig) TestConnection() error {
	address := s.GetAddress()

	// Intentar conexión
	conn, err := net.DialTimeout("tcp", address, s.ConnectTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	// Crear cliente SMTP
	client, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Probar STARTTLS si está habilitado
	if s.EnableTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(s.GetTLSConfig()); err != nil {
				return fmt.Errorf("STARTTLS failed: %w", err)
			}
		} else if s.RequireSTARTTLS {
			return fmt.Errorf("STARTTLS required but not supported by server")
		}
	}

	// Probar autenticación si está configurada
	if s.Username != "" && s.Password != "" {
		auth := s.GetSMTPAuth()
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	return nil
}

// ================================
// FUNCIONES AUXILIARES
// ================================

// isValidDomain valida si un dominio tiene formato válido
func isValidDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// Verificar formato básico
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return domainRegex.MatchString(domain)
}

// extractDomain extrae el dominio de una dirección de email
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[1])
}

// GetProviderNames devuelve los nombres de los proveedores disponibles
func GetProviderNames() []string {
	names := make([]string, 0, len(KnownProviders))
	for name := range KnownProviders {
		names = append(names, name)
	}
	return names
}

// DetectProvider intenta detectar el proveedor basado en el host SMTP
func DetectProvider(host string) string {
	host = strings.ToLower(host)
	for name, config := range KnownProviders {
		if strings.Contains(host, name) || strings.Contains(host, config.Host) {
			return name
		}
	}
	return ""
}
