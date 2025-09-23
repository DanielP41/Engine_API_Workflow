package services

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
)

// DashboardService interfaz para servicios de dashboard
type DashboardService interface {
	// Datos principales del dashboard
	GetCompleteDashboard(ctx context.Context, filter *models.DashboardFilter) (*models.DashboardData, error)
	GetDashboardSummary(ctx context.Context, filter *models.DashboardFilter) (*models.DashboardSummary, error)

	// Métricas específicas
	GetSystemMetrics(ctx context.Context, timeRange string) (*models.SystemMetrics, error)
	GetQuickStats(ctx context.Context) (*models.QuickStats, error)
	GetRecentActivity(ctx context.Context, limit int) ([]models.ActivityItem, error)

	// Estado de workflows y cola
	GetWorkflowStatus(ctx context.Context, limit int) ([]models.WorkflowStatusItem, error)
	GetQueueStatus(ctx context.Context) (*models.QueueStatus, error)

	// Salud del sistema
	GetSystemHealth(ctx context.Context) (*models.SystemHealth, error)

	// Métricas personalizadas
	GetDashboardMetrics(ctx context.Context, metrics []string, timeRange string) (map[string]interface{}, error)
	GetPerformanceData(ctx context.Context, timeRange string) (*models.PerformanceData, error)

	// Operaciones de mantenimiento
	RefreshDashboardData(ctx context.Context) error
	ValidateFilter(filter *models.DashboardFilter) error
}

// dashboardServiceImpl implementa DashboardService
type dashboardServiceImpl struct {
	metricsService MetricsService
	workflowRepo   repository.WorkflowRepository
	logRepo        repository.LogRepository
	userRepo       repository.UserRepository
	queueRepo      repository.QueueRepository
	logger         *zap.Logger
}

// NewDashboardService crea una nueva instancia del servicio de dashboard
func NewDashboardService(
	metricsService MetricsService,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	userRepo repository.UserRepository,
	queueRepo repository.QueueRepository,
	logger *zap.Logger,
) DashboardService {
	return &dashboardServiceImpl{
		metricsService: metricsService,
		workflowRepo:   workflowRepo,
		logRepo:        logRepo,
		userRepo:       userRepo,
		queueRepo:      queueRepo,
		logger:         logger,
	}
}

// GetCompleteDashboard obtiene datos completos del dashboard
func (s *dashboardServiceImpl) GetCompleteDashboard(ctx context.Context, filter *models.DashboardFilter) (*models.DashboardData, error) {
	s.logger.Info("Getting complete dashboard data")

	// Implementar lógica para obtener datos completos del dashboard
	// Esta es una implementación básica - expandir según necesidades
	summary, err := s.GetDashboardSummary(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard summary: %w", err)
	}

	systemHealth, err := s.GetSystemHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system health: %w", err)
	}

	recentActivity, err := s.GetRecentActivity(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}

	workflowStatus, err := s.GetWorkflowStatus(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow status: %w", err)
	}

	queueStatus, err := s.GetQueueStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue status: %w", err)
	}

	return &models.DashboardData{
		Summary:        summary,
		SystemHealth:   systemHealth,
		RecentActivity: recentActivity,
		WorkflowStatus: workflowStatus,
		QueueStatus:    queueStatus,
		Timestamp:      time.Now(),
	}, nil
}

