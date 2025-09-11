// internal/services/metrics_service.go
package services

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
)

// MetricsService define la interfaz para el servicio de métricas
type MetricsService interface {
	// Estadísticas generales
	GetSystemMetrics(ctx context.Context) (map[string]interface{}, error)
	GetExecutionStats(ctx context.Context, filter repository.LogSearchFilter) (*models.LogStats, error)

	// Métricas de workflows
	GetWorkflowMetrics(ctx context.Context, workflowID primitive.ObjectID, days int) (*models.WorkflowStats, error)
	GetWorkflowDistribution(ctx context.Context, timeRange time.Duration) ([]models.WorkflowCount, error)

	// Series temporales
	GetTimeSeriesData(ctx context.Context, metric string, timeRange time.Duration, intervals int) ([]models.TimeSeriesPoint, error)

	// Distribuciones
	GetTriggerDistribution(ctx context.Context, timeRange time.Duration) ([]models.TriggerCount, error)
	GetErrorDistribution(ctx context.Context, timeRange time.Duration) ([]models.ErrorCount, error)

	// Estadísticas por tiempo
	GetHourlyStats(ctx context.Context, timeRange time.Duration) ([]models.HourlyStats, error)
	GetWeeklyStats(ctx context.Context, weeks int) ([]models.WeeklyStats, error)

	// Recursos del sistema
	GetResourceUsage(ctx context.Context, timeRange time.Duration) ([]models.ResourceUsage, error)

	// Health checks
	CheckSystemHealth(ctx context.Context) error
}

// metricsService implementa MetricsService
type metricsService struct {
	userRepo     repository.UserRepository
	workflowRepo repository.WorkflowRepository
	logRepo      repository.LogRepository
	queueRepo    repository.QueueRepository
}

// NewMetricsService crea una nueva instancia del servicio de métricas
func NewMetricsService(
	userRepo repository.UserRepository,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	queueRepo repository.QueueRepository,
) MetricsService {
	return &metricsService{
		userRepo:     userRepo,
		workflowRepo: workflowRepo,
		logRepo:      logRepo,
		queueRepo:    queueRepo,
	}
}

// GetSystemMetrics obtiene métricas generales del sistema
func (s *metricsService) GetSystemMetrics(ctx context.Context) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Contadores básicos
	totalUsers, err := s.userRepo.CountUsers(ctx)
	if err != nil {
		totalUsers = 0
	}
	metrics["total_users"] = totalUsers

	totalWorkflows, err := s.workflowRepo.CountWorkflows(ctx)
	if err != nil {
		totalWorkflows = 0
	}
	metrics["total_workflows"] = totalWorkflows

	activeWorkflows, err := s.workflowRepo.CountActiveWorkflows(ctx)
	if err != nil {
		activeWorkflows = 0
	}
	metrics["active_workflows"] = activeWorkflows

	// Estadísticas de logs
	logStats, err := s.logRepo.GetStats(ctx, repository.LogSearchFilter{})
	if err != nil {
		logStats = &models.LogStats{}
	}
	metrics["total_executions"] = logStats.TotalExecutions
	metrics["successful_executions"] = logStats.SuccessfulExecutions
	metrics["failed_executions"] = logStats.FailedExecutions
	metrics["average_execution_time"] = logStats.AverageExecutionTime

	// Estado de la cola
	queueLength, err := s.queueRepo.GetQueueLength(ctx)
	if err != nil {
		queueLength = 0
	}
	metrics["queue_length"] = queueLength

	// Calcular tasa de éxito
	var successRate float64
	if logStats.TotalExecutions > 0 {
		successRate = (float64(logStats.SuccessfulExecutions) / float64(logStats.TotalExecutions)) * 100
	}
	metrics["success_rate"] = successRate

	// Métricas por tiempo
	metrics["executions_today"] = logStats.ExecutionsToday
	metrics["executions_this_week"] = logStats.ExecutionsThisWeek
	metrics["executions_this_month"] = logStats.ExecutionsThisMonth
	metrics["errors_last_24h"] = logStats.ErrorsLast24h

	// Timestamp de última actualización
	metrics["last_updated"] = time.Now()

	return metrics, nil
}

// GetExecutionStats obtiene estadísticas de ejecución con filtros
func (s *metricsService) GetExecutionStats(ctx context.Context, filter repository.LogSearchFilter) (*models.LogStats, error) {
	return s.logRepo.GetStats(ctx, filter)
}

