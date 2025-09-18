// internal/services/cached_dashboard_service.go
package services

import (
	"context"
	"fmt"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/pkg/cache"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// cachedDashboardService implementa DashboardService con sistema de caché
type cachedDashboardService struct {
	// Servicios base
	baseService    DashboardService
	metricsService MetricsService

	// Repositorios
	workflowRepo repository.WorkflowRepository
	logRepo      repository.LogRepository
	userRepo     repository.UserRepository
	queueRepo    repository.QueueRepository

	// Sistema de caché
	cacheManager *cache.CacheManager
	logger       *zap.Logger

	// Configuración de TTL
	ttlConfig *TTLConfig
}

// TTLConfig configuración de TTL para diferentes tipos de datos
type TTLConfig struct {
	Summary        time.Duration
	Metrics        time.Duration
	RecentActivity time.Duration
	SystemHealth   time.Duration
	QuickStats     time.Duration
	WorkflowStatus time.Duration
	QueueStatus    time.Duration
}

// NewCachedDashboardService crea un nuevo dashboard service con caché
func NewCachedDashboardService(
	baseService DashboardService,
	metricsService MetricsService,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	userRepo repository.UserRepository,
	queueRepo repository.QueueRepository,
	cacheManager *cache.CacheManager,
	logger *zap.Logger,
) DashboardService {

	// Configuración de TTL optimizada para dashboard
	ttlConfig := &TTLConfig{
		Summary:        cache.TTLShort,     // 30 segundos
		Metrics:        cache.TTLVeryShort, // 10 segundos
		RecentActivity: cache.TTLMedium,    // 5 minutos
		SystemHealth:   cache.TTLVeryShort, // 10 segundos
		QuickStats:     cache.TTLShort,     // 30 segundos
		WorkflowStatus: cache.TTLMedium,    // 5 minutos
		QueueStatus:    cache.TTLVeryShort, // 10 segundos
	}

	service := &cachedDashboardService{
		baseService:    baseService,
		metricsService: metricsService,
		workflowRepo:   workflowRepo,
		logRepo:        logRepo,
		userRepo:       userRepo,
		queueRepo:      queueRepo,
		cacheManager:   cacheManager,
		logger:         logger,
		ttlConfig:      ttlConfig,
	}

	// Configurar tareas de warmup
	service.setupWarmupTasks()

	return service
}

// GetCompleteDashboard obtiene datos completos del dashboard con caché
func (s *cachedDashboardService) GetCompleteDashboard(ctx context.Context, filter *models.DashboardFilter) (*models.DashboardData, error) {
	key := cache.DashboardKeys.Build("complete", s.buildFilterKey(filter))

	return s.getOrComputeData(ctx, key, s.ttlConfig.Summary, func() (interface{}, error) {
		s.logger.Debug("Computing complete dashboard data", zap.String("cache_key", key))
		return s.baseService.GetCompleteDashboard(ctx, filter)
	}).(*models.DashboardData), nil
}

// GetDashboardSummary obtiene resumen del dashboard con caché optimizado
func (s *cachedDashboardService) GetDashboardSummary(ctx context.Context) (*models.DashboardSummary, error) {
	key := cache.DashboardKeys.Build("summary")

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.Summary, func() (interface{}, error) {
		s.logger.Debug("Computing dashboard summary", zap.String("cache_key", key))
		return s.computeDashboardSummary(ctx)
	})

	if err != nil {
		s.logger.Error("Failed to get dashboard summary", zap.Error(err))
		return nil, err
	}

	return result.(*models.DashboardSummary), nil
}

// GetSystemHealth obtiene estado de salud del sistema con caché de alta frecuencia
func (s *cachedDashboardService) GetSystemHealth(ctx context.Context) (*models.SystemHealth, error) {
	key := cache.SystemKeys.Build("health")

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.SystemHealth, func() (interface{}, error) {
		s.logger.Debug("Computing system health", zap.String("cache_key", key))
		return s.computeSystemHealth(ctx)
	})

	if err != nil {
		s.logger.Error("Failed to get system health", zap.Error(err))
		return nil, err
	}

	return result.(*models.SystemHealth), nil
}