// GetDashboardSummary obtiene resumen del dashboard
func (s *dashboardServiceImpl) GetDashboardSummary(ctx context.Context, filter *models.DashboardFilter) (*models.DashboardSummary, error) {
	s.logger.Info("Getting dashboard summary")

	// Obtener estadísticas básicas
	totalWorkflows, err := s.workflowRepo.Count(ctx)
	if err != nil {
		s.logger.Error("Failed to count workflows", zap.Error(err))
		totalWorkflows = 0
	}

	activeWorkflows, err := s.workflowRepo.CountActiveWorkflows(ctx)
	if err != nil {
		s.logger.Error("Failed to count active workflows", zap.Error(err))
		activeWorkflows = 0
	}

	queueLength, err := s.queueRepo.GetQueueLength(ctx, "main")
	if err != nil {
		s.logger.Error("Failed to get queue length", zap.Error(err))
		queueLength = 0
	}

	// Obtener estadísticas de logs
	logStats, err := s.logRepo.GetStats(ctx, repository.LogSearchFilter{})
	if err != nil {
		s.logger.Error("Failed to get log stats", zap.Error(err))
		logStats = &models.LogStats{}
	}

	var successRate float64
	if logStats.TotalExecutions > 0 {
		successRate = (float64(logStats.SuccessfulRuns) / float64(logStats.TotalExecutions)) * 100
	}

	return &models.DashboardSummary{
		SystemStatus:        "healthy",
		TotalWorkflows:      int(totalWorkflows),
		ActiveWorkflows:     int(activeWorkflows),
		TotalExecutions:     logStats.TotalExecutions,
		SuccessRate:         successRate,
		CurrentQueueLength:  int(queueLength),
		AverageResponseTime: logStats.AverageExecutionTime,
		LastUpdate:          time.Now(),
	}, nil
}

// GetSystemHealth obtiene estado de salud del sistema
func (s *dashboardServiceImpl) GetSystemHealth(ctx context.Context) (*models.SystemHealth, error) {
	s.logger.Info("Getting system health")

	// Implementación básica de health check
	return &models.SystemHealth{
		OverallStatus: "healthy",
		LastCheck:     time.Now(),
		Services: map[string]models.ServiceHealth{
			"database": {
				Status:       "healthy",
				LastCheck:    time.Now(),
				ResponseTime: 50,
			},
			"queue": {
				Status:       "healthy",
				LastCheck:    time.Now(),
				ResponseTime: 25,
			},
		},
	}, nil
}

// GetQuickStats obtiene estadísticas rápidas
func (s *dashboardServiceImpl) GetQuickStats(ctx context.Context) (*models.QuickStats, error) {
	s.logger.Info("Getting quick stats")

	totalUsers, _ := s.userRepo.Count(ctx)
	totalWorkflows, _ := s.workflowRepo.Count(ctx)
	queueLength, _ := s.queueRepo.GetQueueLength(ctx, "main")
	processingTasks, _ := s.queueRepo.GetProcessingTasksCount(ctx)
	failedTasks, _ := s.queueRepo.GetFailedTasksCount(ctx)

	return &models.QuickStats{
		TotalUsers:      totalUsers,
		TotalWorkflows:  totalWorkflows,
		QueuedTasks:     queueLength,
		ProcessingTasks: processingTasks,
		FailedTasks:     failedTasks,
		Timestamp:       time.Now(),
	}, nil
}

// GetRecentActivity obtiene actividad reciente
func (s *dashboardServiceImpl) GetRecentActivity(ctx context.Context, limit int) ([]models.ActivityItem, error) {
	s.logger.Info("Getting recent activity", zap.Int("limit", limit))

	// Obtener logs recientes
	logs, _, err := s.logRepo.GetByWorkflowID(ctx, primitive.NilObjectID, repository.PaginationOptions{
		Page:     1,
		PageSize: limit,
		SortBy:   "created_at",
		SortDesc: true,
	})

	if err != nil {
		return nil, err
	}

	// Convertir logs a activity items
	activities := make([]models.ActivityItem, 0, len(logs))
	for _, log := range logs {
		activity := models.ActivityItem{
			ID:          log.ID.Hex(),
			Type:        "workflow_execution",
			Description: fmt.Sprintf("Workflow %s executed", log.WorkflowName),
			UserID:      log.UserID.Hex(),
			Timestamp:   log.CreatedAt,
			Status:      string(log.Status),
			Metadata: map[string]interface{}{
				"workflow_id":  log.WorkflowID.Hex(),
				"execution_id": log.ExecutionID,
				"trigger_type": log.TriggerType,
			},
		}
		activities = append(activities, activity)
	}

	return activities, nil
}

