// Agregar al final del archivo: internal/models/workflow.go
// O crear un nuevo archivo: internal/models/queue.go

package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
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
	CreatedAt   time.Time              `json:"created_at" bson:"created_at"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty" bson:"processed_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	FailedAt    *time.Time             `json:"failed_at,omitempty" bson:"failed_at,omitempty"`
	Error       string                 `json:"error,omitempty" bson:"error,omitempty"`
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
