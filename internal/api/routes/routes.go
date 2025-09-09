package routes

import (
	"time"

	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
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

	// Health check en la ruta de API también
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"message":   "Engine API Workflow is running",
			"version":   "1.0.0",
			"timestamp": time.Now(),
		})
	})

	// Rutas de autenticación (públicas por ahora)
	setupAuthRoutes(api, config)

	// Rutas básicas (sin middleware complejo por ahora)
	setupBasicRoutes(api, config)

	// Ruta catch-all para 404
	app.Use("*", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Endpoint not found",
			"path":    c.Path(),
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

	// Profile (sin middleware por ahora)
	auth.Get("/profile", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Profile endpoint - Authentication coming soon",
		})
	})
}

// setupBasicRoutes configura rutas básicas sin middleware complejo
func setupBasicRoutes(api fiber.Router, config *RouteConfig) {
	// Workflows
	workflows := api.Group("/workflows")
	workflows.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Workflows list - Coming soon",
			"data":    []interface{}{},
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

	// Logs
	logs := api.Group("/logs")
	logs.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Logs list - Coming soon",
			"data":    []interface{}{},
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

	// Triggers
	triggers := api.Group("/triggers")
	triggers.Post("/workflow", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"success": true,
			"message": "Trigger workflow - Coming soon",
		})
	})

	// Admin (sin middleware de admin por ahora)
	admin := api.Group("/admin")
	admin.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Admin system stats - Coming soon",
			"data": fiber.Map{
				"system": "healthy",
				"uptime": "coming soon",
			},
		})
	})
}

// SetupRoutesFromDatabase - CORREGIDO: parámetros correctos para los constructores
func SetupRoutesFromDatabase(app *fiber.App, db *mongo.Database, jwtService jwt.JWTService, appLogger *logger.Logger) {
	// Inicializar repositorios
	userRepo := mongodb.NewUserRepository(db)

	// CORREGIDO: NewAuthService espera UserRepository, no JWTService
	authService := services.NewAuthService(userRepo)

	// CORREGIDO: Inicializar validator correctamente
	validator := utils.NewValidator()

	// CORREGIDO: NewAuthHandler espera (AuthService, JWTService, *Validator)
	authHandler := handlers.NewAuthHandler(authService, jwtService, validator)

	// Crear configuración
	config := &RouteConfig{
		AuthHandler: authHandler,
		JWTService:  jwtService,
		Logger:      appLogger,
	}

	// Configurar rutas
	SetupRoutes(app, config)
}
