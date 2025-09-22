package models

import (
	"time"
)

// API RESPONSE STRUCTURES

// APIResponse estructura genérica para todas las respuestas de la API
type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Meta      *MetaInfo   `json:"meta,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// APIError estructura detallada de errores
type APIError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Field      string                 `json:"field,omitempty"`
	Validation []ValidationError      `json:"validation,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// ValidationError error específico de validación
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
	Code    string      `json:"code"`
}

// MetaInfo información adicional en respuestas
type MetaInfo struct {
	RequestID     string      `json:"request_id,omitempty"`
	Version       string      `json:"version,omitempty"`
	ExecutionTime int64       `json:"execution_time,omitempty"` // ms
	Pagination    *Pagination `json:"pagination,omitempty"`
	RateLimit     *RateLimit  `json:"rate_limit,omitempty"`
}

// Pagination información de paginación
type Pagination struct {
	Page         int   `json:"page"`
	PageSize     int   `json:"page_size"`
	Total        int64 `json:"total"`
	TotalPages   int   `json:"total_pages"`
	HasNext      bool  `json:"has_next"`
	HasPrevious  bool  `json:"has_previous"`
	NextPage     *int  `json:"next_page,omitempty"`
	PreviousPage *int  `json:"previous_page,omitempty"`
}

// RateLimit información de límites de rate
type RateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
	Window    string    `json:"window"`
}

// USER RESPONSE STRUCTURES (EXPANDIDAS)

// UserResponse respuesta completa de usuario
type UserResponse struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Email           string                 `json:"email"`
	Role            string                 `json:"role"`
	IsActive        bool                   `json:"is_active"`
	LastLoginAt     *time.Time             `json:"last_login_at,omitempty"`
	ProfilePicture  string                 `json:"profile_picture,omitempty"`
	Preferences     UserPreferences        `json:"preferences"`
	Stats           UserStats              `json:"stats"`
	Permissions     []string               `json:"permissions"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	EmailVerifiedAt *time.Time             `json:"email_verified_at,omitempty"`
}

// UserPreferences preferencias del usuario
type UserPreferences struct {
	Theme         string                   `json:"theme" validate:"oneof=light dark auto"`
	Language      string                   `json:"language" validate:"required"`
	Timezone      string                   `json:"timezone" validate:"required"`
	Notifications UserNotificationSettings `json:"notifications"`
	Dashboard     UserDashboardSettings    `json:"dashboard"`
}

// UserNotificationSettings configuración de notificaciones
type UserNotificationSettings struct {
	Email         bool `json:"email"`
	InApp         bool `json:"in_app"`
	WorkflowStart bool `json:"workflow_start"`
	WorkflowEnd   bool `json:"workflow_end"`
	WorkflowError bool `json:"workflow_error"`
	SystemAlerts  bool `json:"system_alerts"`
}

// UserDashboardSettings configuración del dashboard
type UserDashboardSettings struct {
	DefaultView     string   `json:"default_view" validate:"oneof=overview workflows logs metrics"`
	RefreshInterval int      `json:"refresh_interval" validate:"min=5,max=300"` // seconds
	WidgetsLayout   []Widget `json:"widgets_layout"`
	CompactMode     bool     `json:"compact_mode"`
}

// Widget configuración de widget del dashboard
type Widget struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Position WidgetPosition         `json:"position"`
	Size     WidgetSize             `json:"size"`
	Config   map[string]interface{} `json:"config"`
	Visible  bool                   `json:"visible"`
}

// WidgetPosition posición del widget
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// WidgetSize tamaño del widget
type WidgetSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// UserStats estadísticas del usuario
type UserStats struct {
	TotalWorkflows       int        `json:"total_workflows"`
	ActiveWorkflows      int        `json:"active_workflows"`
	TotalExecutions      int64      `json:"total_executions"`
	SuccessfulExecutions int64      `json:"successful_executions"`
	FailedExecutions     int64      `json:"failed_executions"`
	SuccessRate          float64    `json:"success_rate"`
	AvgExecutionTime     float64    `json:"avg_execution_time"` // ms
	LastActivityAt       *time.Time `json:"last_activity_at,omitempty"`
}

// ===============================================
// WORKFLOW RESPONSE STRUCTURES (EXPANDIDAS)
// ===============================================

// WorkflowFilters filtros aplicados
type WorkflowFilters struct {
	Status      []WorkflowStatus `json:"status,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Category    string           `json:"category,omitempty"`
	Environment string           `json:"environment,omitempty"`
	CreatedBy   string           `json:"created_by,omitempty"`
	DateRange   *DateRange       `json:"date_range,omitempty"`
}

