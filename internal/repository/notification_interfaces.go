package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"Engine_API_Workflow/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationRepository interfaz para el repositorio de notificaciones
type NotificationRepository interface {
	// CRUD básico
	Create(ctx context.Context, notification *models.EmailNotification) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.EmailNotification, error)
	Update(ctx context.Context, notification *models.EmailNotification) error
	Delete(ctx context.Context, id primitive.ObjectID) error

	// Listado y búsqueda
	List(ctx context.Context, filters map[string]interface{}, opts *PaginationOptions) ([]*models.EmailNotification, int64, error)

	// Procesamiento de notificaciones
	GetPending(ctx context.Context, limit int) ([]*models.EmailNotification, error)
	GetScheduled(ctx context.Context, limit int) ([]*models.EmailNotification, error)
	GetFailedForRetry(ctx context.Context, limit int) ([]*models.EmailNotification, error)

	// Estadísticas y métricas
	GetStats(ctx context.Context, timeRange time.Duration) (*models.NotificationStats, error)

	// Mantenimiento
	CleanupOld(ctx context.Context, olderThan time.Duration) (int64, error)
}

// TemplateRepository interfaz para el repositorio de templates
type TemplateRepository interface {
	// CRUD básico
	Create(ctx context.Context, template *models.EmailTemplate) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.EmailTemplate, error)
	GetByName(ctx context.Context, name string) (*models.EmailTemplate, error)
	Update(ctx context.Context, template *models.EmailTemplate) error
	Delete(ctx context.Context, id primitive.ObjectID) error

	// Listado
	List(ctx context.Context, filters map[string]interface{}) ([]*models.EmailTemplate, error)
	ListByType(ctx context.Context, notificationType models.NotificationType) ([]*models.EmailTemplate, error)

	// Versionado
	CreateVersion(ctx context.Context, template *models.EmailTemplate) error
	GetVersions(ctx context.Context, templateName string) ([]*models.EmailTemplate, error)
	GetLatestVersion(ctx context.Context, templateName string) (*models.EmailTemplate, error)

	// Estado
	SetActive(ctx context.Context, id primitive.ObjectID, active bool) error
	GetActive(ctx context.Context) ([]*models.EmailTemplate, error)
}

// Errores específicos del repositorio de notificaciones
var (
	ErrNotificationNotFound = &RepositoryError{
		Code:    "NOTIFICATION_NOT_FOUND",
		Message: "notification not found",
	}
	ErrTemplateNotFound = &RepositoryError{
		Code:    "TEMPLATE_NOT_FOUND",
		Message: "email template not found",
	}
	ErrTemplateAlreadyExists = &RepositoryError{
		Code:    "TEMPLATE_ALREADY_EXISTS",
		Message: "email template already exists",
	}
	ErrInvalidNotificationData = &RepositoryError{
		Code:    "INVALID_NOTIFICATION_DATA",
		Message: "invalid notification data",
	}
	ErrNotificationProcessing = &RepositoryError{
		Code:    "NOTIFICATION_PROCESSING",
		Message: "notification is currently being processed",
	}
)

// NotificationSearchFilters filtros específicos para búsqueda de notificaciones
type NotificationSearchFilters struct {
	// Filtros básicos
	Status   []models.NotificationStatus   `json:"status,omitempty"`
	Type     []models.NotificationType     `json:"type,omitempty"`
	Priority []models.NotificationPriority `json:"priority,omitempty"`

	// Filtros de relación
	UserID      *primitive.ObjectID `json:"user_id,omitempty"`
	WorkflowID  *primitive.ObjectID `json:"workflow_id,omitempty"`
	ExecutionID *string             `json:"execution_id,omitempty"`

	// Filtros de tiempo
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	SentAfter     *time.Time `json:"sent_after,omitempty"`
	SentBefore    *time.Time `json:"sent_before,omitempty"`

	// Filtros de contenido
	ToEmail     *string `json:"to_email,omitempty"`
	SubjectLike *string `json:"subject_like,omitempty"`

	// Filtros de estado
	HasErrors   *bool `json:"has_errors,omitempty"`
	IsScheduled *bool `json:"is_scheduled,omitempty"`
	CanRetry    *bool `json:"can_retry,omitempty"`

	// Metadatos
	MessageID    *string `json:"message_id,omitempty"`
	TemplateName *string `json:"template_name,omitempty"`
}

