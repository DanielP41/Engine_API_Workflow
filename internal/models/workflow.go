package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WorkflowStatus represents the status of a workflow
type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusInactive WorkflowStatus = "inactive"
	WorkflowStatusArchived WorkflowStatus = "archived"
)

// TriggerType represents the type of trigger
type TriggerType string

const (
	TriggerTypeWebhook   TriggerType = "webhook"
	TriggerTypeScheduled TriggerType = "scheduled"
	TriggerTypeManual    TriggerType = "manual"
	TriggerTypeEvent     TriggerType = "event"
)

// ActionType represents the type of action
type ActionType string

const (
	ActionTypeSlackNotification ActionType = "slack_notification"
	ActionTypeEmail             ActionType = "email"
	ActionTypeWebhook           ActionType = "webhook"
	ActionTypeLog               ActionType = "log"
	ActionTypeDelay             ActionType = "delay"
	ActionTypeCondition         ActionType = "condition"
)

// Workflow represents a workflow configuration
type Workflow struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name"`
	Description string             `json:"description" bson:"description"`
	UserID      primitive.ObjectID `json:"user_id" bson:"user_id"`
	Status      WorkflowStatus     `json:"status" bson:"status"`
	Version     int                `json:"version" bson:"version"`

	// Trigger configuration
	Trigger Trigger `json:"trigger" bson:"trigger"`

	// Actions/Steps to execute
	Actions []Action `json:"actions" bson:"actions"`

	// Metadata
	Tags     []string               `json:"tags" bson:"tags"`
	Settings map[string]interface{} `json:"settings" bson:"settings"`

	// Timestamps
	CreatedAt time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" bson:"updated_at"`
	LastRunAt *time.Time `json:"last_run_at,omitempty" bson:"last_run_at,omitempty"`

	// Statistics
	RunCount     int64 `json:"run_count" bson:"run_count"`
	SuccessCount int64 `json:"success_count" bson:"success_count"`
	FailureCount int64 `json:"failure_count" bson:"failure_count"`
}

// Trigger represents a workflow trigger
type Trigger struct {
	Type    TriggerType            `json:"type" bson:"type"`
	Config  map[string]interface{} `json:"config" bson:"config"`
	Enabled bool                   `json:"enabled" bson:"enabled"`
}

// Action represents a workflow action/step
type Action struct {
	ID      string                 `json:"id" bson:"id"`
	Name    string                 `json:"name" bson:"name"`
	Type    ActionType             `json:"type" bson:"type"`
	Config  map[string]interface{} `json:"config" bson:"config"`
	Enabled bool                   `json:"enabled" bson:"enabled"`
	Order   int                    `json:"order" bson:"order"`

	// Conditional execution
	Conditions []Condition `json:"conditions,omitempty" bson:"conditions,omitempty"`

	// Error handling
	ContinueOnError bool `json:"continue_on_error" bson:"continue_on_error"`
	RetryCount      int  `json:"retry_count" bson:"retry_count"`
	RetryDelay      int  `json:"retry_delay" bson:"retry_delay"` // in seconds
}

// Condition represents a condition for action execution
type Condition struct {
	Field    string      `json:"field" bson:"field"`
	Operator string      `json:"operator" bson:"operator"` // eq, ne, gt, lt, contains, etc.
	Value    interface{} `json:"value" bson:"value"`
}

