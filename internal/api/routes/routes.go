package routes

import (
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

func SetupRoutes(app *fiber.App, db *mongo.Database, jwtService jwt.JWTService) {
	// Inicializar repositorios
	userRepo := repository.NewUserRepository(db)
	workflowRepo := repository.NewWorkflowRepository(db)
	logRepo := repository.NewLogRepository(db)

	// Inicializar servicios
	authService := services.NewAuthService(userRepo)
	workflowService := services.NewWorkflowService(workflowRepo, userRepo)
	logService := services.NewLogService(logRepo, workflowRepo, userRepo)

	// Inicializar utilidades
	validator := utils.NewValidator()

	// Inicializar handlers
	authHandler := handlers.NewAuthHandler(authService, jwtService, validator)
	workflowHandler := handlers.NewWorkflowHandler(workflowService, logService, validator)

	// Inicializar middlewares
	authMiddleware := middleware.NewAuthMiddleware(jwtService)

	// Rutas públicas
	api := app.Group("/api/v1")

	// Health check (público)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Engine API Workflow is running",
		})
	})

	// Rutas de autenticación (públicas)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.RefreshToken)

	// Rutas protegidas (requieren autenticación)
	protected := api.Group("", authMiddleware.RequireAuth())

	// Rutas de autenticación protegidas
	auth.Get("/profile", authHandler.GetProfile)
	auth.Post("/logout", authHandler.Logout)

	// Rutas de workflows
	workflows := protected.Group("/workflows")
	workflows.Post("/", workflowHandler.CreateWorkflow)
	workflows.Get("/", workflowHandler.GetWorkflows)
	workflows.Get("/:id", workflowHandler.GetWorkflow)
	workflows.Put("/:id", workflowHandler.UpdateWorkflow)
	workflows.Delete("/:id", workflowHandler.DeleteWorkflow)
	workflows.Post("/:id/toggle", workflowHandler.ToggleWorkflowStatus)
	workflows.Get("/:id/stats", workflowHandler.GetWorkflowStats)
	workflows.Post("/:id/clone", workflowHandler.CloneWorkflow)

	// Rutas de logs
	logs := protected.Group("/logs")
	logs.Get("/", func(c *fiber.Ctx) error {
		// TODO: Implementar handler de logs
		return c.JSON(fiber.Map{"message": "Logs endpoint - Coming soon"})
	})
	logs.Get("/:id", func(c *fiber.Ctx) error {
		// TODO: Implementar handler para obtener log específico
		return c.JSON(fiber.Map{"message": "Get log by ID - Coming soon"})
	})

	// Rutas de triggers (para webhooks)
	triggers := protected.Group("/triggers")
	triggers.Post("/", func(c *fiber.Ctx) error {
		// TODO: Implementar handler de triggers
		return c.JSON(fiber.Map{"message": "Triggers endpoint - Coming soon"})
	})

	// Rutas de administración (solo admins)
	admin := protected.Group("/admin", authMiddleware.RequireAdmin())
	admin.Get("/users", func(c *fiber.Ctx) error {
		// TODO: Implementar listado de usuarios para admins
		return c.JSON(fiber.Map{"message": "Admin users endpoint - Coming soon"})
	})
	admin.Get("/stats", func(c *fiber.Ctx) error {
		// TODO: Implementar estadísticas del sistema
		return c.JSON(fiber.Map{"message": "Admin stats endpoint - Coming soon"})
	})

	// Ruta catch-all para 404
	app.Use("*", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Endpoint not found",
		})
	})
}
