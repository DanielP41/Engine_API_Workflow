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
	Workers           int           `json:"workers"`
	MaxWorkers        int           `json:"max_workers"`
	PollInterval      time.Duration `json:"poll_interval"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	ProcessingTimeout time.Duration `json:"processing_timeout"`
}

// WorkerEngine - ACTUALIZADO CON EJECUTORES REALES CONECTADOS
type WorkerEngine struct {
	queueRepo         repository.QueueRepository
	workflowRepo      repository.WorkflowRepository
	logRepo           repository.LogRepository
	userRepo          repository.UserRepository
	logService        services.LogService
	executor          *WorkflowExecutor
	httpExecutor      *HTTPActionExecutor
	emailExecutor     *EmailActionExecutor
	slackExecutor     *SlackActionExecutor
	webhookExecutor   *WebhookActionExecutor
	transformExecutor *TransformActionExecutor
	databaseExecutor  *DatabaseActionExecutor
	retryManager      *RetryManager
	metrics           *MetricsCollector
	pool              *WorkerPool
	logger            *zap.Logger
	stopCh            chan struct{}
	wg                sync.WaitGroup
	mu                sync.RWMutex
	startTime         time.Time
	currentLoad       int64
	config            WorkerConfig
}

func NewWorkerEngine(
	queueRepo repository.QueueRepository,
	workflowRepo repository.WorkflowRepository,
	logRepo repository.LogRepository,
	userRepo repository.UserRepository,
	logService services.LogService,
	mongoClient *mongo.Client,
	databaseName string,
	logger *zap.Logger,
	config WorkerConfig,
) *WorkerEngine {
	if config.Workers <= 0 {
		config.Workers = 3
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 20
	}
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.ProcessingTimeout == 0 {
		config.ProcessingTimeout = 30 * time.Minute
	}

	executor := NewWorkflowExecutor(logService, logger)
	retryManager := NewRetryManager(queueRepo, logRepo, logger)
	metrics := NewMetricsCollector(logger, queueRepo)

	httpExecutor := NewHTTPActionExecutor(logger)
	emailExecutor := NewEmailActionExecutor(logger)
	slackExecutor := NewSlackActionExecutor(logger)
	webhookExecutor := NewWebhookActionExecutor(logger)
	transformExecutor := NewTransformActionExecutor(logger)
	databaseExecutor := NewDatabaseActionExecutor(mongoClient, databaseName, logger)

	executor.SetActionExecutors(httpExecutor, emailExecutor, slackExecutor, webhookExecutor)
	executor.SetTransformExecutor(transformExecutor)
	executor.SetDatabaseExecutor(databaseExecutor)

	logger.Info("Worker engine initialized with real action executors",
		zap.Int("workers", config.Workers),
		zap.Int("max_workers", config.MaxWorkers),
		zap.Bool("http_executor", httpExecutor != nil),
		zap.Bool("email_executor", emailExecutor != nil),
		zap.Bool("slack_executor", slackExecutor != nil),
		zap.Bool("webhook_executor", webhookExecutor != nil),
		zap.Bool("transform_executor", transformExecutor != nil),
		zap.Bool("database_executor", databaseExecutor != nil))

	engine := &WorkerEngine{
		queueRepo:         queueRepo,
		workflowRepo:      workflowRepo,
		logRepo:           logRepo,
		userRepo:          userRepo,
		logService:        logService,
		executor:          executor,
		httpExecutor:      httpExecutor,
		emailExecutor:     emailExecutor,
		slackExecutor:     slackExecutor,
		webhookExecutor:   webhookExecutor,
		transformExecutor: transformExecutor,
		databaseExecutor:  databaseExecutor,
		retryManager:      retryManager,
		metrics:           metrics,
		logger:            logger,
		stopCh:            make(chan struct{}),
		startTime:         time.Now(),
		currentLoad:       0,
		config:            config,
	}

	engine.pool = NewWorkerPool(engine, config.Workers, config.MaxWorkers, logger)

	return engine
}

func (e *WorkerEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("Starting worker engine with real executors",
		zap.Int("workers", e.config.Workers),
		zap.Int("max_workers", e.config.MaxWorkers))

	if e.pool != nil {
		if err := e.pool.Start(ctx); err != nil {
			return err
		}
	}

	if e.retryManager != nil {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.retryManager.StartProcessing(ctx)
		}()
	}

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

func (e *WorkerEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("Stopping worker engine")

	close(e.stopCh)

	if e.pool != nil {
		e.pool.Stop()
	}

	e.wg.Wait()

	e.logger.Info("Worker engine stopped")
}

func (e *WorkerEngine) processTask(ctx context.Context, task *models.QueueTask, logger *zap.Logger) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}

	logger.Info("Processing task",
		zap.String("task_id", task.ID.Hex()),
		zap.String("workflow_id", task.WorkflowID.Hex()),
		zap.String("task_type", task.TaskType))

	workflow, err := e.workflowRepo.GetByID(ctx, task.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	err = e.executor.Execute(ctx, workflow, task.UserID, task.ID, task.Payload)
	if err != nil {
		policy := e.retryManager.GetWorkflowRetryPolicy(workflow)
		if e.retryManager.ShouldRetry(task, err, policy) {
			retryErr := e.retryManager.ScheduleRetry(ctx, task, err, policy)
			if retryErr != nil {
				logger.Error("Failed to schedule retry",
					zap.String("task_id", task.ID.Hex()),
					zap.Error(retryErr))
			}
			return retryErr
		}
		return err
	}

	return e.queueRepo.MarkCompleted(ctx, task.ID.Hex())
}

func (e *WorkerEngine) GetExecutorInfo() map[string]interface{} {
	return map[string]interface{}{
		"mode": "real",
		"executors": map[string]bool{
			"http":      e.httpExecutor != nil,
			"email":     e.emailExecutor != nil,
			"slack":     e.slackExecutor != nil,
			"webhook":   e.webhookExecutor != nil,
			"transform": e.transformExecutor != nil,
			"database":  e.databaseExecutor != nil,
		},
		"executor_modes":     e.executor.GetExecutorModes(),
		"has_real_executors": e.executor.HasRealExecutors(),
	}
}

func (e *WorkerEngine) GetRetryManager() *RetryManager {
	return e.retryManager
}

func (e *WorkerEngine) GetMetricsCollector() *MetricsCollector {
	return e.metrics
}

func (e *WorkerEngine) GetPool() *WorkerPool {
	return e.pool
}

func (e *WorkerEngine) GetWorkerPool() *WorkerPool {
	return e.pool
}

func (e *WorkerEngine) ExecuteWorkflow(ctx context.Context, workflow *models.Workflow, userID, logID primitive.ObjectID, triggerData map[string]interface{}) error {
	return e.executor.Execute(ctx, workflow, userID, logID, triggerData)
}

func (e *WorkerEngine) GetCurrentLoad() int64 {
	return e.currentLoad
}

func (e *WorkerEngine) GetUptime() time.Duration {
	return time.Since(e.startTime)
}

func (e *WorkerEngine) TestDatabaseConnection(ctx context.Context) error {
	if e.databaseExecutor != nil {
		return e.databaseExecutor.TestConnection(ctx)
	}
	return fmt.Errorf("database executor not configured")
}

func (e *WorkerEngine) GetDatabaseStats(ctx context.Context) (map[string]interface{}, error) {
	if e.databaseExecutor != nil {
		return e.databaseExecutor.GetStats(ctx)
	}
	return nil, fmt.Errorf("database executor not configured")
}

func (e *WorkerEngine) GetAdvancedStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	if e.pool != nil {
		stats["pool"] = e.pool.GetStats()
	}

	if e.retryManager != nil {
		stats["retry"] = e.retryManager.GetRetryStats(ctx)
	}

	if e.metrics != nil {
		stats["metrics"] = e.metrics.GetMetrics()
	}

	stats["engine"] = map[string]interface{}{
		"uptime":       e.GetUptime(),
		"current_load": e.GetCurrentLoad(),
		"config":       e.config,
	}

	return stats, nil
}

func (e *WorkerEngine) GetHealthStatus(ctx context.Context) (map[string]interface{}, error) {
	health := map[string]interface{}{
		"is_healthy": true,
		"timestamp":  time.Now(),
		"components": make(map[string]interface{}),
		"issues":     make([]string, 0),
	}

	issues := make([]string, 0)

	if e.pool != nil {
		poolStats := e.pool.GetStats()
		workerCount := poolStats["total_workers"].(int)
		if workerCount == 0 {
			issues = append(issues, "No workers available")
		}
		health["components"].(map[string]interface{})["pool"] = map[string]interface{}{
			"healthy":      workerCount > 0,
			"worker_count": workerCount,
		}
	}

	if e.databaseExecutor != nil {
		dbErr := e.TestDatabaseConnection(ctx)
		health["components"].(map[string]interface{})["database"] = map[string]interface{}{
			"healthy": dbErr == nil,
			"error":   dbErr,
		}
		if dbErr != nil {
			issues = append(issues, fmt.Sprintf("Database connection failed: %v", dbErr))
		}
	}

	isHealthy := len(issues) == 0
	health["is_healthy"] = isHealthy
	health["issues"] = issues

	return health, nil
}
