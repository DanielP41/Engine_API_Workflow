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
	// Esta es una implementación básica
	summary, err := s.GetDashboardSummary(ctx, filter)
	if err != nil {
		return nil, err
	}

	quickStats, err := s.GetQuickStats(ctx)
	if err != nil {
		return nil, err
	}

	systemMetrics, err := s.GetSystemMetrics(ctx, "1h")
	if err != nil {
		return nil, err
	}

	recentActivity, err := s.GetRecentActivity(ctx, 10)
	if err != nil {
		return nil, err
	}

	workflowStatus, err := s.GetWorkflowStatus(ctx, 10)
	if err != nil {
		return nil, err
	}

	queueStatus, err := s.GetQueueStatus(ctx)
	if err != nil {
		return nil, err
	}

	systemHealth, err := s.GetSystemHealth(ctx)
	if err != nil {
		return nil, err
	}

	return &models.DashboardData{
		Summary:        summary,
		QuickStats:     quickStats,
		SystemMetrics:  systemMetrics,
		RecentActivity: recentActivity,
		WorkflowStatus: workflowStatus,
		QueueStatus:    queueStatus,
		SystemHealth:   systemHealth,
		Alerts:         []models.Alert{}, // Implementar alertas según necesidades
		LastUpdated:    time.Now(),
	}, nil
}

// GetDashboardSummary obtiene resumen del dashboard
func (s *dashboardServiceImpl) GetDashboardSummary(ctx context.Context, filter *models.DashboardFilter) (*models.DashboardSummary, error) {
	s.logger.Info("Getting dashboard summary")

	// Implementación básica - expandir según necesidades
	totalWorkflows, _ := s.workflowRepo.Count(ctx)
	activeWorkflows, _ := s.workflowRepo.CountActiveWorkflows(ctx)
	queueLength, _ := s.queueRepo.GetQueueLength(ctx, "main")
	processingTasks, _ := s.queueRepo.GetProcessingTasksCount(ctx)

	return &models.DashboardSummary{
		TotalWorkflows:  int(totalWorkflows),
		ActiveWorkflows: int(activeWorkflows),
		QueueLength:     queueLength,
		ProcessingTasks: processingTasks,
		LastUpdated:     time.Now(),
	}, nil
}

