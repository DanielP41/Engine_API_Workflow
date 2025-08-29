package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserListResponse respuesta paginada para listado de usuarios
type UserListResponse struct {
	Users      []User `json:"users"`
	TotalCount int64  `json:"total_count"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
	HasNext    bool   `json:"has_next"`
	HasPrev    bool   `json:"has_prev"`
}

// WorkflowListResponse respuesta paginada para listado de workflows
type WorkflowListResponse struct {
	Workflows  []Workflow `json:"workflows"`
	TotalCount int64      `json:"total_count"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
	HasNext    bool       `json:"has_next"`
	HasPrev    bool       `json:"has_prev"`
}

// LogQueryRequest estructura para consultas de logs
type LogQueryRequest struct {
	WorkflowID  *primitive.ObjectID `json:"workflow_id,omitempty"`
	UserID      *primitive.ObjectID `json:"user_id,omitempty"`
	Status      *WorkflowStatus     `json:"status,omitempty"`
	TriggerType *TriggerType        `json:"trigger_type,omitempty"`
	StartDate   *time.Time          `json:"start_date,omitempty"`
	EndDate     *time.Time          `json:"end_date,omitempty"`
	Source      *string             `json:"source,omitempty"`
	Environment *string             `json:"environment,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	HasErrors   *bool               `json:"has_errors,omitempty"`
	MinDuration *int64              `json:"min_duration,omitempty"`
	MaxDuration *int64              `json:"max_duration,omitempty"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"page_size"`
	SortBy      string              `json:"sort_by"`    // created_at, duration, status
	SortOrder   string              `json:"sort_order"` // asc, desc
}

// LogListResponse respuesta paginada para listado de logs
type LogListResponse struct {
	Logs       []WorkflowLog `json:"logs"`
	TotalCount int64         `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
	HasNext    bool          `json:"has_next"`
	HasPrev    bool          `json:"has_prev"`
	Stats      *LogStats     `json:"stats,omitempty"`
}

// PaginationRequest estructura base para paginación
type PaginationRequest struct {
	Page      int    `json:"page" query:"page" validate:"min=1"`
	PageSize  int    `json:"page_size" query:"page_size" validate:"min=1,max=100"`
	SortBy    string `json:"sort_by" query:"sort_by"`
	SortOrder string `json:"sort_order" query:"sort_order" validate:"oneof=asc desc"`
}

// APIResponse estructura genérica de respuesta
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *MetaData   `json:"meta,omitempty"`
}

// APIError estructura de error detallado
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Field   string      `json:"field,omitempty"`
}

// MetaData metadatos adicionales de respuesta
type MetaData struct {
	RequestID     string          `json:"request_id,omitempty"`
	Timestamp     time.Time       `json:"timestamp"`
	Version       string          `json:"version,omitempty"`
	ExecutionTime int64           `json:"execution_time,omitempty"` // en millisegundos
	Pagination    *PaginationMeta `json:"pagination,omitempty"`
}

// PaginationMeta información de paginación
type PaginationMeta struct {
	CurrentPage int   `json:"current_page"`
	PageSize    int   `json:"page_size"`
	TotalPages  int   `json:"total_pages"`
	TotalCount  int64 `json:"total_count"`
	HasNext     bool  `json:"has_next"`
	HasPrev     bool  `json:"has_prev"`
}

// UserFilter filtros para búsqueda de usuarios
type UserFilter struct {
	Email          *string     `json:"email,omitempty"`
	Role           *UserRole   `json:"role,omitempty"`
	Status         *UserStatus `json:"status,omitempty"`
	Search         *string     `json:"search,omitempty"`
	CreatedAfter   *time.Time  `json:"created_after,omitempty"`
	CreatedBefore  *time.Time  `json:"created_before,omitempty"`
	LastLoginAfter *time.Time  `json:"last_login_after,omitempty"`
}

// UserStatus enum para estado del usuario
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusPending   UserStatus = "pending"
)

// UserRole enum para roles de usuario (si no existe en user.go)
type UserRole string

const (
	UserRoleAdmin     UserRole = "admin"
	UserRoleUser      UserRole = "user"
	UserRoleModerator UserRole = "moderator"
	UserRoleViewer    UserRole = "viewer"
)

// SearchRequest estructura para búsquedas generales
type SearchRequest struct {
	Query     string                 `json:"query" validate:"required,min=2"`
	Filters   map[string]interface{} `json:"filters,omitempty"`
	Page      int                    `json:"page" validate:"min=1"`
	PageSize  int                    `json:"page_size" validate:"min=1,max=100"`
	SortBy    string                 `json:"sort_by"`
	SortOrder string                 `json:"sort_order" validate:"oneof=asc desc"`
}

