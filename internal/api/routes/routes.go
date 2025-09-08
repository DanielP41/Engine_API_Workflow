package routes

import (
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/internal/repository/mongodb"
	"Engine_API_Workflow/internal/services"
	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/jwt"
	"Engine_API_Workflow/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"go.mongodb.org/mongo-driver/mongo"
)

// RouteConfig estructura compatible con main.go
type RouteConfig struct {
	DB             *mongo.Database
	JWTService     jwt.JWTService
	TokenBlacklist *jwt.TokenBlacklist
	Logger         *logger.Logger
	Config         *config.Config
}

// SetupSecureRoutes función principal que main.go espera
func SetupSecureRoutes(app *fiber.App, config *RouteConfig) {
	// Inicializar repositorios
	userRepo := mongodb.NewUserRepository(config.DB)

	// Inicializar servicios
	authService := services.NewAuthService(userRepo)

	// Crear validador
	validator := utils.NewValidator()

	// Inicializar handlers
	authHandler := handlers.NewAuthHandler(authService, config.JWTService, validator)

	// Configurar middlewares de seguridad
	setupSecurityMiddlewares(app, config)

	// Configurar rutas principales
	setupMainRoutes(app, config, authHandler)

	config.Logger.Info("Secure routes configured successfully")
}

// setupSecurityMiddlewares configura middlewares de seguridad
func setupSecurityMiddlewares(app *fiber.App, config *RouteConfig) {
	// CORS (si está habilitado)
	if config.Config.EnableCORS {
		origins := config.Config.GetCORSOrigins()
		originString := "http://localhost:3000,http://localhost:3001"
		if len(origins) > 0 {
			originString = origins[0]
			for i := 1; i < len(origins); i++ {
				originString += "," + origins[i]
			}
		}

		corsConfig := cors.Config{
			AllowOrigins:     originString,
			AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
			AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With,X-Request-ID",
			AllowCredentials: true,
			MaxAge:           86400,
		}
		app.Use(cors.New(corsConfig))
		config.Logger.Info("CORS middleware enabled")
	}

	// Rate Limiting (si está habilitado)
	if config.Config.EnableRateLimit {
		rateLimitConfig := limiter.Config{
			Max:        config.Config.RateLimitRequests,
			Expiration: config.Config.RateLimitWindow,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"success":     false,
					"message":     "Too many requests",
					"retry_after": config.Config.RateLimitWindow.Seconds(),
				})
			},
		}
		app.Use(limiter.New(rateLimitConfig))
		config.Logger.Info("Rate limiting middleware enabled")
	}

	// Security headers
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if config.Config.IsProduction() {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		return c.Next()
	})
}

// setupMainRoutes configura las rutas principales de la API
func setupMainRoutes(app *fiber.App, config *RouteConfig, authHandler *handlers.AuthHandler) {
	// Grupo principal de la API
	api := app.Group("/api/v1")

	// Health check interno
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Engine API Workflow is running",
			"version": "1.0.0",
		})
	})

	// Rutas de autenticación (públicas)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.RefreshToken)

	// Crear instancia del middleware de autenticación
	authMiddlewareInstance := middleware.NewAuthMiddlewareWithBlacklist(config.JWTService, config.TokenBlacklist, config.Logger)

	// Grupo de rutas protegidas
	protected := api.Group("", authMiddlewareInstance.RequireAuth())

	// Rutas de autenticación protegidas
	protected.Get("/auth/profile", authHandler.GetProfile)
	protected.Post("/auth/logout", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Logout successful",
		})
	})

	// Workflows (placeholder)
	workflows := protected.Group("/workflows")
	workflows.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Workflows endpoint - Coming soon",
			"status":  "placeholder",
		})
	})

	workflows.Post("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Create workflow endpoint - Coming soon",
			"status":  "placeholder",
		})
	})

	// Logs (placeholder)
	logs := protected.Group("/logs")
	logs.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Logs endpoint - Coming soon",
			"status":  "placeholder",
		})
	})

	// Triggers (placeholder)
	triggers := protected.Group("/triggers")
	triggers.Post("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Triggers endpoint - Coming soon",
			"status":  "placeholder",
		})
	})

	// Admin routes (básico)
	admin := protected.Group("/admin")

	admin.Get("/users", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin users endpoint - Coming soon",
			"status":  "placeholder",
		})
	})

	admin.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin stats endpoint - Coming soon",
			"status":  "placeholder",
		})
	})

	// 404 handler
	app.Use("*", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Endpoint not found",
			"path":    c.Path(),
		})
	})

	config.Logger.Info("Routes configured successfully")
}
