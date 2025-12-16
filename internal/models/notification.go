package models

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ================================
// ENUMS Y CONSTANTES
// ================================

// NotificationStatus estados de notificación
type NotificationStatus string

const (
	NotificationStatusPending    NotificationStatus = "pending"
	NotificationStatusProcessing NotificationStatus = "processing"
	NotificationStatusSent       NotificationStatus = "sent"
	NotificationStatusFailed     NotificationStatus = "failed"
	NotificationStatusCancelled  NotificationStatus = "cancelled"
	NotificationStatusScheduled  NotificationStatus = "scheduled"
)

// NotificationType tipos de notificación
type NotificationType string

const (
	NotificationTypeCustom        NotificationType = "custom"
	NotificationTypeWelcome       NotificationType = "welcome"
	NotificationTypePasswordReset NotificationType = "password_reset"
	NotificationTypeSystemAlert   NotificationType = "system_alert"
	NotificationTypeWorkflow      NotificationType = "workflow"
	NotificationTypeUserActivity  NotificationType = "user_activity"
	NotificationTypeBackup        NotificationType = "backup"
	NotificationTypeScheduled     NotificationType = "scheduled"
	NotificationTypeReminder      NotificationType = "reminder"
)

// NotificationPriority prioridades de notificación
type NotificationPriority string

const (
	NotificationPriorityLow      NotificationPriority = "low"
	NotificationPriorityNormal   NotificationPriority = "normal"
	NotificationPriorityHigh     NotificationPriority = "high"
	NotificationPriorityCritical NotificationPriority = "critical"
)

// Constantes para límites y configuración
const (
	MaxEmailRecipients = 100
	MaxSubjectLength   = 200
	MaxBodyLength      = 1000000          // 1MB
	MaxAttachmentSize  = 25 * 1024 * 1024 // 25MB
	MaxAttachments     = 10
	DefaultMaxAttempts = 3
	DefaultRetryDelay  = 5 * time.Minute
	MaxRetryDelay      = 24 * time.Hour
)

// ================================
// MODELOS PRINCIPALES
// ================================

// EmailNotification modelo principal para notificaciones de email
type EmailNotification struct {
	ID       primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	Type     NotificationType     `json:"type" bson:"type"`
	Status   NotificationStatus   `json:"status" bson:"status"`
	Priority NotificationPriority `json:"priority" bson:"priority"`

	// Destinatarios
	To  []string `json:"to" bson:"to"`
	CC  []string `json:"cc,omitempty" bson:"cc,omitempty"`
	BCC []string `json:"bcc,omitempty" bson:"bcc,omitempty"`

	// Contenido
	Subject string `json:"subject" bson:"subject"`
	Body    string `json:"body" bson:"body"`
	IsHTML  bool   `json:"is_html" bson:"is_html"`

	// Template utilizado (opcional)
	TemplateName string                 `json:"template_name,omitempty" bson:"template_name,omitempty"`
	TemplateData map[string]interface{} `json:"template_data,omitempty" bson:"template_data,omitempty"`

	// Adjuntos
	Attachments []EmailAttachment `json:"attachments,omitempty" bson:"attachments,omitempty"`

	// Programación
	ScheduledAt *time.Time `json:"scheduled_at,omitempty" bson:"scheduled_at,omitempty"`
	SentAt      *time.Time `json:"sent_at,omitempty" bson:"sent_at,omitempty"`

	// Control de reintentos
	Attempts      int                 `json:"attempts" bson:"attempts"`
	MaxAttempts   int                 `json:"max_attempts" bson:"max_attempts"`
	LastAttemptAt *time.Time          `json:"last_attempt_at,omitempty" bson:"last_attempt_at,omitempty"`
	NextRetryAt   *time.Time          `json:"next_retry_at,omitempty" bson:"next_retry_at,omitempty"`
	Errors        []NotificationError `json:"errors,omitempty" bson:"errors,omitempty"`

	// Información de entrega
	MessageID    string                 `json:"message_id,omitempty" bson:"message_id,omitempty"`
	ProviderData map[string]interface{} `json:"provider_data,omitempty" bson:"provider_data,omitempty"`

	// Metadatos de relación
	UserID      *primitive.ObjectID `json:"user_id,omitempty" bson:"user_id,omitempty"`
	WorkflowID  *primitive.ObjectID `json:"workflow_id,omitempty" bson:"workflow_id,omitempty"`
	ExecutionID *string             `json:"execution_id,omitempty" bson:"execution_id,omitempty"`

	// Auditoría
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
	CreatedBy *string   `json:"created_by,omitempty" bson:"created_by,omitempty"`
}

