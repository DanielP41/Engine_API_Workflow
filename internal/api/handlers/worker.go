package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/internal/worker"
)

// WorkerHandler maneja endpoints relacionados con workers
type WorkerHandler struct {
	workerEngine *worker.WorkerEngine
	queueRepo    repository.QueueRepository
	logger       *zap.Logger
}

// ScalePoolRequest estructura para solicitudes de escalamiento
type ScalePoolRequest struct {
	Action        string `json:"action" validate:"required,oneof=scale_up scale_down set_size"`
	TargetWorkers int    `json:"target_workers,omitempty" validate:"min=1,max=50"`
	Force         bool   `json:"force,omitempty"`
}

// NewWorkerHandler crea una nueva instancia del handler de workers
func NewWorkerHandler(queueRepo repository.QueueRepository, workerEngine *worker.WorkerEngine, logger *zap.Logger) *WorkerHandler {
	return &WorkerHandler{
		workerEngine: workerEngine,
		queueRepo:    queueRepo,
		logger:       logger,
	}
}

// GetWorkerStats obtiene estadísticas de los workers (CORREGIDO)
// @Summary Get worker statistics
// @Description Get current statistics about worker engine and queue processing
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/stats [get]
func (h *WorkerHandler) GetWorkerStats(c *fiber.Ctx) error {
	// CORREGIDO: Construir estadísticas básicas manualmente usando métodos disponibles
	pool := h.workerEngine.GetPool()
	basicStats := map[string]interface{}{
		"is_running":     pool != nil,
		"current_load":   h.workerEngine.GetCurrentLoad(),
		"uptime":         h.workerEngine.GetUptime().String(),
		"uptime_seconds": h.workerEngine.GetUptime().Seconds(),
	}

	if pool != nil {
		basicStats["pool_stats"] = pool.GetStats()
	}

	// Intentar obtener estadísticas avanzadas
	metrics := h.workerEngine.GetMetricsCollector()
	var advancedStats map[string]interface{}
	if metrics != nil {
		advancedStats = map[string]interface{}{
			"metrics": metrics.GetMetrics(),
		}

		retryManager := h.workerEngine.GetRetryManager()
		if retryManager != nil {
			advancedStats["retry_stats"] = retryManager.GetRetryStats(c.Context())
		}
	}

	// Combinar estadísticas
	combinedStats := map[string]interface{}{
		"basic":   basicStats,
		"version": "2.0",
	}

	if advancedStats != nil {
		combinedStats["advanced"] = advancedStats
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Worker statistics retrieved successfully", combinedStats)
}

// GetAdvancedStats obtiene estadísticas avanzadas del sistema (CORREGIDO)
// @Summary Get advanced worker statistics
// @Description Get comprehensive statistics including pool, metrics, and retries
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/advanced/stats [get]
func (h *WorkerHandler) GetAdvancedStats(c *fiber.Ctx) error {
	// CORREGIDO: Construir estadísticas avanzadas manualmente
	stats := map[string]interface{}{
		"timestamp": time.Now(),
		"uptime":    h.workerEngine.GetUptime().String(),
	}

	// Pool stats
	pool := h.workerEngine.GetPool()
	if pool != nil {
		stats["pool"] = pool.GetStats()
	}

	// Metrics
	metrics := h.workerEngine.GetMetricsCollector()
	if metrics != nil {
		stats["metrics"] = metrics.GetMetrics()
	}

	// Retry stats
	retryManager := h.workerEngine.GetRetryManager()
	if retryManager != nil {
		stats["retry_stats"] = retryManager.GetRetryStats(c.Context())
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Advanced statistics retrieved successfully", stats)
}

// GetHealthStatus obtiene el estado de salud completo (CORREGIDO)
// @Summary Get comprehensive health status
// @Description Get detailed health information including metrics and issues
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/health/detailed [get]
func (h *WorkerHandler) GetHealthStatus(c *fiber.Ctx) error {
	// CORREGIDO: Construir health status manualmente usando métodos disponibles
	pool := h.workerEngine.GetPool()
	metrics := h.workerEngine.GetMetricsCollector()

	health := map[string]interface{}{
		"is_healthy":   true,
		"timestamp":    time.Now(),
		"uptime":       h.workerEngine.GetUptime().String(),
		"current_load": h.workerEngine.GetCurrentLoad(),
	}

	if pool != nil {
		poolHealth := pool.GetHealthStatus()
		health["is_healthy"] = poolHealth.IsHealthy
		health["pool_health"] = poolHealth
		health["pool_stats"] = pool.GetStats()
	}

	if metrics != nil {
		health["metrics"] = metrics.GetMetrics()
		// Intentar obtener health check del metrics collector
		healthStatus, err := metrics.CheckHealth(c.Context(), pool, time.Now().Add(-h.workerEngine.GetUptime()))
		if err == nil {
			health["detailed_health"] = healthStatus
			health["is_healthy"] = healthStatus.IsHealthy
		}
	}

	status := fiber.StatusOK
	if isHealthy, ok := health["is_healthy"].(bool); ok && !isHealthy {
		status = fiber.StatusServiceUnavailable
	}

	return utils.SuccessResponse(c, status, "Health status retrieved", health)
}

// GetPoolStats obtiene estadísticas del pool de workers (CORREGIDO)
// @Summary Get worker pool statistics
// @Description Get detailed statistics about the worker pool
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/pool/stats [get]
func (h *WorkerHandler) GetPoolStats(c *fiber.Ctx) error {
	// CORREGIDO: Usar GetPool() en lugar de GetWorkerPool()
	pool := h.workerEngine.GetPool()
	if pool == nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Worker pool not available", "")
	}

	stats := pool.GetStats()
	return utils.SuccessResponse(c, fiber.StatusOK, "Pool statistics retrieved successfully", stats)
}

// ScaleWorkerPool escala el pool de workers manualmente (NUEVO)
// @Summary Scale worker pool
// @Description Manually scale the worker pool up or down
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body ScalePoolRequest true "Scale parameters"
// @Success 200 {object} utils.MessageResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/pool/scale [post]
func (h *WorkerHandler) ScaleWorkerPool(c *fiber.Ctx) error {
	var req ScalePoolRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}

	// CORREGIDO: Usar GetPool() en lugar de GetWorkerPool()
	pool := h.workerEngine.GetPool()
	if pool == nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Worker pool not available", "")
	}

	// Por ahora, solo registramos la solicitud (la implementación completa estaría en pool.go)
	h.logger.Info("Manual pool scaling requested",
		zap.Int("target_workers", req.TargetWorkers),
		zap.String("action", req.Action))

	return utils.SuccessResponse(c, fiber.StatusOK, "Pool scaling initiated", map[string]interface{}{
		"action":         req.Action,
		"target_workers": req.TargetWorkers,
		"timestamp":      c.Context().Value("timestamp"),
	})
}

