package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
	"Engine_API_Workflow/internal/repository"
	"Engine_API_Workflow/internal/services"
)

// WorkerConfig configuración del worker engine
type WorkerConfig struct {
	Workers    int `json:"workers"`
	MaxWorkers int `json:"max_workers"`
}

// WorkerEngine - ACTUALIZADO CON EJECUTORES REALES CONECTADOS
type WorkerEngine struct {
	// Repositorios y servicios
	queueRepo    repository.QueueRepository
	workflowRepo repository.WorkflowRepository
	logRepo      repository.LogRepository
	userRepo     repository.UserRepository
	logService   services.LogService

	// Ejecutor principal
	executor *WorkflowExecutor

	// EJECUTORES DE ACCIONES REALES
	httpExecutor      *HTTPActionExecutor
	emailExecutor     *EmailActionExecutor
	slackExecutor     *SlackActionExecutor
	webhookExecutor   *WebhookActionExecutor
	transformExecutor *TransformActionExecutor // 🆕 NUEVO EJECUTOR
	databaseExecutor  *DatabaseActionExecutor  // 🆕 NUEVO EJECUTOR DB

	// Nuevos componentes avanzados
	retryManager *RetryManager
	metrics      *MetricsCollector
	pool         *WorkerPool

	// Control
	logger      *zap.Logger
	stopCh      chan struct{}
	wg          sync.WaitGroup
	mu          sync.RWMutex
	startTime   time.Time
	currentLoad int64

	// Configuración
	config WorkerConfig
}

// NewWorkerEngine - CONSTRUCTOR CORREGIDO CON CONEXIÓN DE EJECUTORES
func NewWorkerEngine(
	queueRepo repository.QueueRepository,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	userRepo repository.UserRepository,
	logService services.LogService,
	mongoClient *mongo.Client, // 🆕 AGREGAR MONGO CLIENT
	databaseName string, // 🆕 AGREGAR DATABASE NAME
	logger *zap.Logger,
	config WorkerConfig,
) *WorkerEngine {
	// Configuración por defecto
	if config.Workers <= 0 {
		config.Workers = 3
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 20
	}

	// Crear ejecutor principal
	executor := NewWorkflowExecutor(logService, logger)

	// Crear componentes avanzados
	retryManager := NewRetryManager(queueRepo, logRepo, logger)
	metrics := NewMetricsCollector(logger, queueRepo)

	// CREAR EJECUTORES DE ACCIONES REALES
	httpExecutor := NewHTTPActionExecutor(logger)
	emailExecutor := NewEmailActionExecutor(logger)
	slackExecutor := NewSlackActionExecutor(logger)
	webhookExecutor := NewWebhookActionExecutor(logger)
	transformExecutor := NewTransformActionExecutor(logger)                          // 🆕 NUEVO
	databaseExecutor := NewDatabaseActionExecutor(mongoClient, databaseName, logger) // 🆕 NUEVO DB

	// 🚀 CRÍTICO: CONECTAR EJECUTORES AL WORKFLOW EXECUTOR
	executor.SetActionExecutors(httpExecutor, emailExecutor, slackExecutor, webhookExecutor)
	executor.SetTransformExecutor(transformExecutor) // 🆕 CONECTAR TRANSFORM
	executor.SetDatabaseExecutor(databaseExecutor)   // 🆕 CONECTAR DATABASE

	logger.Info("Worker engine initialized with real action executors",
		zap.Int("workers", config.Workers),
		zap.Int("max_workers", config.MaxWorkers),
		zap.Bool("http_executor", httpExecutor != nil),
		zap.Bool("email_executor", emailExecutor != nil),
		zap.Bool("slack_executor", slackExecutor != nil),
		zap.Bool("webhook_executor", webhookExecutor != nil),
		zap.Bool("transform_executor", transformExecutor != nil), // 🆕 LOG TRANSFORM
		zap.Bool("database_executor", databaseExecutor != nil))   // 🆕 LOG DATABASE

	engine := &WorkerEngine{
		// Repositorios y servicios
		queueRepo:    queueRepo,
		workflowRepo: workflowRepo,
		logRepo:      logRepo,
		userRepo:     userRepo,
		logService:   logService,
		executor:     executor,

		// Ejecutores reales
		httpExecutor:      httpExecutor,
		emailExecutor:     emailExecutor,
		slackExecutor:     slackExecutor,
		webhookExecutor:   webhookExecutor,
		transformExecutor: transformExecutor, // 🆕 NUEVO
		databaseExecutor:  databaseExecutor,  // 🆕 NUEVO DB

		// Componentes avanzados
		retryManager: retryManager,
		metrics:      metrics,

		// Control
		logger:      logger,
		stopCh:      make(chan struct{}),
		startTime:   time.Now(),
		currentLoad: 0,
		config:      config,
	}

	// Crear worker pool
	engine.pool = NewWorkerPool(engine, config.Workers, config.MaxWorkers, logger)

	return engine
}

