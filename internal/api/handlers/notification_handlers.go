package handlers

import (
	"context"
	"strconv"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

type NotificationHandler struct {
	notificationService *services.NotificationService
	validator           *utils.Validator
	logger              *zap.Logger
}

func NewNotificationHandler(
	notificationService *services.NotificationService,
	validator *utils.Validator,
	logger *zap.Logger,
) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		validator:           validator,
		logger:              logger,
	}
}

type SendEmailRequest struct {
	Type         models.NotificationType     `json:"type" validate:"required"`
	Priority     models.NotificationPriority `json:"priority" validate:"required"`
	To           []string                    `json:"to" validate:"required,min=1"`
	CC           []string                    `json:"cc,omitempty"`
	BCC          []string                    `json:"bcc,omitempty"`
	Subject      string                      `json:"subject,omitempty"`
	Body         string                      `json:"body,omitempty"`
	IsHTML       bool                        `json:"is_html,omitempty"`
	TemplateName string                      `json:"template_name,omitempty"`
	TemplateData map[string]interface{}      `json:"template_data,omitempty"`
	ScheduledAt  *time.Time                  `json:"scheduled_at,omitempty"`
	MaxAttempts  int                         `json:"max_attempts,omitempty"`
	WorkflowID   *primitive.ObjectID         `json:"workflow_id,omitempty"`
	ExecutionID  *string                     `json:"execution_id,omitempty"`
}

type SendTemplatedEmailRequest struct {
	TemplateName string                      `json:"template_name" validate:"required"`
	To           []string                    `json:"to" validate:"required,min=1"`
	CC           []string                    `json:"cc,omitempty"`
	BCC          []string                    `json:"bcc,omitempty"`
	Data         map[string]interface{}      `json:"data,omitempty"`
	ScheduledAt  *time.Time                  `json:"scheduled_at,omitempty"`
	Priority     models.NotificationPriority `json:"priority,omitempty"`
}

type SendSimpleEmailRequest struct {
	To      []string `json:"to" validate:"required,min=1"`
	Subject string   `json:"subject" validate:"required"`
	Body    string   `json:"body" validate:"required"`
	IsHTML  bool     `json:"is_html,omitempty"`
}

type CreateTemplateRequest struct {
	Name        string                    `json:"name" validate:"required,min=1,max=100"`
	Type        models.NotificationType   `json:"type" validate:"required"`
	Language    string                    `json:"language,omitempty"`
	Subject     string                    `json:"subject" validate:"required"`
	BodyText    string                    `json:"body_text" validate:"required"`
	BodyHTML    string                    `json:"body_html,omitempty"`
	Description string                    `json:"description,omitempty"`
	Tags        []string                  `json:"tags,omitempty"`
	Variables   []models.TemplateVariable `json:"variables,omitempty"`
}

type UpdateTemplateRequest struct {
	Subject     string                    `json:"subject,omitempty"`
	BodyText    string                    `json:"body_text,omitempty"`
	BodyHTML    string                    `json:"body_html,omitempty"`
	Description string                    `json:"description,omitempty"`
	Tags        []string                  `json:"tags,omitempty"`
	Variables   []models.TemplateVariable `json:"variables,omitempty"`
	IsActive    *bool                     `json:"is_active,omitempty"`
}

type PreviewTemplateRequest struct {
	Data map[string]interface{} `json:"data" validate:"required"`
}

func (h *NotificationHandler) SendEmail(c *fiber.Ctx) error {
	var req SendEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}
	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}
	if req.Subject == "" && req.TemplateName == "" {
		return utils.BadRequestResponse(c, "Subject or template_name is required", nil)
	}
	if req.Body == "" && req.TemplateName == "" {
		return utils.BadRequestResponse(c, "Body or template_name is required", nil)
	}
	serviceReq := &models.SendEmailRequest{
		Type:         req.Type,
		Priority:     req.Priority,
		To:           req.To,
		CC:           req.CC,
		BCC:          req.BCC,
		Subject:      req.Subject,
		Body:         req.Body,
		IsHTML:       req.IsHTML,
		TemplateName: req.TemplateName,
		TemplateData: req.TemplateData,
		ScheduledAt:  req.ScheduledAt,
		MaxAttempts:  req.MaxAttempts,
		WorkflowID:   req.WorkflowID,
		ExecutionID:  req.ExecutionID,
		UserID:       h.getUserIDFromContext(c),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	notification, err := h.notificationService.SendEmail(ctx, serviceReq)
	if err != nil {
		h.logError(c, "send_email", err)
		return utils.InternalServerErrorResponse(c, "Failed to send email", err)
	}
	h.logInfo(c, "send_email", "Email sent successfully",
		zap.String("notification_id", notification.ID.Hex()),
		zap.Strings("to", notification.To))
	return utils.CreatedResponse(c, "Email sent successfully", notification)
}

