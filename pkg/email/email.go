package email

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ESTRUCTURAS DE MENSAJE Y DATOS

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
	Attachments []EmailAttachment `json:"attachments,omitempty"`

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

// EmailAttachment archivo adjunto para email (renombrado para evitar conflicto)
type EmailAttachment struct {
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

// INTERFACES Y SERVICIOS

// EmailService interfaz del servicio de email (renombrado para evitar conflicto)
type EmailService interface {
	// Envío básico
	Send(ctx context.Context, message *Message) error
	SendWithID(ctx context.Context, message *Message) (string, error)
	SendBatch(ctx context.Context, messages []*Message) error

	// Configuración y estado
	TestConnection() error
	GetConfig() *Config
	IsEnabled() bool

	// Estadísticas
	GetStats() *EmailStats

	// Pool management
	Close() error
}

// EmailStats estadísticas del servicio (renombrado para evitar conflicto)
type EmailStats struct {
	TotalSent   int64         `json:"total_sent"`
	TotalFailed int64         `json:"total_failed"`
	LastSent    *time.Time    `json:"last_sent,omitempty"`
	AverageTime time.Duration `json:"average_time"`
	ActiveConns int           `json:"active_conns"`
	IdleConns   int           `json:"idle_conns"`
	ErrorRate   float64       `json:"error_rate"`
	Uptime      time.Duration `json:"uptime"`
}

// FUNCIONES DE UTILIDAD Y HELPERS

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

// NewEmailAttachment crea un nuevo adjunto
func NewEmailAttachment(filename, contentType string, content []byte) EmailAttachment {
	return EmailAttachment{
		Filename:    filename,
		ContentType: contentType,
		Content:     content,
		Headers:     make(map[string]string),
	}
}

// MÉTODOS DE VALIDACIÓN

// ValidateMessage valida la estructura del mensaje
func ValidateMessage(message *Message) error {
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
		if !IsValidEmail(addr.Email) {
			return fmt.Errorf("invalid email address: %s", addr.Email)
		}
	}

	for _, addr := range message.CC {
		if !IsValidEmail(addr.Email) {
			return fmt.Errorf("invalid CC email address: %s", addr.Email)
		}
	}

	for _, addr := range message.BCC {
		if !IsValidEmail(addr.Email) {
			return fmt.Errorf("invalid BCC email address: %s", addr.Email)
		}
	}

	return nil
}

