package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WorkflowStep representa un paso individual en el workflow
type WorkflowStep struct {
	ID          string                 `json:"id" bson:"id" validate:"required"`
	Name        string                 `json:"name" bson:"name" validate:"required,min=3,max=100"`
	Type        string                 `json:"type" bson:"type" validate:"required,oneof=action condition integration webhook"`
	Config      map[string]interface{} `json:"config" bson:"config"`
	NextStepID  *string                `json:"next_step_id,omitempty" bson:"next_step_id,omitempty"`
	Conditions  []WorkflowCondition    `json:"conditions,omitempty" bson:"conditions,omitempty"`
	Position    Position               `json:"position" bson:"position"`
	Description string                 `json:"description,omitempty" bson:"description,omitempty"`
	IsEnabled   bool                   `json:"is_enabled" bson:"is_enabled" default:"true"`
}

// Position representa la posición visual del paso en un diagrama
type Position struct {
	X int `json:"x" bson:"x"`
	Y int `json:"y" bson:"y"`
}

// WorkflowCondition representa una condición para ejecutar un paso
type WorkflowCondition struct {
	Field    string      `json:"field" bson:"field" validate:"required"`
	Operator string      `json:"operator" bson:"operator" validate:"required,oneof=eq neq gt lt gte lte contains not_contains in not_in"`
	Value    interface{} `json:"value" bson:"value" validate:"required"`
	NextStep string      `json:"next_step" bson:"next_step"`
}

// WorkflowTrigger representa un disparador del workflow
type WorkflowTrigger struct {
	Type   string                 `json:"type" bson:"type" validate:"required,oneof=webhook manual scheduled api"`
	Config map[string]interface{} `json:"config" bson:"config"`
	URL    string                 `json:"url,omitempty" bson:"url,omitempty"`
	Secret string                 `json:"secret,omitempty" bson:"secret,omitempty"`
}

// WorkflowWebhook representa la configuración de webhooks
type WorkflowWebhook struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	WorkflowID  primitive.ObjectID `json:"workflow_id" bson:"workflow_id" validate:"required"`
	URL         string             `json:"url" bson:"url" validate:"required,url"`
	Secret      string             `json:"secret,omitempty" bson:"secret,omitempty"`
	Method      string             `json:"method" bson:"method" validate:"required,oneof=POST PUT PATCH"`
	Headers     map[string]string  `json:"headers,omitempty" bson:"headers,omitempty"`
	IsActive    bool               `json:"is_active" bson:"is_active" default:"true"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" bson:"updated_at"`
	LastUsedAt  *time.Time         `json:"last_used_at,omitempty" bson:"last_used_at,omitempty"`
	UsageCount  int64              `json:"usage_count" bson:"usage_count" default:"0"`
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
}

// WorkflowStats representa estadísticas de ejecución
type WorkflowStats struct {
	TotalExecutions    int64      `json:"total_executions" bson:"total_executions" default:"0"`
	SuccessfulRuns     int64      `json:"successful_runs" bson:"successful_runs" default:"0"`
	FailedRuns         int64      `json:"failed_runs" bson:"failed_runs" default:"0"`
	AvgExecutionTimeMs float64    `json:"avg_execution_time_ms" bson:"avg_execution_time_ms" default:"0"`
	LastExecutedAt     *time.Time `json:"last_executed_at,omitempty" bson:"last_executed_at,omitempty"`
	LastSuccess        *time.Time `json:"last_success,omitempty" bson:"last_success,omitempty"`
	LastFailure        *time.Time `json:"last_failure,omitempty" bson:"last_failure,omitempty"`
}

