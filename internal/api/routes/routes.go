package routes

import (
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"

	"github.com/gofiber/fiber/v2"
)

// RouteConfig contiene todas las dependencias necesarias para configurar las rutas
type RouteConfig struct {
	AuthHandler     *handlers.AuthHandler
	WorkflowHandler *handlers.WorkflowHandler
	TriggerHandler  *handlers.TriggerHandler
	JWTService      jwt.JWTService
	Logger          *logger.Logger
}

func SetupRoutes(app *fiber.App, config *RouteConfig) {
	// Health check público
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Engine API Workflow is running",
			"version": "1.0.0",
		})
	})

	// Grupo principal de la API
	api := app.Group("/api/v1")

	// Rutas de autenticación (públicas)
	auth := api.Group("/auth")
	auth.Post("/register", config.AuthHandler.Register)
	auth.Post("/login", config.AuthHandler.Login)
	auth.Post("/refresh", config.AuthHandler.RefreshToken)

	// Middlewares de autenticación
	authMiddleware := middleware.AuthMiddleware(config.JWTService, config.Logger)
	refreshMiddleware := middleware.RefreshTokenMiddleware(config.JWTService, config.Logger)
	adminMiddleware := middleware.AdminRequiredMiddleware()

	// Rutas protegidas de autenticación
	auth.Get("/profile", authMiddleware, config.AuthHandler.GetProfile)
	auth.Post("/logout", authMiddleware, config.AuthHandler.Logout)

	