func (h *NotificationHandler) SendSimpleEmail(c *fiber.Ctx) error {
	var req SendSimpleEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}
	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	notification, err := h.notificationService.SendSimpleEmail(ctx, req.To, req.Subject, req.Body, req.IsHTML)
	if err != nil {
		h.logError(c, "send_simple_email", err)
		return utils.InternalServerErrorResponse(c, "Failed to send simple email", err)
	}
	h.logInfo(c, "send_simple_email", "Simple email sent successfully",
		zap.String("notification_id", notification.ID.Hex()),
		zap.Strings("to", req.To))
	return utils.CreatedResponse(c, "Simple email sent successfully", notification)
}

func (h *NotificationHandler) SendTemplatedEmail(c *fiber.Ctx) error {
	var req SendTemplatedEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}
	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}
	if req.Priority == "" {
		req.Priority = models.NotificationPriorityNormal
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := h.notificationService.SendTemplatedEmail(ctx, req.TemplateName, req.To, req.Data)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		h.logError(c, "send_templated_email", err)
		return utils.InternalServerErrorResponse(c, "Failed to send templated email", err)
	}
	h.logInfo(c, "send_templated_email", "Templated email sent successfully",
		zap.String("template", req.TemplateName),
		zap.Strings("to", req.To))
	return utils.CreatedResponse(c, "Templated email sent successfully", nil)
}

func (h *NotificationHandler) SendWelcomeEmail(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email" validate:"required,email"`
		UserName string `json:"user_name" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}
	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := h.notificationService.SendWelcomeEmail(ctx, req.Email, req.UserName)
	if err != nil {
		h.logError(c, "send_welcome_email", err)
		return utils.InternalServerErrorResponse(c, "Failed to send welcome email", err)
	}
	h.logInfo(c, "send_welcome_email", "Welcome email sent successfully",
		zap.String("email", req.Email),
		zap.String("user_name", req.UserName))
	return utils.CreatedResponse(c, "Welcome email sent successfully", nil)
}

func (h *NotificationHandler) SendSystemAlert(c *fiber.Ctx) error {
	var req struct {
		Level      string   `json:"level" validate:"required,oneof=info warning error critical"`
		Message    string   `json:"message" validate:"required"`
		Recipients []string `json:"recipients" validate:"required,min=1"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}
	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := h.notificationService.SendSystemAlert(ctx, req.Level, req.Message, req.Recipients)
	if err != nil {
		h.logError(c, "send_system_alert", err)
		return utils.InternalServerErrorResponse(c, "Failed to send system alert", err)
	}
	h.logInfo(c, "send_system_alert", "System alert sent successfully",
		zap.String("level", req.Level),
		zap.String("message", req.Message),
		zap.Strings("recipients", req.Recipients))
	return utils.CreatedResponse(c, "System alert sent successfully", nil)
}

// CORRECCION PRINCIPAL AQUI - Lineas 449-452
func (h *NotificationHandler) GetNotifications(c *fiber.Ctx) error {
	status := c.Query("status")
	notificationType := c.Query("type")
	priority := c.Query("priority")
	userID := c.Query("user_id")
	workflowID := c.Query("workflow_id")
	executionID := c.Query("execution_id")
	toEmail := c.Query("to_email")

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}

	filters := make(map[string]interface{})
	if status != "" {
		filters["status"] = models.NotificationStatus(status)
	}
	if notificationType != "" {
		filters["type"] = models.NotificationType(notificationType)
	}
	if priority != "" {
		filters["priority"] = models.NotificationPriority(priority)
	}
	if userID != "" {
		if objectID, err := primitive.ObjectIDFromHex(userID); err == nil {
			filters["user_id"] = objectID
		}
	}
	if workflowID != "" {
		if objectID, err := primitive.ObjectIDFromHex(workflowID); err == nil {
			filters["workflow_id"] = objectID
		}
	}
	if executionID != "" {
		filters["execution_id"] = executionID
	}
	if toEmail != "" {
		filters["to_email"] = toEmail
	}

	// CORREGIDO: Page y PageSize en lugar de Limit y Skip
	opts := &repository.PaginationOptions{
		Page:     page,
		PageSize: limit,
		SortBy:   "created_at",
		SortDesc: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	notifications, total, err := h.notificationService.GetNotifications(ctx, filters, opts)
	if err != nil {
		h.logError(c, "get_notifications", err)
		return utils.InternalServerErrorResponse(c, "Failed to get notifications", err)
	}

	totalPages := (int(total) + limit - 1) / limit
	response := fiber.Map{
		"notifications": notifications,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    page < totalPages,
			"has_prev":    page > 1,
		},
	}

	// CORREGIDO: Agregado fiber.StatusOK
	return utils.SuccessResponse(c, fiber.StatusOK, "Notifications retrieved successfully", response)
}

