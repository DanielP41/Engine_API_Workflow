package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/gomail.v2"
)

// Config configuración del servicio de email
type Config struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Host      string `json:"host" yaml:"host"`
	Port      int    `json:"port" yaml:"port"`
	Username  string `json:"username" yaml:"username"`
	Password  string `json:"password" yaml:"password"`
	FromEmail string `json:"from_email" yaml:"from_email"`
	FromName  string `json:"from_name" yaml:"from_name"`

	// Configuración SSL/TLS
	UseTLS      bool `json:"use_tls" yaml:"use_tls"`
	UseStartTLS bool `json:"use_starttls" yaml:"use_starttls"`
	SkipVerify  bool `json:"skip_verify" yaml:"skip_verify"`

	// Configuración de conexión
	ConnTimeout time.Duration `json:"conn_timeout" yaml:"conn_timeout"`
	SendTimeout time.Duration `json:"send_timeout" yaml:"send_timeout"`
	MaxRetries  int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay  time.Duration `json:"retry_delay" yaml:"retry_delay"`

	// Pool de conexiones
	MaxIdleConns    int           `json:"max_idle_conns" yaml:"max_idle_conns"`
	MaxOpenConns    int           `json:"max_open_conns" yaml:"max_open_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`

	// Rate limiting
	RateLimit  int `json:"rate_limit" yaml:"rate_limit"`   // emails por minuto
	BurstLimit int `json:"burst_limit" yaml:"burst_limit"` // ráfaga máxima

	// Headers personalizados
	DefaultHeaders map[string]string `json:"default_headers" yaml:"default_headers"`
}