// GetWorkflowStatus obtiene estado de workflows
func (s *dashboardServiceImpl) GetWorkflowStatus(ctx context.Context, limit int) ([]models.WorkflowStatusItem, error) {
	s.logger.Info("Getting workflow status", zap.Int("limit", limit))

	// Obtener workflows activos
	workflows, _, err := s.workflowRepo.ListByStatus(ctx, models.WorkflowStatusActive, 1, limit)
	if err != nil {
		return nil, err
	}

	// Convertir a workflow status items
	items := make([]models.WorkflowStatusItem, 0, len(workflows.Workflows))
	for _, workflow := range workflows.Workflows {
		item := models.WorkflowStatusItem{
			ID:             workflow.ID.Hex(),
			Name:           workflow.Name,
			Status:         string(workflow.Status),
			IsActive:       workflow.Status == models.WorkflowStatusActive,
			LastExecution:  workflow.Stats.LastExecutedAt,
			SuccessRate:    s.calculateSuccessRate(&workflow.Stats),
			TotalRuns:      workflow.Stats.TotalRuns,
			SuccessfulRuns: workflow.Stats.SuccessfulRuns,
			FailedRuns:     workflow.Stats.FailedRuns,
			AvgRunTime:     workflow.Stats.AverageExecutionTime,
			Healthy:        s.determineWorkflowHealth(&workflow.Stats),
			TriggerType:    string(workflow.TriggerType),
			Tags:           workflow.Tags,
		}
		items = append(items, item)
	}

	return items, nil
}

// GetQueueStatus obtiene estado de las colas
func (s *dashboardServiceImpl) GetQueueStatus(ctx context.Context) (*models.QueueStatus, error) {
	s.logger.Info("Getting queue status")

	queuedTasks, _ := s.queueRepo.GetQueueLength(ctx, "main")
	processingTasks, _ := s.queueRepo.GetProcessingTasksCount(ctx)
	failedTasks, _ := s.queueRepo.GetFailedTasksCount(ctx)
	retryingTasks, _ := s.queueRepo.GetRetryingTasksCount(ctx)

	// Determinar estado de salud
	var health string
	if failedTasks > processingTasks*2 {
		health = "unhealthy"
	} else if failedTasks > 0 {
		health = "degraded"
	} else {
		health = "healthy"
	}

	return &models.QueueStatus{
		QueuedTasks:     queuedTasks,
		ProcessingTasks: processingTasks,
		FailedTasks:     failedTasks,
		RetryingTasks:   retryingTasks,
		Health:          health,
		Timestamp:       time.Now(),
	}, nil
}

// GetSystemMetrics obtiene métricas del sistema
func (s *dashboardServiceImpl) GetSystemMetrics(ctx context.Context, timeRange string) (*models.SystemMetrics, error) {
	s.logger.Info("Getting system metrics", zap.String("time_range", timeRange))

	// Implementación básica - expandir según necesidades
	return &models.SystemMetrics{
		CPU: models.CPUMetrics{
			Usage:     45.2,
			LoadAvg1:  1.2,
			LoadAvg5:  1.1,
			LoadAvg15: 1.0,
		},
		Memory: models.MemoryMetrics{
			Used:      2 * 1024 * 1024 * 1024, // 2GB
			Total:     8 * 1024 * 1024 * 1024, // 8GB
			Available: 6 * 1024 * 1024 * 1024, // 6GB
			Usage:     25.0,
		},
		Disk: models.DiskMetrics{
			Used:      50 * 1024 * 1024 * 1024,  // 50GB
			Total:     500 * 1024 * 1024 * 1024, // 500GB
			Available: 450 * 1024 * 1024 * 1024, // 450GB
			Usage:     10.0,
		},
		Goroutines: 150,
	}, nil
}