// GetQuickStats obtiene estadísticas rápidas con caché
func (s *cachedDashboardService) GetQuickStats(ctx context.Context) (*models.QuickStats, error) {
	key := cache.DashboardKeys.Build("quick_stats")

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.QuickStats, func() (interface{}, error) {
		s.logger.Debug("Computing quick stats", zap.String("cache_key", key))
		return s.computeQuickStats(ctx)
	})

	if err != nil {
		s.logger.Error("Failed to get quick stats", zap.Error(err))
		return nil, err
	}

	return result.(*models.QuickStats), nil
}

// GetRecentActivity obtiene actividad reciente con caché
func (s *cachedDashboardService) GetRecentActivity(ctx context.Context, limit int) ([]models.ActivityItem, error) {
	key := cache.DashboardKeys.Build("recent_activity", fmt.Sprintf("limit_%d", limit))

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.RecentActivity, func() (interface{}, error) {
		s.logger.Debug("Computing recent activity", zap.String("cache_key", key), zap.Int("limit", limit))
		return s.computeRecentActivity(ctx, limit)
	})

	if err != nil {
		s.logger.Error("Failed to get recent activity", zap.Error(err))
		return nil, err
	}

	return result.([]models.ActivityItem), nil
}

// GetWorkflowStatus obtiene estado de workflows con caché
func (s *cachedDashboardService) GetWorkflowStatus(ctx context.Context, limit int) ([]models.WorkflowStatusItem, error) {
	key := cache.DashboardKeys.Build("workflow_status", fmt.Sprintf("limit_%d", limit))

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.WorkflowStatus, func() (interface{}, error) {
		s.logger.Debug("Computing workflow status", zap.String("cache_key", key), zap.Int("limit", limit))
		return s.computeWorkflowStatus(ctx, limit)
	})

	if err != nil {
		s.logger.Error("Failed to get workflow status", zap.Error(err))
		return nil, err
	}

	return result.([]models.WorkflowStatusItem), nil
}

// GetQueueStatus obtiene estado de colas con caché de alta frecuencia
func (s *cachedDashboardService) GetQueueStatus(ctx context.Context) (*models.QueueStatus, error) {
	key := cache.QueueKeys.Build("status")

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.QueueStatus, func() (interface{}, error) {
		s.logger.Debug("Computing queue status", zap.String("cache_key", key))
		return s.computeQueueStatus(ctx)
	})

	if err != nil {
		s.logger.Error("Failed to get queue status", zap.Error(err))
		return nil, err
	}

	return result.(*models.QueueStatus), nil
}

// GetDashboardMetrics obtiene métricas específicas con caché
func (s *cachedDashboardService) GetDashboardMetrics(ctx context.Context, metrics []string, timeRange string) (map[string]interface{}, error) {
	key := cache.MetricsKeys.Build("dashboard", timeRange, fmt.Sprintf("metrics_%v", metrics))

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.Metrics, func() (interface{}, error) {
		s.logger.Debug("Computing dashboard metrics",
			zap.String("cache_key", key),
			zap.Strings("metrics", metrics),
			zap.String("time_range", timeRange))
		return s.baseService.GetDashboardMetrics(ctx, metrics, timeRange)
	})

	if err != nil {
		s.logger.Error("Failed to get dashboard metrics", zap.Error(err))
		return nil, err
	}

	return result.(map[string]interface{}), nil
}

// GetActiveAlerts obtiene alertas activas (sin caché por ser crítico)
func (s *cachedDashboardService) GetActiveAlerts(ctx context.Context) ([]models.Alert, error) {
	// Las alertas son críticas, no se cachean para tener datos en tiempo real
	return s.baseService.GetActiveAlerts(ctx)
}

// GetWorkflowHealth obtiene salud de workflow específico con caché
func (s *cachedDashboardService) GetWorkflowHealth(ctx context.Context, workflowID string) (*models.WorkflowStatusItem, error) {
	key := cache.WorkflowKeys.BuildWithID(workflowID, "health")

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.WorkflowStatus, func() (interface{}, error) {
		s.logger.Debug("Computing workflow health",
			zap.String("cache_key", key),
			zap.String("workflow_id", workflowID))
		return s.baseService.GetWorkflowHealth(ctx, workflowID)
	})

	if err != nil {
		s.logger.Error("Failed to get workflow health", zap.String("workflow_id", workflowID), zap.Error(err))
		return nil, err
	}

	return result.(*models.WorkflowStatusItem), nil
}