// Message estructura del mensaje de email
type Message struct {
	ID      string    `json:"id"`
	From    Address   `json:"from"`
	To      []Address `json:"to"`
	CC      []Address `json:"cc,omitempty"`
	BCC     []Address `json:"bcc,omitempty"`
	ReplyTo *Address  `json:"reply_to,omitempty"`

	Subject string `json:"subject"`

	// Contenido
	TextBody string `json:"text_body,omitempty"`
	HTMLBody string `json:"html_body,omitempty"`

	// Attachments
	Attachments []Attachment `json:"attachments,omitempty"`

	// Headers personalizados
	Headers map[string]string `json:"headers,omitempty"`

	// Metadatos
	Priority   Priority  `json:"priority,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	MessageID  string    `json:"message_id,omitempty"`
	References []string  `json:"references,omitempty"`
	InReplyTo  string    `json:"in_reply_to,omitempty"`

	// Tracking
	TrackOpens  bool `json:"track_opens,omitempty"`
	TrackClicks bool `json:"track_clicks,omitempty"`

	// Programación
	SendAt *time.Time `json:"send_at,omitempty"`

	// Tags para categorización
	Tags []string `json:"tags,omitempty"`

	// Metadata adicional
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Address dirección de email
type Address struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// Attachment archivo adjunto
type Attachment struct {
	Filename    string            `json:"filename"`
	ContentType string            `json:"content_type"`
	Content     []byte            `json:"content"`
	Inline      bool              `json:"inline,omitempty"`
	CID         string            `json:"cid,omitempty"` // Content-ID para inline
	Headers     map[string]string `json:"headers,omitempty"`
}

// Priority prioridad del mensaje
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// Service interfaz del servicio de email
type Service interface {
	// Envío básico
	Send(ctx context.Context, message *Message) error
	SendWithID(ctx context.Context, message *Message) (string, error)
	SendBatch(ctx context.Context, messages []*Message) error

	// Configuración y estado
	TestConnection() error
	GetConfig() *Config
	IsEnabled() bool

	// Estadísticas
	GetStats() *Stats

	// Pool management
	Close() error
}

// Stats estadísticas del servicio
type Stats struct {
	TotalSent   int64         `json:"total_sent"`
	TotalFailed int64         `json:"total_failed"`
	LastSent    *time.Time    `json:"last_sent,omitempty"`
	AverageTime time.Duration `json:"average_time"`
	ActiveConns int           `json:"active_conns"`
	IdleConns   int           `json:"idle_conns"`
	ErrorRate   float64       `json:"error_rate"`
	Uptime      time.Duration `json:"uptime"`
}

// SMTPService implementación SMTP del servicio de email
type SMTPService struct {
	config    *Config
	dialer    *gomail.Dialer
	stats     *Stats
	startTime time.Time

	// Rate limiting
	rateLimiter chan struct{}
	lastSent    time.Time
}

// NewSMTPService crea un nuevo servicio SMTP
func NewSMTPService(config *Config) *SMTPService {
	// Validar y establecer valores por defecto
	if config.ConnTimeout == 0 {
		config.ConnTimeout = 10 * time.Second
	}
	if config.SendTimeout == 0 {
		config.SendTimeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 5 * time.Second
	}
	if config.RateLimit == 0 {
		config.RateLimit = 60 // 60 emails por minuto por defecto
	}
	if config.BurstLimit == 0 {
		config.BurstLimit = 10
	}

	// Configurar dialer
	dialer := gomail.NewDialer(config.Host, config.Port, config.Username, config.Password)

	// Configurar TLS
	if config.UseTLS {
		dialer.TLSConfig = &tls.Config{
			ServerName:         config.Host,
			InsecureSkipVerify: config.SkipVerify,
		}
	}

	if config.UseStartTLS {
		dialer.StartTLSPolicy = gomail.MandatoryStartTLS
	}

	// Rate limiter
	rateLimiter := make(chan struct{}, config.BurstLimit)
	for i := 0; i < config.BurstLimit; i++ {
		rateLimiter <- struct{}{}
	}

	return &SMTPService{
		config:      config,
		dialer:      dialer,
		stats:       &Stats{},
		startTime:   time.Now(),
		rateLimiter: rateLimiter,
	}
}

// Send envía un mensaje de email
func (s *SMTPService) Send(ctx context.Context, message *Message) error {
	_, err := s.SendWithID(ctx, message)
	return err
}

// SendWithID envía un mensaje y retorna el ID del mensaje
func (s *SMTPService) SendWithID(ctx context.Context, message *Message) (string, error) {
	if !s.config.Enabled {
		return "", fmt.Errorf("email service is disabled")
	}

	// Validar mensaje
	if err := s.validateMessage(message); err != nil {
		return "", fmt.Errorf("invalid message: %w", err)
	}

	// Rate limiting
	if err := s.applyRateLimit(ctx); err != nil {
		return "", fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Generar ID único si no existe
	if message.ID == "" {
		message.ID = uuid.New().String()
	}

	// Construir mensaje gomail
	m := s.buildGomailMessage(message)

	// Enviar con reintentos
	var lastErr error
	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		startTime := time.Now()

		err := s.dialer.DialAndSend(m)

		duration := time.Since(startTime)
		s.updateStats(err == nil, duration)

		if err == nil {
			s.lastSent = time.Now()
			return message.ID, nil
		}

		lastErr = err

		// Si no es el último intento, esperar antes de reintentar
		if attempt < s.config.MaxRetries {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(s.config.RetryDelay * time.Duration(attempt+1)):
				// Backoff exponencial
			}
		}
	}

	return "", fmt.Errorf("failed to send email after %d attempts: %w", s.config.MaxRetries+1, lastErr)
}

// SendBatch envía múltiples mensajes
func (s *SMTPService) SendBatch(ctx context.Context, messages []*Message) error {
	if len(messages) == 0 {
		return nil
	}

	// Enviar cada mensaje individualmente
	// En una implementación más avanzada, se podría optimizar esto
	for i, message := range messages {
		if err := s.Send(ctx, message); err != nil {
			return fmt.Errorf("failed to send message %d: %w", i, err)
		}
	}

	return nil
}

// TestConnection prueba la conexión SMTP
func (s *SMTPService) TestConnection() error {
	if !s.config.Enabled {
		return fmt.Errorf("email service is disabled")
	}

	// Crear un mensaje de prueba simple
	testMessage := &Message{
		From:     Address{Email: s.config.FromEmail, Name: s.config.FromName},
		To:       []Address{{Email: s.config.FromEmail}},
		Subject:  "Test Connection",
		TextBody: "This is a test message to verify SMTP configuration.",
	}

	m := s.buildGomailMessage(testMessage)

	// Solo probar la conexión, no enviar
	closer, err := s.dialer.Dial()
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer closer.Close()

	return nil
}

// GetConfig retorna la configuración actual
func (s *SMTPService) GetConfig() *Config {
	return s.config
}

// IsEnabled verifica si el servicio está habilitado
func (s *SMTPService) IsEnabled() bool {
	return s.config.Enabled
}

// GetStats retorna las estadísticas del servicio
func (s *SMTPService) GetStats() *Stats {
	s.stats.Uptime = time.Since(s.startTime)
	if s.stats.TotalSent > 0 {
		s.stats.ErrorRate = float64(s.stats.TotalFailed) / float64(s.stats.TotalSent+s.stats.TotalFailed)
	}
	return s.stats
}

// Close cierra el servicio
func (s *SMTPService) Close() error {
	// Limpiar recursos si es necesario
	return nil
}

// validateMessage valida la estructura del mensaje
func (s *SMTPService) validateMessage(message *Message) error {
	if len(message.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	if message.Subject == "" {
		return fmt.Errorf("subject is required")
	}

	if message.TextBody == "" && message.HTMLBody == "" {
		return fmt.Errorf("message body is required")
	}

	// Validar direcciones de email
	for _, addr := range message.To {
		if !isValidEmail(addr.Email) {
			return fmt.Errorf("invalid email address: %s", addr.Email)
		}
	}

	for _, addr := range message.CC {
		if !isValidEmail(addr.Email) {
			return fmt.Errorf("invalid CC email address: %s", addr.Email)
		}
	}

	for _, addr := range message.BCC {
		if !isValidEmail(addr.Email) {
			return fmt.Errorf("invalid BCC email address: %s", addr.Email)
		}
	}

	return nil
}

// buildGomailMessage construye un mensaje gomail desde nuestro Message
func (s *SMTPService) buildGomailMessage(message *Message) *gomail.Message {
	m := gomail.NewMessage()

	// From
	if message.From.Name != "" {
		m.SetHeader("From", m.FormatAddress(message.From.Email, message.From.Name))
	} else {
		m.SetHeader("From", message.From.Email)
	}

	// To
	toAddrs := make([]string, len(message.To))
	for i, addr := range message.To {
		if addr.Name != "" {
			toAddrs[i] = m.FormatAddress(addr.Email, addr.Name)
		} else {
			toAddrs[i] = addr.Email
		}
	}
	m.SetHeader("To", toAddrs...)

	// CC
	if len(message.CC) > 0 {
		ccAddrs := make([]string, len(message.CC))
		for i, addr := range message.CC {
			if addr.Name != "" {
				ccAddrs[i] = m.FormatAddress(addr.Email, addr.Name)
			} else {
				ccAddrs[i] = addr.Email
			}
		}
		m.SetHeader("Cc", ccAddrs...)
	}

	// BCC
	if len(message.BCC) > 0 {
		bccAddrs := make([]string, len(message.BCC))
		for i, addr := range message.BCC {
			if addr.Name != "" {
				bccAddrs[i] = m.FormatAddress(addr.Email, addr.Name)
			} else {
				bccAddrs[i] = addr.Email
			}
		}
		m.SetHeader("Bcc", bccAddrs...)
	}

	// Reply-To
	if message.ReplyTo != nil {
		if message.ReplyTo.Name != "" {
			m.SetHeader("Reply-To", m.FormatAddress(message.ReplyTo.Email, message.ReplyTo.Name))
		} else {
			m.SetHeader("Reply-To", message.ReplyTo.Email)
		}
	}

	// Subject
	m.SetHeader("Subject", message.Subject)

	// Message-ID
	if message.MessageID != "" {
		m.SetHeader("Message-ID", message.MessageID)
	}

	// Referencias
	if len(message.References) > 0 {
		m.SetHeader("References", strings.Join(message.References, " "))
	}

	if message.InReplyTo != "" {
		m.SetHeader("In-Reply-To", message.InReplyTo)
	}

	// Prioridad
	switch message.Priority {
	case PriorityHigh:
		m.SetHeader("X-Priority", "2")
		m.SetHeader("Importance", "high")
	case PriorityCritical:
		m.SetHeader("X-Priority", "1")
		m.SetHeader("Importance", "high")
	case PriorityLow:
		m.SetHeader("X-Priority", "4")
		m.SetHeader("Importance", "low")
	}

	// Headers personalizados
	for key, value := range message.Headers {
		m.SetHeader(key, value)
	}

	// Headers por defecto de la configuración
	for key, value := range s.config.DefaultHeaders {
		if _, exists := message.Headers[key]; !exists {
			m.SetHeader(key, value)
		}
	}

	// Contenido
	if message.HTMLBody != "" && message.TextBody != "" {
		m.SetBody("text/plain", message.TextBody)
		m.AddAlternative("text/html", message.HTMLBody)
	} else if message.HTMLBody != "" {
		m.SetBody("text/html", message.HTMLBody)
	} else {
		m.SetBody("text/plain", message.TextBody)
	}

	// Adjuntos
	for _, attachment := range message.Attachments {
		if attachment.Inline {
			m.EmbedReader(attachment.Filename, strings.NewReader(string(attachment.Content)))
		} else {
			m.AttachReader(attachment.Filename, strings.NewReader(string(attachment.Content)))
		}
	}

	return m
}

// applyRateLimit aplica limitación de velocidad
func (s *SMTPService) applyRateLimit(ctx context.Context) error {
	// Esperar por un token
	select {
	case <-s.rateLimiter:
		// Reponer el token después del intervalo
		go func() {
			time.Sleep(time.Minute / time.Duration(s.config.RateLimit))
			s.rateLimiter <- struct{}{}
		}()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// updateStats actualiza las estadísticas
func (s *SMTPService) updateStats(success bool, duration time.Duration) {
	if success {
		s.stats.TotalSent++
		now := time.Now()
		s.stats.LastSent = &now
	} else {
		s.stats.TotalFailed++
	}

	// Calcular tiempo promedio (simple)
	total := s.stats.TotalSent + s.stats.TotalFailed
	if total > 0 {
		s.stats.AverageTime = time.Duration(
			(int64(s.stats.AverageTime)*(total-1) + int64(duration)) / int64(total),
		)
	}
}

// isValidEmail validación básica de email
func isValidEmail(email string) bool {
	// Validación muy básica - en producción usar una librería más robusta
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// Helper functions para crear mensajes

// NewMessage crea un nuevo mensaje
func NewMessage() *Message {
	return &Message{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Headers:   make(map[string]string),
		Metadata:  make(map[string]interface{}),
	}
}

// NewAddress crea una nueva dirección
func NewAddress(email, name string) Address {
	return Address{
		Email: email,
		Name:  name,
	}
}

// NewAttachment crea un nuevo adjunto
func NewAttachment(filename, contentType string, content []byte) Attachment {
	return Attachment{
		Filename:    filename,
		ContentType: contentType,
		Content:     content,
		Headers:     make(map[string]string),
	}
}

// String methods para mejor logging

func (a Address) String() string {
	if a.Name != "" {
		return fmt.Sprintf("%s <%s>", a.Name, a.Email)
	}
	return a.Email
}

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "normal"
	}
}
