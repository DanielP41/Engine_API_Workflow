package routes

import (
	"Engine_API_Workflow/internal/api/handlers"
	"Engine_API_Workflow/internal/api/middleware"
	"Engine_API_Workflow/pkg/jwt"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// RouteConfig configuración de handlers para rutas
type RouteConfig struct {
	AuthHandler      *handlers.AuthHandler
	WorkflowHandler  *handlers.WorkflowHandler
	TriggerHandler   *handlers.TriggerHandler
	DashboardHandler *handlers.DashboardHandler
	WorkerHandler    *handlers.WorkerHandler
	BackupHandler    *handlers.BackupHandler
	JWTService       jwt.JWTService
	Logger           *zap.Logger
}

func SetupRoutes(app *fiber.App, config *RouteConfig) {
	// Validar configuración
	if err := ValidateRouteConfig(config); err != nil {
		config.Logger.Fatal("Invalid route configuration", zap.Error(err))
	}

	api := app.Group("/api/v1")

	// Health check público
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "2.0",
			"service": "engine-api-workflow",
		})
	})

	// Rutas de autenticación (públicas)
	auth := api.Group("/auth")
	auth.Post("/register", config.AuthHandler.Register)
	auth.Post("/login", config.AuthHandler.Login)
	auth.Post("/refresh", config.AuthHandler.RefreshToken)

	// Middleware de autenticación
	authMiddleware := middleware.NewAuthMiddleware(config.JWTService, nil).RequireAuth()
	protected := api.Group("", authMiddleware)

	// Rutas de autenticación protegidas
	protectedAuth := protected.Group("/auth")
	protectedAuth.Get("/profile", config.AuthHandler.GetProfile)
	protectedAuth.Post("/logout", config.AuthHandler.Logout)

	// Rutas de workflows
	if config.WorkflowHandler != nil {
		workflows := protected.Group("/workflows")
		workflows.Post("/", config.WorkflowHandler.CreateWorkflow)
		workflows.Get("/", config.WorkflowHandler.GetWorkflows)
		workflows.Get("/:id", config.WorkflowHandler.GetWorkflow)
		workflows.Put("/:id", config.WorkflowHandler.UpdateWorkflow)
		workflows.Delete("/:id", config.WorkflowHandler.DeleteWorkflow)
		workflows.Post("/:id/clone", config.WorkflowHandler.CloneWorkflow)
	}

	// RUTAS DE BACKUP
	if config.BackupHandler != nil {
		backup := protected.Group("/backup")

		// Rutas básicas de backup (requieren autenticación)
		backup.Get("/status", config.BackupHandler.GetBackupStatus)
		backup.Get("/list", config.BackupHandler.ListBackups)
		backup.Get("/:id", config.BackupHandler.GetBackupInfo)
		backup.Post("/:id/validate", config.BackupHandler.ValidateBackup)

		// Middleware para operaciones que requieren rol admin
		adminMiddleware := middleware.NewAuthMiddleware(config.JWTService, nil).RequireRole("admin")

		// Rutas que requieren permisos de admin
		backup.Post("/create", adminMiddleware, config.BackupHandler.CreateBackup)
		backup.Delete("/:id", adminMiddleware, config.BackupHandler.DeleteBackup)
		backup.Post("/:id/restore", adminMiddleware, config.BackupHandler.RestoreBackup)
		backup.Get("/:id/download", adminMiddleware, config.BackupHandler.DownloadBackup)

		// Rutas de automatización (solo admin)
		backupAuto := backup.Group("/automated", adminMiddleware)
		backupAuto.Post("/start", config.BackupHandler.StartAutomatedBackups)
		backupAuto.Post("/stop", config.BackupHandler.StopAutomatedBackups)
		backupAuto.Post("/cleanup", config.BackupHandler.CleanupOldBackups)

		config.Logger.Info("Backup routes configured successfully")
	}

	// Rutas de workers
	if config.WorkerHandler != nil {
		workers := protected.Group("/workers")

		// Rutas básicas existentes
		workers.Get("/stats", config.WorkerHandler.GetWorkerStats)
		workers.Get("/health", config.WorkerHandler.HealthCheck)
		workers.Get("/queue/stats", config.WorkerHandler.GetQueueStats)
		workers.Get("/processing", config.WorkerHandler.GetProcessingTasks)
		workers.Get("/failed", config.WorkerHandler.GetFailedTasks)
		workers.Post("/retry/:task_id", config.WorkerHandler.RetryFailedTask)
		workers.Post("/queue/clear", config.WorkerHandler.ClearQueue)

		// Rutas avanzadas de workers
		workersAdvanced := workers.Group("/advanced")
		workersAdvanced.Get("/stats", config.WorkerHandler.GetAdvancedStats)
		workersAdvanced.Get("/health", config.WorkerHandler.GetHealthStatus)

		// Worker Pool Management
		workerPool := workers.Group("/pool")
		workerPool.Get("/stats", config.WorkerHandler.GetPoolStats)
		workerPool.Post("/scale", config.WorkerHandler.ScaleWorkerPool)

		// Metrics System
		metricsGroup := workers.Group("/metrics")
		metricsGroup.Get("/detailed", config.WorkerHandler.GetMetricsDetails)
		metricsGroup.Post("/reset", config.WorkerHandler.ResetMetrics)

		// Retry System
		retryGroup := workers.Group("/retries")
		retryGroup.Get("/stats", config.WorkerHandler.GetRetryStats)

		// Action Executors
		executorsGroup := workers.Group("/executors")
		executorsGroup.Get("/info", config.WorkerHandler.GetExecutorInfo)
	}

	// Rutas de dashboard
	if config.DashboardHandler != nil {
		dashboard := protected.Group("/dashboard")
		dashboard.Get("/", config.DashboardHandler.GetDashboard)
		dashboard.Get("/summary", config.DashboardHandler.GetDashboardSummary)
		dashboard.Get("/health", config.DashboardHandler.GetSystemHealth)
		dashboard.Get("/stats", config.DashboardHandler.GetQuickStats)
	}
}