// EmailAttachment adjunto de email
type EmailAttachment struct {
	Filename    string `json:"filename" bson:"filename"`
	ContentType string `json:"content_type" bson:"content_type"`
	Size        int64  `json:"size" bson:"size"`
	Data        []byte `json:"data,omitempty" bson:"data,omitempty"`                 // Para archivos pequeños
	StoragePath string `json:"storage_path,omitempty" bson:"storage_path,omitempty"` // Para archivos grandes
	Inline      bool   `json:"inline" bson:"inline"`                                 // Para imágenes embebidas
	ContentID   string `json:"content_id,omitempty" bson:"content_id,omitempty"`     // Para referencias inline
}

// NotificationError estructura para errores de notificación
type NotificationError struct {
	Timestamp   time.Time              `json:"timestamp" bson:"timestamp"`
	Code        string                 `json:"code" bson:"code"`
	Message     string                 `json:"message" bson:"message"`
	Details     string                 `json:"details,omitempty" bson:"details,omitempty"`
	Recoverable bool                   `json:"recoverable" bson:"recoverable"`
	Context     map[string]interface{} `json:"context,omitempty" bson:"context,omitempty"`
}

// EmailTemplate modelo para templates de email
type EmailTemplate struct {
	ID       primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name     string             `json:"name" bson:"name"`
	Type     NotificationType   `json:"type" bson:"type"`
	Language string             `json:"language" bson:"language"` // es, en, etc.
	Version  int                `json:"version" bson:"version"`

	// Contenido del template
	Subject  string `json:"subject" bson:"subject"`
	BodyText string `json:"body_text" bson:"body_text"`                     // Versión texto plano
	BodyHTML string `json:"body_html,omitempty" bson:"body_html,omitempty"` // Versión HTML

	// Metadatos
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	Tags        []string           `json:"tags,omitempty" bson:"tags,omitempty"`
	Variables   []TemplateVariable `json:"variables,omitempty" bson:"variables,omitempty"`

	// Configuración
	IsActive  bool                   `json:"is_active" bson:"is_active"`
	IsDefault bool                   `json:"is_default" bson:"is_default"`
	Config    map[string]interface{} `json:"config,omitempty" bson:"config,omitempty"`

	// Auditoría
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
	CreatedBy string    `json:"created_by" bson:"created_by"`
}

// TemplateVariable documentación de variables disponibles en el template
type TemplateVariable struct {
	Name         string      `json:"name" bson:"name"`
	Type         string      `json:"type" bson:"type"` // string, number, boolean, object, array
	Description  string      `json:"description,omitempty" bson:"description,omitempty"`
	Required     bool        `json:"required" bson:"required"`
	DefaultValue interface{} `json:"default_value,omitempty" bson:"default_value,omitempty"`
	Example      interface{} `json:"example,omitempty" bson:"example,omitempty"`
}

// ================================
// ESTRUCTURAS DE REQUEST/RESPONSE
// ================================

// SendEmailRequest estructura para solicitudes de envío
type SendEmailRequest struct {
	Type     NotificationType     `json:"type" validate:"required"`
	Priority NotificationPriority `json:"priority" validate:"required"`

	// Destinatarios
	To  []string `json:"to" validate:"required,min=1"`
	CC  []string `json:"cc,omitempty"`
	BCC []string `json:"bcc,omitempty"`

	// Contenido directo
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
	IsHTML  bool   `json:"is_html,omitempty"`

	// O usar template
	TemplateName string                 `json:"template_name,omitempty"`
	TemplateData map[string]interface{} `json:"template_data,omitempty"`

	// Programación
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`

	// Configuración de reintentos
	MaxAttempts int `json:"max_attempts,omitempty"` // Default: 3

	// Metadatos
	UserID      *primitive.ObjectID `json:"user_id,omitempty"`
	WorkflowID  *primitive.ObjectID `json:"workflow_id,omitempty"`
	ExecutionID *string             `json:"execution_id,omitempty"`
}

// SendEmailResponse respuesta del envío de email
type SendEmailResponse struct {
	NotificationID string             `json:"notification_id"`
	Status         NotificationStatus `json:"status"`
	MessageID      string             `json:"message_id,omitempty"`
	ScheduledAt    *time.Time         `json:"scheduled_at,omitempty"`
	EstimatedSend  *time.Time         `json:"estimated_send,omitempty"`
}

// NotificationStats estadísticas de notificaciones
type NotificationStats struct {
	TotalNotifications int64                          `json:"total_notifications"`
	ByStatus           map[NotificationStatus]int64   `json:"by_status"`
	ByType             map[NotificationType]int64     `json:"by_type"`
	ByPriority         map[NotificationPriority]int64 `json:"by_priority"`
	SuccessRate        float64                        `json:"success_rate"`
	AverageRetries     float64                        `json:"average_retries"`
	LastProcessed      *time.Time                     `json:"last_processed,omitempty"`
	Period             time.Duration                  `json:"period"`
	TotalVolume        int64                          `json:"total_volume"` // Bytes enviados
}

// NotificationBatch estructura para envío en lotes
type NotificationBatch struct {
	ID           primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name         string             `json:"name" bson:"name"`
	Status       string             `json:"status" bson:"status"` // pending, processing, completed, failed
	TotalEmails  int                `json:"total_emails" bson:"total_emails"`
	SentEmails   int                `json:"sent_emails" bson:"sent_emails"`
	FailedEmails int                `json:"failed_emails" bson:"failed_emails"`
	StartedAt    *time.Time         `json:"started_at,omitempty" bson:"started_at,omitempty"`
	CompletedAt  *time.Time         `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	CreatedAt    time.Time          `json:"created_at" bson:"created_at"`
	CreatedBy    string             `json:"created_by" bson:"created_by"`
}