// GetWorkflowMetrics obtiene métricas de un workflow específico
func (s *metricsService) GetWorkflowMetrics(ctx context.Context, workflowID primitive.ObjectID, days int) (*models.WorkflowStats, error) {
	// Obtener estadísticas del workflow
	logStats, err := s.logRepo.GetWorkflowStats(ctx, workflowID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow stats: %w", err)
	}

	// Convertir LogStats a WorkflowStats
	workflowStats := &models.WorkflowStats{
		TotalExecutions:      logStats.TotalExecutions,
		SuccessfulRuns:       logStats.SuccessfulExecutions,
		FailedRuns:           logStats.FailedExecutions,
		SuccessfulExecutions: logStats.SuccessfulExecutions,
		FailedExecutions:     logStats.FailedExecutions,
		AverageExecutionTime: logStats.AverageExecutionTime,
		AvgExecutionTimeMs:   logStats.AverageExecutionTime,
		LastExecutedAt:       logStats.LastExecutedAt,
		LastExecutionAt:      logStats.LastExecutedAt,
		LastSuccess:          logStats.LastSuccess,
		LastFailure:          logStats.LastFailure,
	}

	return workflowStats, nil
}

// GetWorkflowDistribution obtiene la distribución de ejecuciones por workflow
func (s *metricsService) GetWorkflowDistribution(ctx context.Context, timeRange time.Duration) ([]models.WorkflowCount, error) {
	// Por ahora implementamos una versión básica
	// En el futuro esto debería usar agregaciones de MongoDB más sofisticadas

	workflows, err := s.workflowRepo.GetActiveWorkflows(ctx)
	if err != nil {
		return []models.WorkflowCount{}, err
	}

	distribution := make([]models.WorkflowCount, 0, len(workflows))
	totalExecutions := int64(0)

	// Obtener estadísticas para cada workflow
	for _, workflow := range workflows {
		if workflow == nil {
			continue
		}

		stats, err := s.logRepo.GetWorkflowStats(ctx, workflow.ID, int(timeRange.Hours()/24))
		if err != nil {
			continue
		}

		if stats.TotalExecutions > 0 {
			var successRate float64
			if stats.TotalExecutions > 0 {
				successRate = (float64(stats.SuccessfulExecutions) / float64(stats.TotalExecutions)) * 100
			}

			distribution = append(distribution, models.WorkflowCount{
				WorkflowID:   workflow.ID.Hex(),
				WorkflowName: workflow.Name,
				Count:        int(stats.TotalExecutions),
				SuccessRate:  successRate,
			})

			totalExecutions += stats.TotalExecutions
		}
	}

	// Calcular porcentajes
	for i := range distribution {
		if totalExecutions > 0 {
			distribution[i].Percentage = (float64(distribution[i].Count) / float64(totalExecutions)) * 100
		}
	}

	return distribution, nil
}

// GetTimeSeriesData obtiene datos de series temporales para métricas
func (s *metricsService) GetTimeSeriesData(ctx context.Context, metric string, timeRange time.Duration, intervals int) ([]models.TimeSeriesPoint, error) {
	// Implementación básica - generar puntos de tiempo
	points := make([]models.TimeSeriesPoint, 0, intervals)
	intervalDuration := timeRange / time.Duration(intervals)
	endTime := time.Now()

	for i := intervals; i > 0; i-- {
		timestamp := endTime.Add(-time.Duration(i) * intervalDuration)

		var value float64

		// Obtener valor basado en el tipo de métrica
		switch metric {
		case "executions":
			// En el futuro esto debería consultar datos reales por intervalo de tiempo
			value = float64(i * 5) // Placeholder
		case "success_rate":
			value = 95.0 + float64(i%3) // Placeholder entre 95-98%
		case "queue_length":
			value = float64(10 + i%5) // Placeholder
		case "avg_execution_time":
			value = 1000.0 + float64(i*10) // Placeholder en ms
		default:
			value = 0
		}

		points = append(points, models.TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     value,
			Label:     timestamp.Format("15:04"),
		})
	}

	return points, nil
}

// GetTriggerDistribution obtiene la distribución por tipo de trigger
func (s *metricsService) GetTriggerDistribution(ctx context.Context, timeRange time.Duration) ([]models.TriggerCount, error) {
	// Implementación básica - en el futuro esto debería usar agregaciones
	distribution := []models.TriggerCount{
		{TriggerType: "manual", Count: 45, Percentage: 60.0},
		{TriggerType: "webhook", Count: 20, Percentage: 26.7},
		{TriggerType: "scheduled", Count: 10, Percentage: 13.3},
	}

	return distribution, nil
}

// GetErrorDistribution obtiene la distribución de errores
func (s *metricsService) GetErrorDistribution(ctx context.Context, timeRange time.Duration) ([]models.ErrorCount, error) {
	// Implementación básica - en el futuro esto debería analizar logs reales
	distribution := []models.ErrorCount{
		{
			ErrorType:    "timeout",
			ErrorMessage: "Execution timeout",
			Count:        15,
			Percentage:   50.0,
			LastOccurred: time.Now().Add(-2 * time.Hour),
			Severity:     "warning",
		},
		{
			ErrorType:    "validation",
			ErrorMessage: "Invalid input data",
			Count:        10,
			Percentage:   33.3,
			LastOccurred: time.Now().Add(-1 * time.Hour),
			Severity:     "warning",
		},
		{
			ErrorType:    "system",
			ErrorMessage: "Internal server error",
			Count:        5,
			Percentage:   16.7,
			LastOccurred: time.Now().Add(-30 * time.Minute),
			Severity:     "critical",
		},
	}

	return distribution, nil
}

