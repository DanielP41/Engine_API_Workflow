package services

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
)

// DashboardService define la interfaz para el servicio del dashboard
type DashboardService interface {
	// Datos completos del dashboard
	GetCompleteDashboard(ctx context.Context, filter *models.DashboardFilter) (*models.DashboardData, error)

	// Resumen ejecutivo
	GetDashboardSummary(ctx context.Context) (*models.DashboardSummary, error)

	// Estado de salud del sistema
	GetSystemHealth(ctx context.Context) (*models.SystemHealth, error)

	// Métricas específicas
	GetDashboardMetrics(ctx context.Context, metrics []string, timeRange string) (map[string]interface{}, error)

	// Alertas activas
	GetActiveAlerts(ctx context.Context) ([]models.Alert, error)

	// Salud de workflow específico
	GetWorkflowHealth(ctx context.Context, workflowID string) (*models.WorkflowStatusItem, error)

	// Refrescar datos del dashboard
	RefreshDashboardData(ctx context.Context) error

	// Validar filtros
	ValidateFilter(filter *models.DashboardFilter) error
}

// dashboardService implementa DashboardService
type dashboardService struct {
	metricsService MetricsService
	workflowRepo   repository.WorkflowRepository
	logRepo        repository.LogRepository
	userRepo       repository.UserRepository
	queueRepo      repository.QueueRepository
}

// NewDashboardService crea una nueva instancia del servicio dashboard
func NewDashboardService(
	metricsService MetricsService,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	userRepo repository.UserRepository,
	queueRepo repository.QueueRepository,
) DashboardService {
	return &dashboardService{
		metricsService: metricsService,
		workflowRepo:   workflowRepo,
		logRepo:        logRepo,
		userRepo:       userRepo,
		queueRepo:      queueRepo,
	}
}

// GetCompleteDashboard obtiene todos los datos del dashboard
func (s *dashboardService) GetCompleteDashboard(ctx context.Context, filter *models.DashboardFilter) (*models.DashboardData, error) {
	// Obtener todos los componentes del dashboard
	systemHealth, err := s.GetSystemHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system health: %w", err)
	}

	quickStats, err := s.getQuickStats(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get quick stats: %w", err)
	}

	recentActivity, err := s.getRecentActivity(ctx, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}

	workflowStatus, err := s.getWorkflowStatus(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow status: %w", err)
	}

	queueStatus, err := s.getQueueStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue status: %w", err)
	}

	performanceData, err := s.getPerformanceData(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance data: %w", err)
	}

	alerts, err := s.GetActiveAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts: %w", err)
	}

	return &models.DashboardData{
		SystemHealth:    *systemHealth,
		QuickStats:      *quickStats,
		RecentActivity:  recentActivity,
		WorkflowStatus:  workflowStatus,
		QueueStatus:     *queueStatus,
		PerformanceData: *performanceData,
		Alerts:          alerts,
		LastUpdated:     time.Now(),
	}, nil
}

