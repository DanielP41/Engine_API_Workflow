package routes

import (
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

// RouteConfig contiene todas las dependencias necesarias para las rutas
type RouteConfig struct {
	AuthHandler *handlers.AuthHandler
	JWTService  jwt.JWTService
	Logger      *logger.Logger
}

// SetupRoutes configura todas las rutas de la aplicación
func SetupRoutes(app *fiber.App, config *RouteConfig) {
	// Configurar middlewares CORS y seguridad
	app.Use(middleware.CORSMiddleware(middleware.DefaultCORSConfig()))
	app.Use(middleware.SecurityHeadersMiddleware())
	app.Use(middleware.APIResponseHeadersMiddleware())

	// Grupo principal de la API
	api := app.Group("/api/v1")

	// Health check (público)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Engine API Workflow is running",
			"timestamp": fiber.Map{
				"utc": fiber.Now(),
			},
			"version": "1.0.0",
		})
	})

	// Rutas de autenticación (públicas)
	auth := api.Group("/auth")
	auth.Post("/register", config.AuthHandler.Register)
	auth.Post("/login", config.AuthHandler.Login)
	auth.Post("/refresh", config.AuthHandler.RefreshToken)

	// Crear middleware de autenticación
	authMiddleware := middleware.NewAuthMiddleware(config.JWTService)

	// Rutas protegidas
	protected := api.Group("", authMiddleware.RequireAuth())

	// Rutas de autenticación protegidas
	protected.Get("/auth/profile", config.AuthHandler.GetProfile)
	protected.Post("/auth/logout", config.AuthHandler.Logout)

	// Rutas de workflows (placeholder)
	workflows := protected.Group("/workflows")
	workflows.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Workflows endpoint - Coming soon",
			"status":  "not_implemented",
		})
	})
	workflows.Post("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Create workflow endpoint - Coming soon",
			"status":  "not_implemented",
		})
	})

	// Rutas de logs (placeholder)
	logs := protected.Group("/logs")
	logs.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Logs endpoint - Coming soon",
			"status":  "not_implemented",
		})
	})

	// Rutas de administración
	admin := protected.Group("/admin", authMiddleware.RequireAdmin())
	admin.Get("/users", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin users endpoint - Coming soon",
			"status":  "not_implemented",
		})
	})

	admin.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin stats endpoint - Coming soon",
			"status":  "not_implemented",
		})
	})

	// Webhook público (sin autenticación)
	api.Post("/webhook/:id", func(c *fiber.Ctx) error {
		webhookID := c.Params("id")
		return c.JSON(fiber.Map{
			"message":    "Webhook endpoint - Coming soon",
			"webhook_id": webhookID,
			"status":     "not_implemented",
		})
	})

	// Ruta catch-all para 404
	app.Use("*", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Endpoint not found",
			"path":    c.Path(),
		})
	})
}

// SetupRoutesComplete configura todas las rutas con todas las dependencias
func SetupRoutesComplete(app *fiber.App, db *mongo.Database, jwtService jwt.JWTService, appLogger *logger.Logger) {
	// Inicializar repositorios
	userRepo := mongodb.NewUserRepository(db)

	// Inicializar servicios
	authService := services.NewAuthService(userRepo)

	// Inicializar validador
	validator := utils.NewValidator()

	// Inicializar handlers
	authHandler := handlers.NewAuthHandler(authService, jwtService, validator)

	// Configurar rutas
	config := &RouteConfig{
		AuthHandler: authHandler,
		JWTService:  jwtService,
		Logger:      appLogger,
	}

	SetupRoutes(app, config)
}