// TemplateSearchFilters filtros para búsqueda de templates
type TemplateSearchFilters struct {
	Type     *models.NotificationType `json:"type,omitempty"`
	IsActive *bool                    `json:"is_active,omitempty"`
	Language *string                  `json:"language,omitempty"`
	Version  *int                     `json:"version,omitempty"`

	CreatedAfter  *time.Time          `json:"created_after,omitempty"`
	CreatedBefore *time.Time          `json:"created_before,omitempty"`
	CreatedBy     *primitive.ObjectID `json:"created_by,omitempty"`

	NameLike *string `json:"name_like,omitempty"`
}

// NotificationAggregation opciones de agregación para estadísticas
type NotificationAggregation struct {
	GroupBy   []string               `json:"group_by,omitempty"` // status, type, priority, hour, day
	TimeRange time.Duration          `json:"time_range,omitempty"`
	Interval  string                 `json:"interval,omitempty"` // hour, day, week, month
	Metrics   []string               `json:"metrics,omitempty"`  // count, success_rate, avg_retries
	Filters   map[string]interface{} `json:"filters,omitempty"`
}

// RepositoryError error específico del repositorio
type RepositoryError struct {
	Code    string
	Message string
	Err     error
}

func (e *RepositoryError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *RepositoryError) Unwrap() error {
	return e.Err
}

// IsNotificationNotFound verifica si el error es "not found"
func IsNotificationNotFound(err error) bool {
	var repoErr *RepositoryError
	return errors.As(err, &repoErr) && repoErr.Code == "NOTIFICATION_NOT_FOUND"
}

// IsTemplateNotFound verifica si el error es "template not found"
func IsTemplateNotFound(err error) bool {
	var repoErr *RepositoryError
	return errors.As(err, &repoErr) && repoErr.Code == "TEMPLATE_NOT_FOUND"
}

// NotificationBulkOperation operación en lote para notificaciones
type NotificationBulkOperation struct {
	Type    string                      `json:"type"` // create, update, delete
	Filters map[string]interface{}      `json:"filters,omitempty"`
	Updates map[string]interface{}      `json:"updates,omitempty"`
	Data    []*models.EmailNotification `json:"data,omitempty"`
}

// NotificationBulkResult resultado de operación en lote
type NotificationBulkResult struct {
	ProcessedCount int64                   `json:"processed_count"`
	ModifiedCount  int64                   `json:"modified_count"`
	ErrorCount     int64                   `json:"error_count"`
	Errors         []NotificationBulkError `json:"errors,omitempty"`
}

