package models

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LogLevel represents the level/severity of a log entry
type LogLevel string

const (
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
	LogLevelDebug   LogLevel = "debug"
)

// ExecutionStatus represents the status of workflow execution
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// WorkflowLog represents a log entry for workflow executions
type WorkflowLog struct {
	ID               primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	WorkflowID       primitive.ObjectID `json:"workflow_id" bson:"workflow_id"`
	WorkflowName     string             `json:"workflow_name" bson:"workflow_name"`
	UserID           primitive.ObjectID `json:"user_id" bson:"user_id"`
	ExecutionID      string             `json:"execution_id" bson:"execution_id"` // Unique execution identifier
	Status           ExecutionStatus    `json:"status" bson:"status"`
	Level            LogLevel           `json:"level" bson:"level"`
	
	// Execution details
	TriggerType      TriggerType            `json:"trigger_type" bson:"trigger_type"`
	TriggerData      map[string]interface{} `json:"trigger_data,omitempty" bson:"trigger_data,omitempty"`
	
	// Timing
	StartedAt        time.Time  `json:"started_at" bson:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	Duration         int64      `json:"duration" bson:"duration"` // Duration in milliseconds
	
	// Results and errors
	Message          string                 `json:"message" bson:"message"`
	Error            string                 `json:"error,omitempty" bson:"error,omitempty"`
	ActionsExecuted  []ActionExecution      `json:"actions_executed" bson:"actions_executed"`
	
	// Metadata
	Context          map[string