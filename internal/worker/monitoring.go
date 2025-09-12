package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"Engine_API_Workflow/internal/repository"

	"go.uber.org/zap"
)

// MetricsCollector recolecta métricas del sistema de workers
type MetricsCollector struct {
	// Contadores atómicos para thread safety
	tasksProcessed   int64
	tasksSucceeded   int64
	tasksFailed      int64
	tasksRetried     int64
	totalProcessTime int64 // en milisegundos

	// Métricas por tipo de acción
	actionMetrics map[string]*ActionMetrics
	actionMutex   sync.RWMutex

	// Métricas temporales
	hourlyStats map[int]*HourlyStats   // por hora del día (0-23)
	dailyStats  map[string]*DailyStats // por fecha YYYY-MM-DD
	statsMutex  sync.RWMutex

	// Configuración
	logger *zap.Logger
	repo   repository.QueueRepository
}

// ActionMetrics métricas por tipo de acción
type ActionMetrics struct {
	Count         int64
	SuccessCount  int64
	FailureCount  int64
	AvgDuration   int64 // milisegundos
	MaxDuration   int64
	MinDuration   int64
	TotalDuration int64
	LastExecuted  time.Time
	ErrorRate     float64
}

// HourlyStats estadísticas por hora
type HourlyStats struct {
	Hour           int
	TasksProcessed int64
	TasksSucceeded int64
	TasksFailed    int64
	AvgDuration    int64
}

// DailyStats estadísticas por día
type DailyStats struct {
	Date            string
	TasksProcessed  int64
	TasksSucceeded  int64
	TasksFailed     int64
	UniqueWorkflows int64
	AvgDuration     int64
}

// WorkerHealthStatus estado de salud de workers
type WorkerHealthStatus struct {
	IsHealthy       bool                   `json:"is_healthy"`
	ActiveWorkers   int                    `json:"active_workers"`
	QueueLength     int64                  `json:"queue_length"`
	ProcessingTasks int                    `json:"processing_tasks"`
	FailedTasks     int64                  `json:"failed_tasks"`
	AvgResponseTime float64                `json:"avg_response_time_ms"`
	ErrorRate       float64                `json:"error_rate"`
	Uptime          time.Duration          `json:"uptime"`
	LastHealthCheck time.Time              `json:"last_health_check"`
	Issues          []string               `json:"issues,omitempty"`
	Metrics         map[string]interface{} `json:"metrics"`
}

// NewMetricsCollector crea un nuevo collector de métricas
func NewMetricsCollector(logger *zap.Logger, repo repository.QueueRepository) *MetricsCollector {
	return &MetricsCollector{
		actionMetrics: make(map[string]*ActionMetrics),
		hourlyStats:   make(map[int]*HourlyStats),
		dailyStats:    make(map[string]*DailyStats),
		logger:        logger,
		repo:          repo,
	}
}

// RecordTaskProcessed registra una tarea procesada
func (m *MetricsCollector) RecordTaskProcessed(actionType string, duration time.Duration, success bool) {
	durationMs := duration.Milliseconds()

	// Actualizar contadores generales
	atomic.AddInt64(&m.tasksProcessed, 1)
	atomic.AddInt64(&m.totalProcessTime, durationMs)

	if success {
		atomic.AddInt64(&m.tasksSucceeded, 1)
	} else {
		atomic.AddInt64(&m.tasksFailed, 1)
	}

	// Actualizar métricas por tipo de acción
	m.updateActionMetrics(actionType, durationMs, success)

	// Actualizar estadísticas temporales
	m.updateTemporalStats(durationMs, success)
}

// RecordTaskRetried registra un reintento de tarea
func (m *MetricsCollector) RecordTaskRetried(actionType string) {
	atomic.AddInt64(&m.tasksRetried, 1)

	m.actionMutex.RLock()
	if metrics, exists := m.actionMetrics[actionType]; exists {
		atomic.AddInt64(&metrics.Count, 1) // Contar reintentos
	}
	m.actionMutex.RUnlock()
}

// updateActionMetrics actualiza métricas por tipo de acción
func (m *MetricsCollector) updateActionMetrics(actionType string, durationMs int64, success bool) {
	m.actionMutex.Lock()
	defer m.actionMutex.Unlock()

	if m.actionMetrics[actionType] == nil {
		m.actionMetrics[actionType] = &ActionMetrics{
			MinDuration: durationMs,
			MaxDuration: durationMs,
		}
	}

	metrics := m.actionMetrics[actionType]
	metrics.Count++
	metrics.TotalDuration += durationMs
	metrics.LastExecuted = time.Now()

	if success {
		metrics.SuccessCount++
	} else {
		metrics.FailureCount++
	}

	// Actualizar min/max duración
	if durationMs < metrics.MinDuration {
		metrics.MinDuration = durationMs
	}
	if durationMs > metrics.MaxDuration {
		metrics.MaxDuration = durationMs
	}

	// Calcular promedio y tasa de error
	if metrics.Count > 0 {
		metrics.AvgDuration = metrics.TotalDuration / metrics.Count
		metrics.ErrorRate = float64(metrics.FailureCount) / float64(metrics.Count) * 100
	}
}

