package routes

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/pkg/jwt"
)

type RouteConfig struct {
	AuthHandler      *handlers.AuthHandler
	WorkflowHandler  *handlers.WorkflowHandler
	TriggerHandler   *handlers.TriggerHandler
	DashboardHandler *handlers.DashboardHandler
	JWTService       jwt.JWTService
	Logger           *zap.Logger
}

func SetupRoutes(app *fiber.App, config *RouteConfig) {
	api := app.Group("/api/v1")
	
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	auth := api.Group("/auth")
	auth.Post("/register", config.AuthHandler.Register)
	auth.Post("/login", config.AuthHandler.Login)
	auth.Post("/refresh", config.AuthHandler.RefreshToken)
	
	authMiddleware := middleware.NewAuthMiddleware(config.JWTService, nil).RequireAuth()
	protected := api.Group("", authMiddleware)
	
	protectedAuth := protected.Group("/auth")
	protectedAuth.Get("/profile", config.AuthHandler.GetProfile)
	protectedAuth.Post("/logout", config.AuthHandler.Logout)
	
	if config.WorkflowHandler != nil {
		workflows := protected.Group("/workflows")
		workflows.Post("/", config.WorkflowHandler.CreateWorkflow)
		workflows.Get("/", config.WorkflowHandler.GetWorkflows)
		workflows.Get("/:id", config.WorkflowHandler.GetWorkflow)
		workflows.Put("/:id", config.WorkflowHandler.UpdateWorkflow)
		workflows.Delete("/:id", config.WorkflowHandler.DeleteWorkflow)
		workflows.Post("/:id/clone", config.WorkflowHandler.CloneWorkflow)
	}
	
	if config.DashboardHandler != nil {
		dashboard := protected.Group("/dashboard")
		dashboard.Get("/", config.DashboardHandler.GetDashboard)
		dashboard.Get("/summary", config.DashboardHandler.GetDashboardSummary)
		dashboard.Get("/health", config.DashboardHandler.GetSystemHealth)
		dashboard.Get("/stats", config.DashboardHandler.GetQuickStats)
	}
}

func ValidateRouteConfig(config *RouteConfig) error {
	if config == nil { return fmt.Errorf("route config cannot be nil") }
	if config.AuthHandler == nil { return fmt.Errorf("auth handler is required") }
	if config.JWTService == nil { return fmt.Errorf("JWT service is required") }
	return nil
}

func GetRouteInfo() map[string]interface{} {
	return map[string]interface{}{"version": "1.0.0"}
}