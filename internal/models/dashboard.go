package models

import (
	"time"
)

// DashboardData estructura principal que contiene todos los datos del dashboard
type DashboardData struct {
	SystemHealth    SystemHealth         `json:"system_health"`
	QuickStats      QuickStats           `json:"quick_stats"`
	RecentActivity  []ActivityItem       `json:"recent_activity"`
	WorkflowStatus  []WorkflowStatusItem `json:"workflow_status"`
	QueueStatus     QueueStatus          `json:"queue_status"`
	PerformanceData PerformanceData      `json:"performance_data"`
	Alerts          []Alert              `json:"alerts"`
	LastUpdated     time.Time            `json:"last_updated"`
}

// SystemHealth estado general del sistema
type SystemHealth struct {
	Status          string    `json:"status"` // "healthy", "warning", "critical"
	Uptime          string    `json:"uptime"`
	LastHealthy     time.Time `json:"last_healthy"`
	Version         string    `json:"version"`
	Environment     string    `json:"environment"`
	DBConnections   int       `json:"db_connections"`
	RedisConnected  bool      `json:"redis_connected"`
	APIResponseTime float64   `json:"api_response_time"` // ms
	MemoryUsage     float64   `json:"memory_usage"`      // MB
	CPUUsage        float64   `json:"cpu_usage"`         // percentage
}

// QuickStats métricas rápidas y resumidas
type QuickStats struct {
	ActiveWorkflows     int     `json:"active_workflows"`
	TotalUsers          int     `json:"total_users"`
	RunningExecutions   int     `json:"running_executions"`
	QueueLength         int     `json:"queue_length"`
	SuccessRate24h      float64 `json:"success_rate_24h"`
	AvgExecutionTime    float64 `json:"avg_execution_time"` // ms
	ExecutionsToday     int     `json:"executions_today"`
	ExecutionsThisWeek  int     `json:"executions_this_week"`
	ExecutionsThisMonth int     `json:"executions_this_month"`
	ErrorsLast24h       int     `json:"errors_last_24h"`
	TotalExecutions     int64   `json:"total_executions"`
	TotalWorkflows      int     `json:"total_workflows"`
}

// ActivityItem representa una actividad reciente en el sistema
type ActivityItem struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"` // "execution", "error", "warning", "info", "workflow_created"
	WorkflowID   string                 `json:"workflow_id"`
	WorkflowName string                 `json:"workflow_name"`
	UserID       string                 `json:"user_id"`
	UserName     string                 `json:"user_name"`
	Message      string                 `json:"message"`
	Timestamp    time.Time              `json:"timestamp"`
	Severity     string                 `json:"severity"`           // "low", "medium", "high", "critical"
	Status       string                 `json:"status"`             // "success", "failed", "running", "pending"
	Duration     *int64                 `json:"duration,omitempty"` // ms
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Icon         string                 `json:"icon"`  // Para el frontend
	Color        string                 `json:"color"` // Para el frontend
}

// WorkflowStatusItem estado detallado de un workflow
type WorkflowStatusItem struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Status         string     `json:"status"` // "active", "inactive", "error", "warning"
	IsActive       bool       `json:"is_active"`
	LastExecution  *time.Time `json:"last_execution"`
	NextScheduled  *time.Time `json:"next_scheduled,omitempty"`
	SuccessRate    float64    `json:"success_rate"`
	TotalRuns      int64      `json:"total_runs"`
	SuccessfulRuns int64      `json:"successful_runs"`
	FailedRuns     int64      `json:"failed_runs"`
	AvgRunTime     float64    `json:"avg_run_time"`  // ms
	LastRunTime    float64    `json:"last_run_time"` // ms
	Healthy        bool       `json:"is_healthy"`    // CAMBIADO: de IsHealthy a Healthy
	ErrorRate      float64    `json:"error_rate"`
	TriggerType    string     `json:"trigger_type"`
	Priority       int        `json:"priority"`
	Environment    string     `json:"environment"`
	Tags           []string   `json:"tags"`
	CreatedBy      string     `json:"created_by"`
	LastError      string     `json:"last_error,omitempty"`
	RunsToday      int        `json:"runs_today"`
	RunsThisWeek   int        `json:"runs_this_week"`
}

// QueueStatus estado detallado de las colas de procesamiento
type QueueStatus struct {
	Pending            int            `json:"pending"`
	Processing         int            `json:"processing"`
	Failed             int            `json:"failed"`
	Completed          int            `json:"completed"`
	DelayedJobs        int            `json:"delayed_jobs"`
	RetryJobs          int            `json:"retry_jobs"`
	ByPriority         map[string]int `json:"by_priority"`
	AverageWaitTime    float64        `json:"average_wait_time"`    // seconds
	AverageProcessTime float64        `json:"average_process_time"` // seconds
	ProcessingRate     float64        `json:"processing_rate"`      // jobs per minute
	ThroughputLast24h  int            `json:"throughput_last_24h"`
	OldestPending      *time.Time     `json:"oldest_pending,omitempty"`
	SlowestJob         *QueueJobInfo  `json:"slowest_job,omitempty"`
	QueueHealth        string         `json:"queue_health"` // "healthy", "warning", "critical"
	BacklogSize        int            `json:"backlog_size"`
	WorkersActive      int            `json:"workers_active"`
	WorkersTotal       int            `json:"workers_total"`
}

