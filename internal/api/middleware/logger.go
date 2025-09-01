package middleware

import (
	"time"

	"Engine_API_Workflow/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// LoggerMiddleware registra detalles de cada solicitud HTTP
func LoggerMiddleware(log *logger.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Continuar con el siguiente handler
		err := c.Next()

		// Obtener detalles de la solicitud
		status := c.Response().StatusCode()
		path := c.Path()
		method := c.Method()
		latency := time.Since(start).Milliseconds()

		// Registrar la solicitud
		log.Info("HTTP Request",
			"method", method,
			"path", path,
			"status", status,
			"latency_ms", latency,
		)

		return err
	}
}

// SetupMiddleware configura los middlewares comunes
func SetupMiddleware(app *fiber.App, log *logger.Logger) {
	// Middleware de recuperación para evitar pánico
	app.Use(recover.New())

	// Middleware de logging
	app.Use(LoggerMiddleware(log))
}
