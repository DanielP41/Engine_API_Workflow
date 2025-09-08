package handlers

import (
	"strconv"
	"time"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkflowHandler struct {
	workflowService services.WorkflowService
	logService      services.LogService
	validator       utils.Validator
}

func NewWorkflowHandler(workflowService services.WorkflowService, logService services.LogService, validator utils.Validator) *WorkflowHandler {
	return &WorkflowHandler{
		workflowService: workflowService,
		logService:      logService,
		validator:       validator,
	}
}

// CreateWorkflow godoc
// @Summary Create a new workflow
// @Description Create a new workflow with steps and triggers
// @Tags workflows
// @Accept json
// @Produce json
// @Param workflow body models.CreateWorkflowRequest true "Workflow data"
// @Success 201 {object} utils.APIResponse{data=models.WorkflowResponse}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Security Bearer
// @Router /api/v1/workflows [post]
func (h *WorkflowHandler) CreateWorkflow(c *fiber.Ctx) error {
	var req models.CreateWorkflowRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid JSON format", err)
	}

	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	// Get user from context (set by auth middleware)
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", nil)
	}

	// Create workflow
	workflow, err := h.workflowService.Create(c.Context(), &req, userID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create workflow", err)
	}

	return utils.SuccessResponse(c, fiber.StatusCreated, "Workflow created successfully", workflow)
}

// GetWorkflows godoc
// @Summary Get workflows with pagination and filters
// @Description Get a list of workflows with optional filtering and pagination
// @Tags workflows
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param sort_by query string false "Sort field" default("created_at")
// @Param sort_order query string false "Sort order (asc/desc)" default("desc")
// @Param status query string false "Workflow status filter"
// @Param search query string false "Search in name and description"
// @Param tags query string false "Comma-separated tags"
// @Success 200 {object} utils.APIResponse{data=models.WorkflowListResponse}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Security Bearer
// @Router /api/v1/workflows [get]
func (h *WorkflowHandler) GetWorkflows(c *fiber.Ctx) error {
	// Parse pagination parameters
	pagination, err := h.parsePaginationParams(c)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid pagination parameters", err)
	}

	// Parse filters
	filters := h.parseWorkflowFilters(c)

	// Get user from context
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", nil)
	}

	// Check if user is admin (admins can see all workflows)
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !isAdmin {
		filters.UserID = &userID
	}

	// Get workflows - CORREGIDO: ahora coincide con el servicio que retorna []*models.Workflow
	workflows, total, err := h.workflowService.Search(c.Context(), filters, pagination)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get workflows", err)
	}

	// Build response
	totalPages := int((total + int64(pagination.PageSize) - 1) / int64(pagination.PageSize))

	// CORREGIDO: Crear la respuesta correctamente según el modelo
	response := map[string]interface{}{
		"workflows":   workflows, // workflows es []*models.Workflow
		"total":       total,
		"page":        pagination.Page,
		"page_size":   pagination.PageSize,
		"total_pages": totalPages,
		"has_next":    pagination.Page < totalPages,
		"has_prev":    pagination.Page > 1,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Workflows retrieved successfully", response)
}

// GetWorkflow godoc
// @Summary Get workflow by ID
// @Description Get a single workflow by its ID
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} utils.APIResponse{data=models.WorkflowResponse}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Security Bearer
// @Router /api/v1/workflows/{id} [get]
func (h *WorkflowHandler) GetWorkflow(c *fiber.Ctx) error {
	// Parse workflow ID
	workflowID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid workflow ID", err)
	}

	// Get user from context
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", nil)
	}

	// Get workflow
	workflow, err := h.workflowService.GetByID(c.Context(), workflowID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get workflow", err)
	}

	if workflow == nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Workflow not found", nil)
	}

	// Check ownership (admins can access all workflows)
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !isAdmin && workflow.UserID != userID {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Access denied", nil)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Workflow retrieved successfully", workflow)
}

// UpdateWorkflow godoc
// @Summary Update workflow
// @Description Update an existing workflow
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param workflow body models.UpdateWorkflowRequest true "Workflow update data"
// @Success 200 {object} utils.APIResponse{data=models.WorkflowResponse}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Security Bearer
// @Router /api/v1/workflows/{id} [put]
func (h *WorkflowHandler) UpdateWorkflow(c *fiber.Ctx) error {
	// Parse workflow ID
	workflowID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid workflow ID", err)
	}

	var req models.UpdateWorkflowRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid JSON format", err)
	}

	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		return utils.ValidationErrorResponse(c, "Validation failed", err)
	}

	// Get user from context
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", nil)
	}

	// Check if workflow exists and user has permission
	existingWorkflow, err := h.workflowService.GetByID(c.Context(), workflowID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get workflow", err)
	}

	if existingWorkflow == nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Workflow not found", nil)
	}

	// Check ownership (admins can update all workflows)
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !isAdmin && existingWorkflow.UserID != userID {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Access denied", nil)
	}

	// Update workflow
	workflow, err := h.workflowService.Update(c.Context(), workflowID, &req, userID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update workflow", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Workflow updated successfully", workflow)
}