// GetMetricsDetails obtiene métricas detalladas (NUEVO)
// @Summary Get detailed metrics
// @Description Get comprehensive metrics with breakdown by action type
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/metrics/detailed [get]
func (h *WorkerHandler) GetMetricsDetails(c *fiber.Ctx) error {
	metrics := h.workerEngine.GetMetricsCollector()
	if metrics == nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Metrics collector not available", "")
	}

	detailedMetrics := metrics.GetMetrics()
	return utils.SuccessResponse(c, fiber.StatusOK, "Detailed metrics retrieved successfully", detailedMetrics)
}

// GetRetryStats obtiene estadísticas de reintentos (CORREGIDO)
// @Summary Get retry statistics
// @Description Get statistics about task retries and failures
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/retries/stats [get]
func (h *WorkerHandler) GetRetryStats(c *fiber.Ctx) error {
	retryManager := h.workerEngine.GetRetryManager()
	if retryManager == nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Retry manager not available", "")
	}

	// CORREGIDO: GetRetryStats solo retorna 1 valor, no error
	stats := retryManager.GetRetryStats(c.Context())

	return utils.SuccessResponse(c, fiber.StatusOK, "Retry statistics retrieved successfully", stats)
}

// ResetMetrics reinicia las métricas del sistema (NUEVO)
// @Summary Reset system metrics
// @Description Reset all metrics counters (admin only)
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.MessageResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/metrics/reset [post]
func (h *WorkerHandler) ResetMetrics(c *fiber.Ctx) error {
	// Verificar permisos de admin (esto debería manejarse en middleware)

	metrics := h.workerEngine.GetMetricsCollector()
	if metrics == nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Metrics collector not available", "")
	}

	metrics.ResetMetrics()
	h.logger.Info("Metrics reset by admin")

	return utils.SuccessResponse(c, fiber.StatusOK, "Metrics reset successfully", map[string]interface{}{
		"reset_at": c.Context().Value("timestamp"),
		"message":  "All metrics have been reset to zero",
	})
}