// CreateWorkflowRequest represents the request to create a workflow
type CreateWorkflowRequest struct {
	Name        string                 `json:"name" validate:"required,min=3,max=100"`
	Description string                 `json:"description" validate:"max=500"`
	Trigger     Trigger                `json:"trigger" validate:"required"`
	Actions     []Action               `json:"actions" validate:"required,min=1"`
	Tags        []string               `json:"tags,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
}

// UpdateWorkflowRequest represents the request to update a workflow
type UpdateWorkflowRequest struct {
	Name        string                 `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	Description string                 `json:"description,omitempty" validate:"omitempty,max=500"`
	Status      WorkflowStatus         `json:"status,omitempty"`
	Trigger     *Trigger               `json:"trigger,omitempty"`
	Actions     []Action               `json:"actions,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
}

// WorkflowListResponse represents the response for listing workflows
type WorkflowListResponse struct {
	Workflows  []Workflow `json:"workflows"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// ExecuteWorkflowRequest represents a manual workflow execution request
type ExecuteWorkflowRequest struct {
	Data    map[string]interface{} `json:"data,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// BeforeCreate sets default values and timestamps before creating
func (w *Workflow) BeforeCreate() {
	now := time.Now()
	w.CreatedAt = now
	w.UpdatedAt = now
	w.Version = 1
	w.Status = WorkflowStatusDraft
	w.RunCount = 0
	w.SuccessCount = 0
	w.FailureCount = 0

	if w.Settings == nil {
		w.Settings = make(map[string]interface{})
	}

	// Set default order for actions if not provided
	for i := range w.Actions {
		if w.Actions[i].Order == 0 {
			w.Actions[i].Order = i + 1
		}
		if w.Actions[i].ID == "" {
			w.Actions[i].ID = primitive.NewObjectID().Hex()
		}
	}
}

// BeforeUpdate sets updated timestamp before updating
func (w *Workflow) BeforeUpdate() {
	w.UpdatedAt = time.Now()
	w.Version++
}

// IsActive returns true if workflow is active
func (w *Workflow) IsActive() bool {
	return w.Status == WorkflowStatusActive
}

// CanExecute returns true if workflow can be executed
func (w *Workflow) CanExecute() bool {
	return w.IsActive() && w.Trigger.Enabled && len(w.Actions) > 0
}

// GetEnabledActions returns only enabled actions sorted by order
func (w *Workflow) GetEnabledActions() []Action {
	var enabledActions []Action
	for _, action := range w.Actions {
		if action.Enabled {
			enabledActions = append(enabledActions, action)
		}
	}

	// Sort by order (bubble sort for simplicity)
	for i := 0; i < len(enabledActions); i++ {
		for j := i + 1; j < len(enabledActions); j++ {
			if enabledActions[i].Order > enabledActions[j].Order {
				enabledActions[i], enabledActions[j] = enabledActions[j], enabledActions[i]
			}
		}
	}

	return enabledActions
}

// IncrementRunCount increments the run count
func (w *Workflow) IncrementRunCount() {
	w.RunCount++
	now := time.Now()
	w.LastRunAt = &now
}

// IncrementSuccessCount increments the success count
func (w *Workflow) IncrementSuccessCount() {
	w.SuccessCount++
}

// IncrementFailureCount increments the failure count
func (w *Workflow) IncrementFailureCount() {
	w.FailureCount++
}

// GetSuccessRate returns the success rate as a percentage
func (w *Workflow) GetSuccessRate() float64 {
	if w.RunCount == 0 {
		return 0
	}
	return float64(w.SuccessCount) / float64(w.RunCount) * 100
}

// Validate validates workflow data
func (req *CreateWorkflowRequest) Validate() error {
	// Add custom validation logic here if needed
	if len(req.Actions) == 0 {
		return nil // Will be caught by struct validation
	}

	// Validate trigger type
	switch req.Trigger.Type {
	case TriggerTypeWebhook, TriggerTypeScheduled, TriggerTypeManual, TriggerTypeEvent:
		// Valid types
	default:
		return nil // Invalid trigger type
	}

	// Validate action types
	for _, action := range req.Actions {
		switch action.Type {
		case ActionTypeSlackNotification, ActionTypeEmail, ActionTypeWebhook, ActionTypeLog, ActionTypeDelay, ActionTypeCondition:
			// Valid types
		default:
			return nil // Invalid action type
		}
	}

	return nil
}