// Workflow representa la estructura principal de un flujo de trabajo
type Workflow struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name" validate:"required,min=3,max=100"`
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	UserID      primitive.ObjectID `json:"user_id" bson:"user_id" validate:"required"`
	Username    string             `json:"username,omitempty" bson:"username,omitempty"` // Para facilitar búsquedas

	// Configuración del workflow
	Status   WorkflowStatus    `json:"status" bson:"status" validate:"required,oneof=active inactive archived draft" default:"draft"`
	Steps    []WorkflowStep    `json:"steps" bson:"steps" validate:"required,min=1"`
	Triggers []WorkflowTrigger `json:"triggers" bson:"triggers" validate:"required,min=1"`
	Tags     []string          `json:"tags,omitempty" bson:"tags,omitempty"`
	Category string            `json:"category,omitempty" bson:"category,omitempty"`
	Priority int               `json:"priority" bson:"priority" validate:"min=1,max=5" default:"3"`

	// Versionado
	Version     int    `json:"version" bson:"version" default:"1"`
	VersionName string `json:"version_name,omitempty" bson:"version_name,omitempty"`

	// Webhooks
	Webhooks []WorkflowWebhook `json:"webhooks,omitempty" bson:"webhooks,omitempty"`

	// Configuración avanzada
	TimeoutMinutes int                    `json:"timeout_minutes" bson:"timeout_minutes" validate:"min=1,max=1440" default:"30"`
	RetryAttempts  int                    `json:"retry_attempts" bson:"retry_attempts" validate:"min=0,max=10" default:"3"`
	RetryDelayMs   int                    `json:"retry_delay_ms" bson:"retry_delay_ms" validate:"min=1000" default:"5000"`
	MaxConcurrent  int                    `json:"max_concurrent" bson:"max_concurrent" validate:"min=1,max=100" default:"10"`
	Environment    string                 `json:"environment,omitempty" bson:"environment,omitempty" validate:"oneof=development staging production"`
	Configuration  map[string]interface{} `json:"configuration,omitempty" bson:"configuration,omitempty"`

	// Estadísticas y monitoreo
	Stats WorkflowStats `json:"stats" bson:"stats"`

	// Metadatos temporales
	CreatedAt time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" bson:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`

	// Auditoría
	CreatedBy    primitive.ObjectID `json:"created_by" bson:"created_by"`
	LastEditedBy primitive.ObjectID `json:"last_edited_by,omitempty" bson:"last_edited_by,omitempty"`
}

// CreateWorkflowRequest representa la estructura para crear un workflow
type CreateWorkflowRequest struct {
	Name           string                 `json:"name" validate:"required,min=3,max=100"`
	Description    string                 `json:"description,omitempty"`
	Steps          []WorkflowStep         `json:"steps" validate:"required,min=1"`
	Triggers       []WorkflowTrigger      `json:"triggers" validate:"required,min=1"`
	Tags           []string               `json:"tags,omitempty"`
	Category       string                 `json:"category,omitempty"`
	Priority       int                    `json:"priority" validate:"min=1,max=5"`
	TimeoutMinutes int                    `json:"timeout_minutes" validate:"min=1,max=1440"`
	RetryAttempts  int                    `json:"retry_attempts" validate:"min=0,max=10"`
	RetryDelayMs   int                    `json:"retry_delay_ms" validate:"min=1000"`
	MaxConcurrent  int                    `json:"max_concurrent" validate:"min=1,max=100"`
	Environment    string                 `json:"environment,omitempty" validate:"oneof=development staging production"`
	Configuration  map[string]interface{} `json:"configuration,omitempty"`
}

