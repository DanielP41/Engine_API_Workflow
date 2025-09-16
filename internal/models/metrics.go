package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TimeSeriesPoint punto en una serie temporal
type TimeSeriesPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Value     float64                `json:"value"`
	Label     string                 `json:"label,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowCount conteo de workflows
type WorkflowCount struct {
	WorkflowID   primitive.ObjectID `json:"workflow_id"`
	WorkflowName string             `json:"workflow_name"`
	Count        int64              `json:"count"`
	Percentage   float64            `json:"percentage"`
}

// TriggerCount conteo de triggers
type TriggerCount struct {
	TriggerType TriggerType `json:"trigger_type"`
	Count       int64       `json:"count"`
	Percentage  float64     `json:"percentage"`
}

// ErrorCount conteo de errores
type ErrorCount struct {
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Count        int64     `json:"count"`
	Percentage   float64   `json:"percentage"`
	LastOccurred time.Time `json:"last_occurred"`
}

// HourlyStats estadísticas por hora
type HourlyStats struct {
	Hour        int     `json:"hour"`
	Executions  int64   `json:"executions"`
	Successes   int64   `json:"successes"`
	Failures    int64   `json:"failures"`
	SuccessRate float64 `json:"success_rate"`
	AvgDuration float64 `json:"avg_duration"` // ms
}

// WeeklyStats estadísticas por semana
type WeeklyStats struct {
	Week        int     `json:"week"`
	Year        int     `json:"year"`
	Executions  int64   `json:"executions"`
	Successes   int64   `json:"successes"`
	Failures    int64   `json:"failures"`
	SuccessRate float64 `json:"success_rate"`
	AvgDuration float64 `json:"avg_duration"` // ms
}

// WorkflowStats estadísticas de workflow
type WorkflowStats struct {
	TotalExecutions      int64      `json:"total_executions"`
	SuccessfulExecutions int64      `json:"successful_executions"`
	FailedExecutions     int64      `json:"failed_executions"`
	SuccessRate          float64    `json:"success_rate"`
	FailureRate          float64    `json:"failure_rate"`
	AvgExecutionTime     float64    `json:"avg_execution_time"` // ms
	MinExecutionTime     float64    `json:"min_execution_time"` // ms
	MaxExecutionTime     float64    `json:"max_execution_time"` // ms
	LastExecutionAt      *time.Time `json:"last_execution_at,omitempty"`
	ExecutionsToday      int64      `json:"executions_today"`
	ExecutionsThisWeek   int64      `json:"executions_this_week"`
	ExecutionsThisMonth  int64      `json:"executions_this_month"`
	LastError            string     `json:"last_error,omitempty"`
	LastErrorAt          *time.Time `json:"last_error_at,omitempty"`
}

// LogStatistics estadísticas generales de logs
type LogStatistics struct {
	TotalLogs           int64                 `json:"total_logs"`
	LogsToday           int64                 `json:"logs_today"`
	LogsThisWeek        int64                 `json:"logs_this_week"`
	LogsThisMonth       int64                 `json:"logs_this_month"`
	SuccessRate         float64               `json:"success_rate"`
	FailureRate         float64               `json:"failure_rate"`
	AvgExecutionTime    float64               `json:"avg_execution_time"`
	TriggerDistribution []TriggerDistribution `json:"trigger_distribution"`
	ErrorDistribution   []ErrorDistribution   `json:"error_distribution"`
	LastUpdated         time.Time             `json:"last_updated"`
	DataPeriod          string                `json:"data_period"`
}

// TriggerDistribution distribución de triggers
type TriggerDistribution struct {
	TriggerType TriggerType `json:"trigger_type"`
	Count       int64       `json:"count"`
	Percentage  float64     `json:"percentage"`
}

// ErrorDistribution distribución de errores
type ErrorDistribution struct {
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Count        int64     `json:"count"`
	Percentage   float64   `json:"percentage"`
	LastOccurred time.Time `json:"last_occurred"`
}

// MetricsRequest solicitud de métricas
type MetricsRequest struct {
	WorkflowID  *primitive.ObjectID `json:"workflow_id,omitempty"`
	UserID      *primitive.ObjectID `json:"user_id,omitempty"`
	StartDate   *time.Time          `json:"start_date,omitempty"`
	EndDate     *time.Time          `json:"end_date,omitempty"`
	MetricType  string              `json:"metric_type,omitempty"`
	Granularity string              `json:"granularity,omitempty" validate:"omitempty,oneof=hour day week month"`
	Limit       int                 `json:"limit,omitempty" validate:"omitempty,min=1,max=1000"`
}

// MetricsResponse respuesta de métricas
type MetricsResponse struct {
	Metrics   []TimeSeriesPoint `json:"metrics"`
	Summary   MetricsSummary    `json:"summary"`
	Period    string            `json:"period"`
	Generated time.Time         `json:"generated"`
}

// MetricsSummary resumen de métricas
type MetricsSummary struct {
	Total      float64 `json:"total"`
	Average    float64 `json:"average"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Trend      string  `json:"trend"`       // up, down, stable
	ChangeRate float64 `json:"change_rate"` // percentage change
}
