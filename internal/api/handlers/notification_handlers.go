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

// NotificationHandler maneja las operaciones de notificaciones por email
type NotificationHandler struct {
	notificationService *services.NotificationService
	validator           *utils.Validator
	logger              *zap.Logger
}

// NewNotificationHandler crea una nueva instancia del handler
func NewNotificationHandler(notificationService *services.NotificationService, validator *utils.Validator, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		validator:           validator,
		logger:              logger,
	}
}

// ================================
// EMAIL SENDING ENDPOINTS
// ================================

// SendEmailRequest estructura para la solicitud de envío de email via API
type SendEmailRequest struct {
	Type     models.NotificationType     `json:"type" validate:"required"`
	Priority models.NotificationPriority `json:"priority" validate:"required"`

	// Destinatarios
	To  []string `json:"to" validate:"required,min=1"`
	CC  []string `json:"cc,omitempty"`
	BCC []string `json:"bcc,omitempty"`

	// Contenido directo
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
	IsHTML  bool   `json:"is_html,omitempty"`

	// O usar template
	TemplateName string                 `json:"template_name,omitempty"`
	TemplateData map[string]interface{} `json:"template_data,omitempty"`

	// Programación
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`

	// Configuración
	MaxAttempts int `json:"max_attempts,omitempty"`

	// Metadatos (se obtienen del contexto del usuario)
	WorkflowID  *primitive.ObjectID `json:"workflow_id,omitempty"`
	ExecutionID *string             `json:"execution_id,omitempty"`
}

// SendTemplatedEmailRequest estructura para envío con template
type SendTemplatedEmailRequest struct {
	TemplateName string                      `json:"template_name" validate:"required"`
	To           []string                    `json:"to" validate:"required,min=1"`
	CC           []string                    `json:"cc,omitempty"`
	BCC          []string                    `json:"bcc,omitempty"`
	Data         map[string]interface{}      `json:"data,omitempty"`
	ScheduledAt  *time.Time                  `json:"scheduled_at,omitempty"`
	Priority     models.NotificationPriority `json:"priority,omitempty"`
}

// SendEmail envía un email directo
func (h *NotificationHandler) SendEmail(c *fiber.Ctx) error {
	var req SendEmailRequest

	if err := c.BodyParser(&req); err != nil {
		h.logger.Warn("Invalid request body", zap.Error(err))
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	// Obtener información del usuario del contexto
	userID := h.getUserIDFromContext(c)

	// Convertir a request de servicio
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
		UserID:       userID,
		WorkflowID:   req.WorkflowID,
		ExecutionID:  req.ExecutionID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	notification, err := h.notificationService.SendEmail(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Failed to send email", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to send email", err)
	}

	h.logger.Info("Email sent successfully",
		zap.String("notification_id", notification.ID.Hex()),
		zap.Strings("to", req.To))

	return utils.SuccessResponse(c, "Email sent successfully", fiber.Map{
		"notification_id": notification.ID.Hex(),
		"status":          notification.Status,
		"created_at":      notification.CreatedAt,
	})
}

// SendTemplatedEmail envía un email usando un template
func (h *NotificationHandler) SendTemplatedEmail(c *fiber.Ctx) error {
	var req SendTemplatedEmailRequest

	if err := c.BodyParser(&req); err != nil {
		h.logger.Warn("Invalid request body", zap.Error(err))
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Si no se especifica prioridad, usar normal
	if req.Priority == "" {
		req.Priority = models.NotificationPriorityNormal
	}

	err := h.notificationService.SendTemplatedEmail(ctx, req.TemplateName, req.To, req.Data)
	if err != nil {
		h.logger.Error("Failed to send templated email",
			zap.String("template", req.TemplateName),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to send templated email", err)
	}

	h.logger.Info("Templated email sent successfully",
		zap.String("template", req.TemplateName),
		zap.Strings("to", req.To))

	return utils.SuccessResponse(c, "Templated email sent successfully", nil)
}

// ================================
// NOTIFICATION MANAGEMENT
// ================================

// GetNotifications obtiene lista de notificaciones con filtros
func (h *NotificationHandler) GetNotifications(c *fiber.Ctx) error {
	// Parámetros de consulta
	status := c.Query("status")
	notificationType := c.Query("type")
	priority := c.Query("priority")
	userID := c.Query("user_id")
	workflowID := c.Query("workflow_id")
	executionID := c.Query("execution_id")
	toEmail := c.Query("to_email")

	// Paginación
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if limit > 100 {
		limit = 100 // Límite máximo
	}

	// Construir filtros
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

	// Opciones de paginación
	opts := &repository.PaginationOptions{
		Limit:     limit,
		Offset:    (page - 1) * limit,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Obtener repositorio directamente (en una implementación más limpia, esto iría en el servicio)
	notifications, total, err := h.notificationService.GetNotifications(ctx, filters, opts)
	if err != nil {
		h.logger.Error("Failed to get notifications", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get notifications", err)
	}

	totalPages := (int(total) + limit - 1) / limit

	return utils.SuccessResponse(c, "Notifications retrieved successfully", fiber.Map{
		"notifications": notifications,
		"pagination": fiber.Map{
			"current_page":   page,
			"total_pages":    totalPages,
			"total_items":    total,
			"items_per_page": limit,
		},
	})
}

// GetNotification obtiene una notificación específica
func (h *NotificationHandler) GetNotification(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid notification ID", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	notification, err := h.notificationService.GetNotification(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Notification not found")
		}
		h.logger.Error("Failed to get notification", zap.String("id", idParam), zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get notification", err)
	}

	return utils.SuccessResponse(c, "Notification retrieved successfully", notification)
}

// RetryNotification reintenta una notificación fallida
func (h *NotificationHandler) RetryNotification(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid notification ID", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = h.notificationService.RetryNotification(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Notification not found")
		}
		h.logger.Error("Failed to retry notification", zap.String("id", idParam), zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to retry notification", err)
	}

	h.logger.Info("Notification retry initiated", zap.String("id", idParam))

	return utils.SuccessResponse(c, "Notification retry initiated", nil)
}

// CancelNotification cancela una notificación
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
		h.logger.Error("Failed to cancel notification", zap.String("id", idParam), zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to cancel notification", err)
	}

	h.logger.Info("Notification cancelled", zap.String("id", idParam))

	return utils.SuccessResponse(c, "Notification cancelled successfully", nil)
}

// ================================
// TEMPLATE MANAGEMENT
// ================================

// CreateTemplateRequest estructura para crear template
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

// UpdateTemplateRequest estructura para actualizar template
type UpdateTemplateRequest struct {
	Subject     string                    `json:"subject,omitempty"`
	BodyText    string                    `json:"body_text,omitempty"`
	BodyHTML    string                    `json:"body_html,omitempty"`
	Description string                    `json:"description,omitempty"`
	Tags        []string                  `json:"tags,omitempty"`
	Variables   []models.TemplateVariable `json:"variables,omitempty"`
	IsActive    *bool                     `json:"is_active,omitempty"`
}

// CreateTemplate crea un nuevo template
func (h *NotificationHandler) CreateTemplate(c *fiber.Ctx) error {
	var req CreateTemplateRequest

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	// Obtener usuario del contexto
	createdBy := h.getUserFromContext(c)

	template := &models.EmailTemplate{
		Name:        req.Name,
		Type:        req.Type,
		Language:    req.Language,
		Subject:     req.Subject,
		BodyText:    req.BodyText,
		BodyHTML:    req.BodyHTML,
		Description: req.Description,
		Tags:        req.Tags,
		Variables:   req.Variables,
		CreatedBy:   createdBy,
	}

	// Establecer idioma por defecto
	if template.Language == "" {
		template.Language = "en"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.notificationService.CreateTemplate(ctx, template)
	if err != nil {
		if repository.IsAlreadyExistsError(err) {
			return utils.ConflictResponse(c, "Template already exists")
		}
		h.logger.Error("Failed to create template", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to create template", err)
	}

	h.logger.Info("Template created successfully",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name))

	return utils.CreatedResponse(c, "Template created successfully", fiber.Map{
		"id":      template.ID.Hex(),
		"name":    template.Name,
		"version": template.Version,
	})
}

// GetTemplates obtiene lista de templates
func (h *NotificationHandler) GetTemplates(c *fiber.Ctx) error {
	// Parámetros de consulta
	templateType := c.Query("type")
	language := c.Query("language")
	isActive := c.Query("is_active")
	search := c.Query("search")

	filters := make(map[string]interface{})

	if templateType != "" {
		filters["type"] = models.NotificationType(templateType)
	}
	if language != "" {
		filters["language"] = language
	}
	if isActive != "" {
		if active, err := strconv.ParseBool(isActive); err == nil {
			filters["is_active"] = active
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var templates []*models.EmailTemplate
	var err error

	if search != "" {
		// Búsqueda por texto
		templates, err = h.notificationService.SearchTemplates(ctx, search, 50)
	} else {
		templates, err = h.notificationService.ListTemplates(ctx, filters)
	}

	if err != nil {
		h.logger.Error("Failed to get templates", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get templates", err)
	}

	return utils.SuccessResponse(c, "Templates retrieved successfully", fiber.Map{
		"templates": templates,
		"count":     len(templates),
	})
}

// GetTemplate obtiene un template específico
func (h *NotificationHandler) GetTemplate(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid template ID", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	template, err := h.notificationService.GetTemplate(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		h.logger.Error("Failed to get template", zap.String("id", idParam), zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get template", err)
	}

	return utils.SuccessResponse(c, "Template retrieved successfully", template)
}

// UpdateTemplate actualiza un template
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Obtener template existente
	template, err := h.notificationService.GetTemplate(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		return utils.InternalServerErrorResponse(c, "Failed to get template", err)
	}

	// Actualizar campos
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
	if req.Tags != nil {
		template.Tags = req.Tags
	}
	if req.Variables != nil {
		template.Variables = req.Variables
	}
	if req.IsActive != nil {
		template.IsActive = *req.IsActive
	}

	err = h.notificationService.UpdateTemplate(ctx, template)
	if err != nil {
		h.logger.Error("Failed to update template", zap.String("id", idParam), zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to update template", err)
	}

	h.logger.Info("Template updated successfully", zap.String("id", idParam))

	return utils.SuccessResponse(c, "Template updated successfully", template)
}

// DeleteTemplate elimina un template
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
		h.logger.Error("Failed to delete template", zap.String("id", idParam), zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to delete template", err)
	}

	h.logger.Info("Template deleted successfully", zap.String("id", idParam))

	return utils.SuccessResponse(c, "Template deleted successfully", nil)
}

// PreviewTemplate genera vista previa de un template
func (h *NotificationHandler) PreviewTemplate(c *fiber.Ctx) error {
	templateName := c.Params("name")

	var req struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	preview, err := h.notificationService.PreviewTemplate(ctx, templateName, req.Data)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		h.logger.Error("Failed to preview template", zap.String("template", templateName), zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to preview template", err)
	}

	return utils.SuccessResponse(c, "Template preview generated", preview)
}

// ================================
// STATISTICS AND MONITORING
// ================================

// GetNotificationStats obtiene estadísticas de notificaciones
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
		h.logger.Error("Failed to get notification stats", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get notification stats", err)
	}

	return utils.SuccessResponse(c, "Statistics retrieved successfully", stats)
}

// TestEmailConfiguration prueba la configuración de email
func (h *NotificationHandler) TestEmailConfiguration(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := h.notificationService.TestEmailConfiguration(ctx)
	if err != nil {
		h.logger.Error("Email configuration test failed", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Email configuration test failed", err)
	}

	h.logger.Info("Email configuration test successful")

	return utils.SuccessResponse(c, "Email configuration test successful", nil)
}

// ================================
// SYSTEM OPERATIONS
// ================================

// ProcessPendingNotifications procesa notificaciones pendientes manualmente
func (h *NotificationHandler) ProcessPendingNotifications(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := h.notificationService.ProcessPendingNotifications(ctx)
	if err != nil {
		h.logger.Error("Failed to process pending notifications", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to process pending notifications", err)
	}

	h.logger.Info("Pending notifications processed successfully")

	return utils.SuccessResponse(c, "Pending notifications processed successfully", nil)
}

// RetryFailedNotifications reintenta notificaciones fallidas
func (h *NotificationHandler) RetryFailedNotifications(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := h.notificationService.RetryFailedNotifications(ctx)
	if err != nil {
		h.logger.Error("Failed to retry failed notifications", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to retry failed notifications", err)
	}

	h.logger.Info("Failed notifications retry completed")

	return utils.SuccessResponse(c, "Failed notifications retry completed", nil)
}

// CleanupOldNotifications limpia notificaciones antiguas
func (h *NotificationHandler) CleanupOldNotifications(c *fiber.Ctx) error {
	olderThanParam := c.Query("older_than", "720h") // 30 días por defecto
	olderThan, err := time.ParseDuration(olderThanParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid older_than duration format", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deletedCount, err := h.notificationService.CleanupOldNotifications(ctx, olderThan)
	if err != nil {
		h.logger.Error("Failed to cleanup old notifications", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to cleanup old notifications", err)
	}

	h.logger.Info("Old notifications cleaned up", zap.Int64("count", deletedCount))

	return utils.SuccessResponse(c, "Old notifications cleaned up successfully", fiber.Map{
		"deleted_count": deletedCount,
	})
}

// ================================
// HELPER METHODS
// ================================

// getUserIDFromContext obtiene el ID del usuario del contexto
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

// getUserFromContext obtiene el nombre del usuario del contexto
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

// logError registra un error con contexto
func (h *NotificationHandler) logError(c *fiber.Ctx, action string, err error) {
	h.logger.Error("Notification handler error",
		zap.String("action", action),
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.String("ip", c.IP()),
		zap.Error(err))
}

// logInfo registra información con contexto
func (h *NotificationHandler) logInfo(c *fiber.Ctx, action, message string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("action", action),
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.String("ip", c.IP()),
	}
	allFields = append(allFields, fields...)

	h.logger.Info(message, allFields...)
}