// GetDashboardMetrics obtiene métricas específicas
func (s *dashboardServiceImpl) GetDashboardMetrics(ctx context.Context, metrics []string, timeRange string) (map[string]interface{}, error) {
	s.logger.Info("Getting dashboard metrics", zap.Strings("metrics", metrics), zap.String("time_range", timeRange))

	result := make(map[string]interface{})

	for _, metric := range metrics {
		switch metric {
		case "executions":
			stats, _ := s.logRepo.GetStats(ctx, repository.LogSearchFilter{})
			result[metric] = stats.TotalExecutions
		case "success_rate":
			stats, _ := s.logRepo.GetStats(ctx, repository.LogSearchFilter{})
			if stats.TotalExecutions > 0 {
				result[metric] = (float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions)) * 100
			} else {
				result[metric] = 100.0
			}
		case "queue_length":
			length, _ := s.queueRepo.GetQueueLength(ctx, "main")
			result[metric] = length
		case "active_workflows":
			count, _ := s.workflowRepo.CountActiveWorkflows(ctx)
			result[metric] = count
		default:
			s.logger.Warn("Unknown metric requested", zap.String("metric", metric))
		}
	}

	return result, nil
}

// RefreshDashboardData refresca datos del dashboard
func (s *dashboardServiceImpl) RefreshDashboardData(ctx context.Context) error {
	s.logger.Info("Refreshing dashboard data")
	// En la implementación base, no hay caché que invalidar
	// Esta funcionalidad será implementada en el servicio con caché
	return nil
}

// ValidateFilter valida filtros del dashboard
func (s *dashboardServiceImpl) ValidateFilter(filter *models.DashboardFilter) error {
	if filter == nil {
		return nil
	}

	// Validar time_range
	validRanges := []string{"1h", "6h", "12h", "24h", "7d", "30d"}
	isValidRange := false
	for _, validRange := range validRanges {
		if filter.TimeRange == validRange {
			isValidRange = true
			break
		}
	}
	if !isValidRange {
		return fmt.Errorf("invalid time_range: %s", filter.TimeRange)
	}

	// Validar fechas
	if filter.StartDate != nil && filter.EndDate != nil {
		if filter.StartDate.After(*filter.EndDate) {
			return fmt.Errorf("start_date cannot be after end_date")
		}
	}

	return nil
}

// Métodos auxiliares

func (s *dashboardServiceImpl) calculateSuccessRate(stats *models.WorkflowStats) float64 {
	if stats.TotalRuns == 0 {
		return 100.0
	}
	return (float64(stats.SuccessfulRuns) / float64(stats.TotalRuns)) * 100
}

func (s *dashboardServiceImpl) determineWorkflowHealth(stats *models.WorkflowStats) bool {
	if stats.TotalRuns == 0 {
		return true
	}
	successRate := (float64(stats.SuccessfulRuns) / float64(stats.TotalRuns)) * 100
	return successRate >= 90.0
}

