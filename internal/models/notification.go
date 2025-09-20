package models

import (
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationType tipos de notificación
type NotificationType string

const (
	NotificationTypeCustom       NotificationType = "custom"
	NotificationTypeSystemAlert  NotificationType = "system_alert"
	NotificationTypeWorkflow     NotificationType = "workflow"
	NotificationTypeUserActivity NotificationType = "user_activity"
	NotificationTypeBackup       NotificationType = "backup"
	NotificationTypeScheduled    NotificationType = "scheduled"
)

// NotificationStatus estados de las notificaciones
type NotificationStatus string

const (
	NotificationStatusPending   NotificationStatus = "pending"
	NotificationStatusSending   NotificationStatus = "sending"
	NotificationStatusSent      NotificationStatus = "sent"
	NotificationStatusFailed    NotificationStatus = "failed"
	NotificationStatusCancelled NotificationStatus = "cancelled"
	NotificationStatusScheduled NotificationStatus = "scheduled"
)

// NotificationPriority prioridades de notificación
type NotificationPriority string

const (
	NotificationPriorityLow      NotificationPriority = "low"
	NotificationPriorityNormal   NotificationPriority = "normal"
	NotificationPriorityHigh     NotificationPriority = "high"
	NotificationPriorityCritical NotificationPriority = "critical"
	NotificationPriorityUrgent   NotificationPriority = "urgent"
)

// EmailNotification modelo principal para notificaciones por email
type EmailNotification struct {
	ID       primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	Type     NotificationType     `json:"type" bson:"type"`
	Status   NotificationStatus   `json:"status" bson:"status"`
	Priority NotificationPriority `json:"priority" bson:"priority"`

	// Contenido del email
	To      []string `json:"to" bson:"to"`
	CC      []string `json:"cc,omitempty" bson:"cc,omitempty"`
	BCC     []string `json:"bcc,omitempty" bson:"bcc,omitempty"`
	Subject string   `json:"subject" bson:"subject"`
	Body    string   `json:"body" bson:"body"`
	IsHTML  bool     `json:"is_html" bson:"is_html"`

	// Template info
	TemplateName string                 `json:"template_name,omitempty" bson:"template_name,omitempty"`
	TemplateData map[string]interface{} `json:"template_data,omitempty" bson:"template_data,omitempty"`

	// Metadatos de relación
	UserID      *primitive.ObjectID `json:"user_id,omitempty" bson:"user_id,omitempty"`
	WorkflowID  *primitive.ObjectID `json:"workflow_id,omitempty" bson:"workflow_id,omitempty"`
	ExecutionID *string             `json:"execution_id,omitempty" bson:"execution_id,omitempty"`

	// Programación
	ScheduledAt *time.Time `json:"scheduled_at,omitempty" bson:"scheduled_at,omitempty"`
	SentAt      *time.Time `json:"sent_at,omitempty" bson:"sent_at,omitempty"`

	// Manejo de errores y reintentos
	Attempts      int                 `json:"attempts" bson:"attempts"`
	MaxAttempts   int                 `json:"max_attempts" bson:"max_attempts"`
	LastAttemptAt *time.Time          `json:"last_attempt_at,omitempty" bson:"last_attempt_at,omitempty"`
	NextRetryAt   *time.Time          `json:"next_retry_at,omitempty" bson:"next_retry_at,omitempty"`
	Errors        []NotificationError `json:"errors,omitempty" bson:"errors,omitempty"`

	// Información de entrega
	MessageID    string                 `json:"message_id,omitempty" bson:"message_id,omitempty"`
	ProviderData map[string]interface{} `json:"provider_data,omitempty" bson:"provider_data,omitempty"`

	// Auditoría
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
	CreatedBy *string   `json:"created_by,omitempty" bson:"created_by,omitempty"`
}

// NotificationError estructura para errores de notificación
type NotificationError struct {
	Timestamp   time.Time `json:"timestamp" bson:"timestamp"`
	Code        string    `json:"code" bson:"code"`
	Message     string    `json:"message" bson:"message"`
	Details     string    `json:"details,omitempty" bson:"details,omitempty"`
	Recoverable bool      `json:"recoverable" bson:"recoverable"`
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

	// Estado
	IsActive bool `json:"is_active" bson:"is_active"`

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

// Métodos de conveniencia para EmailNotification

// IsDelivered verifica si la notificación fue entregada exitosamente
func (n *EmailNotification) IsDelivered() bool {
	return n.Status == NotificationStatusSent
}

// IsFailed verifica si la notificación falló definitivamente
func (n *EmailNotification) IsFailed() bool {
	return n.Status == NotificationStatusFailed && n.Attempts >= n.MaxAttempts
}

// CanRetry verifica si se puede reintentar la notificación
func (n *EmailNotification) CanRetry() bool {
	return n.Status == NotificationStatusFailed && n.Attempts < n.MaxAttempts
}

// AddError agrega un error a la notificación
func (n *EmailNotification) AddError(code, message, details string, recoverable bool) {
	if n.Errors == nil {
		n.Errors = make([]NotificationError, 0)
	}

	n.Errors = append(n.Errors, NotificationError{
		Timestamp:   time.Now(),
		Code:        code,
		Message:     message,
		Details:     details,
		Recoverable: recoverable,
	})
}

// GetLastError obtiene el último error registrado
func (n *EmailNotification) GetLastError() *NotificationError {
	if len(n.Errors) == 0 {
		return nil
	}
	return &n.Errors[len(n.Errors)-1]
}

// MarkAsSending marca la notificación como enviándose
func (n *EmailNotification) MarkAsSending() {
	n.Status = NotificationStatusSending
	n.UpdatedAt = time.Now()
}

// MarkAsSent marca la notificación como enviada exitosamente
func (n *EmailNotification) MarkAsSent(messageID string) {
	n.Status = NotificationStatusSent
	n.MessageID = messageID
	now := time.Now()
	n.SentAt = &now
	n.UpdatedAt = now
}

// MarkAsFailed marca la notificación como fallida
func (n *EmailNotification) MarkAsFailed() {
	n.Status = NotificationStatusFailed
	n.UpdatedAt = time.Now()
	n.Attempts++
	now := time.Now()
	n.LastAttemptAt = &now

	// Calcular próximo intento si es recuperable
	if n.CanRetry() {
		nextRetry := now.Add(time.Duration(n.Attempts*n.Attempts) * time.Minute) // Backoff exponencial
		n.NextRetryAt = &nextRetry
	}
}

// Validate valida la estructura de la notificación
func (n *EmailNotification) Validate() error {
	if len(n.To) == 0 {
		return errors.New("at least one recipient is required")
	}

	if n.Subject == "" && n.TemplateName == "" {
		return errors.New("subject is required when not using a template")
	}

	if n.Body == "" && n.TemplateName == "" {
		return errors.New("body is required when not using a template")
	}

	if n.MaxAttempts <= 0 {
		n.MaxAttempts = 3 // Default
	}

	return nil
}

// Métodos para EmailTemplate

// GetFullName obtiene el nombre completo del template con versión
func (t *EmailTemplate) GetFullName() string {
	return fmt.Sprintf("%s_v%d", t.Name, t.Version)
}

// HasHTMLVersion verifica si el template tiene versión HTML
func (t *EmailTemplate) HasHTMLVersion() bool {
	return t.BodyHTML != ""
}

// ValidateTemplate valida la estructura del template
func (t *EmailTemplate) ValidateTemplate() error {
	if t.Name == "" {
		return errors.New("template name is required")
	}

	if t.Subject == "" {
		return errors.New("template subject is required")
	}

	if t.BodyText == "" {
		return errors.New("template body is required")
	}

	if t.Language == "" {
		t.Language = "en" // Default
	}

	if t.Version <= 0 {
		t.Version = 1 // Default
	}

	return nil
}

// TimeSeriesPoint punto de datos para series temporales
type TimeSeriesPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Count     int64                  `json:"count"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// DeliveryReport reporte de entrega de notificaciones
type DeliveryReport struct {
	Period     string                     `json:"period"`
	TotalSent  int64                      `json:"total_sent"`
	Delivered  int64                      `json:"delivered"`
	Failed     int64                      `json:"failed"`
	Bounced    int64                      `json:"bounced"`
	ByProvider map[string]int64           `json:"by_provider,omitempty"`
	ByTemplate map[string]int64           `json:"by_template,omitempty"`
	TopErrors  []NotificationErrorSummary `json:"top_errors,omitempty"`
	Timeline   []TimeSeriesPoint          `json:"timeline,omitempty"`
}

// NotificationErrorSummary resumen de errores
type NotificationErrorSummary struct {
	Code           string    `json:"code"`
	Message        string    `json:"message"`
	Count          int64     `json:"count"`
	LastOccurrence time.Time `json:"last_occurrence"`
	Recoverable    bool      `json:"recoverable"`
}

// FailureAnalysis análisis de fallos
type FailureAnalysis struct {
	Period         string                     `json:"period"`
	TotalFailures  int64                      `json:"total_failures"`
	FailureRate    float64                    `json:"failure_rate"`
	TopErrorCodes  []NotificationErrorSummary `json:"top_error_codes"`
	FailuresByHour map[int]int64              `json:"failures_by_hour"`
	RecoveryRate   float64                    `json:"recovery_rate"`
	AverageRetries float64                    `json:"average_retries"`
}

// PerformanceReport reporte de performance
type PerformanceReport struct {
	Period                string        `json:"period"`
	TotalProcessed        int64         `json:"total_processed"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	ThroughputPerSecond   float64       `json:"throughput_per_second"`
	QueueDepth            int64         `json:"queue_depth"`
	WorkerUtilization     float64       `json:"worker_utilization"`
}