// GetDashboardSummary obtiene un resumen ejecutivo
func (s *dashboardService) GetDashboardSummary(ctx context.Context) (*models.DashboardSummary, error) {
	// Contar workflows totales y activos
	totalWorkflows, err := s.workflowRepo.CountWorkflows(ctx)
	if err != nil {
		totalWorkflows = 0
	}

	activeWorkflows, err := s.workflowRepo.CountActiveWorkflows(ctx)
	if err != nil {
		activeWorkflows = 0
	}

	// Estadísticas de logs
	logStats, err := s.logRepo.GetStats(ctx, repository.LogSearchFilter{})
	if err != nil {
		logStats = &models.LogStats{}
	}

	// Estado de la cola
	queueLength, err := s.queueRepo.GetQueueLength(ctx)
	if err != nil {
		queueLength = 0
	}

	// Calcular tasa de éxito
	var successRate float64
	if logStats.TotalExecutions > 0 {
		successRate = (float64(logStats.SuccessfulExecutions) / float64(logStats.TotalExecutions)) * 100
	}

	// Obtener alertas críticas
	alerts, _ := s.GetActiveAlerts(ctx)
	criticalAlerts := 0
	warningAlerts := 0
	for _, alert := range alerts {
		if alert.Severity == "critical" {
			criticalAlerts++
		} else if alert.Severity == "warning" {
			warningAlerts++
		}
	}

	// Determinar estado del sistema
	systemStatus := "healthy"
	if criticalAlerts > 0 || successRate < 80 {
		systemStatus = "critical"
	} else if warningAlerts > 0 || successRate < 90 {
		systemStatus = "warning"
	}

	return &models.DashboardSummary{
		SystemStatus:        systemStatus,
		TotalWorkflows:      int(totalWorkflows),
		ActiveWorkflows:     int(activeWorkflows),
		TotalExecutions:     logStats.TotalExecutions,
		SuccessRate:         successRate,
		CurrentQueueLength:  int(queueLength),
		AverageResponseTime: logStats.AverageExecutionTime,
		LastUpdate:          time.Now(),
		CriticalAlerts:      criticalAlerts,
		WarningAlerts:       warningAlerts,
	}, nil
}

// GetSystemHealth obtiene el estado de salud del sistema
func (s *dashboardService) GetSystemHealth(ctx context.Context) (*models.SystemHealth, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memoria en MB
	memoryUsage := float64(m.Alloc) / 1024 / 1024

	// Verificar conexiones de base de datos
	dbConnections := 10 // Placeholder - esto debería venir de un pool de conexiones real

	// Verificar Redis
	redisConnected := true
	if err := s.queueRepo.Ping(ctx); err != nil {
		redisConnected = false
	}

	// Determinar estado general
	status := "healthy"
	if !redisConnected || memoryUsage > 1000 { // 1GB
		status = "critical"
	} else if memoryUsage > 500 { // 500MB
		status = "warning"
	}

	return &models.SystemHealth{
		Status:          status,
		Uptime:          s.getUptime(),
		LastHealthy:     time.Now(),
		Version:         "1.0.0",
		Environment:     "development", // Esto debería venir de configuración
		DBConnections:   dbConnections,
		RedisConnected:  redisConnected,
		APIResponseTime: 50.0, // Placeholder
		MemoryUsage:     memoryUsage,
		CPUUsage:        0.0, // Placeholder - requiere bibliotecas adicionales
	}, nil
}

// GetDashboardMetrics obtiene métricas específicas
func (s *dashboardService) GetDashboardMetrics(ctx context.Context, metrics []string, timeRange string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, metric := range metrics {
		switch metric {
		case "executions":
			stats, err := s.logRepo.GetStats(ctx, repository.LogSearchFilter{})
			if err != nil {
				result[metric] = 0
			} else {
				result[metric] = stats.TotalExecutions
			}

		case "success_rate":
			stats, err := s.logRepo.GetStats(ctx, repository.LogSearchFilter{})
			if err != nil {
				result[metric] = 0.0
			} else {
				if stats.TotalExecutions > 0 {
					result[metric] = (float64(stats.SuccessfulExecutions) / float64(stats.TotalExecutions)) * 100
				} else {
					result[metric] = 0.0
				}
			}

		case "queue_length":
			length, err := s.queueRepo.GetQueueLength(ctx)
			if err != nil {
				result[metric] = 0
			} else {
				result[metric] = length
			}

		case "active_workflows":
			count, err := s.workflowRepo.CountActiveWorkflows(ctx)
			if err != nil {
				result[metric] = 0
			} else {
				result[metric] = count
			}

		default:
			result[metric] = nil
		}
	}

	return result, nil
}