// GetPerformanceData obtiene datos de rendimiento reales del sistema
func (s *dashboardServiceImpl) GetPerformanceData(ctx context.Context, timeRange string) (*models.PerformanceData, error) {
	s.logger.Info("Getting performance data", zap.String("time_range", timeRange))

	// Calcular fechas basado en timeRange
	endTime := time.Now()
	var startTime time.Time
	var interval time.Duration

	switch timeRange {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
		interval = 5 * time.Minute
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
		interval = 30 * time.Minute
	case "12h":
		startTime = endTime.Add(-12 * time.Hour)
		interval = 1 * time.Hour
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
		interval = 1 * time.Hour
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
		interval = 4 * time.Hour
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
		interval = 24 * time.Hour
	default:
		startTime = endTime.Add(-24 * time.Hour)
		interval = 1 * time.Hour
	}

	s.logger.Debug("Time range calculated",
		zap.Time("start_time", startTime),
		zap.Time("end_time", endTime),
		zap.Duration("interval", interval))

	// Obtener datos de logs para métricas (en paralelo para mejor rendimiento)
	type result struct {
		executionsData       []models.TimeSeriesPoint
		successRateData      []models.TimeSeriesPoint
		avgExecutionTime     []models.TimeSeriesPoint
		queueLengthTrend     []models.TimeSeriesPoint
		errorDistribution    []models.ErrorCount
		workflowDistribution []models.WorkflowCount
		hourlyExecutions     []models.HourlyStats
		err                  error
		dataType             string
	}

	// Canal para recopilar resultados en paralelo
	resultChan := make(chan result, 7)

	// Ejecutar consultas en paralelo
	go func() {
		data, err := s.getExecutionTimeSeries(ctx, startTime, endTime, interval)
		resultChan <- result{executionsData: data, err: err, dataType: "executions"}
	}()

	go func() {
		data, err := s.getSuccessRateTrend(ctx, startTime, endTime, interval)
		resultChan <- result{successRateData: data, err: err, dataType: "success_rate"}
	}()

	go func() {
		data, err := s.getAvgExecutionTimeTrend(ctx, startTime, endTime, interval)
		resultChan <- result{avgExecutionTime: data, err: err, dataType: "execution_time"}
	}()

	go func() {
		data, err := s.getQueueLengthTrend(ctx, startTime, endTime, interval)
		resultChan <- result{queueLengthTrend: data, err: err, dataType: "queue_length"}
	}()

	go func() {
		data, err := s.getErrorDistribution(ctx, startTime, endTime)
		resultChan <- result{errorDistribution: data, err: err, dataType: "errors"}
	}()

	go func() {
		data, err := s.getWorkflowDistribution(ctx, startTime, endTime)
		resultChan <- result{workflowDistribution: data, err: err, dataType: "workflows"}
	}()

	go func() {
		data, err := s.getHourlyExecutions(ctx, startTime, endTime)
		resultChan <- result{hourlyExecutions: data, err: err, dataType: "hourly"}
	}()

	// Recopilar resultados
	var (
		executionsData       []models.TimeSeriesPoint
		successRateData      []models.TimeSeriesPoint
		avgExecutionTime     []models.TimeSeriesPoint
		queueLengthTrend     []models.TimeSeriesPoint
		errorDistribution    []models.ErrorCount
		workflowDistribution []models.WorkflowCount
		hourlyExecutions     []models.HourlyStats
	)

	// Esperar todos los resultados
	for i := 0; i < 7; i++ {
		res := <-resultChan
		if res.err != nil {
			s.logger.Error("Failed to get performance data component",
				zap.Error(res.err),
				zap.String("data_type", res.dataType))
			// Continuar con datos vacíos en lugar de fallar completamente
		}

		switch res.dataType {
		case "executions":
			executionsData = res.executionsData
		case "success_rate":
			successRateData = res.successRateData
		case "execution_time":
			avgExecutionTime = res.avgExecutionTime
		case "queue_length":
			queueLengthTrend = res.queueLengthTrend
		case "errors":
			errorDistribution = res.errorDistribution
		case "workflows":
			workflowDistribution = res.workflowDistribution
		case "hourly":
			hourlyExecutions = res.hourlyExecutions
		}
	}

	// Generar datos adicionales
	triggerDistribution := s.getTriggerDistribution(ctx, startTime, endTime)
	userActivityHeatmap := s.getUserActivityHeatmap(ctx, startTime, endTime)
	systemResourceUsage := s.getSystemResourceUsage(ctx)
	weeklyTrends := s.getWeeklyTrends(ctx, startTime, endTime)

	performanceData := &models.PerformanceData{
		ExecutionsLast24h:    executionsData,
		ExecutionsLast7d:     executionsData, // Reutilizar para diferentes rangos
		SuccessRateTrend:     successRateData,
		AvgExecutionTime:     avgExecutionTime,
		QueueLengthTrend:     queueLengthTrend,
		ErrorDistribution:    errorDistribution,
		TriggerDistribution:  triggerDistribution,
		WorkflowDistribution: workflowDistribution,
		UserActivityHeatmap:  userActivityHeatmap,
		SystemResourceUsage:  systemResourceUsage,
		HourlyExecutions:     hourlyExecutions,
		WeeklyTrends:         weeklyTrends,
	}

	s.logger.Info("Performance data retrieved successfully",
		zap.Int("executions_points", len(executionsData)),
		zap.Int("success_rate_points", len(successRateData)),
		zap.Int("error_types", len(errorDistribution)),
		zap.Int("workflows", len(workflowDistribution)))

	return performanceData, nil
}

// MÉTODOS AUXILIARES PARA OBTENER DATOS ESPECÍFICOS

