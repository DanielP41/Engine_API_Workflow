package handlers

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

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

// NewWorkerHandler crea una nueva instancia del handler de workers
func NewWorkerHandler(workerEngine *worker.WorkerEngine, queueRepo repository.QueueRepository, logger *zap.Logger) *WorkerHandler {
	return &WorkerHandler{
		workerEngine: workerEngine,
		queueRepo:    queueRepo,
		logger:       logger,
	}
}

// GetWorkerStats obtiene estadísticas de los workers
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
	stats, err := h.workerEngine.GetStats(c.Context())
	if err != nil {
		h.logger.Error("Failed to get worker stats", zap.Error(err))
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get worker statistics", err.Error())
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Worker statistics retrieved successfully", stats)
}

// GetQueueStats obtiene estadísticas detalladas de las colas
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
	stats, err := h.queueRepo.GetQueueStats(c.Context(), "workflow:queue")
	if err != nil {
		h.logger.Error("Failed to get queue stats", zap.Error(err))
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get queue statistics", err.Error())
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Queue statistics retrieved successfully", stats)
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

// HealthCheck verifica el estado de los workers
// @Summary Worker health check
// @Description Check if workers are running and healthy
// @Tags workers
// @Accept json
// @Produce json
// @Success 200 {object} utils.DataResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/workers/health [get]
func (h *WorkerHandler) HealthCheck(c *fiber.Ctx) error {
	stats, err := h.workerEngine.GetStats(c.Context())
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Workers unhealthy", err.Error())
	}

	isRunning, ok := stats["is_running"].(bool)
	if !ok || !isRunning {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Workers not running", "")
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Workers are healthy", map[string]interface{}{
		"status":     "healthy",
		"is_running": isRunning,
		"stats":      stats,
	})
}