// SetupMonitoringRoutes configurar rutas de monitoreo (sin autenticación)
func SetupMonitoringRoutes(app *fiber.App, config *RouteConfig) {
	monitoring := app.Group("/monitoring")

	// Health check simple para load balancers
	monitoring.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "engine-api-workflow",
			"version": "2.0",
		})
	})

	// Métricas básicas para sistemas de monitoreo
	if config.WorkerHandler != nil {
		monitoring.Get("/metrics", func(c *fiber.Ctx) error {
			return c.SendString("# Metrics endpoint - TODO: Implement Prometheus format\n")
		})

		// Status detallado para dashboards externos
		monitoring.Get("/status", config.WorkerHandler.GetAdvancedStats)

		// Health detallado para sistemas de monitoreo
		monitoring.Get("/health/detailed", config.WorkerHandler.GetHealthStatus)
	}

	//  Métricas de backup para monitoreo externo (sin autenticación)
	if config.BackupHandler != nil {
		monitoring.Get("/backup/status", config.BackupHandler.GetBackupStatus)
		config.Logger.Info("Backup monitoring routes configured")
	}
}

// SetupAdminRoutes configurar rutas de administración
func SetupAdminRoutes(app *fiber.App, config *RouteConfig) {
	// Middleware que verifica rol de administrador
	adminMiddleware := middleware.NewAuthMiddleware(config.JWTService, nil).RequireAdmin()
	admin := app.Group("/admin", adminMiddleware)

	// Gestión de sistema (solo admin)
	if config.WorkerHandler != nil {
		system := admin.Group("/system")
		system.Get("/stats", config.WorkerHandler.GetAdvancedStats)
		system.Post("/workers/scale", config.WorkerHandler.ScaleWorkerPool)
		system.Post("/metrics/reset", config.WorkerHandler.ResetMetrics)
		system.Post("/queue/clear", config.WorkerHandler.ClearQueue)
	}

	//  Gestión de backup (solo admin)
	if config.BackupHandler != nil {
		backupAdmin := admin.Group("/backup")
		backupAdmin.Post("/create", config.BackupHandler.CreateBackup)
		backupAdmin.Post("/force-cleanup", config.BackupHandler.CleanupOldBackups)
		backupAdmin.Get("/system-status", config.BackupHandler.GetBackupStatus)

		config.Logger.Info("Admin backup routes configured")
	}
}