// updateTemporalStats actualiza estadísticas temporales
func (m *MetricsCollector) updateTemporalStats(durationMs int64, success bool) {
	now := time.Now()
	hour := now.Hour()
	date := now.Format("2006-01-02")

	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()

	// Estadísticas por hora
	if m.hourlyStats[hour] == nil {
		m.hourlyStats[hour] = &HourlyStats{Hour: hour}
	}
	hourStats := m.hourlyStats[hour]
	hourStats.TasksProcessed++
	if success {
		hourStats.TasksSucceeded++
	} else {
		hourStats.TasksFailed++
	}
	if hourStats.TasksProcessed > 0 {
		hourStats.AvgDuration = (hourStats.AvgDuration + durationMs) / 2
	}

	// Estadísticas por día
	if m.dailyStats[date] == nil {
		m.dailyStats[date] = &DailyStats{Date: date}
	}
	dayStats := m.dailyStats[date]
	dayStats.TasksProcessed++
	if success {
		dayStats.TasksSucceeded++
	} else {
		dayStats.TasksFailed++
	}
	if dayStats.TasksProcessed > 0 {
		dayStats.AvgDuration = (dayStats.AvgDuration + durationMs) / 2
	}
}

// GetMetrics obtiene todas las métricas actuales
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	processed := atomic.LoadInt64(&m.tasksProcessed)
	succeeded := atomic.LoadInt64(&m.tasksSucceeded)
	failed := atomic.LoadInt64(&m.tasksFailed)
	retried := atomic.LoadInt64(&m.tasksRetried)
	totalTime := atomic.LoadInt64(&m.totalProcessTime)

	var avgDuration float64
	if processed > 0 {
		avgDuration = float64(totalTime) / float64(processed)
	}

	var successRate float64
	if processed > 0 {
		successRate = float64(succeeded) / float64(processed) * 100
	}

	metrics := map[string]interface{}{
		"summary": map[string]interface{}{
			"tasks_processed": processed,
			"tasks_succeeded": succeeded,
			"tasks_failed":    failed,
			"tasks_retried":   retried,
			"success_rate":    successRate,
			"avg_duration_ms": avgDuration,
			"total_time_ms":   totalTime,
		},
		"by_action_type": m.getActionMetrics(),
		"hourly_stats":   m.getHourlyStats(),
		"daily_stats":    m.getRecentDailyStats(7), // últimos 7 días
		"timestamp":      time.Now(),
	}

	return metrics
}

// getActionMetrics obtiene métricas por tipo de acción
func (m *MetricsCollector) getActionMetrics() map[string]*ActionMetrics {
	m.actionMutex.RLock()
	defer m.actionMutex.RUnlock()

	result := make(map[string]*ActionMetrics)
	for actionType, metrics := range m.actionMetrics {
		// Crear copia para evitar modificaciones concurrentes
		result[actionType] = &ActionMetrics{
			Count:         metrics.Count,
			SuccessCount:  metrics.SuccessCount,
			FailureCount:  metrics.FailureCount,
			AvgDuration:   metrics.AvgDuration,
			MaxDuration:   metrics.MaxDuration,
			MinDuration:   metrics.MinDuration,
			TotalDuration: metrics.TotalDuration,
			LastExecuted:  metrics.LastExecuted,
			ErrorRate:     metrics.ErrorRate,
		}
	}
	return result
}

// getHourlyStats obtiene estadísticas por hora
func (m *MetricsCollector) getHourlyStats() map[int]*HourlyStats {
	m.statsMutex.RLock()
	defer m.statsMutex.RUnlock()

	result := make(map[int]*HourlyStats)
	for hour, stats := range m.hourlyStats {
		result[hour] = &HourlyStats{
			Hour:           stats.Hour,
			TasksProcessed: stats.TasksProcessed,
			TasksSucceeded: stats.TasksSucceeded,
			TasksFailed:    stats.TasksFailed,
			AvgDuration:    stats.AvgDuration,
		}
	}
	return result
}

// getRecentDailyStats obtiene estadísticas de los últimos N días
func (m *MetricsCollector) getRecentDailyStats(days int) map[string]*DailyStats {
	m.statsMutex.RLock()
	defer m.statsMutex.RUnlock()

	result := make(map[string]*DailyStats)
	now := time.Now()

	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		if stats, exists := m.dailyStats[date]; exists {
			result[date] = &DailyStats{
				Date:            stats.Date,
				TasksProcessed:  stats.TasksProcessed,
				TasksSucceeded:  stats.TasksSucceeded,
				TasksFailed:     stats.TasksFailed,
				UniqueWorkflows: stats.UniqueWorkflows,
				AvgDuration:     stats.AvgDuration,
			}
		}
	}
	return result
}