// GetActiveAlerts obtiene alertas activas del sistema
func (s *dashboardService) GetActiveAlerts(ctx context.Context) ([]models.Alert, error) {
	alerts := []models.Alert{}

	// Verificar métricas del sistema para generar alertas
	systemHealth, err := s.GetSystemHealth(ctx)
	if err == nil {
		// Alerta de memoria alta
		if systemHealth.MemoryUsage > 800 {
			alerts = append(alerts, models.Alert{
				ID:        primitive.NewObjectID().Hex(),
				Type:      "system",
				Severity:  "critical",
				Title:     "High Memory Usage",
				Message:   fmt.Sprintf("Memory usage is at %.1f MB", systemHealth.MemoryUsage),
				Timestamp: time.Now(),
				IsActive:  true,
			})
		}

		// Alerta de Redis desconectado
		if !systemHealth.RedisConnected {
			alerts = append(alerts, models.Alert{
				ID:        primitive.NewObjectID().Hex(),
				Type:      "system",
				Severity:  "critical",
				Title:     "Redis Connection Lost",
				Message:   "Redis connection is not available",
				Timestamp: time.Now(),
				IsActive:  true,
			})
		}
	}

	// Verificar cola
	queueLength, err := s.queueRepo.GetQueueLength(ctx)
	if err == nil && queueLength > 100 {
		alerts = append(alerts, models.Alert{
			ID:        primitive.NewObjectID().Hex(),
			Type:      "queue",
			Severity:  "warning",
			Title:     "High Queue Length",
			Message:   fmt.Sprintf("Queue has %d pending tasks", queueLength),
			Timestamp: time.Now(),
			IsActive:  true,
		})
	}

	return alerts, nil
}

// GetWorkflowHealth obtiene la salud de un workflow específico
func (s *dashboardService) GetWorkflowHealth(ctx context.Context, workflowID string) (*models.WorkflowStatusItem, error) {
	// Convertir ID a ObjectID
	objID, err := primitive.ObjectIDFromHex(workflowID)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow ID: %w", err)
	}

	// Obtener workflow
	workflow, err := s.workflowRepo.GetByID(ctx, objID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Obtener estadísticas del workflow
	stats, err := s.logRepo.GetWorkflowStats(ctx, objID, 30)
	if err != nil {
		stats = &models.LogStats{} // Stats vacías por defecto
	}

	// Calcular tasa de éxito
	var successRate float64
	if stats.TotalExecutions > 0 {
		successRate = (float64(stats.SuccessfulExecutions) / float64(stats.TotalExecutions)) * 100
	}

	// Determinar estado de salud
	status := "active"
	if !workflow.IsActive() {
		status = "inactive"
	} else if successRate < 80 {
		status = "error"
	} else if successRate < 95 {
		status = "warning"
	}

	return &models.WorkflowStatusItem{
		ID:             workflow.ID.Hex(),
		Name:           workflow.Name,
		Status:         status,
		IsActive:       workflow.IsActive(),
		LastExecution:  stats.LastExecutedAt,
		SuccessRate:    successRate,
		TotalRuns:      stats.TotalExecutions,
		SuccessfulRuns: stats.SuccessfulExecutions,
		FailedRuns:     stats.FailedExecutions,
		AvgRunTime:     stats.AverageExecutionTime,
		IsHealthy:      successRate >= 90 && workflow.IsActive(),
		ErrorRate:      100 - successRate,
		TriggerType:    s.getTriggerType(workflow),
		Priority:       workflow.Priority,
		Environment:    workflow.Environment,
		Tags:           workflow.Tags,
		CreatedBy:      workflow.CreatedBy.Hex(),
	}, nil
}

// RefreshDashboardData refresca los datos del dashboard
func (s *dashboardService) RefreshDashboardData(ctx context.Context) error {
	// Por ahora, esto es un placeholder
	// En el futuro podríamos limpiar caches, recalcular métricas, etc.
	return nil
}