// GetPerformanceData obtiene datos de rendimiento con caché
func (s *cachedDashboardService) GetPerformanceData(ctx context.Context, timeRange string) (*models.PerformanceData, error) {
	key := cache.MetricsKeys.Build("performance", timeRange)

	result, err := s.cacheManager.GetOrSet(ctx, key, s.ttlConfig.Metrics, func() (interface{}, error) {
		s.logger.Debug("Computing performance data",
			zap.String("cache_key", key),
			zap.String("time_range", timeRange))
		return s.baseService.GetPerformanceData(ctx, timeRange)
	})

	if err != nil {
		s.logger.Error("Failed to get performance data", zap.Error(err))
		return nil, err
	}

	return result.(*models.PerformanceData), nil
}

// RefreshDashboardData refresca los datos del dashboard e invalida caché
func (s *cachedDashboardService) RefreshDashboardData(ctx context.Context) error {
	s.logger.Info("Refreshing dashboard data and invalidating cache")

	// Invalidar todos los cachés relacionados con dashboard
	patterns := []string{
		cache.DashboardKeys.Build("*"),
		cache.MetricsKeys.Build("*"),
		cache.SystemKeys.Build("*"),
		cache.QueueKeys.Build("*"),
	}

	for _, pattern := range patterns {
		if err := s.cacheManager.InvalidatePattern(ctx, pattern, "manual_refresh"); err != nil {
			s.logger.Warn("Failed to invalidate cache pattern", zap.String("pattern", pattern), zap.Error(err))
		}
	}

	// Ejecutar warmup para precargar datos críticos
	if err := s.cacheManager.ExecuteWarmup(ctx); err != nil {
		s.logger.Warn("Failed to execute cache warmup", zap.Error(err))
	}

	return s.baseService.RefreshDashboardData(ctx)
}

// ValidateFilter valida filtros
func (s *cachedDashboardService) ValidateFilter(filter *models.DashboardFilter) error {
	return s.baseService.ValidateFilter(filter)
}

// Métodos de computación (llamados cuando no hay datos en caché)

func (s *cachedDashboardService) computeDashboardSummary(ctx context.Context) (*models.DashboardSummary, error) {
	// Usar operaciones en paralelo para optimizar tiempo de respuesta
	type result struct {
		totalWorkflows  int64
		activeWorkflows int64
		totalUsers      int64
		queueLength     int64
		err             error
	}

	ch := make(chan result, 4)

	// Ejecutar consultas en paralelo
	go func() {
		count, err := s.workflowRepo.Count(ctx)
		ch <- result{totalWorkflows: count, err: err}
	}()

	go func() {
		count, err := s.workflowRepo.CountActiveWorkflows(ctx)
		ch <- result{activeWorkflows: count, err: err}
	}()

	go func() {
		count, err := s.userRepo.Count(ctx)
		ch <- result{totalUsers: count, err: err}
	}()

	go func() {
		length, err := s.queueRepo.GetQueueLength(ctx, "main")
		ch <- result{queueLength: length, err: err}
	}()

	// Recopilar resultados
	summary := &models.DashboardSummary{
		Timestamp: time.Now(),
	}

	for i := 0; i < 4; i++ {
		res := <-ch
		if res.err != nil {
			s.logger.Error("Error computing dashboard summary component", zap.Error(res.err))
			continue
		}

		if res.totalWorkflows > 0 {
			summary.TotalWorkflows = res.totalWorkflows
		}
		if res.activeWorkflows > 0 {
			summary.ActiveWorkflows = res.activeWorkflows
		}
		if res.totalUsers > 0 {
			summary.TotalUsers = res.totalUsers
		}
		if res.queueLength >= 0 {
			summary.QueueLength = res.queueLength
		}
	}

	return summary, nil
}

