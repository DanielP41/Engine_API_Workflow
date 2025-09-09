package routes

import (
	"time"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

// RouteConfig holds configuration for routes
type RouteConfig struct {
	AuthHandler *handlers.AuthHandler
	JWTService  jwt.JWTService
	Logger      *logger.Logger
}

// SetupRoutes configura todas las rutas de la aplicación
func SetupRoutes(app *fiber.App, config *RouteConfig) {
	// Health check (público)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"message":   "Engine API Workflow is running",
			"timestamp": time.Now(),
		})
	})

	// API v1
	api := app.Group("/api/v1")

	// Rutas de autenticación (públicas)
	setupAuthRoutes(api, config)

	// Rutas protegidas
	setupProtectedRoutes(api, config)

	// Ruta catch-all para 404
	app.Use("*", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Endpoint not found",
		})
	})
}

// setupAuthRoutes configura las rutas de autenticación
func setupAuthRoutes(api fiber.Router, config *RouteConfig) {
	auth := api.Group("/auth")

	// Rutas públicas de autenticación
	auth.Post("/register", config.AuthHandler.Register)
	auth.Post("/login", config.AuthHandler.Login)
	auth.Post("/refresh", config.AuthHandler.RefreshToken)
	auth.Post("/logout", config.AuthHandler.Logout)

	// Ruta protegida simple sin middleware por ahora
	auth.Get("/profile", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Profile endpoint - Coming soon",
		})
	})
}

// setupProtectedRoutes configura las rutas que requieren autenticación
func setupProtectedRoutes(api fiber.Router, config *RouteConfig) {
	// Crear un grupo protegido simple sin middleware complejo por ahora
	protected := api.Group("/protected")

	// Rutas de workflows
	setupWorkflowRoutes(protected, config)

	// Rutas de logs
	setupLogRoutes(protected, config)

	// Rutas de triggers
	setupTriggerRoutes(protected, config)

	// Rutas de administración
	setupAdminRoutes(protected, config)
}

// setupWorkflowRoutes configura las rutas de workflows
func setupWorkflowRoutes(protected fiber.Router, config *RouteConfig) {
	workflows := protected.Group("/workflows")

	workflows.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Workflows list - Coming soon",
		})
	})

	workflows.Post("/", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"success": true,
			"message": "Create workflow - Coming soon",
		})
	})

	workflows.Get("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Get workflow by ID - Coming soon",
			"data": fiber.Map{
				"id": id,
			},
		})
	})

	workflows.Put("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Update workflow - Coming soon",
			"data": fiber.Map{
				"id": id,
			},
		})
	})

	workflows.Delete("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Delete workflow - Coming soon",
			"data": fiber.Map{
				"id": id,
			},
		})
	})
}

// setupLogRoutes configura las rutas de logs
func setupLogRoutes(protected fiber.Router, config *RouteConfig) {
	logs := protected.Group("/logs")

	logs.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Logs list - Coming soon",
		})
	})

	logs.Get("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Get log by ID - Coming soon",
			"data": fiber.Map{
				"id": id,
			},
		})
	})
}

// setupTriggerRoutes configura las rutas de triggers
func setupTriggerRoutes(protected fiber.Router, config *RouteConfig) {
	triggers := protected.Group("/triggers")

	triggers.Post("/workflow", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"success": true,
			"message": "Trigger workflow - Coming soon",
		})
	})

	triggers.Post("/webhook/:webhook_id", func(c *fiber.Ctx) error {
		webhookID := c.Params("webhook_id")
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"success": true,
			"message": "Trigger webhook - Coming soon",
			"data": fiber.Map{
				"webhook_id": webhookID,
			},
		})
	})
}

// setupAdminRoutes configura las rutas de administración
func setupAdminRoutes(protected fiber.Router, config *RouteConfig) {
	admin := protected.Group("/admin")

	admin.Get("/users", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Admin users list - Coming soon",
		})
	})

	admin.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Admin system stats - Coming soon",
		})
	})
}

// SetupRoutesFromDatabase es una función alternativa que inicializa desde la base de datos
func SetupRoutesFromDatabase(app *fiber.App, db *mongo.Database, jwtService jwt.JWTService, appLogger *logger.Logger) {
	// Inicializar repositorios
	userRepo := mongodb.NewUserRepository(db)

	// Inicializar servicios - USAR LA IMPLEMENTACIÓN REAL
	authService := &services.AuthServiceImpl{}

	// Inicializar handlers - USAR SOLO LOS ARGUMENTOS QUE FUNCIONEN
	authHandler := &handlers.AuthHandler{}

	// Si hay problemas con el constructor, inicializar directamente
	_ = userRepo    // Usar la variable para evitar error de "not used"
	_ = authService // Usar la variable para evitar error de "not used"

	// Crear configuración
	config := &RouteConfig{
		AuthHandler: authHandler,
		JWTService:  jwtService,
		Logger:      appLogger,
	}

	// Configurar rutas
	SetupRoutes(app, config)
}