// DeleteWorkflow godoc
// @Summary Delete workflow
// @Description Delete a workflow (soft delete)
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Security Bearer
// @Router /api/v1/workflows/{id} [delete]
func (h *WorkflowHandler) DeleteWorkflow(c *fiber.Ctx) error {
	// Parse workflow ID
	workflowID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid workflow ID", err)
	}

	// Get user from context
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", nil)
	}

	// Check if workflow exists and user has permission
	existingWorkflow, err := h.workflowService.GetByID(c.Context(), workflowID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get workflow", err)
	}

	if existingWorkflow == nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Workflow not found", nil)
	}

	// Check ownership (admins can delete all workflows)
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !isAdmin && existingWorkflow.UserID != userID {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Access denied", nil)
	}

	// Delete workflow
	err = h.workflowService.Delete(c.Context(), workflowID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete workflow", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Workflow deleted successfully", nil)
}

// ToggleWorkflowStatus godoc
// @Summary Toggle workflow status (active/inactive)
// @Description Activate or deactivate a workflow
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} utils.APIResponse{data=models.WorkflowResponse}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Security Bearer
// @Router /api/v1/workflows/{id}/toggle [post]
func (h *WorkflowHandler) ToggleWorkflowStatus(c *fiber.Ctx) error {
	// Parse workflow ID
	workflowID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid workflow ID", err)
	}

	// Get user from context
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", nil)
	}

	// Toggle status
	workflow, err := h.workflowService.ToggleStatus(c.Context(), workflowID, userID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to toggle workflow status", err)
	}

	if workflow == nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Workflow not found", nil)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Workflow status updated successfully", workflow)
}

// GetWorkflowStats godoc
// @Summary Get workflow statistics
// @Description Get execution statistics for a specific workflow
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param days query int false "Number of days for stats" default(30)
// @Success 200 {object} utils.APIResponse{data=models.LogStats}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Security Bearer
// @Router /api/v1/workflows/{id}/stats [get]
func (h *WorkflowHandler) GetWorkflowStats(c *fiber.Ctx) error {
	// Parse workflow ID
	workflowID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid workflow ID", err)
	}

	// Parse days parameter
	daysStr := c.Query("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 30
	}

	// Get user from context
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", nil)
	}

	// Check if workflow exists and user has permission
	workflow, err := h.workflowService.GetByID(c.Context(), workflowID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get workflow", err)
	}

	if workflow == nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Workflow not found", nil)
	}

	// Check ownership (admins can view all stats)
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !isAdmin && workflow.UserID != userID {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Access denied", nil)
	}

	// Get stats
	stats, err := h.logService.GetWorkflowStats(c.Context(), workflowID, days)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get workflow stats", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Workflow stats retrieved successfully", stats)
}

// CloneWorkflow godoc
// @Summary Clone workflow
// @Description Create a copy of an existing workflow
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param clone_data body struct{name string; description string} false "Clone options"
// @Success 201 {object} utils.APIResponse{data=models.WorkflowResponse}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Security Bearer
// @Router /api/v1/workflows/{id}/clone [post]
func (h *WorkflowHandler) CloneWorkflow(c *fiber.Ctx) error {
	// Parse workflow ID
	workflowID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid workflow ID", err)
	}

	// Parse clone options
	var cloneData struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&cloneData); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid JSON format", err)
	}

	// Get user from context
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "User not authenticated", nil)
	}

	// Clone workflow
	clonedWorkflow, err := h.workflowService.Clone(c.Context(), workflowID, userID, cloneData.Name, cloneData.Description)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to clone workflow", err)
	}

	if clonedWorkflow == nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Workflow not found", nil)
	}

	return utils.SuccessResponse(c, fiber.StatusCreated, "Workflow cloned successfully", clonedWorkflow)
}

// Helper functions

// parsePaginationParams parses pagination parameters from query string
func (h *WorkflowHandler) parsePaginationParams(c *fiber.Ctx) (repository.PaginationOptions, error) {
	pageStr := c.Query("page", "1")
	pageSizeStr := c.Query("page_size", "10")
	sortBy := c.Query("sort_by", "created_at")
	sortOrder := c.Query("sort_order", "desc")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	sortDesc := sortOrder == "desc"

	return repository.PaginationOptions{
		Page:     page,
		PageSize: pageSize,
		SortBy:   sortBy,
		SortDesc: sortDesc,
	}, nil
}

// parseWorkflowFilters parses workflow filters from query string
func (h *WorkflowHandler) parseWorkflowFilters(c *fiber.Ctx) repository.WorkflowSearchFilters {
	filters := repository.WorkflowSearchFilters{}

	// Status filter
	if statusStr := c.Query("status"); statusStr != "" {
		status := models.WorkflowStatus(statusStr)
		filters.Status = &status
	}

	// Search filter
	if search := c.Query("search"); search != "" {
		filters.Search = &search
	}

	// Tags filter (comma-separated)
	if tagsStr := c.Query("tags"); tagsStr != "" {
		// Simple split by comma - you might want more sophisticated parsing
		filters.Tags = []string{tagsStr} // Simplified for now
	}

	// Environment filter
	if env := c.Query("environment"); env != "" {
		filters.Environment = &env
	}

	// Active filter
	if activeStr := c.Query("active"); activeStr != "" {
		if active, err := strconv.ParseBool(activeStr); err == nil {
			filters.IsActive = &active
		}
	}

	// Date filters
	if createdAfterStr := c.Query("created_after"); createdAfterStr != "" {
		if createdAfter, err := time.Parse(time.RFC3339, createdAfterStr); err == nil {
			filters.CreatedAfter = &createdAfter
		}
	}

	if createdBeforeStr := c.Query("created_before"); createdBeforeStr != "" {
		if createdBefore, err := time.Parse(time.RFC3339, createdBeforeStr); err == nil {
			filters.CreatedBefore = &createdBefore
		}
	}

	return filters
}