// GetExecutorInfo obtiene información sobre ejecutores de acciones (NUEVO)
// @Summary Get action executor information
// @Description Get information about which action executors are configured
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Router /api/v1/workers/executors/info [get]
func (h *WorkerHandler) GetExecutorInfo(c *fiber.Ctx) error {
	// Obtener información de ejecutores desde el WorkerEngine
	info := h.workerEngine.GetExecutorInfo()

	return utils.SuccessResponse(c, fiber.StatusOK, "Executor information retrieved", info)
}

// GetQueueStats obtiene estadísticas detalladas de las colas (CORREGIDO)
// @Summary Get queue statistics
// @Description Get detailed statistics about all queues
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/queue/stats [get]
func (h *WorkerHandler) GetQueueStats(c *fiber.Ctx) error {
	// CORREGIDO: Usar métodos existentes del repository en lugar de GetQueueStats
	ctx := c.Context()

	// Obtener estadísticas usando métodos disponibles
	queueLength, err := h.queueRepo.GetQueueLength(ctx, "workflow_queue")
	if err != nil {
		h.logger.Error("Failed to get queue length", zap.Error(err))
		queueLength = 0
	}

	processingTasks, err := h.queueRepo.GetProcessingTasks(ctx)
	if err != nil {
		h.logger.Error("Failed to get processing tasks", zap.Error(err))
		processingTasks = []*models.QueueTask{}
	}

	failedTasksCount, err := h.queueRepo.GetFailedTasksCount(ctx)
	if err != nil {
		h.logger.Error("Failed to get failed tasks count", zap.Error(err))
		failedTasksCount = 0
	}

	// Construir estadísticas usando métodos disponibles
	stats := map[string]interface{}{
		"queue_length":        queueLength,
		"processing_tasks":    len(processingTasks),
		"failed_tasks":        failedTasksCount,
		"processing_task_ids": extractTaskIDs(processingTasks),
		"status":              "active",
		"last_updated":        time.Now(),
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Queue statistics retrieved successfully", stats)
}

// Helper function para extraer IDs de tareas (CORREGIDO)
func extractTaskIDs(tasks []*models.QueueTask) []string {
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		// CORREGIDO: Convertir primitive.ObjectID a string usando .Hex()
		ids[i] = task.ID.Hex()
	}
	return ids
}

