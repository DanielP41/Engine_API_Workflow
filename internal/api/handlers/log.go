package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/utils"
)

type LogHandler struct {
	logRepo repository.LogRepository
	logger  *zap.Logger
}

func NewLogHandler(logRepo repository.LogRepository, logger *zap.Logger) *LogHandler {
	return &LogHandler{
		logRepo: logRepo,
		logger:  logger,
	}
}

// GetLogsRequest represents the request parameters for getting logs
type GetLogsRequest struct {
	WorkflowID *string `query:"workflow_id"`
	UserID     *string `query:"user_id"`
	Status     *string `query:"status"`
	StartDate  *string `query:"start_date"`
	EndDate    *string `query:"end_date"`
	Page       int     `query:"page"`
	Limit      int     `query:"limit"`
}

// GetLogs retrieves execution logs with filtering and pagination
// @Summary Get execution logs
// @Description Retrieve execution logs with optional filtering by workflow, user, status, and date range
// @Tags logs
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param workflow_id query string false "Filter by workflow ID"
// @Param user_id query string false "Filter by user ID"
// @Param status query string false "Filter by status (success, failed, running, pending)"
// @Param start_date query string false "Start date filter (RFC3339 format)"
// @Param end_date query string false "End date filter (RFC3339 format)"
// @Param page query int false "Page number (default: 1)" default(1)
// @Param limit query int false "Items per page (default: 20)" default(20)
// @Success 200 {object} utils.PaginatedResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/logs [get]
func (h *LogHandler) GetLogs(c *fiber.Ctx) error {
	var req GetLogsRequest

	// Parse query parameters
	if err := c.QueryParser(&req); err != nil {
		h.logger.Error("Failed to parse query parameters", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid query parameters", err.Error()))
	}

	// Set default pagination values
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}

	// Build filter map
	filter := make(map[string]interface{})

	// Add workflow ID filter
	if req.WorkflowID != nil && *req.WorkflowID != "" {
		if workflowObjID, err := primitive.ObjectIDFromHex(*req.WorkflowID); err == nil {
			filter["workflow_id"] = workflowObjID
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid workflow_id format", ""))
		}
	}

	// Add user ID filter
	if req.UserID != nil && *req.UserID != "" {
		if userObjID, err := primitive.ObjectIDFromHex(*req.UserID); err == nil {
			filter["user_id"] = userObjID
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid user_id format", ""))
		}
	}

	// Add status filter
	if req.Status != nil && *req.Status != "" {
		validStatuses := map[string]bool{
			"success": true,
			"failed":  true,
			"running": true,
			"pending": true,
		}
		if !validStatuses[*req.Status] {
			return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid status", "Valid statuses: success, failed, running, pending"))
		}
		filter["status"] = *req.Status
	}

	// Add date range filters
	if req.StartDate != nil && *req.StartDate != "" {
		if startTime, err := time.Parse(time.RFC3339, *req.StartDate); err == nil {
			if filter["created_at"] == nil {
				filter["created_at"] = make(map[string]interface{})
			}
			filter["created_at"].(map[string]interface{})["$gte"] = startTime
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid start_date format", "Use RFC3339 format"))
		}
	}

	if req.EndDate != nil && *req.EndDate != "" {
		if endTime, err := time.Parse(time.RFC3339, *req.EndDate); err == nil {
			if filter["created_at"] == nil {
				filter["created_at"] = make(map[string]interface{})
			}
			filter["created_at"].(map[string]interface{})["$lte"] = endTime
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid end_date format", "Use RFC3339 format"))
		}
	}

	// Get logs with pagination
	logs, total, err := h.logRepo.FindWithPagination(c.Context(), filter, req.Page, req.Limit)
	if err != nil {
		h.logger.Error("Failed to retrieve logs", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to retrieve logs", ""))
	}

	// Calculate pagination info
	totalPages := (int(total) + req.Limit - 1) / req.Limit

	response := utils.PaginatedResponse{
		Data: logs,
		Pagination: utils.PaginationInfo{
			Page:       req.Page,
			Limit:      req.Limit,
			Total:      int(total),
			TotalPages: totalPages,
		},
	}

	h.logger.Info("Logs retrieved successfully",
		zap.Int("count", len(logs)),
		zap.Int64("total", total),
		zap.Int("page", req.Page))

	return c.JSON(utils.SuccessResponse("Logs retrieved successfully", response))
}

// GetLogByID retrieves a specific log by ID
// @Summary Get log by ID
// @Description Retrieve a specific execution log by its ID
// @Tags logs
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Log ID"
// @Success 200 {object} utils.DataResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/logs/{id} [get]
func (h *LogHandler) GetLogByID(c *fiber.Ctx) error {
	logID := c.Params("id")
	if logID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Log ID is required", ""))
	}

	// Convert string ID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(logID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid log ID format", ""))
	}

	// Find log by ID
	log, err := h.logRepo.FindByID(c.Context(), objectID)
	if err != nil {
		if err.Error() == "log not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse("Log not found", ""))
		}
		h.logger.Error("Failed to retrieve log", zap.Error(err), zap.String("log_id", logID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to retrieve log", ""))
	}

	h.logger.Info("Log retrieved successfully", zap.String("log_id", logID))
	return c.JSON(utils.SuccessResponse("Log retrieved successfully", log))
}