// CheckHealth verifica el estado de salud del sistema
func (m *MetricsCollector) CheckHealth(ctx context.Context, workerPool *WorkerPool, startTime time.Time) (*WorkerHealthStatus, error) {
	status := &WorkerHealthStatus{
		IsHealthy:       true,
		LastHealthCheck: time.Now(),
		Uptime:          time.Since(startTime),
		Issues:          make([]string, 0),
		Metrics:         make(map[string]interface{}),
	}

	// Verificar estadísticas básicas
	processed := atomic.LoadInt64(&m.tasksProcessed)
	succeeded := atomic.LoadInt64(&m.tasksSucceeded)
	failed := atomic.LoadInt64(&m.tasksFailed)
	totalTime := atomic.LoadInt64(&m.totalProcessTime)

	if processed > 0 {
		status.ErrorRate = float64(failed) / float64(processed) * 100
		status.AvgResponseTime = float64(totalTime) / float64(processed)
	}

	// Verificar workers activos
	if workerPool != nil {
		poolStats := workerPool.GetStats()
		if totalWorkers, ok := poolStats["total_workers"].(int); ok {
			status.ActiveWorkers = totalWorkers
		}
		if currentLoad, ok := poolStats["current_load"].(int64); ok {
			status.ProcessingTasks = int(currentLoad)
		}
		if queueLength, ok := poolStats["queue_length"].(int64); ok {
			status.QueueLength = queueLength
		}
	}

	// Obtener tareas fallidas
	failedTasks, err := m.repo.GetFailedTasksCount(ctx)
	if err == nil {
		status.FailedTasks = failedTasks
	}

	// Evaluar salud general
	issues := m.evaluateHealth(status)
	status.Issues = issues
	status.IsHealthy = len(issues) == 0

	// Agregar métricas detalladas
	status.Metrics = m.GetMetrics()

	return status, nil
}

// evaluateHealth evalúa problemas de salud
func (m *MetricsCollector) evaluateHealth(status *WorkerHealthStatus) []string {
	issues := make([]string, 0)

	// Verificar tasa de error alta
	if status.ErrorRate > 10.0 {
		issues = append(issues, fmt.Sprintf("High error rate: %.2f%%", status.ErrorRate))
	}

	// Verificar cola muy larga
	if status.QueueLength > 1000 {
		issues = append(issues, fmt.Sprintf("Queue backlog is high: %d tasks", status.QueueLength))
	}

	// Verificar tiempo de respuesta alto
	if status.AvgResponseTime > 30000 { // 30 segundos
		issues = append(issues, fmt.Sprintf("High response time: %.2fms", status.AvgResponseTime))
	}

	// Verificar workers activos
	if status.ActiveWorkers == 0 {
		issues = append(issues, "No active workers")
	}

	// Verificar muchas tareas fallidas
	if status.FailedTasks > 100 {
		issues = append(issues, fmt.Sprintf("Many failed tasks: %d", status.FailedTasks))
	}

	return issues
}

// ResetMetrics reinicia todas las métricas
func (m *MetricsCollector) ResetMetrics() {
	atomic.StoreInt64(&m.tasksProcessed, 0)
	atomic.StoreInt64(&m.tasksSucceeded, 0)
	atomic.StoreInt64(&m.tasksFailed, 0)
	atomic.StoreInt64(&m.tasksRetried, 0)
	atomic.StoreInt64(&m.totalProcessTime, 0)

	m.actionMutex.Lock()
	m.actionMetrics = make(map[string]*ActionMetrics)
	m.actionMutex.Unlock()

	m.statsMutex.Lock()
	m.hourlyStats = make(map[int]*HourlyStats)
	m.dailyStats = make(map[string]*DailyStats)
	m.statsMutex.Unlock()

	m.logger.Info("Metrics reset successfully")
}

// CleanupOldStats limpia estadísticas antiguas
func (m *MetricsCollector) CleanupOldStats(maxDays int) {
	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()

	cutoffDate := time.Now().AddDate(0, 0, -maxDays)
	cutoffStr := cutoffDate.Format("2006-01-02")

	cleanedCount := 0
	for date := range m.dailyStats {
		if date < cutoffStr {
			delete(m.dailyStats, date)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		m.logger.Info("Cleaned up old daily stats",
			zap.Int("removed_days", cleanedCount),
			zap.String("cutoff_date", cutoffStr))
	}
}

// StartPeriodicCleanup inicia limpieza periódica de métricas
func (m *MetricsCollector) StartPeriodicCleanup(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour) // Limpiar diariamente
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.CleanupOldStats(30) // Mantener 30 días de estadísticas
		}
	}
}