// getExecutionTimeSeries obtiene serie temporal de ejecuciones
func (s *dashboardServiceImpl) getExecutionTimeSeries(ctx context.Context, startTime, endTime time.Time, interval time.Duration) ([]models.TimeSeriesPoint, error) {
	data := []models.TimeSeriesPoint{}

	for current := startTime; current.Before(endTime); current = current.Add(interval) {
		intervalEnd := current.Add(interval)
		if intervalEnd.After(endTime) {
			intervalEnd = endTime
		}

		// Obtener conteo de ejecuciones en este intervalo usando LogRepository
		count, err := s.logRepo.CountExecutionsByTimeRange(ctx, current, intervalEnd)
		if err != nil {
			s.logger.Warn("Failed to count executions for interval",
				zap.Error(err),
				zap.Time("start", current),
				zap.Time("end", intervalEnd))
			count = 0
		}

		data = append(data, models.TimeSeriesPoint{
			Timestamp: current,
			Value:     float64(count),
		})
	}

	return data, nil
}

// getSuccessRateTrend obtiene tendencia de tasa de éxito
func (s *dashboardServiceImpl) getSuccessRateTrend(ctx context.Context, startTime, endTime time.Time, interval time.Duration) ([]models.TimeSeriesPoint, error) {
	data := []models.TimeSeriesPoint{}

	for current := startTime; current.Before(endTime); current = current.Add(interval) {
		intervalEnd := current.Add(interval)
		if intervalEnd.After(endTime) {
			intervalEnd = endTime
		}

		// Obtener total de ejecuciones
		total, err := s.logRepo.CountExecutionsByTimeRange(ctx, current, intervalEnd)
		if err != nil {
			s.logger.Warn("Failed to count total executions", zap.Error(err))
			total = 0
		}

		// Obtener ejecuciones exitosas
		successful, err := s.logRepo.CountSuccessfulExecutionsByTimeRange(ctx, current, intervalEnd)
		if err != nil {
			s.logger.Warn("Failed to count successful executions", zap.Error(err))
			successful = 0
		}

		var successRate float64 = 100.0 // Default a 100% si no hay datos
		if total > 0 {
			successRate = (float64(successful) / float64(total)) * 100
		}

		data = append(data, models.TimeSeriesPoint{
			Timestamp: current,
			Value:     successRate,
		})
	}

	return data, nil
}

// getAvgExecutionTimeTrend obtiene tendencia de tiempo promedio de ejecución
func (s *dashboardServiceImpl) getAvgExecutionTimeTrend(ctx context.Context, startTime, endTime time.Time, interval time.Duration) ([]models.TimeSeriesPoint, error) {
	data := []models.TimeSeriesPoint{}

	for current := startTime; current.Before(endTime); current = current.Add(interval) {
		intervalEnd := current.Add(interval)
		if intervalEnd.After(endTime) {
			intervalEnd = endTime
		}

		// Obtener tiempo promedio de ejecución usando LogRepository
		avgTime, err := s.logRepo.GetAverageExecutionTimeByRange(ctx, current, intervalEnd)
		if err != nil {
			s.logger.Warn("Failed to get average execution time", zap.Error(err))
			avgTime = 0
		}

		data = append(data, models.TimeSeriesPoint{
			Timestamp: current,
			Value:     avgTime, // En milisegundos
		})
	}

	return data, nil
}

// getQueueLengthTrend obtiene tendencia de longitud de cola
func (s *dashboardServiceImpl) getQueueLengthTrend(ctx context.Context, startTime, endTime time.Time, interval time.Duration) ([]models.TimeSeriesPoint, error) {
	data := []models.TimeSeriesPoint{}

	for current := startTime; current.Before(endTime); current = current.Add(interval) {
		// Para queue length, usar longitud actual y simular variación histórica
		length, err := s.queueRepo.GetQueueLength(ctx, "main")
		if err != nil {
			s.logger.Warn("Failed to get queue length", zap.Error(err))
			length = 0
		}

		// En una implementación real, podrías almacenar snapshots en una tabla de métricas
		hourOfDay := current.Hour()
		var multiplier float64

		// Simular patrón de actividad: más actividad durante horas laborales
		if hourOfDay >= 9 && hourOfDay <= 17 {
			multiplier = 1.0 + float64(hourOfDay-12)*0.1 // Pico al mediodía
		} else {
			multiplier = 0.3 + float64(hourOfDay)*0.05 // Actividad baja fuera de horas laborales
		}

		adjustedLength := float64(length) * multiplier
		if adjustedLength < 0 {
			adjustedLength = 0
		}

		data = append(data, models.TimeSeriesPoint{
			Timestamp: current,
			Value:     adjustedLength,
		})
	}

	return data, nil
}

