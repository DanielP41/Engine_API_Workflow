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

// ================================
// ESTRUCTURA PRINCIPAL DEL HANDLER
// ================================

// NotificationHandler maneja las operaciones de notificaciones por email
type NotificationHandler struct {
	notificationService *services.NotificationService
	validator           *utils.Validator
	logger              *zap.Logger
}

// NewNotificationHandler crea una nueva instancia del handler
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

// ================================
// ESTRUCTURAS DE REQUEST/RESPONSE
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

// SendSimpleEmailRequest estructura para email simple
type SendSimpleEmailRequest struct {
	To      []string `json:"to" validate:"required,min=1"`
	Subject string   `json:"subject" validate:"required"`
	Body    string   `json:"body" validate:"required"`
	IsHTML  bool     `json:"is_html,omitempty"`
}

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

// PreviewTemplateRequest estructura para vista previa
type PreviewTemplateRequest struct {
	Data map[string]interface{} `json:"data" validate:"required"`
}

// ================================
// ENDPOINTS DE ENVÍO DE EMAILS
// ================================

// SendEmail envía un email
// @Summary Enviar email
// @Description Envía un email directo o usando template
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body SendEmailRequest true "Datos del email"
// @Security BearerAuth
// @Success 201 {object} utils.Response{data=models.EmailNotification}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/send [post]
func (h *NotificationHandler) SendEmail(c *fiber.Ctx) error {
	var req SendEmailRequest

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	// Validar que se proporcione contenido o template
	if req.Subject == "" && req.TemplateName == "" {
		return utils.BadRequestResponse(c, "Subject or template_name is required", nil)
	}

	if req.Body == "" && req.TemplateName == "" {
		return utils.BadRequestResponse(c, "Body or template_name is required", nil)
	}

	// Convertir a modelo de servicio
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

// SendSimpleEmail envía un email simple
// @Summary Enviar email simple
// @Description Envía un email simple sin template
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body SendSimpleEmailRequest true "Datos del email simple"
// @Security BearerAuth
// @Success 201 {object} utils.Response{data=models.EmailNotification}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/send/simple [post]
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

// SendTemplatedEmail envía un email usando template
// @Summary Enviar email con template
// @Description Envía un email usando un template predefinido
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body SendTemplatedEmailRequest true "Datos del email con template"
// @Security BearerAuth
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/send/template [post]
func (h *NotificationHandler) SendTemplatedEmail(c *fiber.Ctx) error {
	var req SendTemplatedEmailRequest

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", err)
	}

	if err := h.validator.Validate(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	// Establecer prioridad por defecto
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

// SendWelcomeEmail envía email de bienvenida
// @Summary Enviar email de bienvenida
// @Description Envía un email de bienvenida a un nuevo usuario
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body fiber.Map true "Email y nombre del usuario"
// @Security BearerAuth
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/send/welcome [post]
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

// SendSystemAlert envía alerta del sistema
// @Summary Enviar alerta del sistema
// @Description Envía una alerta del sistema a los administradores
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body fiber.Map true "Datos de la alerta"
// @Security BearerAuth
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/send/alert [post]
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

// ================================
// GESTIÓN DE NOTIFICACIONES
// ================================

// GetNotifications obtiene lista de notificaciones con filtros
// @Summary Listar notificaciones
// @Description Obtiene una lista paginada de notificaciones con filtros opcionales
// @Tags notifications
// @Accept json
// @Produce json
// @Param status query string false "Estado de la notificación"
// @Param type query string false "Tipo de notificación"
// @Param priority query string false "Prioridad de la notificación"
// @Param user_id query string false "ID del usuario"
// @Param workflow_id query string false "ID del workflow"
// @Param execution_id query string false "ID de ejecución"
// @Param to_email query string false "Email del destinatario"
// @Param page query int false "Número de página" default(1)
// @Param limit query int false "Elementos por página" default(50)
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=fiber.Map}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications [get]
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
	if page < 1 {
		page = 1
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
		Limit:    limit,
		Skip:     (page - 1) * limit,
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

	return utils.SuccessResponse(c, "Notifications retrieved successfully", response)
}

// GetNotification obtiene una notificación específica
// @Summary Obtener notificación
// @Description Obtiene los detalles de una notificación específica
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path string true "ID de la notificación"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.EmailNotification}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/{id} [get]
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

	return utils.SuccessResponse(c, "Notification retrieved successfully", notification)
}