// WorkflowAggregations agregaciones de workflows
type WorkflowAggregations struct {
	ByStatus      map[WorkflowStatus]int64 `json:"by_status"`
	ByCategory    map[string]int64         `json:"by_category"`
	ByEnvironment map[string]int64         `json:"by_environment"`
	ByTriggerType map[TriggerType]int64    `json:"by_trigger_type"`
	TagCloud      []TagCount               `json:"tag_cloud"`
}

// TagCount conteo de tags
type TagCount struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

// DateRange rango de fechas
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// EXECUTION & LOG RESPONSE STRUCTURES

// ExecutionResponse respuesta de ejecución detallada
type ExecutionResponse struct {
	ID            string                      `json:"id"`
	WorkflowID    string                      `json:"workflow_id"`
	WorkflowName  string                      `json:"workflow_name"`
	Status        WorkflowStatus              `json:"status"`
	TriggerType   TriggerType                 `json:"trigger_type"`
	TriggerData   map[string]interface{}      `json:"trigger_data"`
	StartedAt     time.Time                   `json:"started_at"`
	CompletedAt   *time.Time                  `json:"completed_at,omitempty"`
	Duration      *int64                      `json:"duration,omitempty"` // ms
	Steps         []StepExecutionResponse     `json:"steps"`
	Context       map[string]interface{}      `json:"context"`
	ErrorMessage  string                      `json:"error_message,omitempty"`
	Progress      ExecutionProgress           `json:"progress"`
	ResourceUsage ResourceUsage               `json:"resource_usage"`
	Performance   ExecutionPerformanceMetrics `json:"performance"`
}

// StepExecutionResponse respuesta de ejecución de paso
type StepExecutionResponse struct {
	StepID        string                 `json:"step_id"`
	StepName      string                 `json:"step_name"`
	ActionType    ActionType             `json:"action_type"`
	Status        WorkflowStatus         `json:"status"`
	StartedAt     time.Time              `json:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	Duration      *int64                 `json:"duration,omitempty"`
	Input         map[string]interface{} `json:"input"`
	Output        map[string]interface{} `json:"output,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	RetryCount    int                    `json:"retry_count"`
	ExecutionTime int64                  `json:"execution_time"`
	ResourceUsage StepResourceUsage      `json:"resource_usage"`
}

// ExecutionProgress progreso de ejecución
type ExecutionProgress struct {
	TotalSteps     int     `json:"total_steps"`
	CompletedSteps int     `json:"completed_steps"`
	CurrentStep    *string `json:"current_step,omitempty"`
	Percentage     float64 `json:"percentage"`
	EstimatedTime  *int64  `json:"estimated_time,omitempty"` // ms remaining
}

// StepResourceUsage uso de recursos por paso
type StepResourceUsage struct {
	CPUTime     int64 `json:"cpu_time"`     // ms
	MemoryPeak  int64 `json:"memory_peak"`  // bytes
	NetworkSent int64 `json:"network_sent"` // bytes
	NetworkRecv int64 `json:"network_recv"` // bytes
}

// ExecutionPerformanceMetrics métricas de rendimiento de ejecución
type ExecutionPerformanceMetrics struct {
	QueueTime     int64          `json:"queue_time"`     // ms
	ExecutionTime int64          `json:"execution_time"` // ms
	TotalTime     int64          `json:"total_time"`     // ms
	Throughput    float64        `json:"throughput"`     // operations per second
	Latency       LatencyMetrics `json:"latency"`
}

// LatencyMetrics métricas de latencia
type LatencyMetrics struct {
	P50 int64 `json:"p50"` // ms
	P90 int64 `json:"p90"` // ms
	P95 int64 `json:"p95"` // ms
	P99 int64 `json:"p99"` // ms
}

// SYSTEM & HEALTH RESPONSE STRUCTURES

// HealthResponse respuesta de salud del sistema
type HealthResponse struct {
	Status       string                   `json:"status"` // healthy, degraded, unhealthy
	Version      string                   `json:"version"`
	Timestamp    time.Time                `json:"timestamp"`
	Uptime       int64                    `json:"uptime"` // seconds
	Services     map[string]ServiceHealth `json:"services"`
	Metrics      SystemMetrics            `json:"metrics"`
	Dependencies []DependencyHealth       `json:"dependencies"`
}

// ServiceHealth salud de un servicio
type ServiceHealth struct {
	Status       string    `json:"status"`
	LastCheck    time.Time `json:"last_check"`
	ResponseTime int64     `json:"response_time"` // ms
	ErrorRate    float64   `json:"error_rate"`
	Message      string    `json:"message,omitempty"`
}