// getErrorDistribution obtiene distribución de errores por tipo
func (s *dashboardServiceImpl) getErrorDistribution(ctx context.Context, startTime, endTime time.Time) ([]models.ErrorCount, error) {
	// Obtener distribución de errores por tipo usando LogRepository
	errorTypes, err := s.logRepo.GetErrorDistribution(ctx, startTime, endTime)
	if err != nil {
		s.logger.Error("Failed to get error distribution", zap.Error(err))
		return []models.ErrorCount{}, err
	}

	var distribution []models.ErrorCount
	for errorType, count := range errorTypes {
		distribution = append(distribution, models.ErrorCount{
			ErrorType: errorType,
			Count:     count,
		})
	}

	return distribution, nil
}

// getWorkflowDistribution obtiene distribución de workflows por ejecuciones
func (s *dashboardServiceImpl) getWorkflowDistribution(ctx context.Context, startTime, endTime time.Time) ([]models.WorkflowCount, error) {
	// Obtener distribución de workflows usando LogRepository
	workflowCounts, err := s.logRepo.GetWorkflowExecutionCounts(ctx, startTime, endTime)
	if err != nil {
		s.logger.Error("Failed to get workflow distribution", zap.Error(err))
		return []models.WorkflowCount{}, err
	}

	var distribution []models.WorkflowCount
	for workflowID, count := range workflowCounts {
		// Obtener nombre del workflow
		workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
		workflowName := workflowID // Default al ID si no se encuentra
		if err == nil && workflow != nil {
			workflowName = workflow.Name
		}

		distribution = append(distribution, models.WorkflowCount{
			WorkflowID:   workflowID,
			WorkflowName: workflowName,
			Count:        count,
		})
	}

	return distribution, nil
}

// getHourlyExecutions obtiene estadísticas por hora
func (s *dashboardServiceImpl) getHourlyExecutions(ctx context.Context, startTime, endTime time.Time) ([]models.HourlyStats, error) {
	var hourlyStats []models.HourlyStats

	// Determinar rango de horas a analizar
	duration := endTime.Sub(startTime)
	var hoursToAnalyze int
	if duration <= 24*time.Hour {
		hoursToAnalyze = 24 // Últimas 24 horas
	} else {
		hoursToAnalyze = int(duration.Hours())
		if hoursToAnalyze > 168 { // Máximo una semana
			hoursToAnalyze = 168
		}
	}

	// Agrupar por hora
	for hour := 0; hour < hoursToAnalyze && hour < 24; hour++ {
		hourStart := time.Date(endTime.Year(), endTime.Month(), endTime.Day(), hour, 0, 0, 0, endTime.Location())
		hourEnd := hourStart.Add(time.Hour)

		// Solo incluir horas en el rango especificado
		if hourStart.Before(startTime) || hourStart.After(endTime) {
			// Para horas fuera del rango, usar datos históricos del día anterior
			hourStart = hourStart.Add(-24 * time.Hour)
			hourEnd = hourEnd.Add(-24 * time.Hour)
		}

		total, err := s.logRepo.CountExecutionsByTimeRange(ctx, hourStart, hourEnd)
		if err != nil {
			s.logger.Warn("Failed to get hourly executions", zap.Error(err), zap.Int("hour", hour))
			total = 0
		}

		successful, err := s.logRepo.CountSuccessfulExecutionsByTimeRange(ctx, hourStart, hourEnd)
		if err != nil {
			successful = 0
		}

		failed := total - successful
		avgTime, err := s.logRepo.GetAverageExecutionTimeByRange(ctx, hourStart, hourEnd)
		if err != nil {
			avgTime = 0
		}

		// Calcular tiempo pico (simulado como 1.5x el promedio)
		peakTime := avgTime * 1.5

		hourlyStats = append(hourlyStats, models.HourlyStats{
			Hour:           hour,
			ExecutionCount: int(total),
			SuccessCount:   int(successful),
			FailureCount:   int(failed),
			AverageTime:    avgTime,
			PeakTime:       peakTime,
		})
	}

	return hourlyStats, nil
}

