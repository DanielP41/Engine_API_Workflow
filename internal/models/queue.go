package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ===============================================
// QUEUE & TASK STRUCTURES
// ===============================================

// QueueTask representa una tarea en la cola
type QueueTask struct {
	ID          primitive.ObjectID     `json:"id" bson:"_id,omitempty"`
	WorkflowID  primitive.ObjectID     `json:"workflow_id" bson:"workflow_id"`
	ExecutionID *string                `json:"execution_id" bson:"execution_id"`
	UserID      primitive.ObjectID     `json:"user_id" bson:"user_id"`
	TaskName    string                 `json:"task_name" bson:"task_name"`
	TaskType    string                 `json:"task_type" bson:"task_type"`
	Status      TaskStatus             `json:"status" bson:"status"`
	Priority    TaskPriority           `json:"priority" bson:"priority"`
	QueueName   string                 `json:"queue_name" bson:"queue_name"`
	Payload     map[string]interface{} `json:"payload" bson:"payload"`
	Result      map[string]interface{} `json:"result,omitempty" bson:"result,omitempty"`
	Config      *TaskConfig            `json:"config,omitempty" bson:"config,omitempty"`

	// Timestamps
	ScheduledAt *time.Time `json:"scheduled_at,omitempty" bson:"scheduled_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at" bson:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty" bson:"started_at,omitempty"`
	ProcessedAt *time.Time `json:"processed_at,omitempty" bson:"processed_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at" bson:"updated_at"`

	// Reintento y manejo de errores
	Attempts    int        `json:"attempts" bson:"attempts"`
	MaxAttempts int        `json:"max_attempts" bson:"max_attempts"`
	RetryCount  int        `json:"retry_count" bson:"retry_count"` // Alias para Attempts
	MaxRetries  int        `json:"max_retries" bson:"max_retries"` // Alias para MaxAttempts
	LastAttempt *time.Time `json:"last_attempt,omitempty" bson:"last_attempt,omitempty"`
	NextRetry   *time.Time `json:"next_retry,omitempty" bson:"next_retry,omitempty"`

	// Errores
	LastError    string `json:"last_error,omitempty" bson:"last_error"`
	ErrorMessage string `json:"error_message,omitempty" bson:"error_message"` // Alias para LastError

	// Resultados (alias para compatibilidad)
	Results map[string]interface{} `json:"results,omitempty" bson:"results,omitempty"`

	// Campos adicionales para compatibilidad
	Type string `json:"type,omitempty" bson:"type,omitempty"` // Alias para TaskType
}

// TaskStatus estados de una tarea
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusRetrying   TaskStatus = "retrying"
	TaskStatusScheduled  TaskStatus = "scheduled"
)

// QueueTaskStatus alias para TaskStatus (compatibilidad con Redis repository)
type QueueTaskStatus = TaskStatus

// IsActive verifica si una tarea está en estado activo
func (ts TaskStatus) IsActive() bool {
	return ts == TaskStatusQueued || ts == TaskStatusProcessing || ts == TaskStatusRetrying
}

// IsCompleted verifica si una tarea está completada (exitosa o no)
func (ts TaskStatus) IsCompleted() bool {
	return ts == TaskStatusCompleted || ts == TaskStatusFailed || ts == TaskStatusCancelled
}

// TaskPriority prioridades de tareas
type TaskPriority int

const (
	TaskPriorityLow      TaskPriority = 1
	TaskPriorityNormal   TaskPriority = 5
	TaskPriorityHigh     TaskPriority = 8
	TaskPriorityCritical TaskPriority = 10
)

// String devuelve la representación en string de la prioridad
func (tp TaskPriority) String() string {
	switch tp {
	case TaskPriorityLow:
		return "low"
	case TaskPriorityNormal:
		return "normal"
	case TaskPriorityHigh:
		return "high"
	case TaskPriorityCritical:
		return "critical"
	default:
		return "normal"
	}
}

// TaskConfig configuración específica de la tarea
type TaskConfig struct {
	Timeout         *int                   `json:"timeout,omitempty"` // segundos
	RetryPolicy     *RetryPolicy           `json:"retry_policy,omitempty"`
	Resources       *ResourceLimits        `json:"resources,omitempty"`
	Environment     map[string]string      `json:"environment,omitempty"`
	Dependencies    []string               `json:"dependencies,omitempty"`
	Notifications   *NotificationConfig    `json:"notifications,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	AllowParallel   bool                   `json:"allow_parallel"`
	PreserveResults bool                   `json:"preserve_results"`
	LogLevel        string                 `json:"log_level,omitempty"`
}

// RetryPolicy política de reintentos
type RetryPolicy struct {
	MaxAttempts     int           `json:"max_attempts"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffStrategy string        `json:"backoff_strategy"` // linear, exponential, fixed
	RetryableErrors []string      `json:"retryable_errors,omitempty"`
}