func (s *cachedDashboardService) computeSystemHealth(ctx context.Context) (*models.SystemHealth, error) {
	health := &models.SystemHealth{
		Status:     "healthy",
		Timestamp:  time.Now(),
		Components: make(map[string]models.ComponentHealth),
	}

	// Verificar MongoDB
	if _, err := s.workflowRepo.Count(ctx); err != nil {
		health.Components["mongodb"] = models.ComponentHealth{
			Status:  "unhealthy",
			Message: err.Error(),
		}
		health.Status = "degraded"
	} else {
		health.Components["mongodb"] = models.ComponentHealth{
			Status:  "healthy",
			Message: "Connected",
		}
	}

	// Verificar Redis/Queue
	if err := s.queueRepo.Ping(ctx); err != nil {
		health.Components["redis"] = models.ComponentHealth{
			Status:  "unhealthy",
			Message: err.Error(),
		}
		health.Status = "degraded"
	} else {
		health.Components["redis"] = models.ComponentHealth{
			Status:  "healthy",
			Message: "Connected",
		}
	}

	// Verificar Cache
	if err := s.cacheManager.Ping(ctx); err != nil {
		health.Components["cache"] = models.ComponentHealth{
			Status:  "unhealthy",
			Message: err.Error(),
		}
		// Cache no es crítico, solo degraded si está caído
		if health.Status == "healthy" {
			health.Status = "degraded"
		}
	} else {
		health.Components["cache"] = models.ComponentHealth{
			Status:  "healthy",
			Message: "Connected",
		}
	}

	return health, nil
}

func (s *cachedDashboardService) computeQuickStats(ctx context.Context) (*models.QuickStats, error) {
	stats := &models.QuickStats{
		Timestamp: time.Now(),
	}

	// Obtener estadísticas básicas en paralelo
	type statResult struct {
		name  string
		value int64
		err   error
	}

	ch := make(chan statResult, 5)

	go func() {
		count, err := s.workflowRepo.Count(ctx)
		ch <- statResult{"workflows", count, err}
	}()

	go func() {
		count, err := s.userRepo.Count(ctx)
		ch <- statResult{"users", count, err}
	}()

	go func() {
		length, err := s.queueRepo.GetQueueLength(ctx, "main")
		ch <- statResult{"queue", length, err}
	}()

	go func() {
		count, err := s.queueRepo.GetProcessingTasksCount(ctx)
		ch <- statResult{"processing", count, err}
	}()

	go func() {
		count, err := s.queueRepo.GetFailedTasksCount(ctx)
		ch <- statResult{"failed", count, err}
	}()

	// Recopilar resultados
	for i := 0; i < 5; i++ {
		res := <-ch
		if res.err != nil {
			s.logger.Warn("Error computing quick stat", zap.String("stat", res.name), zap.Error(res.err))
			continue
		}

		switch res.name {
		case "workflows":
			stats.TotalWorkflows = res.value
		case "users":
			stats.TotalUsers = res.value
		case "queue":
			stats.QueuedTasks = res.value
		case "processing":
			stats.ProcessingTasks = res.value
		case "failed":
			stats.FailedTasks = res.value
		}
	}

	return stats, nil
}

