// internal/api/handlers/dashboard.go
package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
)

// DashboardHandler maneja las peticiones del dashboard
type DashboardHandler struct {
	dashboardService services.DashboardService
	logger           *zap.Logger
}

// NewDashboardHandler crea una nueva instancia del handler del dashboard
func NewDashboardHandler(dashboardService services.DashboardService, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
		logger:           logger,
	}
}

// GetDashboard obtiene todos los datos del dashboard
// @Summary Get complete dashboard data
// @Description Get comprehensive dashboard data including system health, stats, workflows, and alerts
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param time_range query string false "Time range for metrics" default("24h") Enums(1h, 6h, 12h, 24h, 7d, 30d)
// @Param workflow_ids query string false "Comma-separated workflow IDs to filter"
// @Param user_ids query string false "Comma-separated user IDs to filter"
// @Param status query string false "Comma-separated status values to filter"
// @Param environment query string false "Environment filter"
// @Success 200 {object} utils.DataResponse{data=models.DashboardData}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard [get]
func (h *DashboardHandler) GetDashboard(c *fiber.Ctx) error {
	// Verificar autenticación
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Parsear filtros de query parameters
	filter, err := h.parseFilterFromQuery(c)
	if err != nil {
		h.logger.Warn("Invalid filter parameters", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid filter parameters", err.Error()))
	}

	// Validar filtros
	if err := h.dashboardService.ValidateFilter(filter); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid filter", err.Error()))
	}

	// Obtener datos del dashboard
	dashboardData, err := h.dashboardService.GetCompleteDashboard(c.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get dashboard data", ""))
	}

	h.logger.Info("Dashboard data retrieved successfully",
		zap.String("user_id", userID.Hex()),
		zap.String("time_range", filter.TimeRange))

	return c.JSON(utils.SuccessResponse("Dashboard data retrieved successfully", dashboardData))
}

// GetDashboardSummary obtiene un resumen ejecutivo del dashboard
// @Summary Get dashboard summary
// @Description Get executive summary of dashboard metrics
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse{data=models.DashboardSummary}
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/summary [get]
func (h *DashboardHandler) GetDashboardSummary(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	summary, err := h.dashboardService.GetDashboardSummary(c.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard summary", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get dashboard summary", ""))
	}

	return c.JSON(utils.SuccessResponse("Dashboard summary retrieved successfully", summary))
}