// TemplatePreview vista previa de template
type TemplatePreview struct {
	Subject   string                 `json:"subject"`
	Body      string                 `json:"body"`
	IsHTML    bool                   `json:"is_html"`
	Variables map[string]interface{} `json:"variables"`
}

// ================================
// MÉTODOS DE VALIDACIÓN
// ================================

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// Validate valida una notificación de email
func (n *EmailNotification) Validate() error {
	// Validar destinatarios
	if len(n.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	if len(n.To) > MaxEmailRecipients {
		return fmt.Errorf("too many recipients: maximum %d allowed", MaxEmailRecipients)
	}

	// Validar emails
	allEmails := append(n.To, n.CC...)
	allEmails = append(allEmails, n.BCC...)
	for _, email := range allEmails {
		if !emailRegex.MatchString(email) {
			return fmt.Errorf("invalid email address: %s", email)
		}
	}

	// Validar contenido
	if strings.TrimSpace(n.Subject) == "" {
		return fmt.Errorf("subject is required")
	}

	if len(n.Subject) > MaxSubjectLength {
		return fmt.Errorf("subject too long: maximum %d characters", MaxSubjectLength)
	}

	if strings.TrimSpace(n.Body) == "" && n.TemplateName == "" {
		return fmt.Errorf("body or template is required")
	}

	if len(n.Body) > MaxBodyLength {
		return fmt.Errorf("body too long: maximum %d characters", MaxBodyLength)
	}

	// Validar adjuntos
	if len(n.Attachments) > MaxAttachments {
		return fmt.Errorf("too many attachments: maximum %d allowed", MaxAttachments)
	}

	var totalSize int64
	for _, attachment := range n.Attachments {
		if attachment.Size > MaxAttachmentSize {
			return fmt.Errorf("attachment %s too large: maximum %d bytes", attachment.Filename, MaxAttachmentSize)
		}
		totalSize += attachment.Size
	}

	if totalSize > MaxAttachmentSize*2 {
		return fmt.Errorf("total attachments size too large")
	}

	// Validar configuración de reintentos
	if n.MaxAttempts < 1 || n.MaxAttempts > 10 {
		return fmt.Errorf("max_attempts must be between 1 and 10")
	}

	// Validar programación
	if n.ScheduledAt != nil && n.ScheduledAt.Before(time.Now()) {
		return fmt.Errorf("scheduled_at must be in the future")
	}

	return nil
}

// ValidateTemplate valida un template de email
func (t *EmailTemplate) ValidateTemplate() error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("template name is required")
	}

	if len(t.Name) > 100 {
		return fmt.Errorf("template name too long: maximum 100 characters")
	}

	if strings.TrimSpace(t.Subject) == "" {
		return fmt.Errorf("template subject is required")
	}

	if strings.TrimSpace(t.BodyText) == "" && strings.TrimSpace(t.BodyHTML) == "" {
		return fmt.Errorf("template must have either text or HTML body")
	}

	// Validar sintaxis de template
	if err := t.validateTemplateSyntax(); err != nil {
		return fmt.Errorf("template syntax error: %w", err)
	}

	return nil
}

// validateTemplateSyntax valida la sintaxis del template
func (t *EmailTemplate) validateTemplateSyntax() error {
	// Validar subject
	if _, err := template.New("subject").Parse(t.Subject); err != nil {
		return fmt.Errorf("invalid subject template: %w", err)
	}

	// Validar body text
	if t.BodyText != "" {
		if _, err := template.New("body_text").Parse(t.BodyText); err != nil {
			return fmt.Errorf("invalid text body template: %w", err)
		}
	}

	// Validar body HTML
	if t.BodyHTML != "" {
		if _, err := template.New("body_html").Parse(t.BodyHTML); err != nil {
			return fmt.Errorf("invalid HTML body template: %w", err)
		}
	}

	return nil
}

