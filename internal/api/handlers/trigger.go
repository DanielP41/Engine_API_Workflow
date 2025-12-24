package handlers

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

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

// Helper functions to get user information from context
func getCurrentUserID(c *fiber.Ctx) (primitive.ObjectID, error) {
	userID := c.Locals("userID")
	if userID == nil {
		return primitive.NilObjectID, errors.New("user not authenticated")
	}

	objID, ok := userID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, errors.New("invalid user ID")
	}

	return objID, nil
}

func getCurrentUserRole(c *fiber.Ctx) (string, error) {
	userRole := c.Locals("userRole")
	if userRole == nil {
		return "", errors.New("user role not found")
	}

	role, ok := userRole.(string)
	if !ok {
		return "", errors.New("invalid user role")
	}

	return role, nil
}

func (h *TriggerHandler) TriggerWorkflow(c *fiber.Ctx) error {
	var req TriggerWorkflowRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error("Failed to parse trigger request", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Invalid request body", err.Error()))
	}

	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Validation failed", err.Error()))
	}

	// Get current user ID
	userID, err := getCurrentUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(utils.NewErrorResponse("Authentication required", err.Error()))
	}

	// Convert workflow ID to ObjectID
	workflowID := req.WorkflowID
	if workflowID == "" {
		workflowID = c.Params("id")
	}

	if workflowID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Workflow ID is required", "Please provide workflow_id in body or id in URL"))
	}

	workflowObjID, err := primitive.ObjectIDFromHex(workflowID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Invalid workflow ID format", ""))
	}

	// Get workflow - CORREGIDO: usar GetByID en lugar de FindByID
	workflow, err := h.workflowRepo.GetByID(c.Context(), workflowObjID)
	if err != nil {
		if err.Error() == "workflow not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.NewErrorResponse("Workflow not found", ""))
		}
		h.logger.Error("Failed to get workflow", zap.Error(err), zap.String("workflow_id", req.WorkflowID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to get workflow", ""))
	}

	// Check if workflow is active
	if !workflow.IsActive() {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Workflow is not active", ""))
	}

	// Check user permissions (user can only trigger their own workflows unless admin)
	userRole, _ := getCurrentUserRole(c)
	if userRole != string(models.RoleAdmin) && workflow.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(utils.NewErrorResponse("Access denied", "You can only trigger your own workflows"))
	}

	// Create execution log
	logEntry := &models.WorkflowLog{
		ID:          primitive.NewObjectID(),
		WorkflowID:  workflowObjID,
		UserID:      userID,
		Status:      models.WorkflowStatusActive, // Estado inicial
		TriggerData: req.Data,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save log entry
	if err := h.logRepo.Create(c.Context(), logEntry); err != nil {
		h.logger.Error("Failed to create log entry", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to create execution log", ""))
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
		updateData := map[string]interface{}{
			"status":     "failed",
			"error":      err.Error(),
			"updated_at": time.Now(),
		}
		h.logRepo.Update(c.Context(), logEntry.ID, updateData)

		h.logger.Error("Failed to enqueue workflow execution", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to enqueue workflow execution", ""))
	}

	h.logger.Info("Workflow execution triggered successfully",
		zap.String("workflow_id", req.WorkflowID),
		zap.String("log_id", logEntry.ID.Hex()),
		zap.String("user_id", userID.Hex()))

	// CORREGIDO: usar SuccessResponseSimple
	return c.Status(fiber.StatusAccepted).JSON(utils.Response{
		Success:   true,
		Message:   "Workflow execution triggered successfully",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"log_id":      logEntry.ID.Hex(),
			"workflow_id": req.WorkflowID,
			"status":      "pending",
			"created_at":  logEntry.CreatedAt,
		},
	})
}

func (h *TriggerHandler) TriggerWebhook(c *fiber.Ctx) error {
	webhookID := c.Params("webhook_id")
	if webhookID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Webhook ID is required", ""))
	}

	// Parse request body
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		h.logger.Error("Failed to parse webhook payload", zap.Error(err))
		// Allow empty payload for webhooks
		payload = make(map[string]interface{})
	}

	// Get request headers
	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	h.logger.Info("Webhook trigger received",
		zap.String("webhook_id", webhookID),
		zap.Any("payload", payload))

	// Find workflows with this webhook_id
	workflows, err := h.workflowRepo.FindByWebhookID(c.Context(), webhookID)
	if err != nil {
		h.logger.Error("Failed to find workflows by webhook_id",
			zap.Error(err),
			zap.String("webhook_id", webhookID))
		return c.Status(fiber.StatusInternalServerError).JSON(
			utils.NewErrorResponse("Failed to find workflow", ""))
	}

	if len(workflows) == 0 {
		h.logger.Warn("No workflows found for webhook_id",
			zap.String("webhook_id", webhookID))
		return c.Status(fiber.StatusNotFound).JSON(
			utils.NewErrorResponse("No workflow found for this webhook", ""))
	}

	// Use the first matching workflow
	workflow := workflows[0]

	// Verify workflow is active
	if !workflow.IsActive() {
		h.logger.Warn("Workflow is not active",
			zap.String("workflow_id", workflow.ID.Hex()),
			zap.String("webhook_id", webhookID))
		return c.Status(fiber.StatusBadRequest).JSON(
			utils.NewErrorResponse("Workflow is not active", ""))
	}

	// Create execution log
	logEntry := &models.WorkflowLog{
		ID:         primitive.NewObjectID(),
		WorkflowID: workflow.ID,
		UserID:     workflow.UserID,
		Status:     models.WorkflowStatusActive,
		TriggerData: map[string]interface{}{
			"webhook_id": webhookID,
			"payload":    payload,
			"headers":    headers,
			"source":     "webhook",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save log entry
	if err := h.logRepo.Create(c.Context(), logEntry); err != nil {
		h.logger.Error("Failed to create log entry", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(
			utils.NewErrorResponse("Failed to create execution log", ""))
	}

	// Enqueue workflow execution
	triggerData := map[string]interface{}{
		"log_id":     logEntry.ID.Hex(),
		"workflow":   workflow,
		"trigger_by": "webhook",
		"data":       payload,
		"metadata": map[string]interface{}{
			"webhook_id":   webhookID,
			"headers":      headers,
			"triggered_at": time.Now(),
		},
		"user_id": workflow.UserID.Hex(),
	}

	if err := h.queueService.EnqueueWorkflowExecution(c.Context(), workflow.ID.Hex(), triggerData); err != nil {
		// Update log status to failed
		updateData := map[string]interface{}{
			"status":     "failed",
			"error":      err.Error(),
			"updated_at": time.Now(),
		}
		h.logRepo.Update(c.Context(), logEntry.ID, updateData)

		h.logger.Error("Failed to enqueue webhook execution", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(
			utils.NewErrorResponse("Failed to enqueue workflow execution", ""))
	}

	h.logger.Info("Webhook workflow triggered successfully",
		zap.String("webhook_id", webhookID),
		zap.String("workflow_id", workflow.ID.Hex()),
		zap.String("log_id", logEntry.ID.Hex()))

	return c.Status(fiber.StatusAccepted).JSON(utils.Response{
		Success:   true,
		Message:   "Webhook workflow triggered successfully",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"log_id":        logEntry.ID.Hex(),
			"workflow_id":   workflow.ID.Hex(),
			"workflow_name": workflow.Name,
			"status":        "pending",
			"created_at":    logEntry.CreatedAt,
		},
	})
}

// @Router /api/v1/triggers/scheduled/{workflow_id} [post]
func (h *TriggerHandler) TriggerScheduled(c *fiber.Ctx) error {
	workflowID := c.Params("workflow_id")
	if workflowID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Workflow ID is required", ""))
	}

	// Convert workflow ID to ObjectID
	workflowObjID, err := primitive.ObjectIDFromHex(workflowID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Invalid workflow ID format", ""))
	}

	// Get workflow
	workflow, err := h.workflowRepo.GetByID(c.Context(), workflowObjID)
	if err != nil {
		if err.Error() == "workflow not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.NewErrorResponse("Workflow not found", ""))
		}
		h.logger.Error("Failed to get scheduled workflow", zap.Error(err), zap.String("workflow_id", workflowID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to get workflow", ""))
	}

	// Check if workflow is active - CORREGIDO: usar método IsActive()
	if !workflow.IsActive() {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Workflow is not active", ""))
	}

	// Create execution log - CORREGIDO: usar WorkflowLog
	logEntry := &models.WorkflowLog{
		ID:         primitive.NewObjectID(),
		WorkflowID: workflowObjID,
		UserID:     workflow.UserID,
		Status:     models.WorkflowStatusActive,
		TriggerData: map[string]interface{}{
			"scheduled_at": time.Now(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save log entry
	if err := h.logRepo.Create(c.Context(), logEntry); err != nil {
		h.logger.Error("Failed to create scheduled log entry", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to create execution log", ""))
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
		updateData := map[string]interface{}{
			"status":     "failed",
			"error":      err.Error(),
			"updated_at": time.Now(),
		}
		h.logRepo.Update(c.Context(), logEntry.ID, updateData)

		h.logger.Error("Failed to enqueue scheduled execution", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to enqueue execution", ""))
	}

	h.logger.Info("Scheduled workflow triggered successfully",
		zap.String("workflow_id", workflowID),
		zap.String("log_id", logEntry.ID.Hex()))

	return c.Status(fiber.StatusAccepted).JSON(utils.Response{
		Success:   true,
		Message:   "Scheduled workflow triggered successfully",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"log_id":      logEntry.ID.Hex(),
			"workflow_id": workflowID,
			"status":      "pending",
			"created_at":  logEntry.CreatedAt,
		},
	})
}

func (h *TriggerHandler) GetTriggerStatus(c *fiber.Ctx) error {
	logID := c.Params("log_id")
	if logID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Log ID is required", ""))
	}

	// Convert log ID to ObjectID
	logObjID, err := primitive.ObjectIDFromHex(logID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Invalid log ID format", ""))
	}

	// Get current user ID
	userID, err := getCurrentUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(utils.NewErrorResponse("Authentication required", err.Error()))
	}

	// Get log entry - CORREGIDO: usar GetByID
	log, err := h.logRepo.GetByID(c.Context(), logObjID)
	if err != nil {
		if err.Error() == "log not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.NewErrorResponse("Execution log not found", ""))
		}
		h.logger.Error("Failed to get log entry", zap.Error(err), zap.String("log_id", logID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to get execution status", ""))
	}

	// Check user permissions
	userRole, _ := getCurrentUserRole(c)
	if userRole != string(models.RoleAdmin) && log.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(utils.NewErrorResponse("Access denied", "You can only view your own execution logs"))
	}

	return c.JSON(utils.Response{
		Success:   true,
		Message:   "Execution status retrieved successfully",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"log_id":       log.ID.Hex(),
			"workflow_id":  log.WorkflowID.Hex(),
			"status":       log.Status,
			"trigger_data": log.TriggerData,
			"started_at":   log.StartedAt,
			"completed_at": log.CompletedAt,
			"created_at":   log.CreatedAt,
			"updated_at":   log.UpdatedAt,
		},
	})
}

func (h *TriggerHandler) CancelTrigger(c *fiber.Ctx) error {
	logID := c.Params("log_id")
	if logID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Log ID is required", ""))
	}

	// Convert log ID to ObjectID
	logObjID, err := primitive.ObjectIDFromHex(logID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.NewErrorResponse("Invalid log ID format", ""))
	}

	// Get current user ID
	userID, err := getCurrentUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(utils.NewErrorResponse("Authentication required", err.Error()))
	}

	// Get log entry
	log, err := h.logRepo.GetByID(c.Context(), logObjID)
	if err != nil {
		if err.Error() == "log not found" {
			return c.Status(fiber.StatusNotFound).JSON(utils.NewErrorResponse("Execution log not found", ""))
		}
		h.logger.Error("Failed to get log entry", zap.Error(err), zap.String("log_id", logID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to get execution log", ""))
	}

	// Check user permissions
	userRole, _ := getCurrentUserRole(c)
	if userRole != string(models.RoleAdmin) && log.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(utils.NewErrorResponse("Access denied", "You can only cancel your own executions"))
	}

	// Check if execution can be cancelled
	if string(log.Status) != "pending" && string(log.Status) != "running" {
		return c.Status(fiber.StatusConflict).JSON(utils.NewErrorResponse("Cannot cancel execution", "Execution is already completed or failed"))
	}

	// Update log status to cancelled
	now := time.Now()
	updateData := map[string]interface{}{
		"status":        "cancelled",
		"error_message": "Cancelled by user",
		"completed_at":  &now,
		"updated_at":    now,
	}

	if err := h.logRepo.Update(c.Context(), log.ID, updateData); err != nil {
		h.logger.Error("Failed to update log status", zap.Error(err), zap.String("log_id", logID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.NewErrorResponse("Failed to cancel execution", ""))
	}

	h.logger.Info("Execution cancelled successfully",
		zap.String("log_id", logID),
		zap.String("workflow_id", log.WorkflowID.Hex()),
		zap.String("user_id", userID.Hex()))

	return c.JSON(utils.Response{
		Success:   true,
		Message:   "Execution cancelled successfully",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"log_id":     logID,
			"status":     "cancelled",
			"updated_at": now,
		},
	})
}