func (h *NotificationHandler) GetNotification(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid notification ID", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	notification, err := h.notificationService.GetNotification(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Notification not found")
		}
		h.logError(c, "get_notification", err)
		return utils.InternalServerErrorResponse(c, "Failed to get notification", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, "Notification retrieved successfully", notification)
}

func (h *NotificationHandler) CancelNotification(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid notification ID", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = h.notificationService.CancelNotification(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Notification not found")
		}
		h.logError(c, "cancel_notification", err)
		return utils.InternalServerErrorResponse(c, "Failed to cancel notification", err)
	}
	h.logInfo(c, "cancel_notification", "Notification cancelled successfully", zap.String("id", idParam))
	return utils.SuccessResponse(c, fiber.StatusOK, "Notification cancelled successfully", nil)
}

func (h *NotificationHandler) ResendNotification(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid notification ID", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = h.notificationService.ResendNotification(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Notification not found")
		}
		h.logError(c, "resend_notification", err)
		return utils.InternalServerErrorResponse(c, "Failed to resend notification", err)
	}
	h.logInfo(c, "resend_notification", "Notification resent successfully", zap.String("id", idParam))
	return utils.SuccessResponse(c, fiber.StatusOK, "Notification resent successfully", nil)
}

func (h *NotificationHandler) CreateTemplate(c *fiber.Ctx) error {
	var req CreateTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}
	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}
	createdBy := h.getUserFromContext(c)
	template := &models.EmailTemplate{
		Name: req.Name, Type: req.Type, Language: req.Language, Subject: req.Subject,
		BodyText: req.BodyText, BodyHTML: req.BodyHTML, Description: req.Description,
		Tags: req.Tags, Variables: req.Variables, CreatedBy: createdBy,
	}
	if template.Language == "" {
		template.Language = "en"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := h.notificationService.CreateTemplate(ctx, template)
	if err != nil {
		h.logError(c, "create_template", err)
		return utils.InternalServerErrorResponse(c, "Failed to create template", err)
	}
	h.logInfo(c, "create_template", "Template created successfully",
		zap.String("id", template.ID.Hex()), zap.String("name", template.Name))
	return utils.CreatedResponse(c, "Template created successfully", template)
}

func (h *NotificationHandler) GetTemplates(c *fiber.Ctx) error {
	filters := make(map[string]interface{})
	if notificationType := c.Query("type"); notificationType != "" {
		filters["type"] = models.NotificationType(notificationType)
	}
	if language := c.Query("language"); language != "" {
		filters["language"] = language
	}
	if activeStr := c.Query("active"); activeStr != "" {
		if active, err := strconv.ParseBool(activeStr); err == nil {
			filters["is_active"] = active
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	templates, err := h.notificationService.ListTemplates(ctx, filters)
	if err != nil {
		h.logError(c, "get_templates", err)
		return utils.InternalServerErrorResponse(c, "Failed to get templates", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, "Templates retrieved successfully", templates)
}

func (h *NotificationHandler) GetTemplate(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid template ID", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	template, err := h.notificationService.GetTemplate(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		h.logError(c, "get_template", err)
		return utils.InternalServerErrorResponse(c, "Failed to get template", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, "Template retrieved successfully", template)
}

func (h *NotificationHandler) UpdateTemplate(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid template ID", err)
	}
	var req UpdateTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}
	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	template, err := h.notificationService.GetTemplate(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		return utils.InternalServerErrorResponse(c, "Failed to get template", err)
	}
	if req.Subject != "" {
		template.Subject = req.Subject
	}
	if req.BodyText != "" {
		template.BodyText = req.BodyText
	}
	if req.BodyHTML != "" {
		template.BodyHTML = req.BodyHTML
	}
	if req.Description != "" {
		template.Description = req.Description
	}
	if len(req.Tags) > 0 {
		template.Tags = req.Tags
	}
	if len(req.Variables) > 0 {
		template.Variables = req.Variables
	}
	if req.IsActive != nil {
		template.IsActive = *req.IsActive
	}
	err = h.notificationService.UpdateTemplate(ctx, template)
	if err != nil {
		h.logError(c, "update_template", err)
		return utils.InternalServerErrorResponse(c, "Failed to update template", err)
	}
	h.logInfo(c, "update_template", "Template updated successfully", zap.String("id", idParam))
	return utils.SuccessResponse(c, fiber.StatusOK, "Template updated successfully", template)
}

func (h *NotificationHandler) DeleteTemplate(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid template ID", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = h.notificationService.DeleteTemplate(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		h.logError(c, "delete_template", err)
		return utils.InternalServerErrorResponse(c, "Failed to delete template", err)
	}
	h.logInfo(c, "delete_template", "Template deleted successfully", zap.String("id", idParam))
	return utils.SuccessResponse(c, fiber.StatusOK, "Template deleted successfully", nil)
}