// GetHourlyStats obtiene estadísticas por hora
func (s *metricsService) GetHourlyStats(ctx context.Context, timeRange time.Duration) ([]models.HourlyStats, error) {
	hours := int(timeRange.Hours())
	if hours > 24 {
		hours = 24 // Máximo 24 horas
	}

	stats := make([]models.HourlyStats, 0, hours)

	for i := 0; i < hours; i++ {
		hour := time.Now().Add(-time.Duration(hours-i) * time.Hour).Hour()

		// Datos simulados - en el futuro esto debería consultar datos reales
		executionCount := 10 + i%8
		successCount := int(float64(executionCount) * 0.9)
		failureCount := executionCount - successCount

		stats = append(stats, models.HourlyStats{
			Hour:           hour,
			ExecutionCount: executionCount,
			SuccessCount:   successCount,
			FailureCount:   failureCount,
			AverageTime:    1000.0 + float64(i*50),  // ms
			PeakTime:       2000.0 + float64(i*100), // ms
		})
	}

	return stats, nil
}

// GetWeeklyStats obtiene estadísticas por semana
func (s *metricsService) GetWeeklyStats(ctx context.Context, weeks int) ([]models.WeeklyStats, error) {
	if weeks > 12 {
		weeks = 12 // Máximo 12 semanas
	}

	stats := make([]models.WeeklyStats, 0, weeks)

	for i := 0; i < weeks; i++ {
		weekStart := time.Now().AddDate(0, 0, -(weeks-i)*7)
		year, week := weekStart.ISOWeek()
		weekStr := fmt.Sprintf("%d-W%02d", year, week)

		// Datos simulados
		executionCount := 200 + i*30
		successRate := 92.0 + float64(i%5)

		stats = append(stats, models.WeeklyStats{
			Week:            weekStr,
			ExecutionCount:  executionCount,
			SuccessRate:     successRate,
			AverageTime:     1200.0 + float64(i*25),
			ActiveWorkflows: 10 + i%3,
		})
	}

	return stats, nil
}

// GetResourceUsage obtiene el uso de recursos del sistema
func (s *metricsService) GetResourceUsage(ctx context.Context, timeRange time.Duration) ([]models.ResourceUsage, error) {
	intervals := int(timeRange.Minutes() / 5) // Cada 5 minutos
	if intervals > 288 {                      // Máximo 24 horas (288 intervalos de 5 min)
		intervals = 288
	}

	usage := make([]models.ResourceUsage, 0, intervals)

	for i := 0; i < intervals; i++ {
		timestamp := time.Now().Add(-time.Duration(intervals-i) * 5 * time.Minute)

		// Datos simulados - en producción esto vendría de métricas reales
		usage = append(usage, models.ResourceUsage{
			Timestamp:         timestamp,
			CPUPercent:        15.0 + float64(i%20),
			MemoryPercent:     40.0 + float64(i%30),
			MemoryUsedMB:      512.0 + float64(i*5),
			DiskUsedPercent:   60.0 + float64(i%10),
			ActiveConnections: 20 + i%15,
		})
	}

	return usage, nil
}

// CheckSystemHealth verifica la salud del sistema
func (s *metricsService) CheckSystemHealth(ctx context.Context) error {
	// Verificar conexión a MongoDB
	if err := s.checkDatabaseHealth(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Verificar conexión a Redis
	if err := s.queueRepo.Ping(ctx); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}

// Métodos helper privados

// checkDatabaseHealth verifica la salud de la base de datos
func (s *metricsService) checkDatabaseHealth(ctx context.Context) error {
	// Intentar hacer una consulta simple
	_, err := s.userRepo.CountUsers(ctx)
	return err
}

// parseTimeRange convierte un string de timeRange a time.Duration
func (s *metricsService) parseTimeRange(timeRange string) time.Duration {
	switch timeRange {
	case "1h":
		return time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour // Default a 24 horas
	}
}

// Helper para calcular intervalos apropiados basados en el rango de tiempo
func (s *metricsService) calculateIntervals(timeRange time.Duration) int {
	if timeRange <= time.Hour {
		return 12 // 5 minutos cada punto
	} else if timeRange <= 6*time.Hour {
		return 24 // 15 minutos cada punto
	} else if timeRange <= 24*time.Hour {
		return 24 // 1 hora cada punto
	} else if timeRange <= 7*24*time.Hour {
		return 7 // 1 día cada punto
	} else {
		return 30 // 1 día cada punto para 30 días
	}
}