// Start inicia el motor de workers
func (e *WorkerEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("Starting worker engine with real executors",
		zap.Int("workers", e.config.Workers),
		zap.Int("max_workers", e.config.MaxWorkers))

	// Iniciar worker pool
	if e.pool != nil {
		if err := e.pool.Start(ctx); err != nil {
			return err
		}
	}

	// Iniciar retry manager
	if e.retryManager != nil {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.retryManager.Start(ctx)
		}()
	}

	// Iniciar métricas collector
	if e.metrics != nil {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.metrics.StartPeriodicCleanup(ctx)
		}()
	}

	e.logger.Info("Worker engine started successfully")
	return nil
}

// Stop detiene el motor de workers
func (e *WorkerEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("Stopping worker engine")

	close(e.stopCh)

	// Detener worker pool
	if e.pool != nil {
		e.pool.Stop()
	}

	// Esperar que todas las goroutines terminen
	e.wg.Wait()

	e.logger.Info("Worker engine stopped")
}

// GetExecutorInfo obtiene información sobre ejecutores configurados
func (e *WorkerEngine) GetExecutorInfo() map[string]interface{} {
	return map[string]interface{}{
		"mode": "real", // YA NO ES SIMULADO
		"executors": map[string]bool{
			"http":      e.httpExecutor != nil,
			"email":     e.emailExecutor != nil,
			"slack":     e.slackExecutor != nil,
			"webhook":   e.webhookExecutor != nil,
			"transform": e.transformExecutor != nil, // 🆕 NUEVO
			"database":  e.databaseExecutor != nil,  // 🆕 NUEVO DB
		},
		"executor_modes":     e.executor.GetExecutorModes(),
		"has_real_executors": e.executor.HasRealExecutors(),
	}
}

// GetRetryManager obtiene el retry manager
func (e *WorkerEngine) GetRetryManager() *RetryManager {
	return e.retryManager
}

// GetMetricsCollector obtiene el metrics collector
func (e *WorkerEngine) GetMetricsCollector() *MetricsCollector {
	return e.metrics
}

// GetPool obtiene el worker pool
func (e *WorkerEngine) GetPool() *WorkerPool {
	return e.pool
}

// ExecuteWorkflow ejecuta un workflow específico
func (e *WorkerEngine) ExecuteWorkflow(ctx context.Context, workflow *models.Workflow, userID, logID primitive.ObjectID, triggerData map[string]interface{}) error {
	return e.executor.Execute(ctx, workflow, userID, logID, triggerData)
}

// GetCurrentLoad obtiene la carga actual
func (e *WorkerEngine) GetCurrentLoad() int64 {
	return e.currentLoad
}

// GetUptime obtiene el tiempo de actividad
func (e *WorkerEngine) GetUptime() time.Duration {
	return time.Since(e.startTime)
}

// 🆕 NUEVO MÉTODO: Test database connection
func (e *WorkerEngine) TestDatabaseConnection(ctx context.Context) error {
	if e.databaseExecutor != nil {
		return e.databaseExecutor.TestConnection(ctx)
	}
	return fmt.Errorf("database executor not configured")
}

// 🆕 NUEVO MÉTODO: Get database stats
func (e *WorkerEngine) GetDatabaseStats(ctx context.Context) (map[string]interface{}, error) {
	if e.databaseExecutor != nil {
		return e.databaseExecutor.GetStats(ctx)
	}
	return nil, fmt.Errorf("database executor not configured")
}