// SetupAllRoutes configuración completa con todas las rutas
func SetupAllRoutes(app *fiber.App, config *RouteConfig) {
	// Rutas principales de la API
	SetupRoutes(app, config)

	// Rutas de monitoreo (sin autenticación)
	SetupMonitoringRoutes(app, config)

	// Rutas de administración (con autenticación de admin)
	SetupAdminRoutes(app, config)

	// Log de rutas configuradas
	config.Logger.Info("All routes configured successfully",
		zap.Bool("auth_routes", config.AuthHandler != nil),
		zap.Bool("workflow_routes", config.WorkflowHandler != nil),
		zap.Bool("worker_routes", config.WorkerHandler != nil),
		zap.Bool("dashboard_routes", config.DashboardHandler != nil),
		zap.Bool("backup_routes", config.BackupHandler != nil)) // AGREGADO
}

func ValidateRouteConfig(config *RouteConfig) error {
	if config == nil {
		return fmt.Errorf("route config cannot be nil")
	}
	if config.AuthHandler == nil {
		return fmt.Errorf("auth handler is required")
	}
	if config.JWTService == nil {
		return fmt.Errorf("JWT service is required")
	}
	if config.Logger == nil {
		return fmt.Errorf("logger is required")
	}

	// Advertencias para handlers opcionales
	if config.WorkerHandler == nil {
		config.Logger.Warn("Worker handler not configured - worker routes will be skipped")
	}
	if config.WorkflowHandler == nil {
		config.Logger.Warn("Workflow handler not configured - workflow routes will be skipped")
	}
	if config.DashboardHandler == nil {
		config.Logger.Warn("Dashboard handler not configured - dashboard routes will be skipped")
	}
	//  AGREGADO
	if config.BackupHandler == nil {
		config.Logger.Warn("Backup handler not configured - backup routes will be skipped")
	}

	return nil
}

func GetRouteInfo() map[string]interface{} {
	return map[string]interface{}{
		"version": "2.0.0",
		"features": []string{
			"authentication",
			"workflows",
			"workers",
			"advanced_workers",
			"metrics",
			"retries",
			"monitoring",
			"admin",
			"backup", //  AGREGADO
		},
		"endpoints": map[string]interface{}{
			"auth":       []string{"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/refresh"},
			"workflows":  []string{"/api/v1/workflows", "/api/v1/workflows/:id"},
			"workers":    []string{"/api/v1/workers/stats", "/api/v1/workers/health", "/api/v1/workers/advanced/stats"},
			"monitoring": []string{"/monitoring/health", "/monitoring/status", "/monitoring/metrics"},
			"admin":      []string{"/admin/system/stats"},
			"backup":     []string{"/api/v1/backup/status", "/api/v1/backup/list", "/api/v1/backup/create"}, //  AGREGADO
		},
	}
}

// GetBackupRouteInfo obtener información de rutas específicas de backup
func GetBackupRouteInfo() map[string]interface{} {
	return map[string]interface{}{
		"public_routes": []string{
			// Ninguna ruta de backup es pública por seguridad
		},
		"authenticated_routes": []string{
			"GET /api/v1/backup/status",
			"GET /api/v1/backup/list",
			"GET /api/v1/backup/:id",
			"POST /api/v1/backup/:id/validate",
		},
		"admin_only_routes": []string{
			"POST /api/v1/backup/create",
			"DELETE /api/v1/backup/:id",
			"POST /api/v1/backup/:id/restore",
			"GET /api/v1/backup/:id/download",
			"POST /api/v1/backup/automated/start",
			"POST /api/v1/backup/automated/stop",
			"POST /api/v1/backup/automated/cleanup",
		},
		"monitoring_routes": []string{
			"GET /monitoring/backup/status",
		},
		"admin_panel_routes": []string{
			"POST /admin/backup/create",
			"POST /admin/backup/force-cleanup",
			"GET /admin/backup/system-status",
		},
		"supported_backup_types": []string{
			"full", "incremental", "mongodb", "redis", "config",
		},
		"required_permissions": map[string]string{
			"view_status":      "authenticated_user",
			"list_backups":     "authenticated_user",
			"create_backup":    "admin",
			"delete_backup":    "admin",
			"restore_backup":   "admin",
			"manage_automated": "admin",
		},
	}
}

