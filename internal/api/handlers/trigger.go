package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
)

type TriggerHandler struct {
	workflowRepo repository.WorkflowRepository
	logRepo      repository.LogRepository
	queueService *services.QueueService
	logger       *zap.Logger
}

func NewTriggerHandler(
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	queueService *services.QueueService,
	logger *zap.Logger,
) *TriggerHandler {
	return &TriggerHandler{
		workflowRepo: workflowRepo,
		logRepo:      logRepo,
		queueService: queueService,
		logger:       logger,
	}
}

// TriggerWorkflowRequest represents the request to trigger a workflow
type TriggerWorkflowRequest struct {
	WorkflowID string                 `json:"workflow_id" validate:"required"`
	TriggerBy  string                 `json:"trigger_by,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// WebhookTriggerRequest represents a webhook trigger request
type WebhookTriggerRequest struct {
	WebhookID string                 `json:"webhook_id" validate:"required"`
	Event     string                 `json:"event,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Source    string                 `json:"source,omitempty"`
}

// TriggerWorkflow triggers a specific workflow execution
// @Summary Trigger workflow execution
// @Description Trigger the execution of a specific workflow
// @Tags triggers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body TriggerWorkflowRequest true "Trigger request"
// @Success 202 {object} utils.DataResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/triggers/workflow [post]
func (h *TriggerHandler) TriggerWorkflow(c *fiber.Ctx) error {
	var req TriggerWorkflowRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error("Failed to parse trigger request", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid request body", err.Error()))
	}

	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Validation failed", err.Error()))
	}

	// Get current user ID
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Convert workflow ID to ObjectID
	workflowObjID, err := primitive.ObjectIDFromHex(req.WorkflowID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid workflow ID format", ""))
	}

	// Get workflow
	workflow, err := h.workflowRepo.FindByID(c.Context(), workflowObjID)
	if err != nil {
		if err.Error() == "workflow not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse("Workflow not found", ""))
		}
		h.logger.Error("Failed to get workflow", zap.Error(err), zap.String("workflow_id", req.WorkflowID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get workflow", ""))
	}

	// Check if workflow is active
	if !workflow.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Workflow is not active", ""))
	}

	// Check user permissions (user can only trigger their own workflows unless admin)
	userRole, _ := middleware.GetCurrentUserRole(c)
	if userRole != models.RoleAdmin && workflow.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(utils.ErrorResponse("Access denied", "You can only trigger your own workflows"))
	}

	// Create execution log
	logEntry := &models.Log{
		ID:          primitive.NewObjectID(),
		WorkflowID:  workflowObjID,
		UserID:      userID,
		TriggerBy:   req.TriggerBy,
		Status:      "pending",
		TriggerData: req.Data,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save log entry
	if err := h.logRepo.Create(c.Context(), logEntry); err != nil {
		h.logger.Error("Failed to create log entry", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to create execution log", ""))
	}

	// Enqueue workflow execution
	triggerData := map[string]interface{}{
		"log_id":     logEntry.ID.Hex(),
		"workflow":   workflow,
		"trigger_by": req.TriggerBy,
		"data":       req.Data,
		"metadata":   req.Metadata,
		"user_id":    userID.Hex(),
	}

	if err := h.queueService.EnqueueWorkflowExecution(c.Context(), req.WorkflowID, triggerData); err != nil {
		// Update log status to failed
		logEntry.Status = "failed"
		logEntry.Error = err.Error()
		logEntry.UpdatedAt = time.Now()
		h.logRepo.Update(c.Context(), logEntry)

		h.logger.Error("Failed to enqueue workflow execution", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to enqueue workflow execution", ""))
	}

	h.logger.Info("Workflow execution triggered successfully",
		zap.String("workflow_id", req.WorkflowID),
		zap.String("log_id", logEntry.ID.Hex()),
		zap.String("user_id", userID.Hex()))

	return c.Status(fiber.StatusAccepted).JSON(utils.SuccessResponse("Workflow execution triggered successfully", map[string]interface{}{
		"log_id":      logEntry.ID.Hex(),
		"workflow_id": req.WorkflowID,
		"status":      "pending",
		"created_at":  logEntry.CreatedAt,
	}))
}

// TriggerWebhook handles webhook triggers
// @Summary Trigger workflow via webhook
// @Description Trigger workflow execution via webhook
// @Tags triggers
// @Accept json
// @Produce json
// @Param webhook_id path string true "Webhook ID"
// @Param request body map[string]interface{} false "Webhook payload"
// @Success 202 {object} utils.DataResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/triggers/webhook/{webhook_id} [post]
func (h *TriggerHandler) TriggerWebhook(c *fiber.Ctx) error {
	webhookID := c.Params("webhook_id")
	if webhookID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Webhook ID is required", ""))
	}

	// Parse request body
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		h.logger.Error("Failed to parse webhook payload", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid payload", err.Error()))
	}

	// Get request headers
	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	// Find workflow by webhook ID
	filter := map[string]interface{}{
		"webhooks.id": webhookID,
		"is_active":   true,
	}

	workflows, _, err := h.workflowRepo.FindWithPagination(c.Context(), filter, 1, 1)
	if err != nil {
		h.logger.Error("Failed to find workflow by webhook", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to find workflow", ""))
	}

	if len(workflows) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse("Webhook not found or workflow inactive", ""))
	}

	workflow := workflows[0]

	// Find the specific webhook configuration
	var webhookConfig *models.WebhookTrigger
	for _, wh := range workflow.Webhooks {
		if wh.ID == webhookID {
			webhookConfig = &wh
			break
		}
	}

	if webhookConfig == nil {
		return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse("Webhook configuration not found", ""))
	}

	// Create execution log
	logEntry := &models.Log{
		ID:          primitive.NewObjectID(),
		WorkflowID:  workflow.ID,
		UserID:      workflow.UserID,
		TriggerBy:   "webhook:" + webhookID,
		Status:      "pending",
		TriggerData: payload,
		Metadata: map[string]interface{}{
			"webhook_id": webhookID,
			"headers":    headers,
			"source_ip":  c.IP(),
			"user_agent": c.Get("User-Agent"),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save log entry
	if err := h.logRepo.Create(c.Context(), logEntry); err != nil {
		h.logger.Error("Failed to create webhook log entry", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to create execution log", ""))
	}

	// Enqueue workflow execution
	triggerData := map[string]interface{}{
		"log_id":         logEntry.ID.Hex(),
		"workflow":       workflow,
		"trigger_by":     "webhook:" + webhookID,
		"data":           payload,
		"metadata":       logEntry.Metadata,
		"user_id":        workflow.UserID.Hex(),
		"webhook_config": webhookConfig,
	}

	if err := h.queueService.EnqueueWorkflowExecution(c.Context(), workflow.ID.Hex(), triggerData); err != nil {
		// Update log status to failed
		logEntry.Status = "failed"
		logEntry.Error = err.Error()
		logEntry.UpdatedAt = time.Now()
		h.logRepo.Update(c.Context(), logEntry)

		h.logger.Error("Failed to enqueue webhook execution", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to enqueue execution", ""))
	}

	h.logger.Info("Webhook triggered successfully",
		zap.String("webhook_id", webhookID),
		zap.String("workflow_id", workflow.ID.Hex()),
		zap.String("log_id", logEntry.ID.Hex()))

	return c.Status(fiber.StatusAccepted).JSON(utils.SuccessResponse("Webhook triggered successfully", map[string]interface{}{
		"log_id":      logEntry.ID.Hex(),
		"workflow_id": workflow.ID.Hex(),
		"webhook_id":  webhookID,
		"status":      "pending",
		"created_at":  logEntry.CreatedAt,
	}))
}

// TriggerScheduled handles scheduled workflow triggers
// @Summary Trigger scheduled workflow
// @Description Trigger a workflow that was scheduled for execution
// @Tags triggers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param workflow_id path string true "Workflow ID"
// @Success 202 {object} utils.DataResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/triggers/scheduled/{workflow_id} [post]
func (h *TriggerHandler) TriggerScheduled(c *fiber.Ctx) error {
	workflowID := c.Params("workflow_id")
	if workflowID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Workflow ID is required", ""))
	}

	// Convert workflow ID to ObjectID
	workflowObjID, err := primitive.ObjectIDFromHex(workflowID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid workflow ID format", ""))
	}

	// Get workflow
	workflow, err := h.workflowRepo.FindByID(c.Context(), workflowObjID)
	if err != nil {
		if err.Error() == "workflow not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse("Workflow not found", ""))
		}
		h.logger.Error("Failed to get scheduled workflow", zap.Error(err), zap.String("workflow_id", workflowID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get workflow", ""))
	}

	// Check if workflow is active
	if !workflow.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Workflow is not active", ""))
	}

	// Create execution log
	logEntry := &models.Log{
		ID:         primitive.NewObjectID(),
		WorkflowID: workflowObjID,
		UserID:     workflow.UserID,
		TriggerBy:  "scheduled",
		Status:     "pending",
		TriggerData: map[string]interface{}{
			"scheduled_at": time.Now(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save log entry
	if err := h.logRepo.Create(c.Context(), logEntry); err != nil {
		h.logger.Error("Failed to create scheduled log entry", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to create execution log", ""))
	}

	// Enqueue workflow execution
	triggerData := map[string]interface{}{
		"log_id":     logEntry.ID.Hex(),
		"workflow":   workflow,
		"trigger_by": "scheduled",
		"data":       map[string]interface{}{},
		"metadata": map[string]interface{}{
			"scheduled_at": time.Now(),
		},
		"user_id": workflow.UserID.Hex(),
	}

	if err := h.queueService.EnqueueWorkflowExecution(c.Context(), workflowID, triggerData); err != nil {
		// Update log status to failed
		logEntry.Status = "failed"
		logEntry.Error = err.Error()
		logEntry.UpdatedAt = time.Now()
		h.logRepo.Update(c.Context(), logEntry)

		h.logger.Error("Failed to enqueue scheduled execution", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to enqueue execution", ""))
	}

	h.logger.Info("Scheduled workflow triggered successfully",
		zap.String("workflow_id", workflowID),
		zap.String("log_id", logEntry.ID.Hex()))

	return c.Status(fiber.StatusAccepted).JSON(utils.SuccessResponse("Scheduled workflow triggered successfully", map[string]interface{}{
		"log_id":      logEntry.ID.Hex(),
		"workflow_id": workflowID,
		"status":      "pending",
		"created_at":  logEntry.CreatedAt,
	}))
}

// GetTriggerStatus gets the status of a triggered workflow execution
// @Summary Get trigger execution status
// @Description Get the current status of a triggered workflow execution
// @Tags triggers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param log_id path string true "Log ID"
// @Success 200 {object} utils.DataResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/triggers/status/{log_id} [get]
func (h *TriggerHandler) GetTriggerStatus(c *fiber.Ctx) error {
	logID := c.Params("log_id")
	if logID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Log ID is required", ""))
	}

	// Convert log ID to ObjectID
	logObjID, err := primitive.ObjectIDFromHex(logID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid log ID format", ""))
	}

	// Get current user ID
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Get log entry
	log, err := h.logRepo.FindByID(c.Context(), logObjID)
	if err != nil {
		if err.Error() == "log not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse("Execution log not found", ""))
		}
		h.logger.Error("Failed to get log entry", zap.Error(err), zap.String("log_id", logID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get execution status", ""))
	}

	// Check user permissions
	userRole, _ := middleware.GetCurrentUserRole(c)
	if userRole != models.RoleAdmin && log.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(utils.ErrorResponse("Access denied", "You can only view your own execution logs"))
	}

	return c.JSON(utils.SuccessResponse("Execution status retrieved successfully", map[string]interface{}{
		"log_id":       log.ID.Hex(),
		"workflow_id":  log.WorkflowID.Hex(),
		"status":       log.Status,
		"trigger_by":   log.TriggerBy,
		"trigger_data": log.TriggerData,
		"result":       log.Result,
		"error":        log.Error,
		"started_at":   log.StartedAt,
		"completed_at": log.CompletedAt,
		"created_at":   log.CreatedAt,
		"created_at":   log.CreatedAt,
		"updated_at":   log.UpdatedAt,
	}))
}

// CancelTrigger cancels a pending workflow execution
// @Summary Cancel workflow execution
// @Description Cancel a pending workflow execution
// @Tags triggers
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param log_id path string true "Log ID"
// @Success 200 {object} utils.MessageResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 409 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/triggers/cancel/{log_id} [post]
func (h *TriggerHandler) CancelTrigger(c *fiber.Ctx) error {
	logID := c.Params("log_id")
	if logID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Log ID is required", ""))
	}

	// Convert log ID to ObjectID
	logObjID, err := primitive.ObjectIDFromHex(logID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid log ID format", ""))
	}

	// Get current user ID
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Get log entry
	log, err := h.logRepo.FindByID(c.Context(), logObjID)
	if err != nil {
		if err.Error() == "log not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse("Execution log not found", ""))
		}
		h.logger.Error("Failed to get log entry", zap.Error(err), zap.String("log_id", logID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get execution log", ""))
	}

	// Check user permissions
	userRole, _ := middleware.GetCurrentUserRole(c)
	if userRole != models.RoleAdmin && log.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(utils.ErrorResponse("Access denied", "You can only cancel your own executions"))
	}

	// Check if execution can be cancelled
	if log.Status != "pending" && log.Status != "running" {
		return c.Status(fiber.StatusConflict).JSON(utils.ErrorResponse("Cannot cancel execution", "Execution is already completed or failed"))
	}

	// Update log status to cancelled
	log.Status = "cancelled"
	log.Error = "Cancelled by user"
	log.CompletedAt = &[]time.Time{time.Now()}[0]
	log.UpdatedAt = time.Now()

	if err := h.logRepo.Update(c.Context(), log); err != nil {
		h.logger.Error("Failed to update log status", zap.Error(err), zap.String("log_id", logID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to cancel execution", ""))
	}

	h.logger.Info("Execution cancelled successfully",
		zap.String("log_id", logID),
		zap.String("workflow_id", log.WorkflowID.Hex()),
		zap.String("user_id", userID.Hex()))

	return c.JSON(utils.SuccessResponse("Execution cancelled successfully", map[string]interface{}{
		"log_id":     logID,
		"status":     "cancelled",
		"updated_at": log.UpdatedAt,
	}))
}