// QueueJobInfo información de un job específico
type QueueJobInfo struct {
	ID           string    `json:"id"`
	WorkflowID   string    `json:"workflow_id"`
	WorkflowName string    `json:"workflow_name"`
	StartedAt    time.Time `json:"started_at"`
	Duration     int64     `json:"duration"` // ms
	Priority     int       `json:"priority"`
	RetryCount   int       `json:"retry_count"`
}

// PerformanceData datos para gráficos y tendencias
type PerformanceData struct {
	ExecutionsLast24h    []TimeSeriesPoint `json:"executions_last_24h"`
	ExecutionsLast7d     []TimeSeriesPoint `json:"executions_last_7d"`
	SuccessRateTrend     []TimeSeriesPoint `json:"success_rate_trend"`
	AvgExecutionTime     []TimeSeriesPoint `json:"avg_execution_time"`
	QueueLengthTrend     []TimeSeriesPoint `json:"queue_length_trend"`
	ErrorDistribution    []ErrorCount      `json:"error_distribution"`
	TriggerDistribution  []TriggerCount    `json:"trigger_distribution"`
	WorkflowDistribution []WorkflowCount   `json:"workflow_distribution"`
	UserActivityHeatmap  []HeatmapPoint    `json:"user_activity_heatmap"`
	SystemResourceUsage  []ResourceUsage   `json:"system_resource_usage"`
	HourlyExecutions     []HourlyStats     `json:"hourly_executions"`
	WeeklyTrends         []WeeklyStats     `json:"weekly_trends"`
}

// TimeSeriesPoint punto en una serie temporal
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Label     string    `json:"label,omitempty"`
}

// ErrorCount conteo de errores por tipo
type ErrorCount struct {
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Count        int       `json:"count"`
	Percentage   float64   `json:"percentage"`
	LastOccurred time.Time `json:"last_occurred"`
	WorkflowID   string    `json:"workflow_id,omitempty"`
	Severity     string    `json:"severity"`
}

// TriggerCount conteo de triggers por tipo
type TriggerCount struct {
	TriggerType string  `json:"trigger_type"`
	Count       int     `json:"count"`
	Percentage  float64 `json:"percentage"`
}

// WorkflowCount conteo de ejecuciones por workflow
type WorkflowCount struct {
	WorkflowID   string  `json:"workflow_id"`
	WorkflowName string  `json:"workflow_name"`
	Count        int     `json:"count"`
	Percentage   float64 `json:"percentage"`
	SuccessRate  float64 `json:"success_rate"`
}

// HeatmapPoint punto para heatmap de actividad
type HeatmapPoint struct {
	Hour      int     `json:"hour"`        // 0-23
	DayOfWeek int     `json:"day_of_week"` // 0-6 (Sunday=0)
	Value     float64 `json:"value"`
	Date      string  `json:"date"`
}

// ResourceUsage uso de recursos del sistema
type ResourceUsage struct {
	Timestamp         time.Time `json:"timestamp"`
	CPUPercent        float64   `json:"cpu_percent"`
	MemoryPercent     float64   `json:"memory_percent"`
	MemoryUsedMB      float64   `json:"memory_used_mb"`
	DiskUsedPercent   float64   `json:"disk_used_percent"`
	ActiveConnections int       `json:"active_connections"`
}

// Alert alerta del sistema
type Alert struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`     // "performance", "error", "queue", "system", "workflow"
	Severity       string                 `json:"severity"` // "info", "warning", "critical"
	Title          string                 `json:"title"`
	Message        string                 `json:"message"`
	Timestamp      time.Time              `json:"timestamp"`
	IsActive       bool                   `json:"is_active"`
	IsAcknowledged bool                   `json:"is_acknowledged"`
	WorkflowID     *string                `json:"workflow_id,omitempty"`
	UserID         *string                `json:"user_id,omitempty"`
	Metrics        map[string]interface{} `json:"metrics,omitempty"`
	Actions        []AlertAction          `json:"actions,omitempty"`
	Duration       *int64                 `json:"duration,omitempty"` // ms desde que se activó
	Count          int                    `json:"count"`              // número de veces que se ha repetido
}

// AlertAction acción sugerida para una alerta
type AlertAction struct {
	Type        string `json:"type"` // "restart", "scale", "investigate", "ignore"
	Description string `json:"description"`
	URL         string `json:"url,omitempty"`
}

// DashboardConfig configuración del dashboard
type DashboardConfig struct {
	RefreshInterval      time.Duration `json:"refresh_interval"`
	MaxRecentActivity    int           `json:"max_recent_activity"`
	MetricsRetentionDays int           `json:"metrics_retention_days"`
	AlertRetentionDays   int           `json:"alert_retention_days"`
	EnableRealTime       bool          `json:"enable_real_time"`
	EnableAlerts         bool          `json:"enable_alerts"`
	Theme                string        `json:"theme"` // "light", "dark"
}

// DashboardFilter filtros para el dashboard
type DashboardFilter struct {
	TimeRange   string     `json:"time_range"` // "1h", "24h", "7d", "30d"
	WorkflowIDs []string   `json:"workflow_ids,omitempty"`
	UserIDs     []string   `json:"user_ids,omitempty"`
	Status      []string   `json:"status,omitempty"`
	Environment string     `json:"environment,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
}