func (s *cachedDashboardService) computeRecentActivity(ctx context.Context, limit int) ([]models.ActivityItem, error) {
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

func (s *cachedDashboardService) computeWorkflowStatus(ctx context.Context, limit int) ([]models.WorkflowStatusItem, error) {
	// Obtener workflows activos
	workflows, _, err := s.workflowRepo.ListByStatus(ctx, models.WorkflowStatusActive, 1, limit)
	if err != nil {
		return nil, err
	}

	// Convertir a workflow status items
	items := make([]models.WorkflowStatusItem, 0, len(workflows.Workflows))
	for _, workflow := range workflows.Workflows {
		item := models.WorkflowStatusItem{
			ID:          workflow.ID.Hex(),
			Name:        workflow.Name,
			Status:      string(workflow.Status),
			LastRun:     workflow.Stats.LastExecutedAt,
			SuccessRate: s.calculateSuccessRate(workflow.Stats),
			Health:      s.determineWorkflowHealth(workflow.Stats),
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *cachedDashboardService) computeQueueStatus(ctx context.Context) (*models.QueueStatus, error) {
	status := &models.QueueStatus{
		Timestamp: time.Now(),
	}

	// Obtener estadísticas de cola en paralelo
	type queueResult struct {
		name  string
		value int64
		err   error
	}

	ch := make(chan queueResult, 4)

	go func() {
		length, err := s.queueRepo.GetQueueLength(ctx, "main")
		ch <- queueResult{"queued", length, err}
	}()

	go func() {
		count, err := s.queueRepo.GetProcessingTasksCount(ctx)
		ch <- queueResult{"processing", count, err}
	}()

	go func() {
		count, err := s.queueRepo.GetFailedTasksCount(ctx)
		ch <- queueResult{"failed", count, err}
	}()

	go func() {
		count, err := s.queueRepo.GetRetryingTasksCount(ctx)
		ch <- queueResult{"retrying", count, err}
	}()

	// Recopilar resultados
	for i := 0; i < 4; i++ {
		res := <-ch
		if res.err != nil {
			s.logger.Warn("Error computing queue status", zap.String("metric", res.name), zap.Error(res.err))
			continue
		}

		switch res.name {
		case "queued":
			status.QueuedTasks = res.value
		case "processing":
			status.ProcessingTasks = res.value
		case "failed":
			status.FailedTasks = res.value
		case "retrying":
			status.RetryingTasks = res.value
		}
	}

	// Determinar estado de salud de la cola
	if status.FailedTasks > status.ProcessingTasks*2 {
		status.Health = "unhealthy"
	} else if status.FailedTasks > 0 {
		status.Health = "degraded"
	} else {
		status.Health = "healthy"
	}

	return status, nil
}

// Métodos auxiliares

func (s *cachedDashboardService) buildFilterKey(filter *models.DashboardFilter) string {
	if filter == nil {
		return "default"
	}

	// Construir clave basada en filtros
	return fmt.Sprintf("user_%s_range_%s", filter.UserID, filter.TimeRange)
}

func (s *cachedDashboardService) getOrComputeData(ctx context.Context, key string, ttl time.Duration, computeFunc func() (interface{}, error)) interface{} {
	result, err := s.cacheManager.GetOrSet(ctx, key, ttl, computeFunc)
	if err != nil {
		s.logger.Error("Cache operation failed", zap.String("key", key), zap.Error(err))
		// Fallback: computar directamente si el caché falla
		if result, err := computeFunc(); err == nil {
			return result
		}
		return nil
	}
	return result
}

func (s *cachedDashboardService) calculateSuccessRate(stats models.WorkflowStats) float64 {
	total := stats.SuccessfulRuns + stats.FailedRuns
	if total == 0 {
		return 0
	}
	return float64(stats.SuccessfulRuns) / float64(total) * 100
}

func (s *cachedDashboardService) determineWorkflowHealth(stats models.WorkflowStats) string {
	successRate := s.calculateSuccessRate(stats)

	switch {
	case successRate >= 95:
		return "excellent"
	case successRate >= 85:
		return "good"
	case successRate >= 70:
		return "fair"
	default:
		return "poor"
	}
}

// setupWarmupTasks configura tareas de precalentamiento
func (s *cachedDashboardService) setupWarmupTasks() {
	// Warmup para datos críticos del dashboard
	tasks := []cache.WarmupTask{
		{
			Name: "dashboard_summary",
			Key:  cache.DashboardKeys.Build("summary"),
			TTL:  s.ttlConfig.Summary,
			Fetcher: func(ctx context.Context) (interface{}, error) {
				return s.computeDashboardSummary(ctx)
			},
			Schedule: time.Minute,
			Priority: 1,
		},
		{
			Name: "system_health",
			Key:  cache.SystemKeys.Build("health"),
			TTL:  s.ttlConfig.SystemHealth,
			Fetcher: func(ctx context.Context) (interface{}, error) {
				return s.computeSystemHealth(ctx)
			},
			Schedule: 30 * time.Second,
			Priority: 1,
		},
		{
			Name: "quick_stats",
			Key:  cache.DashboardKeys.Build("quick_stats"),
			TTL:  s.ttlConfig.QuickStats,
			Fetcher: func(ctx context.Context) (interface{}, error) {
				return s.computeQuickStats(ctx)
			},
			Schedule: time.Minute,
			Priority: 2,
		},
		{
			Name: "queue_status",
			Key:  cache.QueueKeys.Build("status"),
			TTL:  s.ttlConfig.QueueStatus,
			Fetcher: func(ctx context.Context) (interface{}, error) {
				return s.computeQueueStatus(ctx)
			},
			Schedule: 10 * time.Second,
			Priority: 1,
		},
	}

	for _, task := range tasks {
		s.cacheManager.AddWarmupTask(task)
	}

	s.logger.Info("Dashboard cache warmup tasks configured", zap.Int("tasks", len(tasks)))
}
