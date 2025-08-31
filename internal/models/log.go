package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WorkflowLog representa el registro de ejecución de un workflow
type WorkflowLog struct {
	ID           primitive.ObjectID     `json:"id" bson:"_id,omitempty"`
	WorkflowID   primitive.ObjectID     `json:"workflow_id" bson:"workflow_id"`
	WorkflowName string                 `json:"workflow_name" bson:"workflow_name"`
	UserID       primitive.ObjectID     `json:"user_id" bson:"user_id"`
	Status       WorkflowStatus         `json:"status" bson:"status"`             // Usar el tipo de types.go
	TriggerType  TriggerType            `json:"trigger_type" bson:"trigger_type"` // Usar el tipo de types.go
	TriggerData  map[string]interface{} `json:"trigger_data" bson:"trigger_data"`
	StartedAt    time.Time              `json:"started_at" bson:"started_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	Duration     *int64                 `json:"duration,omitempty" bson:"duration,omitempty"` // en millisegundos
	Steps        []StepExecution        `json:"steps" bson:"steps"`
	ErrorMessage string                 `json:"error_message,omitempty" bson:"error_message,omitempty"`
	Context      map[string]interface{} `json:"context" bson:"context"`
	Metadata     LogMetadata            `json:"metadata" bson:"metadata"`
	CreatedAt    time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" bson:"updated_at"`
}

// StepExecution representa la ejecución de un paso específico
type StepExecution struct {
	StepID        string                 `json:"step_id" bson:"step_id"`
	StepName      string                 `json:"step_name" bson:"step_name"`
	ActionType    ActionType             `json:"action_type" bson:"action_type"` // Usar el tipo de types.go
	Status        WorkflowStatus         `json:"status" bson:"status"`           // Usar el tipo de types.go
	StartedAt     time.Time              `json:"started_at" bson:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	Duration      *int64                 `json:"duration,omitempty" bson:"duration,omitempty"`
	Input         map[string]interface{} `json:"input" bson:"input"`
	Output        map[string]interface{} `json:"output,omitempty" bson:"output,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty" bson:"error_message,omitempty"`
	RetryCount    int                    `json:"retry_count" bson:"retry_count"`
	ExecutionTime int64                  `json:"execution_time" bson:"execution_time"` // en millisegundos
}

// LogMetadata contiene metadatos adicionales del log
type LogMetadata struct {
	IPAddress   string            `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty" bson:"user_agent,omitempty"`
	Source      string            `json:"source,omitempty" bson:"source,omitempty"` // webhook, manual, scheduled
	Version     string            `json:"version,omitempty" bson:"version,omitempty"`
	Environment string            `json:"environment,omitempty" bson:"environment,omitempty"`
	Tags        []string          `json:"tags,omitempty" bson:"tags,omitempty"`
	Custom      map[string]string `json:"custom,omitempty" bson:"custom,omitempty"`
}

// LogFilter para filtrar logs en consultas
type LogFilter struct {
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
}

// LogStats para estadísticas de logs
type LogStats struct {
	TotalExecutions      int64          `json:"total_executions"`
	SuccessfulRuns       int64          `json:"successful_runs"`
	FailedRuns           int64          `json:"failed_runs"`
	AverageExecutionTime float64        `json:"average_execution_time"`
	TotalExecutionTime   int64          `json:"total_execution_time"`
	SuccessRate          float64        `json:"success_rate"`
	MostUsedTriggers     []TriggerStats `json:"most_used_triggers"`
	ErrorDistribution    []ErrorStats   `json:"error_distribution"`
}

// TriggerStats para estadísticas de triggers
type TriggerStats struct {
	Type  TriggerType `json:"type"`
	Count int64       `json:"count"`
}

// ErrorStats para estadísticas de errores
type ErrorStats struct {
	Error string `json:"error"`
	Count int64  `json:"count"`
}

// AGREGADO: Tipos adicionales faltantes para compatibilidad completa

// Log alias para WorkflowLog (compatibilidad con log_repository.go)
type Log = WorkflowLog

// ExecutionStatus representa el estado de ejecución (alias para WorkflowStatus)
type ExecutionStatus = WorkflowStatus

// LogStatistics para estadísticas del sistema completo
type LogStatistics struct {
	TotalExecutions      int64                 `json:"total_executions"`
	SuccessfulRuns       int64                 `json:"successful_runs"`
	FailedRuns           int64                 `json:"failed_runs"`
	ExecutionsToday      int64                 `json:"executions_today"`
	ExecutionsThisWeek   int64                 `json:"executions_this_week"`
	ExecutionsThisMonth  int64                 `json:"executions_this_month"`
	AverageExecutionTime float64               `json:"average_execution_time"`
	SuccessRate          float64               `json:"success_rate"`
	FailureRate          float64               `json:"failure_rate"`
	TriggerDistribution  []TriggerDistribution `json:"trigger_distribution"`
	ErrorDistribution    []ErrorDistribution   `json:"error_distribution"`
	HourlyDistribution   []HourlyStats         `json:"hourly_distribution"`
	LastUpdated          time.Time             `json:"last_updated"`
	DataPeriod           string                `json:"data_period"`
}

// TriggerDistribution para distribución de triggers
type TriggerDistribution struct {
	TriggerType TriggerType `json:"trigger_type"`
	Count       int64       `json:"count"`
	Percentage  float64     `json:"percentage"`
}

// ErrorDistribution para distribución de errores
type ErrorDistribution struct {
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Count        int64     `json:"count"`
	Percentage   float64   `json:"percentage"`
	LastOccurred time.Time `json:"last_occurred"`
}

// HourlyStats para estadísticas por hora
type HourlyStats struct {
	Hour           int     `json:"hour"`
	ExecutionCount int64   `json:"execution_count"`
	SuccessCount   int64   `json:"success_count"`
	FailureCount   int64   `json:"failure_count"`
	AverageTime    float64 `json:"average_time"`
}

// LogQueryRequest para consultas de logs
type LogQueryRequest struct {
	WorkflowID  *primitive.ObjectID `json:"workflow_id,omitempty"`
	UserID      *primitive.ObjectID `json:"user_id,omitempty"`
	Status      *WorkflowStatus     `json:"status,omitempty"`
	TriggerType *TriggerType        `json:"trigger_type,omitempty"`
	StartDate   *time.Time          `json:"start_date,omitempty"`
	EndDate     *time.Time          `json:"end_date,omitempty"`
	Page        int                 `json:"page" validate:"min=1"`
	PageSize    int                 `json:"page_size" validate:"min=1,max=100"`
	SortBy      string              `json:"sort_by,omitempty"`
	SortOrder   string              `json:"sort_order,omitempty" validate:"omitempty,oneof=asc desc"`
}

// LogListResponse para respuestas de lista de logs
type LogListResponse struct {
	Logs       []*WorkflowLog `json:"logs"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
}

// WorkflowListResponse para respuestas de lista de workflows
type WorkflowListResponse struct {
	Workflows  []*Workflow `json:"workflows"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// ExecutedWorkflowStep para respuestas (alias de WorkflowStep)
type ExecutedWorkflowStep = WorkflowStep
