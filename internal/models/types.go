package models

// WorkflowStatus representa el estado de un workflow
type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusInactive WorkflowStatus = "inactive"
	WorkflowStatusArchived WorkflowStatus = "archived"
)

// TriggerType representa el tipo de trigger de un workflow
type TriggerType string

const (
	TriggerTypeWebhook   TriggerType = "webhook"
	TriggerTypeManual    TriggerType = "manual"
	TriggerTypeScheduled TriggerType = "scheduled"
	TriggerTypeAPI       TriggerType = "api"
	TriggerTypeEvent     TriggerType = "event"
)

// ActionType representa el tipo de acción en un paso del workflow
type ActionType string

const (
	ActionTypeHTTP         ActionType = "http"
	ActionTypeEmail        ActionType = "email"
	ActionTypeSlack        ActionType = "slack"
	ActionTypeWebhook      ActionType = "webhook"
	ActionTypeDatabase     ActionType = "database"
	ActionTypeCondition    ActionType = "condition"
	ActionTypeDelay        ActionType = "delay"
	ActionTypeTransform    ActionType = "transform"
	ActionTypeIntegration  ActionType = "integration"
	ActionTypeNotification ActionType = "notification"
)

// StepStatus representa el estado específico de un paso del workflow
type StepStatus string

const (
	StepStatusIdle        StepStatus = "idle"
	StepStatusWaiting     StepStatus = "waiting"
	StepStatusExecuting   StepStatus = "executing"
	StepStatusCompleted   StepStatus = "completed"
	StepStatusFailed      StepStatus = "failed"
	StepStatusSkipped     StepStatus = "skipped"
	StepStatusConditional StepStatus = "conditional"
)

// Priority representa la prioridad de un workflow
type Priority int

const (
	PriorityLowest  Priority = 1
	PriorityLow     Priority = 2
	PriorityMedium  Priority = 3
	PriorityHigh    Priority = 4
	PriorityHighest Priority = 5
)

// Environment representa el entorno de ejecución
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentStaging     Environment = "staging"
	EnvironmentProduction  Environment = "production"
	EnvironmentTesting     Environment = "testing"
)

// LogLevel representa el nivel de logging
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// ValidWorkflowStatuses retorna todos los estados válidos de workflow
func ValidWorkflowStatuses() []WorkflowStatus {
	return []WorkflowStatus{
		WorkflowStatusDraft,
		WorkflowStatusActive,
		WorkflowStatusInactive,
		WorkflowStatusArchived,
	}
}

// ValidTriggerTypes retorna todos los tipos válidos de trigger
func ValidTriggerTypes() []TriggerType {
	return []TriggerType{
		TriggerTypeWebhook,
		TriggerTypeManual,
		TriggerTypeScheduled,
		TriggerTypeAPI,
		TriggerTypeEvent,
	}
}

// ValidActionTypes retorna todos los tipos válidos de acción
func ValidActionTypes() []ActionType {
	return []ActionType{
		ActionTypeHTTP,
		ActionTypeEmail,
		ActionTypeSlack,
		ActionTypeWebhook,
		ActionTypeDatabase,
		ActionTypeCondition,
		ActionTypeDelay,
		ActionTypeTransform,
		ActionTypeIntegration,
		ActionTypeNotification,
	}
}

// IsValid valida si el WorkflowStatus es válido
func (ws WorkflowStatus) IsValid() bool {
	for _, status := range ValidWorkflowStatuses() {
		if ws == status {
			return true
		}
	}
	return false
}

// IsActive verifica si el workflow está activo
func (ws WorkflowStatus) IsActive() bool {
	return ws == WorkflowStatusActive
}

// IsValid valida si el TriggerType es válido
func (tt TriggerType) IsValid() bool {
	for _, triggerType := range ValidTriggerTypes() {
		if tt == triggerType {
			return true
		}
	}
	return false
}

// IsValid valida si el ActionType es válido
func (at ActionType) IsValid() bool {
	for _, actionType := range ValidActionTypes() {
		if at == actionType {
			return true
		}
	}
	return false
}

// String retorna la representación string del Priority
func (p Priority) String() string {
	switch p {
	case PriorityLowest:
		return "lowest"
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityHighest:
		return "highest"
	default:
		return "medium"
	}
}

// IsValid valida si la prioridad está en el rango correcto
func (p Priority) IsValid() bool {
	return p >= PriorityLowest && p <= PriorityHighest
}