// ValidateFilter valida los filtros del dashboard
func (s *dashboardService) ValidateFilter(filter *models.DashboardFilter) error {
	if filter == nil {
		return fmt.Errorf("filter cannot be nil")
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

	// Validar ObjectIDs en workflow_ids
	for _, id := range filter.WorkflowIDs {
		if _, err := primitive.ObjectIDFromHex(id); err != nil {
			return fmt.Errorf("invalid workflow ID: %s", id)
		}
	}

	// Validar ObjectIDs en user_ids
	for _, id := range filter.UserIDs {
		if _, err := primitive.ObjectIDFromHex(id); err != nil {
			return fmt.Errorf("invalid user ID: %s", id)
		}
	}

	return nil
}

// Métodos helper privados

// getQuickStats obtiene estadísticas rápidas
func (s *dashboardService) getQuickStats(ctx context.Context, filter *models.DashboardFilter) (*models.QuickStats, error) {
	// Workflows activos
	activeWorkflows, err := s.workflowRepo.CountActiveWorkflows(ctx)
	if err != nil {
		activeWorkflows = 0
	}

	// Total de usuarios
	totalUsers, err := s.userRepo.CountUsers(ctx)
	if err != nil {
		totalUsers = 0
	}

	// Estadísticas de logs
	logStats, err := s.logRepo.GetStats(ctx, repository.LogSearchFilter{})
	if err != nil {
		logStats = &models.LogStats{}
	}

	// Cola
	queueLength, err := s.queueRepo.GetQueueLength(ctx)
	if err != nil {
		queueLength = 0
	}

	// Calcular tasa de éxito 24h
	var successRate24h float64
	if logStats.TotalExecutions > 0 {
		successRate24h = (float64(logStats.SuccessfulExecutions) / float64(logStats.TotalExecutions)) * 100
	}

	return &models.QuickStats{
		ActiveWorkflows:     int(activeWorkflows),
		TotalUsers:          int(totalUsers),
		RunningExecutions:   0, // Placeholder
		QueueLength:         int(queueLength),
		SuccessRate24h:      successRate24h,
		AvgExecutionTime:    logStats.AverageExecutionTime,
		ExecutionsToday:     int(logStats.ExecutionsToday),
		ExecutionsThisWeek:  int(logStats.ExecutionsThisWeek),
		ExecutionsThisMonth: int(logStats.ExecutionsThisMonth),
		ErrorsLast24h:       int(logStats.ErrorsLast24h),
		TotalExecutions:     logStats.TotalExecutions,
		TotalWorkflows:      int(activeWorkflows), // Usar activos por ahora
	}, nil
}

// getRecentActivity obtiene actividad reciente
func (s *dashboardService) getRecentActivity(ctx context.Context, limit int) ([]models.ActivityItem, error) {
	// Obtener logs recientes
	logs, _, err := s.logRepo.SearchLogs(ctx, repository.LogSearchFilter{}, repository.PaginationOptions{
		Page:     1,
		PageSize: limit,
		SortBy:   "created_at",
	})
	if err != nil {
		return []models.ActivityItem{}, nil
	}

	// Convertir logs a items de actividad
	activity := make([]models.ActivityItem, 0, len(logs))
	for _, log := range logs {
		item := models.ActivityItem{
			ID:           log.ID.Hex(),
			Type:         "execution",
			WorkflowID:   log.WorkflowID.Hex(),
			WorkflowName: log.WorkflowName,
			UserID:       log.UserID.Hex(),
			Message:      fmt.Sprintf("Workflow %s executed", log.WorkflowName),
			Timestamp:    log.CreatedAt,
			Status:       log.Status,
			Duration:     &log.ExecutionTime,
		}

		// Determinar severidad basada en el estado
		switch log.Status {
		case "completed":
			item.Severity = "low"
		case "failed":
			item.Severity = "high"
		case "running":
			item.Severity = "medium"
		default:
			item.Severity = "low"
		}

		activity = append(activity, item)
	}

	return activity, nil
}

// getWorkflowStatus obtiene estado de workflows
func (s *dashboardService) getWorkflowStatus(ctx context.Context, limit int) ([]models.WorkflowStatusItem, error) {
	// Obtener workflows activos
	workflows, err := s.workflowRepo.GetActiveWorkflows(ctx)
	if err != nil {
		return []models.WorkflowStatusItem{}, nil
	}

	// Limitar resultados
	if len(workflows) > limit {
		workflows = workflows[:limit]
	}

	// Convertir a WorkflowStatusItem
	status := make([]models.WorkflowStatusItem, 0, len(workflows))
	for _, workflow := range workflows {
		if workflow == nil {
			continue
		}

		// Obtener estadísticas del workflow
		stats, err := s.logRepo.GetWorkflowStats(ctx, workflow.ID, 30)
		if err != nil {
			stats = &models.LogStats{}
		}

		var successRate float64
		if stats.TotalExecutions > 0 {
			successRate = (float64(stats.SuccessfulExecutions) / float64(stats.TotalExecutions)) * 100
		}

		item := models.WorkflowStatusItem{
			ID:             workflow.ID.Hex(),
			Name:           workflow.Name,
			Status:         "active",
			IsActive:       workflow.IsActive(),
			LastExecution:  stats.LastExecutedAt,
			SuccessRate:    successRate,
			TotalRuns:      stats.TotalExecutions,
			SuccessfulRuns: stats.SuccessfulExecutions,
			FailedRuns:     stats.FailedExecutions,
			AvgRunTime:     stats.AverageExecutionTime,
			IsHealthy:      successRate >= 90,
			ErrorRate:      100 - successRate,
			TriggerType:    s.getTriggerType(*workflow),
			Priority:       workflow.Priority,
			Environment:    workflow.Environment,
			Tags:           workflow.Tags,
			CreatedBy:      workflow.CreatedBy.Hex(),
		}

		status = append(status, item)
	}

	return status, nil
}

// getQueueStatus obtiene estado de la cola
func (s *dashboardService) getQueueStatus(ctx context.Context) (*models.QueueStatus, error) {
	queueLength, err := s.queueRepo.GetQueueLength(ctx)
	if err != nil {
		queueLength = 0
	}

	// Por ahora retornamos datos básicos
	// En el futuro esto debería obtener métricas reales de la cola
	return &models.QueueStatus{
		Pending:            int(queueLength),
		Processing:         0, // Placeholder
		Failed:             0, // Placeholder
		Completed:          0, // Placeholder
		DelayedJobs:        0, // Placeholder
		RetryJobs:          0, // Placeholder
		ByPriority:         make(map[string]int),
		AverageWaitTime:    0.0,
		AverageProcessTime: 0.0,
		ProcessingRate:     0.0,
		ThroughputLast24h:  0,
		QueueHealth:        "healthy",
		BacklogSize:        int(queueLength),
		WorkersActive:      1,
		WorkersTotal:       1,
	}, nil
}

// getPerformanceData obtiene datos de rendimiento
func (s *dashboardService) getPerformanceData(ctx context.Context, filter *models.DashboardFilter) (*models.PerformanceData, error) {
	// Por ahora retornamos datos vacíos
	// En el futuro esto debería generar series temporales reales
	return &models.PerformanceData{
		ExecutionsLast24h:    []models.TimeSeriesPoint{},
		ExecutionsLast7d:     []models.TimeSeriesPoint{},
		SuccessRateTrend:     []models.TimeSeriesPoint{},
		AvgExecutionTime:     []models.TimeSeriesPoint{},
		QueueLengthTrend:     []models.TimeSeriesPoint{},
		ErrorDistribution:    []models.ErrorCount{},
		TriggerDistribution:  []models.TriggerCount{},
		WorkflowDistribution: []models.WorkflowCount{},
		UserActivityHeatmap:  []models.HeatmapPoint{},
		SystemResourceUsage:  []models.ResourceUsage{},
		HourlyExecutions:     []models.HourlyStats{},
		WeeklyTrends:         []models.WeeklyStats{},
	}, nil
}

// getTriggerType obtiene el tipo de trigger principal del workflow
func (s *dashboardService) getTriggerType(workflow models.Workflow) string {
	if len(workflow.Triggers) == 0 {
		return "manual"
	}
	return workflow.Triggers[0].Type
}

// getUptime calcula el uptime del sistema
func (s *dashboardService) getUptime() string {
	// Placeholder - en el futuro esto debería ser el tiempo real de uptime
	return "1 day, 2 hours"
}