// NotificationBulkError error en operación en lote
type NotificationBulkError struct {
	Index   int    `json:"index"`
	ID      string `json:"id,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Extensiones para repositorios avanzados (opcional)
type AdvancedNotificationRepository interface {
	NotificationRepository

	// Operaciones en lote
	BulkCreate(ctx context.Context, notifications []*models.EmailNotification) (*NotificationBulkResult, error)
	BulkUpdate(ctx context.Context, operation *NotificationBulkOperation) (*NotificationBulkResult, error)
	BulkDelete(ctx context.Context, filters map[string]interface{}) (*NotificationBulkResult, error)

	// Agregaciones avanzadas
	Aggregate(ctx context.Context, aggregation *NotificationAggregation) ([]bson.M, error)
	GetTimeSeriesStats(ctx context.Context, timeRange time.Duration, interval string) ([]TimeSeriesPoint, error)

	// Análisis y reporting
	GetDeliveryReport(ctx context.Context, filters *NotificationSearchFilters) (*DeliveryReport, error)
	GetFailureAnalysis(ctx context.Context, timeRange time.Duration) (*FailureAnalysis, error)

	// Optimización y mantenimiento
	OptimizeIndexes(ctx context.Context) error
	AnalyzePerformance(ctx context.Context) (*PerformanceReport, error)
}

// AdvancedTemplateRepository repositorio avanzado para templates
type AdvancedTemplateRepository interface {
	TemplateRepository

	// Testing y validación
	ValidateTemplate(ctx context.Context, template *models.EmailTemplate, testData map[string]interface{}) error
	RenderPreview(ctx context.Context, templateName string, data map[string]interface{}) (*TemplatePreview, error)

	// Análisis de uso
	GetUsageStats(ctx context.Context, templateName string, timeRange time.Duration) (*TemplateUsageStats, error)
	GetPopularTemplates(ctx context.Context, limit int) ([]*TemplatePopularity, error)

	// Importación/Exportación
	ExportTemplate(ctx context.Context, templateName string) (*TemplateExport, error)
	ImportTemplate(ctx context.Context, templateData *TemplateImport) error

	// Comparación y diff
	CompareVersions(ctx context.Context, templateName string, version1, version2 int) (*TemplateDiff, error)
}

// Helper types para funcionalidades avanzadas
type TimeSeriesPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
}

type DeliveryReport struct {
	TotalNotifications   int64                             `json:"total_notifications"`
	SuccessfulDeliveries int64                             `json:"successful_deliveries"`
	FailedDeliveries     int64                             `json:"failed_deliveries"`
	DeliveryRate         float64                           `json:"delivery_rate"`
	ByType               map[models.NotificationType]int64 `json:"by_type"`
	ByProvider           map[string]int64                  `json:"by_provider"`
	Timeline             []TimeSeriesPoint                 `json:"timeline"`
}

type FailureAnalysis struct {
	TotalFailures    int64            `json:"total_failures"`
	FailuresByReason map[string]int64 `json:"failures_by_reason"`
	FailuresByHour   map[int]int64    `json:"failures_by_hour"`
	TopFailedEmails  []string         `json:"top_failed_emails"`
	Recommendations  []string         `json:"recommendations"`
}

type PerformanceReport struct {
	AvgProcessingTime time.Duration          `json:"avg_processing_time"`
	AvgDeliveryTime   time.Duration          `json:"avg_delivery_time"`
	ThroughputPerHour float64                `json:"throughput_per_hour"`
	IndexUsage        map[string]interface{} `json:"index_usage"`
	Recommendations   []string               `json:"recommendations"`
}

type TemplatePreview struct {
	Subject  string   `json:"subject"`
	BodyHTML string   `json:"body_html"`
	BodyText string   `json:"body_text"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type TemplateUsageStats struct {
	TemplateName    string           `json:"template_name"`
	TotalUsage      int64            `json:"total_usage"`
	SuccessfulSends int64            `json:"successful_sends"`
	FailedSends     int64            `json:"failed_sends"`
	LastUsed        time.Time        `json:"last_used"`
	UsageByDay      map[string]int64 `json:"usage_by_day"`
}

type TemplatePopularity struct {
	TemplateName string    `json:"template_name"`
	UsageCount   int64     `json:"usage_count"`
	SuccessRate  float64   `json:"success_rate"`
	LastUsed     time.Time `json:"last_used"`
}

type TemplateExport struct {
	Template   *models.EmailTemplate  `json:"template"`
	Metadata   map[string]interface{} `json:"metadata"`
	Version    string                 `json:"version"`
	ExportedAt time.Time              `json:"exported_at"`
}

type TemplateImport struct {
	Template *models.EmailTemplate  `json:"template"`
	Metadata map[string]interface{} `json:"metadata"`
	Options  TemplateImportOptions  `json:"options"`
}

type TemplateImportOptions struct {
	OverwriteExisting bool `json:"overwrite_existing"`
	CreateNewVersion  bool `json:"create_new_version"`
	ValidateOnly      bool `json:"validate_only"`
	ImportAsInactive  bool `json:"import_as_inactive"`
}

type TemplateDiff struct {
	TemplateName string              `json:"template_name"`
	Version1     int                 `json:"version1"`
	Version2     int                 `json:"version2"`
	Changes      []TemplateChange    `json:"changes"`
	Summary      TemplateDiffSummary `json:"summary"`
}

type TemplateChange struct {
	Field    string      `json:"field"`
	Type     string      `json:"type"` // added, removed, modified
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
}

type TemplateDiffSummary struct {
	TotalChanges   int `json:"total_changes"`
	FieldsAdded    int `json:"fields_added"`
	FieldsRemoved  int `json:"fields_removed"`
	FieldsModified int `json:"fields_modified"`
}