// GetWorkerRouteInfo obtener información de rutas específicas de workers
func GetWorkerRouteInfo() map[string]interface{} {
	return map[string]interface{}{
		"basic_routes": []string{
			"GET /api/v1/workers/stats",
			"GET /api/v1/workers/health",
			"GET /api/v1/workers/queue/stats",
			"GET /api/v1/workers/processing",
			"GET /api/v1/workers/failed",
			"POST /api/v1/workers/retry/:task_id",
			"POST /api/v1/workers/queue/clear",
		},
		"advanced_routes": []string{
			"GET /api/v1/workers/advanced/stats",
			"GET /api/v1/workers/advanced/health",
			"GET /api/v1/workers/pool/stats",
			"POST /api/v1/workers/pool/scale",
			"GET /api/v1/workers/metrics/detailed",
			"POST /api/v1/workers/metrics/reset",
			"GET /api/v1/workers/retries/stats",
			"GET /api/v1/workers/executors/info",
		},
		"monitoring_routes": []string{
			"GET /monitoring/health",
			"GET /monitoring/status",
			"GET /monitoring/health/detailed",
		},
		"admin_routes": []string{
			"GET /admin/system/stats",
			"POST /admin/workers/scale",
			"POST /admin/metrics/reset",
			"POST /admin/queue/clear",
		},
	}
}

// GetAllRouteInfo obtener información completa de todas las rutas
func GetAllRouteInfo() map[string]interface{} {
	return map[string]interface{}{
		"api_version": "2.0.0",
		"service":     "engine-api-workflow",
		"routes": map[string]interface{}{
			"general": GetRouteInfo(),
			"workers": GetWorkerRouteInfo(),
			"backup":  GetBackupRouteInfo(), // AGREGADO
		},
		"security": map[string]interface{}{
			"authentication_required": []string{
				"/api/v1/auth/profile",
				"/api/v1/workflows/*",
				"/api/v1/workers/*",
				"/api/v1/backup/*", // AGREGADO
			},
			"admin_required": []string{
				"/admin/*",
				"/api/v1/backup/create",
				"/api/v1/backup/*/restore",
				"/api/v1/backup/*/delete",
				"/api/v1/backup/automated/*", // AGREGADO
			},
			"public_endpoints": []string{
				"/api/v1/health",
				"/api/v1/auth/register",
				"/api/v1/auth/login",
				"/api/v1/auth/refresh",
				"/monitoring/health",
			},
		},
		"features": []string{
			"jwt_authentication",
			"role_based_access_control",
			"workflow_management",
			"worker_pool_management",
			"advanced_metrics",
			"automated_backups",  // AGREGADO
			"backup_restoration", // AGREGADO
			"system_monitoring",
			"admin_panel",
		},
	}
}

// LogRouteAccess middleware de logging para rutas
func LogRouteAccess(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start)

		// MEJORADO: Logging especial para rutas críticas de backup
		logLevel := zap.InfoLevel
		if c.Path() == "/api/v1/backup/create" ||
			c.Path() == "/api/v1/backup/*/restore" ||
			c.Path() == "/api/v1/backup/*/delete" {
			logLevel = zap.WarnLevel // Nivel de advertencia para operaciones críticas
		}

		logger.Log(logLevel, "Route accessed",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("duration", duration),
			zap.String("ip", c.IP()),
			zap.String("user_agent", c.Get("User-Agent")))

		return err
	}
}