// ResourceLimits límites de recursos
type ResourceLimits struct {
	CPULimit    *float64 `json:"cpu_limit,omitempty"`    // cores
	MemoryLimit *int64   `json:"memory_limit,omitempty"` // bytes
	TimeLimit   *int     `json:"time_limit,omitempty"`   // seconds
}

// NotificationConfig configuración de notificaciones
type NotificationConfig struct {
	OnSuccess []NotificationTarget `json:"on_success,omitempty"`
	OnFailure []NotificationTarget `json:"on_failure,omitempty"`
	OnStart   []NotificationTarget `json:"on_start,omitempty"`
}

// NotificationTarget destino de notificación
type NotificationTarget struct {
	Type   string                 `json:"type"` // email, webhook, slack
	Target string                 `json:"target"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// ===============================================
// QUEUE STATISTICS & MONITORING
// ===============================================

// QueueStats estadísticas de la cola
type QueueStats struct {
	QueueName       string            `json:"queue_name"`
	TotalTasks      int64             `json:"total_tasks"`
	PendingTasks    int64             `json:"pending_tasks"`
	ProcessingTasks int64             `json:"processing_tasks"`
	CompletedTasks  int64             `json:"completed_tasks"`
	FailedTasks     int64             `json:"failed_tasks"`
	RetryingTasks   int64             `json:"retrying_tasks"`
	StatusCounts    map[string]int64  `json:"status_counts"`
	PriorityCounts  map[string]int64  `json:"priority_counts"`
	AverageWaitTime float64           `json:"average_wait_time"` // seconds
	Throughput      ThroughputMetrics `json:"throughput"`
	LastUpdated     time.Time         `json:"last_updated"`
}

// ThroughputMetrics métricas de rendimiento de la cola
type ThroughputMetrics struct {
	TasksPerSecond float64 `json:"tasks_per_second"`
	TasksPerMinute float64 `json:"tasks_per_minute"`
	TasksPerHour   float64 `json:"tasks_per_hour"`
	SuccessRate    float64 `json:"success_rate"` // percentage
	ErrorRate      float64 `json:"error_rate"`   // percentage
}

// QueueHealth salud de la cola
type QueueHealth struct {
	QueueName        string    `json:"queue_name"`
	IsHealthy        bool      `json:"is_healthy"`
	Status           string    `json:"status"` // healthy, warning, critical
	Issues           []string  `json:"issues,omitempty"`
	BacklogSize      int64     `json:"backlog_size"`
	OldestTaskAge    *int64    `json:"oldest_task_age,omitempty"` // seconds
	ProcessingRate   float64   `json:"processing_rate"`
	ErrorRate        float64   `json:"error_rate"`
	AvailableWorkers int       `json:"available_workers"`
	ActiveWorkers    int       `json:"active_workers"`
	LastHealthCheck  time.Time `json:"last_health_check"`
}

// ===============================================
// BULK OPERATIONS
// ===============================================

// BulkTaskRequest solicitud para operaciones en lote
type BulkTaskRequest struct {
	Tasks     []QueueTask            `json:"tasks"`
	QueueName string                 `json:"queue_name,omitempty"`
	Priority  *TaskPriority          `json:"priority,omitempty"`
	Config    *TaskConfig            `json:"config,omitempty"`
	BatchID   string                 `json:"batch_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BulkTaskResponse respuesta de operaciones en lote
type BulkTaskResponse struct {
	BatchID         string               `json:"batch_id"`
	TotalTasks      int                  `json:"total_tasks"`
	SuccessfulTasks int                  `json:"successful_tasks"`
	FailedTasks     int                  `json:"failed_tasks"`
	TaskResults     []BulkTaskResult     `json:"task_results"`
	ProcessingTime  int64                `json:"processing_time"` // milliseconds
	Errors          []BulkOperationError `json:"errors,omitempty"`
}

// BulkTaskResult resultado de una tarea en operación en lote
type BulkTaskResult struct {
	TaskID  string `json:"task_id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// BulkOperationError error en operación en lote
type BulkOperationError struct {
	TaskIndex int    `json:"task_index"`
	TaskID    string `json:"task_id,omitempty"`
	Error     string `json:"error"`
	Code      string `json:"code"`
}

// ===============================================
// FILTERING & SEARCH
// ===============================================

// TaskFilter filtros para búsqueda de tareas
type TaskFilter struct {
	WorkflowID      *string        `json:"workflow_id,omitempty"`
	UserID          *string        `json:"user_id,omitempty"`
	Status          []TaskStatus   `json:"status,omitempty"`
	Priority        []TaskPriority `json:"priority,omitempty"`
	Type            *string        `json:"type,omitempty"`
	CreatedAfter    *time.Time     `json:"created_after,omitempty"`
	CreatedBefore   *time.Time     `json:"created_before,omitempty"`
	CompletedAfter  *time.Time     `json:"completed_after,omitempty"`
	CompletedBefore *time.Time     `json:"completed_before,omitempty"`
	HasError        *bool          `json:"has_error,omitempty"`
	Queue           *string        `json:"queue,omitempty"`
	Limit           int            `json:"limit,omitempty"`
	Offset          int            `json:"offset,omitempty"`
	SortBy          string         `json:"sort_by,omitempty"`    // created_at, priority, status
	SortOrder       string         `json:"sort_order,omitempty"` // asc, desc
}

// TaskSearchResult resultado de búsqueda de tareas
type TaskSearchResult struct {
	Tasks      []QueueTask `json:"tasks"`
	TotalCount int64       `json:"total_count"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	HasMore    bool        `json:"has_more"`
}

// ===============================================
// UTILITY METHODS
// ===============================================

// CanRetry verifica si una tarea puede ser reintentada
func (qt *QueueTask) CanRetry() bool {
	maxRetries := qt.MaxRetries
	if maxRetries == 0 {
		maxRetries = qt.MaxAttempts
	}

	retryCount := qt.RetryCount
	if retryCount == 0 {
		retryCount = qt.Attempts
	}

	return qt.Status == TaskStatusFailed && retryCount < maxRetries
}

// GetDuration obtiene la duración de ejecución de la tarea
func (qt *QueueTask) GetDuration() *time.Duration {
	startTime := qt.StartedAt
	if startTime == nil {
		startTime = qt.ProcessedAt
	}

	if startTime != nil && qt.CompletedAt != nil {
		duration := qt.CompletedAt.Sub(*startTime)
		return &duration
	}
	return nil
}

// IsExpired verifica si una tarea ha expirado
func (qt *QueueTask) IsExpired(timeout time.Duration) bool {
	startTime := qt.StartedAt
	if startTime == nil {
		startTime = qt.ProcessedAt
	}

	if startTime == nil {
		return false
	}
	return time.Since(*startTime) > timeout
}

// GetWaitTime obtiene el tiempo de espera en cola
func (qt *QueueTask) GetWaitTime() time.Duration {
	startTime := qt.StartedAt
	if startTime == nil {
		startTime = qt.ProcessedAt
	}

	if startTime != nil {
		if qt.ScheduledAt != nil {
			return startTime.Sub(*qt.ScheduledAt)
		}
		return startTime.Sub(qt.CreatedAt)
	}

	if qt.ScheduledAt != nil {
		return time.Since(*qt.ScheduledAt)
	}
	return time.Since(qt.CreatedAt)
}

// UpdateStatus actualiza el estado de la tarea
func (qt *QueueTask) UpdateStatus(status TaskStatus) {
	qt.Status = status
	qt.UpdatedAt = time.Now()

	switch status {
	case TaskStatusProcessing:
		now := time.Now()
		if qt.StartedAt == nil {
			qt.StartedAt = &now
		}
		if qt.ProcessedAt == nil {
			qt.ProcessedAt = &now
		}
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled:
		if qt.CompletedAt == nil {
			now := time.Now()
			qt.CompletedAt = &now
		}
	}
}

// AddError añade un error a la tarea e incrementa el contador de reintentos
func (qt *QueueTask) AddError(err error) {
	qt.LastError = err.Error()
	qt.ErrorMessage = err.Error()
	qt.RetryCount++
	qt.Attempts++
	qt.UpdatedAt = time.Now()

	maxRetries := qt.MaxRetries
	if maxRetries == 0 {
		maxRetries = qt.MaxAttempts
	}

	if qt.RetryCount >= maxRetries {
		qt.Status = TaskStatusFailed
		now := time.Now()
		qt.CompletedAt = &now
	} else {
		qt.Status = TaskStatusRetrying
		qt.LastAttempt = &qt.UpdatedAt
	}
}

// SyncAliasFields sincroniza campos que son alias entre sí
func (qt *QueueTask) SyncAliasFields() {
	// Sincronizar contadores de reintentos
	if qt.Attempts == 0 && qt.RetryCount > 0 {
		qt.Attempts = qt.RetryCount
	} else if qt.RetryCount == 0 && qt.Attempts > 0 {
		qt.RetryCount = qt.Attempts
	}

	// Sincronizar límites máximos
	if qt.MaxAttempts == 0 && qt.MaxRetries > 0 {
		qt.MaxAttempts = qt.MaxRetries
	} else if qt.MaxRetries == 0 && qt.MaxAttempts > 0 {
		qt.MaxRetries = qt.MaxAttempts
	}

	// Sincronizar errores
	if qt.ErrorMessage == "" && qt.LastError != "" {
		qt.ErrorMessage = qt.LastError
	} else if qt.LastError == "" && qt.ErrorMessage != "" {
		qt.LastError = qt.ErrorMessage
	}

	// Sincronizar tipos
	if qt.Type == "" && qt.TaskType != "" {
		qt.Type = qt.TaskType
	} else if qt.TaskType == "" && qt.Type != "" {
		qt.TaskType = qt.Type
	}

	// Sincronizar resultados
	if qt.Results == nil && qt.Result != nil {
		qt.Results = qt.Result
	} else if qt.Result == nil && qt.Results != nil {
		qt.Result = qt.Results
	}
}