// CancelNotification cancela una notificación
// @Summary Cancelar notificación
// @Description Cancela una notificación pendiente o programada
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path string true "ID de la notificación"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/{id}/cancel [post]
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

	h.logInfo(c, "cancel_notification", "Notification cancelled successfully",
		zap.String("id", idParam))

	return utils.SuccessResponse(c, "Notification cancelled successfully", nil)
}

// ResendNotification reenvía una notificación
// @Summary Reenviar notificación
// @Description Reenvía una notificación fallida o cancelada
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path string true "ID de la notificación"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/{id}/resend [post]
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

	h.logInfo(c, "resend_notification", "Notification resent successfully",
		zap.String("id", idParam))

	return utils.SuccessResponse(c, "Notification resent successfully", nil)
}

// ================================
// GESTIÓN DE TEMPLATES
// ================================

// CreateTemplate crea un nuevo template
// @Summary Crear template
// @Description Crea un nuevo template de email
// @Tags templates
// @Accept json
// @Produce json
// @Param request body CreateTemplateRequest true "Datos del template"
// @Security BearerAuth
// @Success 201 {object} utils.Response{data=models.EmailTemplate}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/templates [post]
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
		h.logError(c, "create_template", err)
		return utils.InternalServerErrorResponse(c, "Failed to create template", err)
	}

	h.logInfo(c, "create_template", "Template created successfully",
		zap.String("id", template.ID.Hex()),
		zap.String("name", template.Name))

	return utils.CreatedResponse(c, "Template created successfully", template)
}

// GetTemplates obtiene lista de templates
// @Summary Listar templates
// @Description Obtiene una lista de templates con filtros opcionales
// @Tags templates
// @Accept json
// @Produce json
// @Param type query string false "Tipo de notificación"
// @Param language query string false "Idioma del template"
// @Param active query bool false "Solo templates activos"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=[]models.EmailTemplate}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/templates [get]
func (h *NotificationHandler) GetTemplates(c *fiber.Ctx) error {
	// Construir filtros
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

	return utils.SuccessResponse(c, "Templates retrieved successfully", templates)
}

// GetTemplate obtiene un template específico
// @Summary Obtener template
// @Description Obtiene los detalles de un template específico
// @Tags templates
// @Accept json
// @Produce json
// @Param id path string true "ID del template"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.EmailTemplate}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/templates/{id} [get]
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

	return utils.SuccessResponse(c, "Template retrieved successfully", template)
}

// UpdateTemplate actualiza un template
// @Summary Actualizar template
// @Description Actualiza un template existente
// @Tags templates
// @Accept json
// @Produce json
// @Param id path string true "ID del template"
// @Param request body UpdateTemplateRequest true "Datos de actualización"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.EmailTemplate}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/templates/{id} [put]
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

	// Obtener template existente
	template, err := h.notificationService.GetTemplate(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return utils.NotFoundResponse(c, "Template not found")
		}
		return utils.InternalServerErrorResponse(c, "Failed to get template", err)
	}

	// Actualizar campos proporcionados
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

	h.logInfo(c, "update_template", "Template updated successfully",
		zap.String("id", idParam))

	return utils.SuccessResponse(c, "Template updated successfully", template)
}

// DeleteTemplate elimina un template
// @Summary Eliminar template
// @Description Elimina un template existente
// @Tags templates
// @Accept json
// @Produce json
// @Param id path string true "ID del template"
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/templates/{id} [delete]
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

	h.logInfo(c, "delete_template", "Template deleted successfully",
		zap.String("id", idParam))

	return utils.SuccessResponse(c, "Template deleted successfully", nil)
}

