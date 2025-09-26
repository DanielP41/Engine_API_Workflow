package models

import (
	"fmt"   
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ===============================================
// DASHBOARD DATA STRUCTURES
// ===============================================

// DashboardData estructura principal del dashboard
type DashboardData struct {
	Summary        *DashboardSummary    `json:"summary"`
	QuickStats     *QuickStats          `json:"quick_stats"`
	SystemMetrics  *SystemMetrics       `json:"system_metrics"`
	RecentActivity []ActivityItem       `json:"recent_activity"`
	WorkflowStatus []WorkflowStatusItem `json:"workflow_status"`
	QueueStatus    *QueueStatus         `json:"queue_status"`
	SystemHealth   *SystemHealth        `json:"system_health"`
	Alerts         []Alert              `json:"alerts"`
	LastUpdated    time.Time            `json:"last_updated"`
}

// DashboardSummary resumen general del dashboard
type DashboardSummary struct {
	TotalWorkflows       int       `json:"total_workflows"`
	ActiveWorkflows      int       `json:"active_workflows"`
	TotalExecutions      int64     `json:"total_executions"`
	SuccessfulRuns       int64     `json:"successful_runs"`
	FailedRuns           int64     `json:"failed_runs"`
	SuccessRate          float64   `json:"success_rate"`
	AverageExecutionTime float64   `json:"avg_execution_time"` // segundos
	QueueLength          int64     `json:"queue_length"`
	ProcessingTasks      int64     `json:"processing_tasks"`
	LastUpdated          time.Time `json:"last_updated"`
}

// DashboardFilter filtros para el dashboard
type DashboardFilter struct {
	UserIDs     []string   `json:"user_ids,omitempty"`
	WorkflowIDs []string   `json:"workflow_ids,omitempty"`
	Status      []string   `json:"status,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	TimeRange   string     `json:"time_range,omitempty"` // "1h", "24h", "7d", "30d"
}

// ===============================================
// QUEUE STATUS - ESTRUCTURA CORREGIDA
// ===============================================

// QueueStatus representa el estado actual de las colas de tareas
type QueueStatus struct {
	// CAMPOS PRINCIPALES QUE FALTABAN Y CAUSABAN LOS ERRORES
	QueuedTasks     int64     `json:"queued_tasks" bson:"queued_tasks"`
	ProcessingTasks int64     `json:"processing_tasks" bson:"processing_tasks"`
	FailedTasks     int64     `json:"failed_tasks" bson:"failed_tasks"`
	RetryingTasks   int64     `json:"retrying_tasks" bson:"retrying_tasks"`
	Health          string    `json:"health" bson:"health"`
	Timestamp       time.Time `json:"timestamp" bson:"timestamp"`

	// Campos de compatibilidad para no romper código existente
	Pending     int    `json:"pending,omitempty" bson:"pending,omitempty"`
	Processing  int    `json:"processing,omitempty" bson:"processing,omitempty"`
	Failed      int    `json:"failed,omitempty" bson:"failed,omitempty"`
	RetryJobs   int    `json:"retry_jobs,omitempty" bson:"retry_jobs,omitempty"`
	QueueHealth string `json:"queue_health,omitempty" bson:"queue_health,omitempty"`
}

// GetQueueHealth determina el estado de salud basado en los contadores actuales
func (qs *QueueStatus) GetQueueHealth() string {
	if qs.FailedTasks > qs.ProcessingTasks*2 {
		return "unhealthy"
	} else if qs.FailedTasks > 0 {
		return "degraded"
	}
	return "healthy"
}

// ===============================================
// WORKFLOW STATUS
// ===============================================

// WorkflowStatusItem representa el estado de un workflow individual
type WorkflowStatusItem struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Status         string     `json:"status"`
	IsActive       bool       `json:"is_active"`
	LastExecution  *time.Time `json:"last_execution"`
	SuccessRate    float64    `json:"success_rate"`
	TotalRuns      int64      `json:"total_runs"`
	SuccessfulRuns int64      `json:"successful_runs"`
	FailedRuns     int64      `json:"failed_runs"`
	AvgRunTime     float64    `json:"avg_run_time"` // segundos
	Healthy        bool       `json:"healthy"`
	TriggerType    string     `json:"trigger_type"`
	Tags           []string   `json:"tags,omitempty"`
}

// ===============================================
// QUICK STATS
// ===============================================

// QuickStats estadísticas rápidas para el dashboard
type QuickStats struct {
	ActiveWorkflows  int64     `json:"active_workflows"`
	QueuedTasks      int64     `json:"queued_tasks"`
	RunningTasks     int64     `json:"running_tasks"`
	CompletedToday   int64     `json:"completed_today"`
	FailedToday      int64     `json:"failed_today"`
	SuccessRateToday float64   `json:"success_rate_today"`
	AvgResponseTime  float64   `json:"avg_response_time"` // segundos
	SystemLoad       float64   `json:"system_load"`
	LastUpdated      time.Time `json:"last_updated"`
}

// ===============================================
// SYSTEM METRICS
// ===============================================

// SystemMetrics métricas del sistema
type SystemMetrics struct {
	CPU         CPUMetrics    `json:"cpu"`
	Memory      MemoryMetrics `json:"memory"`
	Disk        DiskMetrics   `json:"disk"`
	Goroutines  int           `json:"goroutines"`
	LastUpdated time.Time     `json:"last_updated"`
}

// CPUMetrics métricas de CPU
type CPUMetrics struct {
	Usage     float64 `json:"usage"`       // porcentaje
	LoadAvg1  float64 `json:"load_avg_1"`  // load average 1 min
	LoadAvg5  float64 `json:"load_avg_5"`  // load average 5 min
	LoadAvg15 float64 `json:"load_avg_15"` // load average 15 min
}

// MemoryMetrics métricas de memoria
type MemoryMetrics struct {
	Used      int64   `json:"used"`      // bytes
	Total     int64   `json:"total"`     // bytes
	Available int64   `json:"available"` // bytes
	Usage     float64 `json:"usage"`     // porcentaje
}

// DiskMetrics métricas de disco
type DiskMetrics struct {
	Used      int64   `json:"used"`      // bytes
	Total     int64   `json:"total"`     // bytes
	Available int64   `json:"available"` // bytes
	Usage     float64 `json:"usage"`     // porcentaje
}

// ===============================================
// SYSTEM HEALTH
// ===============================================

// SystemHealth estado de salud del sistema
type SystemHealth struct {
	Status     string            `json:"status"` // "healthy", "warning", "critical"
	Score      int               `json:"score"`  // 0-100
	Components []ComponentHealth `json:"components"`
	Issues     []HealthIssue     `json:"issues,omitempty"`
	LastCheck  time.Time         `json:"last_check"`
}

// ComponentHealth salud de un componente específico
type ComponentHealth struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"` // "healthy", "warning", "critical"
	Description string    `json:"description,omitempty"`
	LastCheck   time.Time `json:"last_check"`
}

// HealthIssue problema de salud detectado
type HealthIssue struct {
	Component string     `json:"component"`
	Severity  string     `json:"severity"` // "low", "medium", "high", "critical"
	Message   string     `json:"message"`
	Detected  time.Time  `json:"detected"`
	Resolved  *time.Time `json:"resolved,omitempty"`
}

// ===============================================
// ACTIVITY ITEMS
// ===============================================

// ActivityItem representa una actividad reciente
type ActivityItem struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"` // "workflow_started", "workflow_completed", "workflow_failed", etc.
	Title        string                 `json:"title"`
	Description  string                 `json:"description,omitempty"`
	UserID       *primitive.ObjectID    `json:"user_id,omitempty"`
	UserName     string                 `json:"user_name,omitempty"`
	WorkflowID   *primitive.ObjectID    `json:"workflow_id,omitempty"`
	WorkflowName string                 `json:"workflow_name,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// ===============================================
// ALERTS
// ===============================================

// Alert representa una alerta del sistema
type Alert struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"` // "info", "warning", "error", "critical"
	Title      string        `json:"title"`
	Message    string        `json:"message"`
	Severity   string        `json:"severity"` // "low", "medium", "high", "critical"
	Source     string        `json:"source,omitempty"`
	Component  string        `json:"component,omitempty"`
	Actions    []AlertAction `json:"actions,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
	Resolved   *time.Time    `json:"resolved,omitempty"`
	ResolvedBy string        `json:"resolved_by,omitempty"`
}

// AlertAction acción sugerida para una alerta
type AlertAction struct {
	Type        string `json:"type"` // "investigate", "restart", "ignore", etc.
	Description string `json:"description"`
	URL         string `json:"url,omitempty"`
}

// ===============================================
// PERFORMANCE DATA
// ===============================================

// PerformanceData datos de rendimiento del sistema
type PerformanceData struct {
	TimeRange           string                    `json:"time_range"`
	ExecutionTrends     []ExecutionTrendPoint     `json:"execution_trends"`
	QueueMetrics        []QueueMetricPoint        `json:"queue_metrics"`
	SuccessRateHistory  []SuccessRatePoint        `json:"success_rate_history"`
	ResponseTimeHistory []ResponseTimePoint       `json:"response_time_history"`
	TopWorkflows        []WorkflowPerformanceItem `json:"top_workflows"`
	LastUpdated         time.Time                 `json:"last_updated"`
}

// ExecutionTrendPoint punto de tendencia de ejecuciones
type ExecutionTrendPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Total      int64     `json:"total"`
	Successful int64     `json:"successful"`
	Failed     int64     `json:"failed"`
}

// QueueMetricPoint punto de métricas de cola
type QueueMetricPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Queued     int64     `json:"queued"`
	Processing int64     `json:"processing"`
	Failed     int64     `json:"failed"`
	Retrying   int64     `json:"retrying"`
}

// SuccessRatePoint punto de tasa de éxito
type SuccessRatePoint struct {
	Timestamp   time.Time `json:"timestamp"`
	SuccessRate float64   `json:"success_rate"`
}

// ResponseTimePoint punto de tiempo de respuesta
type ResponseTimePoint struct {
	Timestamp   time.Time `json:"timestamp"`
	AvgResponse float64   `json:"avg_response"`
	P50Response float64   `json:"p50_response"`
	P95Response float64   `json:"p95_response"`
	P99Response float64   `json:"p99_response"`
}

// WorkflowPerformanceItem elemento de rendimiento de workflow
type WorkflowPerformanceItem struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	TotalRuns        int64      `json:"total_runs"`
	SuccessfulRuns   int64      `json:"successful_runs"`
	FailedRuns       int64      `json:"failed_runs"`
	SuccessRate      float64    `json:"success_rate"`
	AvgExecutionTime float64    `json:"avg_execution_time"`
	LastExecution    *time.Time `json:"last_execution"`
}

// ===============================================
// METRICS & ANALYTICS STRUCTURES (AGREGADAS PARA INTERFACES.GO)
// ===============================================

// TimeSeriesPoint punto de datos de serie temporal
type TimeSeriesPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Value     float64                `json:"value"`
	Label     string                 `json:"label,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowCount conteo de workflows por categoría
type WorkflowCount struct {
	WorkflowID   string  `json:"workflow_id"`
	WorkflowName string  `json:"workflow_name"`
	Count        int64   `json:"count"`
	Percentage   float64 `json:"percentage"`
}

// TriggerCount conteo por tipo de trigger
type TriggerCount struct {
	TriggerType string  `json:"trigger_type"`
	Count       int64   `json:"count"`
	Percentage  float64 `json:"percentage"`
}

// ErrorCount conteo de errores por tipo
type ErrorCount struct {
	ErrorType  string     `json:"error_type"`
	ErrorCode  string     `json:"error_code,omitempty"`
	Count      int64      `json:"count"`
	Percentage float64    `json:"percentage"`
	LastSeen   *time.Time `json:"last_seen,omitempty"`
}

// HourlyStats estadísticas por hora
type HourlyStats struct {
	Hour        int     `json:"hour"` // 0-23
	Date        string  `json:"date"` // YYYY-MM-DD
	Executions  int64   `json:"executions"`
	Successful  int64   `json:"successful"`
	Failed      int64   `json:"failed"`
	AvgDuration float64 `json:"avg_duration"` // milliseconds
	SuccessRate float64 `json:"success_rate"` // percentage
}

// ===============================================
// 🆕 MODELOS FALTANTES - AGREGADOS
// ===============================================

// WeeklyStats estadísticas por semana
type WeeklyStats struct {
	Week            string  `json:"week" bson:"week"`                         // Formato "2024-W01"
	ExecutionCount  int     `json:"execution_count" bson:"execution_count"`   // Número de ejecuciones
	SuccessRate     float64 `json:"success_rate" bson:"success_rate"`         // Porcentaje de éxito
	AverageTime     float64 `json:"average_time" bson:"average_time"`         // Tiempo promedio en ms
	ActiveWorkflows int     `json:"active_workflows" bson:"active_workflows"` // Workflows activos
}

// ResourceUsage uso de recursos del sistema
type ResourceUsage struct {
	Timestamp         time.Time `json:"timestamp" bson:"timestamp"`                     // Timestamp del punto de datos
	CPUPercent        float64   `json:"cpu_percent" bson:"cpu_percent"`                 // Porcentaje de uso de CPU
	MemoryPercent     float64   `json:"memory_percent" bson:"memory_percent"`           // Porcentaje de uso de memoria
	MemoryUsedMB      float64   `json:"memory_used_mb" bson:"memory_used_mb"`           // Memoria usada en MB
	DiskUsedPercent   float64   `json:"disk_used_percent" bson:"disk_used_percent"`     // Porcentaje de uso de disco
	ActiveConnections int       `json:"active_connections" bson:"active_connections"`   // Conexiones activas
}

// ===============================================
// MÉTODOS HELPER PARA LOS NUEVOS MODELOS
// ===============================================

// GetWeekNumber extrae el número de semana de la cadena Week
func (ws *WeeklyStats) GetWeekNumber() int {
	// Extraer número de semana de formato "2024-W01"
	if len(ws.Week) >= 6 && ws.Week[4:6] == "-W" {
		weekNum := 0
		fmt.Sscanf(ws.Week[6:], "%d", &weekNum)
		return weekNum
	}
	return 0
}

// GetYear extrae el año de la cadena Week
func (ws *WeeklyStats) GetYear() int {
	// Extraer año de formato "2024-W01"
	if len(ws.Week) >= 4 {
		year := 0
		fmt.Sscanf(ws.Week[:4], "%d", &year)
		return year
	}
	return 0
}

// IsHighLoad determina si el uso de recursos es alto
func (ru *ResourceUsage) IsHighLoad() bool {
	return ru.CPUPercent > 80.0 || ru.MemoryPercent > 85.0 || ru.DiskUsedPercent > 90.0
}

// GetLoadLevel retorna el nivel de carga del sistema
func (ru *ResourceUsage) GetLoadLevel() string {
	if ru.CPUPercent > 90.0 || ru.MemoryPercent > 95.0 || ru.DiskUsedPercent > 95.0 {
		return "critical"
	} else if ru.CPUPercent > 80.0 || ru.MemoryPercent > 85.0 || ru.DiskUsedPercent > 90.0 {
		return "high"
	} else if ru.CPUPercent > 60.0 || ru.MemoryPercent > 70.0 || ru.DiskUsedPercent > 80.0 {
		return "medium"
	}
	return "low"
}