// IsValidEmail validación básica de email
func IsValidEmail(email string) bool {
	// Validación muy básica - en producción usar una librería más robusta
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// MÉTODOS STRING PARA LOGGING

// String devuelve representación en string de Address
func (a Address) String() string {
	if a.Name != "" {
		return fmt.Sprintf("%s <%s>", a.Name, a.Email)
	}
	return a.Email
}

// String devuelve representación en string de Priority
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

// GetPriorityValue retorna el valor numérico de prioridad para headers
func (p Priority) GetPriorityValue() string {
	switch p {
	case PriorityHigh:
		return "2"
	case PriorityCritical:
		return "1"
	case PriorityLow:
		return "4"
	default:
		return "3" // Normal
	}
}

// GetImportance retorna el valor de importancia para headers
func (p Priority) GetImportance() string {
	switch p {
	case PriorityHigh, PriorityCritical:
		return "high"
	case PriorityLow:
		return "low"
	default:
		return "normal"
	}
}

// FUNCIONES DE CONVERSIÓN

// ToEmailMessage convierte de Message interno a formato de envío
func (m *Message) ToEmailMessage() *EmailMessage {
	return &EmailMessage{
		ID:          m.ID,
		From:        m.From,
		To:          m.To,
		CC:          m.CC,
		BCC:         m.BCC,
		ReplyTo:     m.ReplyTo,
		Subject:     m.Subject,
		TextBody:    m.TextBody,
		HTMLBody:    m.HTMLBody,
		Attachments: convertAttachments(m.Attachments),
		Headers:     m.Headers,
		Priority:    m.Priority,
		Timestamp:   m.Timestamp,
		MessageID:   m.MessageID,
		References:  m.References,
		InReplyTo:   m.InReplyTo,
		TrackOpens:  m.TrackOpens,
		TrackClicks: m.TrackClicks,
		SendAt:      m.SendAt,
		Tags:        m.Tags,
		Metadata:    m.Metadata,
	}
}

// convertAttachments convierte EmailAttachment a Attachment
func convertAttachments(emailAttachments []EmailAttachment) []Attachment {
	attachments := make([]Attachment, len(emailAttachments))
	for i, ea := range emailAttachments {
		attachments[i] = Attachment{
			Filename:    ea.Filename,
			ContentType: ea.ContentType,
			Data:        ea.Content,
			Inline:      ea.Inline,
			ContentID:   ea.CID,
		}
	}
	return attachments
}

// ESTRUCTURAS PARA COMPATIBILIDAD

// EmailMessage estructura para el sistema de email más complejo
type EmailMessage struct {
	ID          string                 `json:"id"`
	From        Address                `json:"from"`
	To          []Address              `json:"to"`
	CC          []Address              `json:"cc,omitempty"`
	BCC         []Address              `json:"bcc,omitempty"`
	ReplyTo     *Address               `json:"reply_to,omitempty"`
	Subject     string                 `json:"subject"`
	TextBody    string                 `json:"text_body,omitempty"`
	HTMLBody    string                 `json:"html_body,omitempty"`
	Attachments []Attachment           `json:"attachments,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Priority    Priority               `json:"priority,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	MessageID   string                 `json:"message_id,omitempty"`
	References  []string               `json:"references,omitempty"`
	InReplyTo   string                 `json:"in_reply_to,omitempty"`
	TrackOpens  bool                   `json:"track_opens,omitempty"`
	TrackClicks bool                   `json:"track_clicks,omitempty"`
	SendAt      *time.Time             `json:"send_at,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// FUNCIONES DE CREACIÓN DE MENSAJES RÁPIDOS

// NewSimpleMessage crea un mensaje simple con los campos básicos
func NewSimpleMessage(from, to, subject, body string) *Message {
	message := NewMessage()
	message.From = NewAddress(from, "")
	message.To = []Address{NewAddress(to, "")}
	message.Subject = subject
	message.TextBody = body
	return message
}

// NewHTMLMessage crea un mensaje con contenido HTML
func NewHTMLMessage(from, to, subject, htmlBody, textBody string) *Message {
	message := NewMessage()
	message.From = NewAddress(from, "")
	message.To = []Address{NewAddress(to, "")}
	message.Subject = subject
	message.HTMLBody = htmlBody
	message.TextBody = textBody
	return message
}

// AddAttachment añade un adjunto al mensaje
func (m *Message) AddAttachment(filename, contentType string, content []byte) {
	attachment := NewEmailAttachment(filename, contentType, content)
	m.Attachments = append(m.Attachments, attachment)
}

// AddInlineAttachment añade un adjunto inline al mensaje
func (m *Message) AddInlineAttachment(filename, contentType, contentID string, content []byte) {
	attachment := NewEmailAttachment(filename, contentType, content)
	attachment.Inline = true
	attachment.CID = contentID
	m.Attachments = append(m.Attachments, attachment)
}

// SetPriority establece la prioridad del mensaje
func (m *Message) SetPriority(priority Priority) {
	m.Priority = priority
}

// AddHeader añade un header personalizado
func (m *Message) AddHeader(key, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
}

// AddTag añade una etiqueta al mensaje
func (m *Message) AddTag(tag string) {
	m.Tags = append(m.Tags, tag)
}

// SetMetadata establece metadatos del mensaje
func (m *Message) SetMetadata(key string, value interface{}) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
}