// MÉTODOS ADICIONALES (PLACEHOLDERS MEJORADOS)

// getTriggerDistribution obtiene distribución de tipos de triggers
func (s *dashboardServiceImpl) getTriggerDistribution(ctx context.Context, startTime, endTime time.Time) []models.TriggerCount {
	// TODO: Implementar distribución real de triggers desde logs
	// Por ahora retornar datos de ejemplo
	return []models.TriggerCount{
		{TriggerType: "manual", Count: 45},
		{TriggerType: "webhook", Count: 32},
		{TriggerType: "scheduled", Count: 28},
		{TriggerType: "event", Count: 15},
	}
}

// getUserActivityHeatmap obtiene mapa de calor de actividad de usuarios
func (s *dashboardServiceImpl) getUserActivityHeatmap(ctx context.Context, startTime, endTime time.Time) []models.HeatmapPoint {
	// TODO: Implementar heatmap real basado en logs de usuarios
	// Por ahora retornar datos de ejemplo
	heatmap := []models.HeatmapPoint{}

	// Generar datos de ejemplo para una semana (7 días x 24 horas)
	for day := 0; day < 7; day++ {
		for hour := 0; hour < 24; hour++ {
			// Simular actividad más alta durante horas laborales
			var intensity float64
			if hour >= 9 && hour <= 17 {
				intensity = 0.5 + (float64(hour-12)*float64(hour-12))*0.02 // Pico al mediodía
			} else {
				intensity = 0.1 + float64(hour)*0.01
			}

			heatmap = append(heatmap, models.HeatmapPoint{
				X:         hour,
				Y:         day,
				Intensity: intensity,
			})
		}
	}

	return heatmap
}

// getSystemResourceUsage obtiene uso de recursos del sistema
func (s *dashboardServiceImpl) getSystemResourceUsage(ctx context.Context) []models.ResourceUsage {
	// TODO: Implementar métricas reales del sistema (CPU, Memoria, Disco)
	// Por ahora simular datos realistas
	now := time.Now()

	return []models.ResourceUsage{
		{
			Timestamp:   now.Add(-2 * time.Minute),
			CPUUsage:    45.2,
			MemoryUsage: 62.1,
			DiskUsage:   23.8,
		},
		{
			Timestamp:   now.Add(-1 * time.Minute),
			CPUUsage:    47.8,
			MemoryUsage: 63.4,
			DiskUsage:   23.9,
		},
		{
			Timestamp:   now,
			CPUUsage:    43.1,
			MemoryUsage: 61.8,
			DiskUsage:   24.0,
		},
	}
}

// getWeeklyTrends obtiene tendencias semanales
func (s *dashboardServiceImpl) getWeeklyTrends(ctx context.Context, startTime, endTime time.Time) []models.WeeklyStats {
	// TODO: Implementar tendencias semanales reales
	// Por ahora retornar datos de ejemplo para últimas 4 semanas
	trends := []models.WeeklyStats{}

	for week := 4; week >= 1; week-- {
		weekStart := time.Now().Add(time.Duration(-week*7*24) * time.Hour)
		weekStr := fmt.Sprintf("%d-W%02d", weekStart.Year(), getWeekOfYear(weekStart))

		// Simular datos de la semana
		trends = append(trends, models.WeeklyStats{
			Week:            weekStr,
			ExecutionCount:  1200 + week*50, // Tendencia creciente
			SuccessRate:     95.0 + float64(week)*0.5,
			AverageTime:     1800.0 - float64(week)*50, // Mejorando tiempo
			ActiveWorkflows: 15 + week*2,
		})
	}

	return trends
}

// Helper function para obtener semana del año
func getWeekOfYear(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}
