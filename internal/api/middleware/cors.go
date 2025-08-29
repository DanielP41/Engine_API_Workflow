package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
		},
		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"DELETE",
			"OPTIONS",
			"PATCH",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
			"X-Request-ID",
			"X-API-Key",
		},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	}
}

// ProductionCORSConfig returns CORS configuration for production
func ProductionCORSConfig(allowedOrigins []string) CORSConfig {
	config := DefaultCORSConfig()

	if len(allowedOrigins) > 0 {
		config.AllowOrigins = allowedOrigins
	} else {
		// In production, be more restrictive
		config.AllowOrigins = []string{
			"https://yourdomain.com",
			"https://www.yourdomain.com",
		}
	}

	return config
}

// CORSMiddleware creates a CORS middleware with the given configuration
func CORSMiddleware(config CORSConfig) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     strings.Join(config.AllowOrigins, ","),
		AllowMethods:     strings.Join(config.AllowMethods, ","),
		AllowHeaders:     strings.Join(config.AllowHeaders, ","),
		AllowCredentials: config.AllowCredentials,
		MaxAge:           config.MaxAge,
		Next: func(c *fiber.Ctx) bool {
			// Skip CORS for health check endpoints
			return c.Path() == "/health" || c.Path() == "/healthz"
		},
	})
}

// DynamicCORSMiddleware creates a CORS middleware that can handle dynamic origins
func DynamicCORSMiddleware(config CORSConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")

		// If no origin header, continue
		if origin == "" {
			return c.Next()
		}

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range config.AllowOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}

			// Support wildcard subdomains (e.g., *.example.com)
			if strings.HasPrefix(allowedOrigin, "*.") {
				domain := strings.TrimPrefix(allowedOrigin, "*.")
				if strings.HasSuffix(origin, domain) {
					allowed = true
					break
				}
			}
		}

		// Set CORS headers if origin is allowed
		if allowed {
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
			c.Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))

			if config.AllowCredentials {
				c.Set("Access-Control-Allow-Credentials", "true")
			}

			if config.MaxAge > 0 {
				c.Set("Access-Control-Max-Age", string(rune(config.MaxAge)))
			}
		}

		// Handle preflight requests
		if c.Method() == "OPTIONS" {
			if allowed {
				return c.SendStatus(fiber.StatusNoContent)
			}
			return c.SendStatus(fiber.StatusForbidden)
		}

		return c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Security headers
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Don't set HSTS in development
		// In production, you might want to add:
		// c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		return c.Next()
	}
}

// APIResponseHeadersMiddleware adds API-specific response headers
func APIResponseHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Set API version header
		c.Set("X-API-Version", "1.0")

		// Set content type for JSON APIs
		if strings.HasPrefix(c.Path(), "/api/") {
			c.Set("Content-Type", "application/json; charset=utf-8")
		}

		// Add cache control for API responses
		if c.Method() != "GET" {
			c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		return c.Next()
	}
}