// BulkOperationRequest para operaciones en lote
type BulkOperationRequest struct {
	IDs       []primitive.ObjectID   `json:"ids" validate:"required,min=1"`
	Operation string                 `json:"operation" validate:"required"` // delete, update, activate, deactivate
	Data      map[string]interface{} `json:"data,omitempty"`
}

// BulkOperationResponse respuesta de operaciones en lote
type BulkOperationResponse struct {
	Success      []primitive.ObjectID `json:"success"`
	Failed       []BulkOperationError `json:"failed"`
	TotalCount   int                  `json:"total_count"`
	SuccessCount int                  `json:"success_count"`
	FailedCount  int                  `json:"failed_count"`
}

// BulkOperationError error específico en operación en lote
type BulkOperationError struct {
	ID    primitive.ObjectID `json:"id"`
	Error string             `json:"error"`
}

// ExecutionStatus enum para estado de ejecución
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusTimeout   ExecutionStatus = "timeout"
	ExecutionStatusRetrying  ExecutionStatus = "retrying"
)

// LogStatistics estadísticas detalladas de logs
type LogStatistics struct {
	// Estadísticas generales
	TotalExecutions int64 `json:"total_executions"`
	SuccessfulRuns  int64 `json:"successful_runs"`
	FailedRuns      int64 `json:"failed_runs"`
	CancelledRuns   int64 `json:"cancelled_runs"`
	TimeoutRuns     int64 `json:"timeout_runs"`

	// Métricas de tiempo
	AverageExecutionTime float64 `json:"average_execution_time"` // en millisegundos
	MinExecutionTime     int64   `json:"min_execution_time"`
	MaxExecutionTime     int64   `json:"max_execution_time"`
	TotalExecutionTime   int64   `json:"total_execution_time"`

	// Tasas y porcentajes
	SuccessRate float64 `json:"success_rate"`
	FailureRate float64 `json:"failure_rate"`

	// Estadísticas por período
	ExecutionsToday     int64 `json:"executions_today"`
	ExecutionsThisWeek  int64 `json:"executions_this_week"`
	ExecutionsThisMonth int64 `json:"executions_this_month"`

	// Distribuciones
	TriggerDistribution []TriggerDistribution `json:"trigger_distribution"`
	ErrorDistribution   []ErrorDistribution   `json:"error_distribution"`
	HourlyDistribution  []HourlyStats         `json:"hourly_distribution"`

	// Metadatos
	LastUpdated time.Time `json:"last_updated"`
	DataPeriod  string    `json:"data_period"` // last_24h, last_7d, last_30d, all_time
}

// TriggerDistribution distribución por tipo de trigger
type TriggerDistribution struct {
	TriggerType TriggerType `json:"trigger_type"`
	Count       int64       `json:"count"`
	Percentage  float64     `json:"percentage"`
}

// ErrorDistribution distribución de errores
type ErrorDistribution struct {
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Count        int64     `json:"count"`
	Percentage   float64   `json:"percentage"`
	LastOccurred time.Time `json:"last_occurred"`
}

// HourlyStats estadísticas por hora
type HourlyStats struct {
	Hour           int     `json:"hour"` // 0-23
	ExecutionCount int64   `json:"execution_count"`
	SuccessCount   int64   `json:"success_count"`
	FailureCount   int64   `json:"failure_count"`
	AverageTime    float64 `json:"average_time"`
}

// Log es un alias para WorkflowLog (para compatibilidad)
type Log = WorkflowLog

// ExecutedWorkflowStep representa un paso ejecutado en la respuesta de un workflow
type ExecutedWorkflowStep struct {
	ID          string                 `json:"id" bson:"id"`
	Name        string                 `json:"name" bson:"name"`
	Type        ActionType             `json:"type" bson:"type"`
	Config      map[string]interface{} `json:"config" bson:"config"`
	Order       int                    `json:"order" bson:"order"`
	Status      WorkflowStatus         `json:"status" bson:"status"`
	StartedAt   *time.Time             `json:"started_at,omitempty" bson:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	Duration    *int64                 `json:"duration,omitempty" bson:"duration,omitempty"`
	Input       map[string]interface{} `json:"input,omitempty" bson:"input,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty" bson:"output,omitempty"`
	Error       string                 `json:"error,omitempty" bson:"error,omitempty"`
	RetryCount  int                    `json:"retry_count" bson:"retry_count"`
}