// GetWorkflowStatus obtiene estado de workflows
func (s *dashboardServiceImpl) GetWorkflowStatus(ctx context.Context, limit int) ([]models.WorkflowStatusItem, error) {
	s.logger.Info("Getting workflow status", zap.Int("limit", limit))

	workflows, err := s.workflowRepo.ListByStatus(ctx, models.WorkflowStatusActive, 1, limit)
	if err != nil {
		return nil, err
	}

	items := make([]models.WorkflowStatusItem, 0, len(workflows.Workflows))
	for _, workflow := range workflows.Workflows {
		item := models.WorkflowStatusItem{
			ID:             workflow.ID.Hex(),
			Name:           workflow.Name,
			Status:         string(workflow.Status),
			IsActive:       workflow.Status == models.WorkflowStatusActive,
			LastExecution:  s.getLastExecutionFromStats(workflow.Stats),
			SuccessRate:    s.calculateSuccessRateFromStats(workflow.Stats),
			TotalRuns:      s.getTotalExecutionsFromStats(workflow.Stats),      // CORREGIDO: usar TotalExecutions
			SuccessfulRuns: s.getSuccessfulExecutionsFromStats(workflow.Stats), // CORREGIDO: método helper
			FailedRuns:     s.getFailedExecutionsFromStats(workflow.Stats),     // CORREGIDO: método helper
			AvgRunTime:     s.getAverageExecutionTimeFromStats(workflow.Stats), // CORREGIDO: método helper
			Healthy:        s.determineWorkflowHealthFromStats(workflow.Stats), // CORREGIDO: método helper
			TriggerType:    s.getTriggerTypeFromWorkflow(workflow),             // CORREGIDO: método helper
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

	status := &models.QueueStatus{
		QueuedTasks:     queuedTasks,
		ProcessingTasks: processingTasks,
		FailedTasks:     failedTasks,
		RetryingTasks:   retryingTasks,
		Health:          health,
		Timestamp:       time.Now(),

		// Mantener compatibilidad con campos alternativos
		Pending:     int(queuedTasks),
		Processing:  int(processingTasks),
		Failed:      int(failedTasks),
		RetryJobs:   int(retryingTasks),
		QueueHealth: health,
	}

	return status, nil
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
		Goroutines:  150,
		LastUpdated: time.Now(),
	}, nil
}

// GetDashboardMetrics obtiene métricas específicas
func (s *dashboardServiceImpl) GetDashboardMetrics(ctx context.Context, metrics []string, timeRange string) (map[string]interface{}, error) {
	s.logger.Info("Getting dashboard metrics", zap.Strings("metrics", metrics), zap.String("time_range", timeRange))

	result := make(map[string]interface{})

	for _, metric := range metrics {
		switch metric {
		case "total_workflows":
			count, _ := s.workflowRepo.Count(ctx)
			result[metric] = count
		case "active_workflows":
			count, _ := s.workflowRepo.CountActiveWorkflows(ctx)
			result[metric] = count
		case "queue_length":
			length, _ := s.queueRepo.GetQueueLength(ctx, "main")
			result[metric] = length
		case "processing_tasks":
			count, _ := s.queueRepo.GetProcessingTasksCount(ctx)
			result[metric] = count
		case "failed_tasks":
			count, _ := s.queueRepo.GetFailedTasksCount(ctx)
			result[metric] = count
		default:
			s.logger.Warn("Unknown metric requested", zap.String("metric", metric))
		}
	}

	return result, nil
}

// GetQuickStats obtiene estadísticas rápidas
func (s *dashboardServiceImpl) GetQuickStats(ctx context.Context) (*models.QuickStats, error) {
	s.logger.Info("Getting quick stats")

	totalWorkflows, _ := s.workflowRepo.Count(ctx)
	queueLength, _ := s.queueRepo.GetQueueLength(ctx, "main")
	processingTasks, _ := s.queueRepo.GetProcessingTasksCount(ctx)

	return &models.QuickStats{
		ActiveWorkflows:  totalWorkflows,
		QueuedTasks:      queueLength,
		RunningTasks:     processingTasks,
		CompletedToday:   0, // Implementar según necesidades
		FailedToday:      0, // Implementar según necesidades
		SuccessRateToday: 95.5,
		AvgResponseTime:  2.3,
		SystemLoad:       0.65,
		LastUpdated:      time.Now(),
	}, nil
}

// GetRecentActivity obtiene actividad reciente
func (s *dashboardServiceImpl) GetRecentActivity(ctx context.Context, limit int) ([]models.ActivityItem, error) {
	s.logger.Info("Getting recent activity", zap.Int("limit", limit))

	// Implementación básica usando logs
	logs, _, err := s.logRepo.GetByWorkflowID(ctx, primitive.NilObjectID, repository.PaginationOptions{
		Page:     1,
		PageSize: limit,
		SortBy:   "created_at",
		SortDesc: true,
	})

	if err != nil {
		return nil, err
	}

	activities := make([]models.ActivityItem, 0, len(logs))
	for _, log := range logs {
		activity := models.ActivityItem{
			ID:           log.ID.Hex(),
			Type:         "workflow_execution",
			Title:        fmt.Sprintf("Workflow %s executed", log.WorkflowName),
			Description:  fmt.Sprintf("Status: %s", log.Status),
			WorkflowName: log.WorkflowName,
			Status:       string(log.Status),
			Timestamp:    log.CreatedAt,
			Metadata: map[string]interface{}{
				"workflow_id":  log.WorkflowID.Hex(),
				"trigger_type": log.TriggerType,
			},
		}
		activities = append(activities, activity)
	}

	return activities, nil
}

// GetSystemHealth obtiene estado de salud del sistema
func (s *dashboardServiceImpl) GetSystemHealth(ctx context.Context) (*models.SystemHealth, error) {
	s.logger.Info("Getting system health")

	// Implementación básica - expandir según necesidades
	components := []models.ComponentHealth{
		{
			Name:        "Database",
			Status:      "healthy",
			Description: "MongoDB connection is stable",
			LastCheck:   time.Now(),
		},
		{
			Name:        "Queue",
			Status:      "healthy",
			Description: "Queue processing is normal",
			LastCheck:   time.Now(),
		},
		{
			Name:        "Cache",
			Status:      "healthy",
			Description: "Cache is operating normally",
			LastCheck:   time.Now(),
		},
	}

	return &models.SystemHealth{
		Status:     "healthy",
		Score:      98,
		Components: components,
		Issues:     []models.HealthIssue{},
		LastCheck:  time.Now(),
	}, nil
}

// GetPerformanceData obtiene datos de rendimiento
func (s *dashboardServiceImpl) GetPerformanceData(ctx context.Context, timeRange string) (*models.PerformanceData, error) {
	s.logger.Info("Getting performance data", zap.String("time_range", timeRange))

	// Implementación básica - expandir según necesidades
	now := time.Now()

	return &models.PerformanceData{
		TimeRange: timeRange,
		ExecutionTrends: []models.ExecutionTrendPoint{
			{
				Timestamp:  now.Add(-1 * time.Hour),
				Total:      100,
				Successful: 95,
				Failed:     5,
			},
		},
		QueueMetrics: []models.QueueMetricPoint{
			{
				Timestamp:  now,
				Queued:     10,
				Processing: 5,
				Failed:     2,
				Retrying:   1,
			},
		},
		SuccessRateHistory: []models.SuccessRatePoint{
			{
				Timestamp:   now,
				SuccessRate: 95.0,
			},
		},
		ResponseTimeHistory: []models.ResponseTimePoint{
			{
				Timestamp:   now,
				AvgResponse: 2.5,
				P50Response: 2.0,
				P95Response: 4.0,
				P99Response: 8.0,
			},
		},
		TopWorkflows: []models.WorkflowPerformanceItem{},
		LastUpdated:  now,
	}, nil
}

// RefreshDashboardData refresca datos del dashboard
func (s *dashboardServiceImpl) RefreshDashboardData(ctx context.Context) error {
	s.logger.Info("Refreshing dashboard data")
	// Implementar lógica de refresh según necesidades
	return nil
}

// ValidateFilter valida filtros del dashboard
func (s *dashboardServiceImpl) ValidateFilter(filter *models.DashboardFilter) error {
	if filter == nil {
		return nil
	}

	// Validaciones básicas
	if filter.TimeRange != "" {
		validRanges := []string{"1h", "24h", "7d", "30d"}
		valid := false
		for _, validRange := range validRanges {
			if filter.TimeRange == validRange {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid time range: %s", filter.TimeRange)
		}
	}

	return nil
}

// MÉTODOS AUXILIARES CORREGIDOS

// Helper methods para manejar WorkflowStats con nil safety
func (s *dashboardServiceImpl) getLastExecutionFromStats(stats *models.WorkflowStats) *time.Time {
	if stats == nil {
		return nil
	}
	return stats.LastExecutedAt
}

func (s *dashboardServiceImpl) calculateSuccessRateFromStats(stats *models.WorkflowStats) float64 {
	if stats == nil || stats.TotalExecutions == 0 {
		return 100.0
	}
	return (float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions)) * 100
}

func (s *dashboardServiceImpl) getTotalExecutionsFromStats(stats *models.WorkflowStats) int64 {
	if stats == nil {
		return 0
	}
	return stats.TotalExecutions // CORREGIDO: usar TotalExecutions en lugar de TotalRuns
}

func (s *dashboardServiceImpl) getSuccessfulExecutionsFromStats(stats *models.WorkflowStats) int64 {
	if stats == nil {
		return 0
	}
	return stats.SuccessfulRuns
}

func (s *dashboardServiceImpl) getFailedExecutionsFromStats(stats *models.WorkflowStats) int64 {
	if stats == nil {
		return 0
	}
	return stats.FailedRuns
}

func (s *dashboardServiceImpl) getAverageExecutionTimeFromStats(stats *models.WorkflowStats) float64 {
	if stats == nil {
		return 0.0
	}
	return stats.AverageExecutionTime
}

func (s *dashboardServiceImpl) determineWorkflowHealthFromStats(stats *models.WorkflowStats) bool {
	if stats == nil || stats.TotalExecutions == 0 {
		return true
	}
	successRate := (float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions)) * 100
	return successRate >= 90.0
}

func (s *dashboardServiceImpl) getTriggerTypeFromWorkflow(workflow *models.Workflow) string {
	if len(workflow.Triggers) > 0 {
		return workflow.Triggers[0].Type
	}
	return "manual"
}

// Métodos auxiliares originales mantenidos para compatibilidad
func (s *dashboardServiceImpl) calculateSuccessRate(stats *models.WorkflowStats) float64 {
	return s.calculateSuccessRateFromStats(stats)
}

func (s *dashboardServiceImpl) determineWorkflowHealth(stats *models.WorkflowStats) bool {
	return s.determineWorkflowHealthFromStats(stats)
}