// GetLogStats retrieves statistics about execution logs
// @Summary Get log statistics
// @Description Retrieve statistics about execution logs including success rate, error counts, etc.
// @Tags logs
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param workflow_id query string false "Filter statistics by workflow ID"
// @Param days query int false "Number of days to include in statistics (default: 7)" default(7)
// @Success 200 {object} utils.DataResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/logs/stats [get]
func (h *LogHandler) GetLogStats(c *fiber.Ctx) error {
	workflowID := c.Query("workflow_id")
	daysStr := c.Query("days", "7")

	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid days parameter", "Must be a positive integer"))
	}

	// Build filter for date range
	filter := make(map[string]interface{})

	// Add workflow filter if provided
	if workflowID != "" {
		if workflowObjID, err := primitive.ObjectIDFromHex(workflowID); err == nil {
			filter["workflow_id"] = workflowObjID
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid workflow_id format", ""))
		}
	}

	// Add date range filter
	startDate := time.Now().AddDate(0, 0, -days)
	filter["created_at"] = map[string]interface{}{
		"$gte": startDate,
	}

	// Get statistics
	stats, err := h.logRepo.GetStatistics(c.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to retrieve log statistics", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to retrieve statistics", ""))
	}

	h.logger.Info("Log statistics retrieved successfully",
		zap.String("workflow_id", workflowID),
		zap.Int("days", days))

	return c.JSON(utils.SuccessResponse("Statistics retrieved successfully", stats))
}

// DeleteOldLogs deletes logs older than specified days
// @Summary Delete old logs
// @Description Delete execution logs older than the specified number of days (admin only)
// @Tags logs
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param days query int true "Delete logs older than this many days"
// @Success 200 {object} utils.MessageResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/logs/cleanup [delete]
func (h *LogHandler) DeleteOldLogs(c *fiber.Ctx) error {
	daysStr := c.Query("days")
	if daysStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Days parameter is required", ""))
	}

	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid days parameter", "Must be a positive integer"))
	}

	// Calculate cutoff date
	cutoffDate := time.Now().AddDate(0, 0, -days)

	// Delete old logs
	deletedCount, err := h.logRepo.DeleteOldLogs(c.Context(), cutoffDate)
	if err != nil {
		h.logger.Error("Failed to delete old logs", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to delete old logs", ""))
	}

	h.logger.Info("Old logs deleted successfully",
		zap.Int64("deleted_count", deletedCount),
		zap.Int("days", days))

	return c.JSON(utils.SuccessResponse("Old logs deleted successfully", map[string]interface{}{
		"deleted_count": deletedCount,
		"cutoff_date":   cutoffDate,
	}))
}