// PreviewTemplate genera vista previa de un template
// @Summary Vista previa de template
// @Description Genera una vista previa de un template con datos de prueba
// @Tags templates
// @Accept json
// @Produce json
// @Param name path string true "Nombre del template"
// @Param request body PreviewTemplateRequest true "Datos para la vista previa"
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.TemplatePreview}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/templates/{name}/preview [post]
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

	return utils.SuccessResponse(c, "Template preview generated successfully", preview)
}

// ================================
// ESTADÍSTICAS Y MONITOREO
// ================================

// GetNotificationStats obtiene estadísticas de notificaciones
// @Summary Estadísticas de notificaciones
// @Description Obtiene estadísticas de notificaciones para un rango de tiempo
// @Tags stats
// @Accept json
// @Produce json
// @Param time_range query string false "Rango de tiempo (ej: 24h, 7d, 30d)" default(24h)
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=models.NotificationStats}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/stats [get]
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

	return utils.SuccessResponse(c, "Statistics retrieved successfully", stats)
}

// GetServiceStats obtiene estadísticas del servicio
// @Summary Estadísticas del servicio
// @Description Obtiene estadísticas en tiempo real del servicio de notificaciones
// @Tags stats
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=services.ServiceStats}
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/stats/service [get]
func (h *NotificationHandler) GetServiceStats(c *fiber.Ctx) error {
	stats := h.notificationService.GetServiceStats()
	return utils.SuccessResponse(c, "Service statistics retrieved successfully", stats)
}

// GetHealthStatus verifica el estado de salud del servicio
// @Summary Estado de salud
// @Description Verifica el estado de salud del sistema de notificaciones
// @Tags health
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=fiber.Map}
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/health [get]
func (h *NotificationHandler) GetHealthStatus(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health := h.notificationService.GetHealthStatus(ctx)

	// Determinar código de respuesta basado en el estado
	if health["service"] == "healthy" {
		return utils.SuccessResponse(c, "System is healthy", health)
	} else {
		return c.Status(fiber.StatusServiceUnavailable).JSON(utils.Response{
			Success:   false,
			Message:   "System is not fully healthy",
			Data:      health,
			Timestamp: time.Now(),
		})
	}
}

// ================================
// ADMINISTRACIÓN Y MANTENIMIENTO
// ================================

// ProcessPendingNotifications procesa manualmente las notificaciones pendientes
// @Summary Procesar notificaciones pendientes
// @Description Procesa manualmente todas las notificaciones pendientes
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/admin/process-pending [post]
func (h *NotificationHandler) ProcessPendingNotifications(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := h.notificationService.ProcessPendingNotifications(ctx)
	if err != nil {
		h.logError(c, "process_pending", err)
		return utils.InternalServerErrorResponse(c, "Failed to process pending notifications", err)
	}

	h.logInfo(c, "process_pending", "Pending notifications processed successfully")

	return utils.SuccessResponse(c, "Pending notifications processed successfully", nil)
}

// RetryFailedNotifications reintenta notificaciones fallidas
// @Summary Reintentar notificaciones fallidas
// @Description Reintenta todas las notificaciones fallidas que pueden ser reintentadas
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/admin/retry-failed [post]
func (h *NotificationHandler) RetryFailedNotifications(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := h.notificationService.RetryFailedNotifications(ctx)
	if err != nil {
		h.logError(c, "retry_failed", err)
		return utils.InternalServerErrorResponse(c, "Failed to retry failed notifications", err)
	}

	h.logInfo(c, "retry_failed", "Failed notifications retry completed")

	return utils.SuccessResponse(c, "Failed notifications retry completed", nil)
}

// CleanupOldNotifications limpia notificaciones antiguas
// @Summary Limpiar notificaciones antiguas
// @Description Elimina notificaciones antiguas del sistema
// @Tags admin
// @Accept json
// @Produce json
// @Param older_than query string false "Eliminar notificaciones más antiguas que (ej: 90d)" default(90d)
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=fiber.Map}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/admin/cleanup [delete]
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
		zap.Int64("deleted_count", deletedCount),
		zap.Duration("older_than", olderThan))

	response := fiber.Map{
		"deleted_count": deletedCount,
		"older_than":    olderThanParam,
	}

	return utils.SuccessResponse(c, "Old notifications cleaned up successfully", response)
}

