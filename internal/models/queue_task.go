// internal/models/queue_task.go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// QueueTaskStatus representa los estados de una tarea en cola
type QueueTaskStatus string

const (
	QueueTaskStatusPending    QueueTaskStatus = "pending"
	QueueTaskStatusProcessing QueueTaskStatus = "processing"
	QueueTaskStatusCompleted  QueueTaskStatus = "completed"
	QueueTaskStatusFailed     QueueTaskStatus = "failed"
	QueueTaskStatusRetrying   QueueTaskStatus = "retrying"
	QueueTaskStatusCancelled  QueueTaskStatus = "cancelled"
)

// QueueTask represents a task in the processing queue
type QueueTask struct {
	ID          string                 `json:"id" bson:"_id"`
	WorkflowID  primitive.ObjectID     `json:"workflow_id" bson:"workflow_id"`
	ExecutionID string                 `json:"execution_id" bson:"execution_id"`
	UserID      primitive.ObjectID     `json:"user_id" bson:"user_id"`
	Payload     map[string]interface{} `json:"payload" bson:"payload"`
	Priority    int                    `json:"priority" bson:"priority"`
	RetryCount  int                    `json:"retry_count" bson:"retry_count"`
	MaxRetries  int                    `json:"max_retries" bson:"max_retries"`

	// 🆕 CAMPOS AGREGADOS PARA ARREGLAR ERRORES DE COMPILACIÓN
	Status      QueueTaskStatus `json:"status" bson:"status"`
	LastError   string          `json:"last_error,omitempty" bson:"last_error,omitempty"`
	ScheduledAt *time.Time      `json:"scheduled_at,omitempty" bson:"scheduled_at,omitempty"`

	// Campos de timestamp existentes
	CreatedAt   time.Time  `json:"created_at" bson:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty" bson:"processed_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	FailedAt    *time.Time `json:"failed_at,omitempty" bson:"failed_at,omitempty"`
	Error       string     `json:"error,omitempty" bson:"error,omitempty"`

	// 🆕 CAMPOS ADICIONALES PARA CONTEXTO
	Context  map[string]interface{} `json:"context,omitempty" bson:"context,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

// 🆕 MÉTODOS ÚTILES PARA GESTIÓN DE TAREAS

// IsRetryable verifica si la tarea puede ser reintentada
func (t *QueueTask) IsRetryable() bool {
	return t.RetryCount < t.MaxRetries &&
		t.Status != QueueTaskStatusCompleted &&
		t.Status != QueueTaskStatusCancelled
}

// CanBeProcessed verifica si la tarea está lista para procesamiento
func (t *QueueTask) CanBeProcessed() bool {
	return t.Status == QueueTaskStatusPending || t.Status == QueueTaskStatusRetrying
}

// MarkAsProcessing marca la tarea como en procesamiento
func (t *QueueTask) MarkAsProcessing() {
	t.Status = QueueTaskStatusProcessing
	now := time.Now()
	t.ProcessedAt = &now
}

// MarkAsCompleted marca la tarea como completada
func (t *QueueTask) MarkAsCompleted() {
	t.Status = QueueTaskStatusCompleted
	now := time.Now()
	t.CompletedAt = &now
}

// MarkAsFailed marca la tarea como fallida
func (t *QueueTask) MarkAsFailed(errorMsg string) {
	t.Status = QueueTaskStatusFailed
	t.LastError = errorMsg
	t.Error = errorMsg
	now := time.Now()
	t.FailedAt = &now
}

// IncrementRetry incrementa el contador de reintentos
func (t *QueueTask) IncrementRetry() {
	t.RetryCount++
	t.Status = QueueTaskStatusRetrying
}

// GetAge obtiene la edad de la tarea desde su creación
func (t *QueueTask) GetAge() time.Duration {
	return time.Since(t.CreatedAt)
}

// GetProcessingDuration obtiene el tiempo de procesamiento
func (t *QueueTask) GetProcessingDuration() time.Duration {
	if t.ProcessedAt == nil {
		return 0
	}

	endTime := time.Now()
	if t.CompletedAt != nil {
		endTime = *t.CompletedAt
	} else if t.FailedAt != nil {
		endTime = *t.FailedAt
	}

	return endTime.Sub(*t.ProcessedAt)
}

// QueueStats represents queue statistics
type QueueStats struct {
	Queued       int64 `json:"queued"`
	Processing   int64 `json:"processing"`
	Completed    int64 `json:"completed"`
	Failed       int64 `json:"failed"`
	Delayed      int64 `json:"delayed"`
	Retries      int64 `json:"retries"`
	Total        int64 `json:"total"`
	CurrentQueue int64 `json:"current_queue_length"`
	CurrentDelay int64 `json:"current_delayed_length"`
	CurrentProc  int64 `json:"current_processing_length"`
}