// UpdateWorkflowRequest representa la estructura para actualizar un workflow
type UpdateWorkflowRequest struct {
	Name           *string                `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	Description    *string                `json:"description,omitempty"`
	Status         *WorkflowStatus        `json:"status,omitempty" validate:"omitempty,oneof=active inactive archived draft"`
	Steps          []WorkflowStep         `json:"steps,omitempty" validate:"omitempty,min=1"`
	Triggers       []WorkflowTrigger      `json:"triggers,omitempty" validate:"omitempty,min=1"`
	Tags           []string               `json:"tags,omitempty"`
	Category       *string                `json:"category,omitempty"`
	Priority       *int                   `json:"priority,omitempty" validate:"omitempty,min=1,max=5"`
	TimeoutMinutes *int                   `json:"timeout_minutes,omitempty" validate:"omitempty,min=1,max=1440"`
	RetryAttempts  *int                   `json:"retry_attempts,omitempty" validate:"omitempty,min=0,max=10"`
	RetryDelayMs   *int                   `json:"retry_delay_ms,omitempty" validate:"omitempty,min=1000"`
	MaxConcurrent  *int                   `json:"max_concurrent,omitempty" validate:"omitempty,min=1,max=100"`
	Environment    *string                `json:"environment,omitempty" validate:"omitempty,oneof=development staging production"`
	Configuration  map[string]interface{} `json:"configuration,omitempty"`
}

// WorkflowResponse representa la respuesta de un workflow
type WorkflowResponse struct {
	ID             primitive.ObjectID     `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	UserID         primitive.ObjectID     `json:"user_id"`
	Username       string                 `json:"username,omitempty"`
	Status         WorkflowStatus         `json:"status"`
	Steps          []ExecutedWorkflowStep `json:"steps"`
	Triggers       []WorkflowTrigger      `json:"triggers"`
	Tags           []string               `json:"tags,omitempty"`
	Category       string                 `json:"category,omitempty"`
	Priority       int                    `json:"priority"`
	Version        int                    `json:"version"`
	VersionName    string                 `json:"version_name,omitempty"`
	TimeoutMinutes int                    `json:"timeout_minutes"`
	RetryAttempts  int                    `json:"retry_attempts"`
	RetryDelayMs   int                    `json:"retry_delay_ms"`
	MaxConcurrent  int                    `json:"max_concurrent"`
	Environment    string                 `json:"environment,omitempty"`
	Stats          WorkflowStats          `json:"stats"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// IsActive verifica si el workflow está activo
func (w *Workflow) IsActive() bool {
	return w.Status == WorkflowStatusActive && w.DeletedAt == nil
}

// CanBeExecuted verifica si el workflow puede ser ejecutado
func (w *Workflow) CanBeExecuted() bool {
	return w.IsActive() && len(w.Steps) > 0 && len(w.Triggers) > 0
}

// GetStepByID obtiene un paso por su ID
func (w *Workflow) GetStepByID(stepID string) *WorkflowStep {
	for i, step := range w.Steps {
		if step.ID == stepID {
			return &w.Steps[i]
		}
	}
	return nil
}

// GetFirstStep obtiene el primer paso del workflow
func (w *Workflow) GetFirstStep() *WorkflowStep {
	if len(w.Steps) == 0 {
		return nil
	}
	return &w.Steps[0]
}

// HasTag verifica si el workflow tiene un tag específico
func (w *Workflow) HasTag(tag string) bool {
	for _, t := range w.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// AddTag agrega un tag al workflow (si no existe)
func (w *Workflow) AddTag(tag string) {
	if !w.HasTag(tag) {
		w.Tags = append(w.Tags, tag)
	}
}

// RemoveTag elimina un tag del workflow
func (w *Workflow) RemoveTag(tag string) {
	for i, t := range w.Tags {
		if t == tag {
			w.Tags = append(w.Tags[:i], w.Tags[i+1:]...)
			break
		}
	}
}

// IncrementVersion incrementa la versión del workflow
func (w *Workflow) IncrementVersion() {
	w.Version++
	w.UpdatedAt = time.Now()
}

// SetDefaults establece valores por defecto
func (w *Workflow) SetDefaults() {
	if w.Status == "" {
		w.Status = WorkflowStatusDraft
	}
	if w.Priority == 0 {
		w.Priority = 3
	}
	if w.TimeoutMinutes == 0 {
		w.TimeoutMinutes = 30
	}
	if w.RetryAttempts == 0 {
		w.RetryAttempts = 3
	}
	if w.RetryDelayMs == 0 {
		w.RetryDelayMs = 5000
	}
	if w.MaxConcurrent == 0 {
		w.MaxConcurrent = 10
	}
	if w.Version == 0 {
		w.Version = 1
	}

	now := time.Now()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	w.UpdatedAt = now
}