// ================================
// MÉTODOS DE UTILIDAD
// ================================

// IsValid verifica si el status es válido
func (s NotificationStatus) IsValid() bool {
	switch s {
	case NotificationStatusPending, NotificationStatusProcessing, NotificationStatusSent,
		NotificationStatusFailed, NotificationStatusCancelled, NotificationStatusScheduled:
		return true
	default:
		return false
	}
}

// IsValid verifica si el tipo es válido
func (t NotificationType) IsValid() bool {
	switch t {
	case NotificationTypeCustom, NotificationTypeWelcome, NotificationTypePasswordReset,
		NotificationTypeSystemAlert, NotificationTypeWorkflow, NotificationTypeUserActivity,
		NotificationTypeBackup, NotificationTypeScheduled, NotificationTypeReminder:
		return true
	default:
		return false
	}
}

// IsValid verifica si la prioridad es válida
func (p NotificationPriority) IsValid() bool {
	switch p {
	case NotificationPriorityLow, NotificationPriorityNormal,
		NotificationPriorityHigh, NotificationPriorityCritical:
		return true
	default:
		return false
	}
}

// CanRetry determina si una notificación puede ser reintentada
func (n *EmailNotification) CanRetry() bool {
	return n.Status == NotificationStatusFailed &&
		n.Attempts < n.MaxAttempts &&
		(n.NextRetryAt == nil || n.NextRetryAt.Before(time.Now()))
}

// IsScheduled determina si una notificación está programada
func (n *EmailNotification) IsScheduled() bool {
	return n.ScheduledAt != nil && n.ScheduledAt.After(time.Now())
}

// ShouldProcess determina si una notificación debe procesarse ahora
func (n *EmailNotification) ShouldProcess() bool {
	if n.Status != NotificationStatusPending && n.Status != NotificationStatusScheduled {
		return false
	}

	if n.ScheduledAt != nil && n.ScheduledAt.After(time.Now()) {
		return false
	}

	return true
}

// AddError añade un error a la notificación
func (n *EmailNotification) AddError(code, message, details string, recoverable bool) {
	n.Errors = append(n.Errors, NotificationError{
		Timestamp:   time.Now(),
		Code:        code,
		Message:     message,
		Details:     details,
		Recoverable: recoverable,
	})
}

// GetLastError obtiene el último error
func (n *EmailNotification) GetLastError() *NotificationError {
	if len(n.Errors) == 0 {
		return nil
	}
	return &n.Errors[len(n.Errors)-1]
}

// CalculateNextRetry calcula el próximo intento con backoff exponencial
func (n *EmailNotification) CalculateNextRetry() time.Time {
	if n.Attempts == 0 {
		return time.Now().Add(DefaultRetryDelay)
	}

	// Backoff exponencial con jitter
	delay := DefaultRetryDelay
	for i := 1; i < n.Attempts; i++ {
		delay *= 2
		if delay > MaxRetryDelay {
			delay = MaxRetryDelay
			break
		}
	}

	// Añadir jitter del 10%
	jitter := time.Duration(float64(delay) * 0.1)
	return time.Now().Add(delay + jitter)
}

// IsDelivered verifica si la notificación fue entregada
func (n *EmailNotification) IsDelivered() bool {
	return n.Status == NotificationStatusSent
}

// HasHTMLVersion verifica si el template tiene versión HTML
func (t *EmailTemplate) HasHTMLVersion() bool {
	return t.BodyHTML != ""
}

// GetFullName obtiene el nombre completo con versión
func (t *EmailTemplate) GetFullName() string {
	return fmt.Sprintf("%s_v%d", t.Name, t.Version)
}

// ToEmailNotification convierte SendEmailRequest a EmailNotification
func (req *SendEmailRequest) ToEmailNotification() *EmailNotification {
	notification := &EmailNotification{
		Type:         req.Type,
		Priority:     req.Priority,
		To:           req.To,
		CC:           req.CC,
		BCC:          req.BCC,
		Subject:      req.Subject,
		Body:         req.Body,
		IsHTML:       req.IsHTML,
		TemplateName: req.TemplateName,
		TemplateData: req.TemplateData,
		ScheduledAt:  req.ScheduledAt,
		MaxAttempts:  req.MaxAttempts,
		UserID:       req.UserID,
		WorkflowID:   req.WorkflowID,
		ExecutionID:  req.ExecutionID,
		Status:       NotificationStatusPending,
		Attempts:     0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Configurar valores por defecto
	if notification.MaxAttempts == 0 {
		notification.MaxAttempts = DefaultMaxAttempts
	}

	if notification.ScheduledAt != nil {
		notification.Status = NotificationStatusScheduled
	}

	return notification
}