// HourlyStats estadísticas por hora
type HourlyStats struct {
	Hour           int     `json:"hour"`
	ExecutionCount int     `json:"execution_count"`
	SuccessCount   int     `json:"success_count"`
	FailureCount   int     `json:"failure_count"`
	AverageTime    float64 `json:"average_time"` // ms
	PeakTime       float64 `json:"peak_time"`    // ms
}

// WeeklyStats estadísticas por semana
type WeeklyStats struct {
	Week            string  `json:"week"` // "2024-W01"
	ExecutionCount  int     `json:"execution_count"`
	SuccessRate     float64 `json:"success_rate"`
	AverageTime     float64 `json:"average_time"`
	ActiveWorkflows int     `json:"active_workflows"`
}

// DashboardSummary resumen ejecutivo para la vista principal
type DashboardSummary struct {
	SystemStatus        string    `json:"system_status"`
	TotalWorkflows      int       `json:"total_workflows"`
	ActiveWorkflows     int       `json:"active_workflows"`
	TotalExecutions     int64     `json:"total_executions"`
	SuccessRate         float64   `json:"success_rate"`
	CurrentQueueLength  int       `json:"current_queue_length"`
	AverageResponseTime float64   `json:"average_response_time"`
	LastUpdate          time.Time `json:"last_update"`
	CriticalAlerts      int       `json:"critical_alerts"`
	WarningAlerts       int       `json:"warning_alerts"`
}

// Métodos helper para los modelos

// IsHealthy determina si el sistema está saludable
func (sh *SystemHealth) IsHealthy() bool {
	return sh.Status == "healthy"
}

// GetStatusColor retorna el color apropiado para el estado
func (sh *SystemHealth) GetStatusColor() string {
	switch sh.Status {
	case "healthy":
		return "green"
	case "warning":
		return "yellow"
	case "critical":
		return "red"
	default:
		return "gray"
	}
}

// IsHealthy determina si el workflow está saludable
func (ws *WorkflowStatusItem) IsHealthy() bool {
	return ws.SuccessRate >= 90.0 && ws.ErrorRate < 10.0 && ws.IsActive
}

// GetHealthStatus retorna el estado de salud del workflow
func (ws *WorkflowStatusItem) GetHealthStatus() string {
	if !ws.IsActive {
		return "inactive"
	}
	if ws.SuccessRate >= 95.0 {
		return "excellent"
	}
	if ws.SuccessRate >= 90.0 {
		return "good"
	}
	if ws.SuccessRate >= 80.0 {
		return "warning"
	}
	return "critical"
}

// GetQueueHealth determina el estado de salud de la cola
func (qs *QueueStatus) GetQueueHealth() string {
	if qs.Pending > 1000 || qs.AverageWaitTime > 300 { // 5 minutes
		return "critical"
	}
	if qs.Pending > 500 || qs.AverageWaitTime > 120 { // 2 minutes
		return "warning"
	}
	return "healthy"
}

// GetSeverityLevel retorna el nivel numérico de severidad
func (a *Alert) GetSeverityLevel() int {
	switch a.Severity {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// GetActivityIcon retorna el icono apropiado para el tipo de actividad
func (ai *ActivityItem) GetActivityIcon() string {
	switch ai.Type {
	case "execution":
		if ai.Status == "success" {
			return "✅"
		} else if ai.Status == "failed" {
			return "❌"
		} else if ai.Status == "running" {
			return "🔄"
		}
		return "⏳"
	case "error":
		return "🚨"
	case "warning":
		return "⚠️"
	case "workflow_created":
		return "📝"
	default:
		return "ℹ️"
	}
}

// GetActivityColor retorna el color apropiado para el tipo de actividad
func (ai *ActivityItem) GetActivityColor() string {
	switch ai.Severity {
	case "critical":
		return "#dc2626" // red-600
	case "high":
		return "#ea580c" // orange-600
	case "medium":
		return "#ca8a04" // yellow-600
	case "low":
		return "#16a34a" // green-600
	default:
		return "#6b7280" // gray-500
	}
}
