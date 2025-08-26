package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"Engine_API_Workflow/internal/config"
	"Engine_API_Workflow/pkg/database"
	"Engine_API_Workflow/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Cargar configuración
	cfg := config.Load()

	// Inicializar logger
	logger.Init(cfg.Environment)
	logger.Info("Starting Engine API Workflow...")

	// Conectar a MongoDB
	logger.Info("Connecting to MongoDB...")
	mongoClient, err := database.ConnectMongoDB(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB: " + err.Error())
	}
	defer func() {
		if err = database.DisconnectMongoDB(mongoClient); err != nil {
			logger.Error("Failed to disconnect from MongoDB: " + err.Error())
		}
	}()
	logger.Info("Connected to MongoDB successfully")

	// Conectar a Redis
	logger.Info("Connecting to Redis...")
	redisClient, err := database.ConnectRedis(cfg.RedisURL)
	if err != nil {
		logger.Fatal("Failed to connect to Redis: " + err.Error())
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Error("Failed to close Redis connection: " + err.Error())
		}
	}()
	logger.Info("Connected to Redis successfully")

	// Crear aplicación Fiber
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			logger.Error("HTTP Error: " + err.Error())
			return c.Status(code).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"time":   time.Now().UTC(),
		})
	})

	// API routes
	api := app.Group("/api/v1")

	// TODO: Configurar rutas cuando tengas los handlers listos
	api.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Engine API Workflow v1.0",
			"status":  "running",
		})
	})

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("Shutting down server...")
		app.Shutdown()
	}()

	// Iniciar servidor
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	logger.Info("Server starting on port " + port)
	if err := app.Listen(":" + port); err != nil {
		logger.Fatal("Failed to start server: " + err.Error())
	}
}