// TestEmailConfiguration prueba la configuración de email
// @Summary Probar configuración de email
// @Description Prueba la configuración SMTP enviando un email de prueba
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/admin/test-config [post]
func (h *NotificationHandler) TestEmailConfiguration(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := h.notificationService.TestEmailConfiguration(ctx)
	if err != nil {
		h.logError(c, "test_config", err)
		return utils.InternalServerErrorResponse(c, "Email configuration test failed", err)
	}

	h.logInfo(c, "test_config", "Email configuration test successful")

	return utils.SuccessResponse(c, "Email configuration test successful", nil)
}

// CreateDefaultTemplates crea templates por defecto del sistema
// @Summary Crear templates por defecto
// @Description Crea los templates por defecto del sistema si no existen
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/notifications/admin/default-templates [post]
func (h *NotificationHandler) CreateDefaultTemplates(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := h.notificationService.CreateDefaultTemplates(ctx)
	if err != nil {
		h.logError(c, "create_default_templates", err)
		return utils.InternalServerErrorResponse(c, "Failed to create default templates", err)
	}

	h.logInfo(c, "create_default_templates", "Default templates created successfully")

	return utils.SuccessResponse(c, "Default templates created successfully", nil)
}

// ================================
// MÉTODOS AUXILIARES
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
	userID := "unknown"
	if uid := h.getUserIDFromContext(c); uid != nil {
		userID = uid.Hex()
	}

	h.logger.Error("Notification handler error",
		zap.String("action", action),
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.String("ip", c.IP()),
		zap.String("user_id", userID),
		zap.String("user_agent", c.Get("User-Agent")),
		zap.Error(err))
}

// logInfo registra información con contexto
func (h *NotificationHandler) logInfo(c *fiber.Ctx, action, message string, fields ...zap.Field) {
	userID := "unknown"
	if uid := h.getUserIDFromContext(c); uid != nil {
		userID = uid.Hex()
	}

	allFields := []zap.Field{
		zap.String("action", action),
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.String("ip", c.IP()),
		zap.String("user_id", userID),
	}
	allFields = append(allFields, fields...)

	h.logger.Info(message, allFields...)
}

// validateNotificationAccess valida si el usuario tiene acceso a una notificación
func (h *NotificationHandler) validateNotificationAccess(c *fiber.Ctx, notification *models.EmailNotification) bool {
	userID := h.getUserIDFromContext(c)
	if userID == nil {
		return false
	}

	// Si es admin, tiene acceso a todas las notificaciones
	if h.isAdmin(c) {
		return true
	}

	// Si la notificación pertenece al usuario, tiene acceso
	if notification.UserID != nil && notification.UserID.Hex() == userID.Hex() {
		return true
	}

	// Si no tiene user_id asociado, cualquier usuario autenticado puede verla
	if notification.UserID == nil {
		return true
	}

	return false
}

// isAdmin verifica si el usuario es administrador
func (h *NotificationHandler) isAdmin(c *fiber.Ctx) bool {
	userRole := c.Locals("user_role")
	return userRole == "admin"
}

// parseTimeRange convierte string a time.Duration con valores por defecto
func (h *NotificationHandler) parseTimeRange(timeRangeStr string) time.Duration {
	if timeRangeStr == "" {
		return 24 * time.Hour
	}

	duration, err := time.ParseDuration(timeRangeStr)
	if err != nil {
		// Si no se puede parsear, usar valor por defecto
		return 24 * time.Hour
	}

	// Limitar el rango máximo a 1 año
	if duration > 365*24*time.Hour {
		return 365 * 24 * time.Hour
	}

	return duration
}

// sanitizeFilters sanitiza y valida los filtros de entrada
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
