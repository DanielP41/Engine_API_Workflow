package models

// WebhookTrigger para configuración de webhooks
type WebhookTrigger struct {
	ID          string            `json:"id" bson:"id"`
	URL         string            `json:"url" bson:"url"`
	Secret      string            `json:"secret,omitempty" bson:"secret,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" bson:"headers,omitempty"`
	Method      string            `json:"method" bson:"method"`
	IsActive    bool              `json:"is_active" bson:"is_active"`
	Description string            `json:"description,omitempty" bson:"description,omitempty"`
}

// Log alias para mantener compatibilidad
type Log = WorkflowLog

// IsActive método para verificar si el workflow está activo
func (w *Workflow) IsActive() bool {
	if w.Active != nil {
		return *w.Active && w.Status == WorkflowStatusActive
	}
	return w.Status == WorkflowStatusActive
}