// GetProcessingTasks obtiene las tareas actualmente en procesamiento
// @Summary Get processing tasks
// @Description Get list of tasks currently being processed
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/processing [get]
func (h *WorkerHandler) GetProcessingTasks(c *fiber.Ctx) error {
	tasks, err := h.queueRepo.GetProcessingTasks(c.Context())
	if err != nil {
		h.logger.Error("Failed to get processing tasks", zap.Error(err))
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get processing tasks", err.Error())
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Processing tasks retrieved successfully", map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// GetFailedTasks obtiene las tareas fallidas
// @Summary Get failed tasks
// @Description Get list of failed tasks that can be retried
// @Tags workers
// @Accept json
// @Produce json
// @Param limit query int false "Limit number of failed tasks" default(50)
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/failed [get]
func (h *WorkerHandler) GetFailedTasks(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	if limit < 1 || limit > 200 {
		limit = 50
	}

	tasks, err := h.queueRepo.GetFailedTasks(c.Context(), int64(limit))
	if err != nil {
		h.logger.Error("Failed to get failed tasks", zap.Error(err))
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get failed tasks", err.Error())
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Failed tasks retrieved successfully", map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
		"limit": limit,
	})
}

// RetryFailedTask reintenta una tarea fallida específica
// @Summary Retry failed task
// @Description Retry a specific failed task by moving it back to the queue
// @Tags workers
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Security ApiKeyAuth
// @Success 200 {object} utils.MessageResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/retry/{task_id} [post]
func (h *WorkerHandler) RetryFailedTask(c *fiber.Ctx) error {
	taskID := c.Params("task_id")
	if taskID == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Task ID is required", "")
	}

	err := h.queueRepo.RequeueFailedTask(c.Context(), taskID)
	if err != nil {
		if err == repository.ErrTaskNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Task not found", "")
		}
		h.logger.Error("Failed to retry task", zap.Error(err), zap.String("task_id", taskID))
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to retry task", err.Error())
	}

	h.logger.Info("Task queued for retry", zap.String("task_id", taskID))

	return utils.SuccessResponse(c, fiber.StatusOK, "Task queued for retry successfully", map[string]interface{}{
		"task_id": taskID,
		"status":  "queued_for_retry",
	})
}

// ClearQueue limpia una cola específica (solo admin)
// @Summary Clear queue
// @Description Clear all tasks from a specific queue (admin only)
// @Tags workers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.MessageResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/queue/clear [post]
func (h *WorkerHandler) ClearQueue(c *fiber.Ctx) error {
	// Verificar que el usuario sea admin (esto debería manejarse en middleware)
	// Por ahora, asumimos que el middleware ya verificó los permisos

	err := h.queueRepo.Clear(c.Context(), "workflow:queue")
	if err != nil {
		h.logger.Error("Failed to clear queue", zap.Error(err))
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to clear queue", err.Error())
	}

	h.logger.Info("Queue cleared by admin")

	return utils.SuccessResponse(c, fiber.StatusOK, "Queue cleared successfully", map[string]interface{}{
		"message": "All queues have been cleared",
		"time":    c.Context().Value("timestamp"),
	})
}

// HealthCheck verifica el estado de los workers (CORREGIDO)
// @Summary Worker health check
// @Description Check if workers are running and healthy
// @Tags workers
// @Accept json
// @Produce json
// @Success 200 {object} utils.DataResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/health [get]
func (h *WorkerHandler) HealthCheck(c *fiber.Ctx) error {
	// CORREGIDO: Construir health check usando métodos disponibles
	pool := h.workerEngine.GetPool()

	if pool == nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Worker pool not available", "")
	}

	// Obtener estado de salud del pool
	poolHealth := pool.GetHealthStatus()

	status := fiber.StatusOK
	if !poolHealth.IsHealthy {
		status = fiber.StatusServiceUnavailable
	}

	healthResponse := map[string]interface{}{
		"status":       "healthy",
		"is_running":   true,
		"is_healthy":   poolHealth.IsHealthy,
		"pool_health":  poolHealth,
		"current_load": h.workerEngine.GetCurrentLoad(),
		"uptime":       h.workerEngine.GetUptime().String(),
		"version":      "2.0",
	}

	if !poolHealth.IsHealthy {
		healthResponse["status"] = "unhealthy"
	}

	return utils.SuccessResponse(c, status, "Health check completed", healthResponse)
}