// SystemMetrics métricas del sistema
type SystemMetrics struct {
	CPU        CPUMetrics     `json:"cpu"`
	Memory     MemoryMetrics  `json:"memory"`
	Disk       DiskMetrics    `json:"disk"`
	Network    NetworkMetrics `json:"network"`
	Goroutines int            `json:"goroutines"`
}

// CPUMetrics métricas de CPU
type CPUMetrics struct {
	Usage     float64 `json:"usage"` // percentage
	LoadAvg1  float64 `json:"load_avg_1"`
	LoadAvg5  float64 `json:"load_avg_5"`
	LoadAvg15 float64 `json:"load_avg_15"`
}

// MemoryMetrics métricas de memoria
type MemoryMetrics struct {
	Used      int64   `json:"used"`      // bytes
	Total     int64   `json:"total"`     // bytes
	Available int64   `json:"available"` // bytes
	Usage     float64 `json:"usage"`     // percentage
}

// DiskMetrics métricas de disco
type DiskMetrics struct {
	Used      int64   `json:"used"`      // bytes
	Total     int64   `json:"total"`     // bytes
	Available int64   `json:"available"` // bytes
	Usage     float64 `json:"usage"`     // percentage
	IOPS      int64   `json:"iops"`
}

// NetworkMetrics métricas de red
type NetworkMetrics struct {
	BytesSent       int64 `json:"bytes_sent"`
	BytesReceived   int64 `json:"bytes_received"`
	PacketsSent     int64 `json:"packets_sent"`
	PacketsReceived int64 `json:"packets_received"`
	ErrorsIn        int64 `json:"errors_in"`
	ErrorsOut       int64 `json:"errors_out"`
}

// DependencyHealth salud de dependencias
type DependencyHealth struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"` // database, cache, external_api, etc.
	Status       string    `json:"status"`
	ResponseTime int64     `json:"response_time"` // ms
	LastCheck    time.Time `json:"last_check"`
	Message      string    `json:"message,omitempty"`
	Critical     bool      `json:"critical"`
}

// SEARCH & FILTER STRUCTURES

// SearchRequest estructura genérica de búsqueda
type SearchRequest struct {
	Query        string                 `json:"query,omitempty"`
	Filters      map[string]interface{} `json:"filters,omitempty"`
	Sort         []SortField            `json:"sort,omitempty"`
	Pagination   PaginationRequest      `json:"pagination"`
	Aggregations []string               `json:"aggregations,omitempty"`
	Highlight    HighlightConfig        `json:"highlight,omitempty"`
}

// SortField campo de ordenamiento
type SortField struct {
	Field string `json:"field"`
	Order string `json:"order" validate:"oneof=asc desc"`
}

// PaginationRequest solicitud de paginación
type PaginationRequest struct {
	Page     int `json:"page" validate:"min=1"`
	PageSize int `json:"page_size" validate:"min=1,max=100"`
}

// HighlightConfig configuración de resaltado
type HighlightConfig struct {
	Fields   []string `json:"fields"`
	PreTag   string   `json:"pre_tag"`
	PostTag  string   `json:"post_tag"`
	FragSize int      `json:"frag_size"`
	NumFrags int      `json:"num_frags"`
}

// SearchResponse respuesta genérica de búsqueda
type SearchResponse struct {
	Results      []interface{}          `json:"results"`
	Total        int64                  `json:"total"`
	Took         int64                  `json:"took"` // ms
	Pagination   Pagination             `json:"pagination"`
	Aggregations map[string]interface{} `json:"aggregations,omitempty"`
	Suggestions  []SearchSuggestion     `json:"suggestions,omitempty"`
}

// SearchSuggestion sugerencia de búsqueda
type SearchSuggestion struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

// VALIDATION HELPERS

// ValidateWorkflowStatus valida estado de workflow
func (ws WorkflowStatus) IsValid() bool {
	switch ws {
	case WorkflowStatusDraft, WorkflowStatusActive, WorkflowStatusInactive, WorkflowStatusArchived:
		return true
	}
	return false
}

// ValidateTriggerType valida tipo de trigger
func (tt TriggerType) IsValid() bool {
	switch tt {
	case TriggerTypeWebhook, TriggerTypeManual, TriggerTypeScheduled, TriggerTypeAPI, TriggerTypeEvent:
		return true
	}
	return false
}

// ValidateActionType valida tipo de acción
func (at ActionType) IsValid() bool {
	switch at {
	case ActionTypeHTTP, ActionTypeEmail, ActionTypeSlack, ActionTypeWebhook,
		ActionTypeDatabase, ActionTypeCondition, ActionTypeDelay, ActionTypeTransform,
		ActionTypeIntegration, ActionTypeNotification:
		return true
	}
	return false
}