// GetSystemHealth obtiene el estado de salud del sistema
// @Summary Get system health
// @Description Get current system health status and metrics
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse{data=models.SystemHealth}
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/health [get]
func (h *DashboardHandler) GetSystemHealth(c *fiber.Ctx) error {
	systemHealth, err := h.dashboardService.GetSystemHealth(c.Context())
	if err != nil {
		h.logger.Error("Failed to get system health", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get system health", ""))
	}

	return c.JSON(utils.SuccessResponse("System health retrieved successfully", systemHealth))
}

// GetQuickStats obtiene estadísticas rápidas
// @Summary Get quick stats
// @Description Get quick dashboard statistics
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse{data=models.QuickStats}
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/stats [get]
func (h *DashboardHandler) GetQuickStats(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	quickStats, err := h.dashboardService.GetQuickStats(c.Context())
	if err != nil {
		h.logger.Error("Failed to get quick stats", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get quick stats", ""))
	}

	return c.JSON(utils.SuccessResponse("Quick stats retrieved successfully", quickStats))
}

// GetRecentActivity obtiene la actividad reciente
// @Summary Get recent activity
// @Description Get recent system activity and workflow executions
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param limit query int false "Number of activities to retrieve" default(50)
// @Success 200 {object} utils.DataResponse{data=[]models.ActivityItem}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/activity [get]
func (h *DashboardHandler) GetRecentActivity(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Parsear parámetro limit
	limitStr := c.Query("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 200 {
		limit = 50
	}

	activities, err := h.dashboardService.GetRecentActivity(c.Context(), limit)
	if err != nil {
		h.logger.Error("Failed to get recent activity", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get recent activity", ""))
	}

	return c.JSON(utils.SuccessResponse("Recent activity retrieved successfully", activities))
}

// GetWorkflowStatus obtiene el estado de los workflows
// @Summary Get workflow status
// @Description Get status and health information for all workflows
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param limit query int false "Number of workflows to retrieve" default(100)
// @Success 200 {object} utils.DataResponse{data=[]models.WorkflowStatusItem}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/workflows [get]
func (h *DashboardHandler) GetWorkflowStatus(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Parsear parámetro limit
	limitStr := c.Query("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 500 {
		limit = 100
	}

	workflowStatus, err := h.dashboardService.GetWorkflowStatus(c.Context(), limit)
	if err != nil {
		h.logger.Error("Failed to get workflow status", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get workflow status", ""))
	}

	return c.JSON(utils.SuccessResponse("Workflow status retrieved successfully", workflowStatus))
}

// GetQueueStatus obtiene el estado de las colas
// @Summary Get queue status
// @Description Get current status of processing queues
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse{data=models.QueueStatus}
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/queue [get]
func (h *DashboardHandler) GetQueueStatus(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	queueStatus, err := h.dashboardService.GetQueueStatus(c.Context())
	if err != nil {
		h.logger.Error("Failed to get queue status", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get queue status", ""))
	}

	return c.JSON(utils.SuccessResponse("Queue status retrieved successfully", queueStatus))
}

// GetPerformanceData obtiene datos de rendimiento para gráficos
// @Summary Get performance data
// @Description Get performance metrics and trends for dashboard charts
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param time_range query string false "Time range for performance data" default("24h") Enums(1h, 6h, 12h, 24h, 7d, 30d)
// @Success 200 {object} utils.DataResponse{data=models.PerformanceData}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/performance [get]
func (h *DashboardHandler) GetPerformanceData(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Parsear time_range
	timeRange := c.Query("time_range", "24h")
	validRanges := []string{"1h", "6h", "12h", "24h", "7d", "30d"}
	isValid := false
	for _, validRange := range validRanges {
		if timeRange == validRange {
			isValid = true
			break
		}
	}
	if !isValid {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Invalid time_range", "Valid options: 1h, 6h, 12h, 24h, 7d, 30d"))
	}

	performanceData, err := h.dashboardService.GetPerformanceData(c.Context(), timeRange)
	if err != nil {
		h.logger.Error("Failed to get performance data", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get performance data", ""))
	}

	return c.JSON(utils.SuccessResponse("Performance data retrieved successfully", performanceData))
}

// GetActiveAlerts obtiene las alertas activas
// @Summary Get active alerts
// @Description Get current active alerts and warnings
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.DataResponse{data=[]models.Alert}
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/alerts [get]
func (h *DashboardHandler) GetActiveAlerts(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	alerts, err := h.dashboardService.GetActiveAlerts(c.Context())
	if err != nil {
		h.logger.Error("Failed to get active alerts", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get active alerts", ""))
	}

	return c.JSON(utils.SuccessResponse("Active alerts retrieved successfully", alerts))
}

// GetWorkflowHealth obtiene la salud de un workflow específico
// @Summary Get workflow health
// @Description Get detailed health information for a specific workflow
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Workflow ID"
// @Success 200 {object} utils.DataResponse{data=models.WorkflowStatusItem}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/workflows/{id}/health [get]
func (h *DashboardHandler) GetWorkflowHealth(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	workflowID := c.Params("id")
	if workflowID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Workflow ID is required", ""))
	}

	workflowHealth, err := h.dashboardService.GetWorkflowHealth(c.Context(), workflowID)
	if err != nil {
		if err.Error() == "workflow not found: "+workflowID {
			return c.Status(fiber.StatusNotFound).JSON(utils.ErrorResponse("Workflow not found", ""))
		}
		h.logger.Error("Failed to get workflow health", zap.Error(err), zap.String("user_id", userID.Hex()), zap.String("workflow_id", workflowID))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get workflow health", ""))
	}

	return c.JSON(utils.SuccessResponse("Workflow health retrieved successfully", workflowHealth))
}

// GetMetrics obtiene métricas específicas del dashboard
// @Summary Get specific metrics
// @Description Get specific dashboard metrics by name
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param metrics query string true "Comma-separated metric names" example("executions,success_rate,queue_length")
// @Param time_range query string false "Time range for metrics" default("24h") Enums(1h, 6h, 12h, 24h, 7d, 30d)
// @Success 200 {object} utils.DataResponse{data=map[string]interface{}}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/metrics [get]
func (h *DashboardHandler) GetMetrics(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Parsear métricas solicitadas
	metricsParam := c.Query("metrics")
	if metricsParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ErrorResponse("Metrics parameter is required", ""))
	}

	metricNames := []string{}
	for _, metric := range utils.SplitAndTrim(metricsParam, ",") {
		metricNames = append(metricNames, metric)
	}

	// Parsear time_range
	timeRange := c.Query("time_range", "24h")

	metrics, err := h.dashboardService.GetDashboardMetrics(c.Context(), metricNames, timeRange)
	if err != nil {
		h.logger.Error("Failed to get metrics", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to get metrics", ""))
	}

	return c.JSON(utils.SuccessResponse("Metrics retrieved successfully", metrics))
}

// RefreshDashboard fuerza la actualización de los datos del dashboard
// @Summary Refresh dashboard data
// @Description Force refresh of all dashboard data and metrics
// @Tags dashboard
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} utils.MessageResponse
// @Failure 401 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/dashboard/refresh [post]
func (h *DashboardHandler) RefreshDashboard(c *fiber.Ctx) error {
	userID, err := middleware.GetCurrentUserID(c)
	if err != nil {
		return utils.HandleError(c, err)
	}

	// Verificar que el usuario sea admin para esta operación
	userRole, _ := middleware.GetCurrentUserRole(c)
	if userRole != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(utils.ErrorResponse("Admin access required", ""))
	}

	err = h.dashboardService.RefreshDashboardData(c.Context())
	if err != nil {
		h.logger.Error("Failed to refresh dashboard data", zap.Error(err), zap.String("user_id", userID.Hex()))
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ErrorResponse("Failed to refresh dashboard data", ""))
	}

	h.logger.Info("Dashboard data refreshed successfully", zap.String("user_id", userID.Hex()))
	return c.JSON(utils.SuccessResponse("Dashboard data refreshed successfully", map[string]interface{}{
		"refreshed_at": time.Now(),
	}))
}

// ServeDashboardPage sirve la página HTML del dashboard
// @Summary Serve dashboard HTML page
// @Description Serve the main dashboard HTML interface
// @Tags dashboard
// @Accept html
// @Produce html
// @Success 200 {string} string "HTML page"
// @Router /dashboard [get]
func (h *DashboardHandler) ServeDashboardPage(c *fiber.Ctx) error {
	return c.SendFile("./web/templates/dashboard.html")
}

// Métodos helper privados

// parseFilterFromQuery parsea los filtros desde query parameters
func (h *DashboardHandler) parseFilterFromQuery(c *fiber.Ctx) (*models.DashboardFilter, error) {
	filter := &models.DashboardFilter{}

	// Time range
	filter.TimeRange = c.Query("time_range", "24h")

	// Workflow IDs
	if workflowIDs := c.Query("workflow_ids"); workflowIDs != "" {
		filter.WorkflowIDs = utils.SplitAndTrim(workflowIDs, ",")
	}

	// User IDs
	if userIDs := c.Query("user_ids"); userIDs != "" {
		filter.UserIDs = utils.SplitAndTrim(userIDs, ",")
	}

	// Status
	if status := c.Query("status"); status != "" {
		filter.Status = utils.SplitAndTrim(status, ",")
	}

	// Environment
	filter.Environment = c.Query("environment")

	// Start date
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		startDate, err := time.Parse(time.RFC3339, startDateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid start_date format: %w", err)
		}
		filter.StartDate = &startDate
	}

	// End date
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		endDate, err := time.Parse(time.RFC3339, endDateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid end_date format: %w", err)
		}
		filter.EndDate = &endDate
	}

	return filter, nil
}

// validateTimeRange valida que el time_range sea válido
func (h *DashboardHandler) validateTimeRange(timeRange string) bool {
	validRanges := []string{"1h", "6h", "12h", "24h", "7d", "30d"}
	for _, validRange := range validRanges {
		if timeRange == validRange {
			return true
		}
	}
	return false
}

// logDashboardAccess registra el acceso al dashboard para auditoría
func (h *DashboardHandler) logDashboardAccess(userID, endpoint, method string) {
	h.logger.Info("Dashboard access",
		zap.String("user_id", userID),
		zap.String("endpoint", endpoint),
		zap.String("method", method),
		zap.Time("timestamp", time.Now()))
}