func (h *NotificationHandler) PreviewTemplate(c *fiber.Ctx) error {
	templateName := c.Params("name")
	var req PreviewTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}
	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	preview, err := h.notificationService.PreviewTemplate(ctx, templateName, req.Data)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		h.logError(c, "preview_template", err)
		return utils.InternalServerErrorResponse(c, "Failed to preview template", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, "Template preview generated successfully", preview)
}

func (h *NotificationHandler) GetNotificationStats(c *fiber.Ctx) error {
	timeRangeParam := c.Query("time_range", "24h")
	timeRange, err := time.ParseDuration(timeRangeParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid time range format", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stats, err := h.notificationService.GetNotificationStats(ctx, timeRange)
	if err != nil {
		h.logError(c, "get_notification_stats", err)
		return utils.InternalServerErrorResponse(c, "Failed to get notification stats", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, "Statistics retrieved successfully", stats)
}

func (h *NotificationHandler) GetServiceStats(c *fiber.Ctx) error {
	stats := h.notificationService.GetServiceStats()
	return utils.SuccessResponse(c, fiber.StatusOK, "Service statistics retrieved successfully", stats)
}

func (h *NotificationHandler) GetHealthStatus(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	health := h.notificationService.GetHealthStatus(ctx)
	if health["service"] == "healthy" {
		return utils.SuccessResponse(c, fiber.StatusOK, "System is healthy", health)
	}
	return c.Status(fiber.StatusServiceUnavailable).JSON(utils.Response{
		Success: false, Message: "System is not fully healthy", Data: health, Timestamp: time.Now(),
	})
}

func (h *NotificationHandler) ProcessPendingNotifications(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	err := h.notificationService.ProcessPendingNotifications(ctx)
	if err != nil {
		h.logError(c, "process_pending", err)
		return utils.InternalServerErrorResponse(c, "Failed to process pending notifications", err)
	}
	h.logInfo(c, "process_pending", "Pending notifications processed successfully")
	return utils.SuccessResponse(c, fiber.StatusOK, "Pending notifications processed successfully", nil)
}

func (h *NotificationHandler) RetryFailedNotifications(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	err := h.notificationService.RetryFailedNotifications(ctx)
	if err != nil {
		h.logError(c, "retry_failed", err)
		return utils.InternalServerErrorResponse(c, "Failed to retry failed notifications", err)
	}
	h.logInfo(c, "retry_failed", "Failed notifications retry completed")
	return utils.SuccessResponse(c, fiber.StatusOK, "Failed notifications retry completed", nil)
}

func (h *NotificationHandler) CleanupOldNotifications(c *fiber.Ctx) error {
	olderThanParam := c.Query("older_than", "90d")
	olderThan, err := time.ParseDuration(olderThanParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid older_than format", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	deletedCount, err := h.notificationService.CleanupOldNotifications(ctx, olderThan)
	if err != nil {
		h.logError(c, "cleanup_old", err)
		return utils.InternalServerErrorResponse(c, "Failed to cleanup old notifications", err)
	}
	h.logInfo(c, "cleanup_old", "Old notifications cleaned up",
		zap.Int64("deleted_count", deletedCount), zap.Duration("older_than", olderThan))
	response := fiber.Map{"deleted_count": deletedCount, "older_than": olderThanParam}
	return utils.SuccessResponse(c, fiber.StatusOK, "Old notifications cleaned up successfully", response)
}

func (h *NotificationHandler) TestEmailConfiguration(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := h.notificationService.TestEmailConfiguration(ctx)
	if err != nil {
		h.logError(c, "test_config", err)
		return utils.InternalServerErrorResponse(c, "Email configuration test failed", err)
	}
	h.logInfo(c, "test_config", "Email configuration test successful")
	return utils.SuccessResponse(c, fiber.StatusOK, "Email configuration test successful", nil)
}

func (h *NotificationHandler) CreateDefaultTemplates(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := h.notificationService.CreateDefaultTemplates(ctx)
	if err != nil {
		h.logError(c, "create_default_templates", err)
		return utils.InternalServerErrorResponse(c, "Failed to create default templates", err)
	}
	h.logInfo(c, "create_default_templates", "Default templates created successfully")
	return utils.SuccessResponse(c, fiber.StatusOK, "Default templates created successfully", nil)
}

func (h *NotificationHandler) getUserIDFromContext(c *fiber.Ctx) *primitive.ObjectID {
	userID := c.Locals("user_id")
	if userID == nil {
		return nil
	}
	if oid, ok := userID.(primitive.ObjectID); ok {
		return &oid
	}
	if str, ok := userID.(string); ok {
		if oid, err := primitive.ObjectIDFromHex(str); err == nil {
			return &oid
		}
	}
	return nil
}

func (h *NotificationHandler) getUserFromContext(c *fiber.Ctx) string {
	user := c.Locals("user_name")
	if user == nil {
		return "system"
	}
	if str, ok := user.(string); ok {
		return str
	}
	return "system"
}

func (h *NotificationHandler) logError(c *fiber.Ctx, action string, err error) {
	userID := "unknown"
	if uid := h.getUserIDFromContext(c); uid != nil {
		userID = uid.Hex()
	}
	h.logger.Error("Notification handler error",
		zap.String("action", action), zap.String("method", c.Method()), zap.String("path", c.Path()),
		zap.String("ip", c.IP()), zap.String("user_id", userID),
		zap.String("user_agent", c.Get("User-Agent")), zap.Error(err))
}

func (h *NotificationHandler) logInfo(c *fiber.Ctx, action, message string, fields ...zap.Field) {
	userID := "unknown"
	if uid := h.getUserIDFromContext(c); uid != nil {
		userID = uid.Hex()
	}
	allFields := []zap.Field{
		zap.String("action", action), zap.String("method", c.Method()),
		zap.String("path", c.Path()), zap.String("ip", c.IP()), zap.String("user_id", userID),
	}
	allFields = append(allFields, fields...)
	h.logger.Info(message, allFields...)
}

func (h *NotificationHandler) validateNotificationAccess(c *fiber.Ctx, notification *models.EmailNotification) bool {
	userID := h.getUserIDFromContext(c)
	if userID == nil {
		return false
	}
	if h.isAdmin(c) {
		return true
	}
	if notification.UserID != nil && notification.UserID.Hex() == userID.Hex() {
		return true
	}
	if notification.UserID == nil {
		return true
	}
	return false
}

func (h *NotificationHandler) isAdmin(c *fiber.Ctx) bool {
	userRole := c.Locals("user_role")
	return userRole == "admin"
}

func (h *NotificationHandler) parseTimeRange(timeRangeStr string) time.Duration {
	if timeRangeStr == "" {
		return 24 * time.Hour
	}
	duration, err := time.ParseDuration(timeRangeStr)
	if err != nil {
		return 24 * time.Hour
	}
	if duration > 365*24*time.Hour {
		return 365 * 24 * time.Hour
	}
	return duration
}

func (h *NotificationHandler) sanitizeFilters(filters map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for key, value := range filters {
		switch key {
		case "status":
			if status := models.NotificationStatus(value.(string)); status.IsValid() {
				sanitized[key] = status
			}
		case "type":
			if notifType := models.NotificationType(value.(string)); notifType.IsValid() {
				sanitized[key] = notifType
			}
		case "priority":
			if priority := models.NotificationPriority(value.(string)); priority.IsValid() {
				sanitized[key] = priority
			}
		default:
			sanitized[key] = value
		}
	}
	return sanitized
}
