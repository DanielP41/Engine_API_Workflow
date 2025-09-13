package worker

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"Engine_API_Workflow/internal/models"

	"go.uber.org/zap"
)

// WorkerPool maneja un pool dinámico de workers
type WorkerPool struct {
	engine       *WorkerEngine
	workers      map[int]*Worker
	workersMutex sync.RWMutex
	maxWorkers   int
	minWorkers   int
	currentLoad  int64
	scalingMutex sync.Mutex
	logger       *zap.Logger
	stopCh       chan struct{}
	wg           sync.WaitGroup
	isRunning    bool

	// Funcionalidades avanzadas
	healthChecker   *HealthChecker
	metrics         *PoolMetrics
	scalingHistory  []ScalingEvent
	lastScaleTime   time.Time
	emergencyMode   bool
	emergencyReason string
	config          PoolConfig
}

// Worker representa un worker individual
type Worker struct {
	ID       int
	engine   *WorkerEngine
	tasksCh  chan *models.QueueTask
	stopCh   chan struct{}
	isActive bool
	mutex    sync.RWMutex
	stats    WorkerStats
	logger   *zap.Logger

	// Funcionalidades avanzadas
	startTime     time.Time
	lastHeartbeat time.Time
	isHealthy     bool
	taskTimeout   time.Duration
}

// WorkerStats estadísticas de un worker
type WorkerStats struct {
	TasksProcessed  int64
	TasksSucceeded  int64
	TasksFailed     int64
	LastTaskTime    time.Time
	TotalRunTime    time.Duration
	IsIdle          bool
	AverageTaskTime time.Duration
	TasksPerSecond  float64
	ErrorRate       float64
	LastErrorTime   time.Time
	LastError       string
}

// PoolConfig configuración específica del pool
type PoolConfig struct {
	ScalingInterval     time.Duration
	HealthCheckInterval time.Duration
	TaskChannelSize     int
	HeartbeatInterval   time.Duration
	MaxIdleTime         time.Duration
	ScalingThreshold    float64
	DownscaleThreshold  float64
	EmergencyThreshold  int64
	EnableAutoScaling   bool
	EnableHealthChecks  bool
	MaxScalingFrequency time.Duration
}

// ScalingEvent evento de escalamiento
type ScalingEvent struct {
	Timestamp   time.Time
	Action      string
	FromWorkers int
	ToWorkers   int
	Reason      string
	QueueLength int64
	AvgLoad     float64
}

// PoolMetrics métricas del pool
type PoolMetrics struct {
	TotalTasksProcessed int64
	TotalTasksSucceeded int64
	TotalTasksFailed    int64
	AverageProcessTime  time.Duration
	ThroughputPerSecond float64
	UpTime              time.Duration
	LastResetTime       time.Time
	ScalingEvents       int
}

// HealthChecker verificador de salud
type HealthChecker struct {
	checkInterval time.Duration
	lastCheck     time.Time
	healthStatus  HealthStatus
	mutex         sync.RWMutex
}

// HealthStatus estado de salud
type HealthStatus struct {
	IsHealthy      bool
	UnhealthyCount int
	LastCheckTime  time.Time
	Issues         []string
}

// DefaultPoolConfig retorna configuración por defecto del pool
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		ScalingInterval:     30 * time.Second,
		HealthCheckInterval: 15 * time.Second,
		TaskChannelSize:     10,
		HeartbeatInterval:   30 * time.Second,
		MaxIdleTime:         5 * time.Minute,
		ScalingThreshold:    0.8,
		DownscaleThreshold:  0.3,
		EmergencyThreshold:  100,
		EnableAutoScaling:   true,
		EnableHealthChecks:  true,
		MaxScalingFrequency: 1 * time.Minute,
	}
}

// NewWorkerPool crea un nuevo pool de workers compatible con tu engine
func NewWorkerPool(engine *WorkerEngine, minWorkers, maxWorkers int, logger *zap.Logger) *WorkerPool {
	config := DefaultPoolConfig()

	pool := &WorkerPool{
		engine:         engine,
		workers:        make(map[int]*Worker),
		maxWorkers:     maxWorkers,
		minWorkers:     minWorkers,
		logger:         logger,
		stopCh:         make(chan struct{}),
		isRunning:      false,
		config:         config,
		scalingHistory: make([]ScalingEvent, 0),
		healthChecker: &HealthChecker{
			checkInterval: config.HealthCheckInterval,
			healthStatus:  HealthStatus{IsHealthy: true, Issues: make([]string, 0)},
		},
		metrics: &PoolMetrics{
			LastResetTime: time.Now(),
		},
	}

	return pool
}

// Start inicia el pool de workers
func (p *WorkerPool) Start(ctx context.Context) error {
	p.logger.Info("Starting enhanced worker pool",
		zap.Int("min_workers", p.minWorkers),
		zap.Int("max_workers", p.maxWorkers),
		zap.Bool("auto_scaling", p.config.EnableAutoScaling),
		zap.Bool("health_checks", p.config.EnableHealthChecks))

	p.isRunning = true
	p.metrics.LastResetTime = time.Now()

	// Iniciar workers mínimos
	for i := 0; i < p.minWorkers; i++ {
		if err := p.createWorker(i); err != nil {
			return fmt.Errorf("failed to create initial worker %d: %w", i, err)
		}
	}

	// Iniciar procesos de monitoreo
	if p.config.EnableAutoScaling {
		p.wg.Add(1)
		go p.scalingMonitor(ctx)
	}

	if p.config.EnableHealthChecks {
		p.wg.Add(1)
		go p.healthMonitor(ctx)
	}

	// Iniciar dispatcher mejorado
	p.wg.Add(1)
	go p.enhancedTaskDispatcher(ctx)

	// Iniciar collector de métricas
	p.wg.Add(1)
	go p.metricsCollector(ctx)

	p.logger.Info("Enhanced worker pool started successfully")
	return nil
}

// Stop detiene el pool de workers
func (p *WorkerPool) Stop() error {
	p.logger.Info("Stopping enhanced worker pool...")

	p.isRunning = false
	close(p.stopCh)

	// Graceful shutdown con timeout
	done := make(chan struct{})
	go func() {
		p.workersMutex.Lock()
		for _, worker := range p.workers {
			worker.Stop()
		}
		p.workersMutex.Unlock()
		p.wg.Wait()
		close(done)
	}()

	// Timeout para shutdown
	select {
	case <-done:
		p.logger.Info("Enhanced worker pool stopped gracefully")
	case <-time.After(30 * time.Second):
		p.logger.Warn("Worker pool shutdown timeout, forcing stop")
	}

	return nil
}

// createWorker crea un nuevo worker
func (p *WorkerPool) createWorker(id int) error {
	worker := &Worker{
		ID:            id,
		engine:        p.engine,
		tasksCh:       make(chan *models.QueueTask, p.config.TaskChannelSize),
		stopCh:        make(chan struct{}),
		logger:        p.logger.With(zap.Int("worker_id", id)),
		stats:         WorkerStats{IsIdle: true},
		startTime:     time.Now(),
		lastHeartbeat: time.Now(),
		isHealthy:     true,
		taskTimeout:   30 * time.Minute, // Usar timeout del engine
	}

	p.workersMutex.Lock()
	p.workers[id] = worker
	p.workersMutex.Unlock()

	p.wg.Add(1)
	go worker.Run()

	// Registrar evento de escalamiento
	p.recordScalingEvent("scale_up", len(p.workers)-1, len(p.workers), "worker_created")

	p.logger.Info("Created enhanced worker",
		zap.Int("worker_id", id),
		zap.Int("total_workers", len(p.workers)))
	return nil
}

// removeWorker elimina un worker del pool
func (p *WorkerPool) removeWorker(id int) {
	p.workersMutex.Lock()
	defer p.workersMutex.Unlock()

	if worker, exists := p.workers[id]; exists {
		worker.Stop()

		// Esperar a que termine las tareas actuales
		go func() {
			time.Sleep(5 * time.Second) // Grace period
			delete(p.workers, id)
		}()

		// Registrar evento de escalamiento
		p.recordScalingEvent("scale_down", len(p.workers), len(p.workers)-1, "worker_removed")

		p.logger.Info("Removed worker",
			zap.Int("worker_id", id),
			zap.Int("remaining_workers", len(p.workers)-1))
	}
}

// scalingMonitor monitorea la carga y escala automáticamente
func (p *WorkerPool) scalingMonitor(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.ScalingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.evaluateScaling()
		}
	}
}

// evaluateScaling evalúa si necesita escalar workers
func (p *WorkerPool) evaluateScaling() {
	p.scalingMutex.Lock()
	defer p.scalingMutex.Unlock()

	// Evitar scaling muy frecuente
	if time.Since(p.lastScaleTime) < p.config.MaxScalingFrequency {
		return
	}

	currentWorkers := len(p.workers)
	queueLength := p.getQueueLength()
	avgLoad := p.calculateAverageLoad()

	// Obtener métricas adicionales
	errorRate := p.calculateErrorRate()

	p.logger.Debug("Enhanced scaling evaluation",
		zap.Int("current_workers", currentWorkers),
		zap.Int64("queue_length", queueLength),
		zap.Float64("avg_load", avgLoad),
		zap.Float64("error_rate", errorRate))

	// Detectar modo de emergencia
	if queueLength > p.config.EmergencyThreshold {
		p.enterEmergencyMode("high_queue_length")
	} else if p.emergencyMode && queueLength < p.config.EmergencyThreshold/2 {
		p.exitEmergencyMode()
	}

	// Lógica de escalamiento
	scalingDecision := p.makeScalingDecision(currentWorkers, queueLength, avgLoad, errorRate)

	if scalingDecision.action != "none" {
		p.executeScalingDecision(scalingDecision)
		p.lastScaleTime = time.Now()
	}
}

type ScalingDecision struct {
	action     string // "scale_up", "scale_down", "none"
	targetSize int
	reason     string
	priority   int
}

func (p *WorkerPool) makeScalingDecision(currentWorkers int, queueLength int64, avgLoad, errorRate float64) ScalingDecision {
	// Modo emergencia: escalar agresivamente
	if p.emergencyMode {
		targetWorkers := min(p.maxWorkers, currentWorkers*2)
		return ScalingDecision{
			action:     "scale_up",
			targetSize: targetWorkers,
			reason:     "emergency_mode",
			priority:   10,
		}
	}

	// Escalar hacia arriba
	if avgLoad > p.config.ScalingThreshold && currentWorkers < p.maxWorkers {
		workersNeeded := int(float64(queueLength)/5.0) + 1
		targetWorkers := min(p.maxWorkers, currentWorkers+workersNeeded)

		return ScalingDecision{
			action:     "scale_up",
			targetSize: targetWorkers,
			reason:     fmt.Sprintf("high_load_%.2f", avgLoad),
			priority:   5,
		}
	}

	// Escalar hacia abajo
	if avgLoad < p.config.DownscaleThreshold && currentWorkers > p.minWorkers && queueLength == 0 {
		targetWorkers := max(p.minWorkers, currentWorkers-1)

		return ScalingDecision{
			action:     "scale_down",
			targetSize: targetWorkers,
			reason:     fmt.Sprintf("low_load_%.2f", avgLoad),
			priority:   1,
		}
	}

	return ScalingDecision{action: "none"}
}

func (p *WorkerPool) executeScalingDecision(decision ScalingDecision) {
	currentWorkers := len(p.workers)

	if decision.action == "scale_up" {
		workersToAdd := decision.targetSize - currentWorkers
		for i := 0; i < workersToAdd; i++ {
			newWorkerID := p.getNextWorkerID()
			if err := p.createWorker(newWorkerID); err != nil {
				p.logger.Error("Failed to scale up", zap.Error(err))
				break
			}
		}
	} else if decision.action == "scale_down" {
		workersToRemove := currentWorkers - decision.targetSize
		for i := 0; i < workersToRemove; i++ {
			idleWorkerID := p.findIdleWorker()
			if idleWorkerID != -1 {
				p.removeWorker(idleWorkerID)
			}
		}
	}
}

// enhancedTaskDispatcher distribuye tareas de forma optimizada
func (p *WorkerPool) enhancedTaskDispatcher(ctx context.Context) {
	defer p.wg.Done()

	taskBuffer := make([]*models.QueueTask, 0, 10)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			// Procesar tasks en batch para mejor performance
			newTasks := p.fetchTasksBatch(10)
			taskBuffer = append(taskBuffer, newTasks...)

			// Distribuir tasks usando algoritmo optimizado
			taskBuffer = p.distributeTasksOptimally(taskBuffer)
		}
	}
}

func (p *WorkerPool) fetchTasksBatch(maxTasks int) []*models.QueueTask {
	tasks := make([]*models.QueueTask, 0, maxTasks)

	for i := 0; i < maxTasks; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		task, err := p.engine.queueRepo.Dequeue(ctx)
		cancel()

		if err != nil || task == nil {
			break
		}
		tasks = append(tasks, task)
	}

	return tasks
}

func (p *WorkerPool) distributeTasksOptimally(tasks []*models.QueueTask) []*models.QueueTask {
	if len(tasks) == 0 {
		return tasks
	}

	// Obtener workers ordenados por carga (menos cargados primero)
	workers := p.getWorkersByLoad()
	distributed := 0

	for _, task := range tasks {
		if distributed >= len(tasks) {
			break
		}

		// Buscar el worker menos cargado disponible
		for _, worker := range workers {
			select {
			case worker.tasksCh <- task:
				atomic.AddInt64(&p.currentLoad, 1)
				distributed++
				goto nextTask
			default:
				continue // Worker ocupado, probar el siguiente
			}
		}

		// Si no se pudo distribuir, mantener en buffer
		if distributed < len(tasks) {
			return tasks[distributed:]
		}

	nextTask:
	}

	return tasks[distributed:]
}

func (p *WorkerPool) getWorkersByLoad() []*Worker {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	workers := make([]*Worker, 0, len(p.workers))
	for _, worker := range p.workers {
		if worker.isActive {
			workers = append(workers, worker)
		}
	}

	// Ordenar por carga (workers menos ocupados primero)
	sort.Slice(workers, func(i, j int) bool {
		return len(workers[i].tasksCh) < len(workers[j].tasksCh)
	})

	return workers
}

// healthMonitor ejecuta chequeos de salud periódicos
func (p *WorkerPool) healthMonitor(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.performHealthCheck()
		}
	}
}

func (p *WorkerPool) performHealthCheck() {
	p.healthChecker.mutex.Lock()
	defer p.healthChecker.mutex.Unlock()

	issues := make([]string, 0)
	unhealthyWorkers := 0

	p.workersMutex.RLock()
	for _, worker := range p.workers {
		if !worker.isHealthy || time.Since(worker.lastHeartbeat) > p.config.HealthCheckInterval*2 {
			unhealthyWorkers++
			issues = append(issues, fmt.Sprintf("worker_%d_unhealthy", worker.ID))
		}
	}
	p.workersMutex.RUnlock()

	// Verificar salud del pool
	queueLength := p.getQueueLength()
	if queueLength > p.config.EmergencyThreshold {
		issues = append(issues, "queue_length_critical")
	}

	avgLoad := p.calculateAverageLoad()
	if avgLoad > 0.95 {
		issues = append(issues, "high_system_load")
	}

	// Actualizar estado de salud
	p.healthChecker.healthStatus = HealthStatus{
		IsHealthy:      len(issues) == 0,
		UnhealthyCount: unhealthyWorkers,
		LastCheckTime:  time.Now(),
		Issues:         issues,
	}

	p.healthChecker.lastCheck = time.Now()

	if len(issues) > 0 {
		p.logger.Warn("Health check found issues", zap.Strings("issues", issues))
	}
}

// metricsCollector actualiza métricas del pool
func (p *WorkerPool) metricsCollector(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.updatePoolMetrics()
		}
	}
}

func (p *WorkerPool) updatePoolMetrics() {
	var totalProcessed, totalSucceeded, totalFailed int64
	var totalRunTime time.Duration

	p.workersMutex.RLock()
	for _, worker := range p.workers {
		worker.mutex.RLock()
		totalProcessed += worker.stats.TasksProcessed
		totalSucceeded += worker.stats.TasksSucceeded
		totalFailed += worker.stats.TasksFailed
		totalRunTime += worker.stats.TotalRunTime
		worker.mutex.RUnlock()
	}
	p.workersMutex.RUnlock()

	upTime := time.Since(p.metrics.LastResetTime)

	p.metrics.TotalTasksProcessed = totalProcessed
	p.metrics.TotalTasksSucceeded = totalSucceeded
	p.metrics.TotalTasksFailed = totalFailed
	p.metrics.UpTime = upTime

	if totalProcessed > 0 {
		p.metrics.AverageProcessTime = totalRunTime / time.Duration(totalProcessed)
		p.metrics.ThroughputPerSecond = float64(totalProcessed) / upTime.Seconds()
	}
}

// WORKER METHODS

// Run ejecuta el loop principal del worker
func (w *Worker) Run() {
	defer func() {
		w.engine.wg.Done()
		w.logger.Info("Enhanced worker stopped",
			zap.Duration("uptime", time.Since(w.startTime)),
			zap.Int64("tasks_processed", w.stats.TasksProcessed))
	}()

	w.mutex.Lock()
	w.isActive = true
	w.mutex.Unlock()

	w.logger.Info("Enhanced worker started")

	// Iniciar heartbeat
	go w.heartbeatLoop()

	for {
		select {
		case <-w.stopCh:
			return
		case task := <-w.tasksCh:
			w.processTaskEnhanced(task)
		case <-time.After(w.engine.config.ProcessingTimeout):
			// Timeout de inactividad
			w.mutex.RLock()
			isIdle := w.stats.IsIdle
			w.mutex.RUnlock()

			if isIdle {
				w.logger.Debug("Worker idle timeout")
			}
		}
	}
}

// processTaskEnhanced procesa una tarea con métricas mejoradas
func (w *Worker) processTaskEnhanced(task *models.QueueTask) {
	startTime := time.Now()

	w.mutex.Lock()
	w.stats.IsIdle = false
	w.stats.LastTaskTime = startTime
	w.lastHeartbeat = startTime
	w.mutex.Unlock()

	defer func() {
		duration := time.Since(startTime)

		w.mutex.Lock()
		w.stats.IsIdle = true
		w.stats.TasksProcessed++
		w.stats.TotalRunTime += duration

		// Calcular métricas derivadas
		if w.stats.TasksProcessed > 0 {
			w.stats.AverageTaskTime = w.stats.TotalRunTime / time.Duration(w.stats.TasksProcessed)
			uptime := time.Since(w.startTime)
			if uptime.Seconds() > 0 {
				w.stats.TasksPerSecond = float64(w.stats.TasksProcessed) / uptime.Seconds()
			}
			w.stats.ErrorRate = float64(w.stats.TasksFailed) / float64(w.stats.TasksProcessed)
		}
		w.mutex.Unlock()

		atomic.AddInt64(&w.engine.currentLoad, -1)
	}()

	w.logger.Info("Processing enhanced task",
		zap.String("task_id", task.ID),
		zap.Int64("worker_tasks_processed", w.stats.TasksProcessed))

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), w.taskTimeout)
	defer cancel()

	// Procesar tarea usando el método del engine
	err := w.engine.processTask(ctx, task, w.logger)

	w.mutex.Lock()
	if err != nil {
		w.stats.TasksFailed++
		w.stats.LastError = err.Error()
		w.stats.LastErrorTime = time.Now()
		w.isHealthy = false // Marcar como no saludable temporalmente
		w.logger.Error("Enhanced task failed",
			zap.Error(err),
			zap.String("task_id", task.ID))
	} else {
		w.stats.TasksSucceeded++
		w.isHealthy = true // Restaurar salud en éxito
		w.logger.Info("Enhanced task completed successfully",
			zap.String("task_id", task.ID),
			zap.Duration("duration", time.Since(startTime)))
	}
	w.mutex.Unlock()
}

// heartbeatLoop mantiene el heartbeat del worker
func (w *Worker) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second) // Usar valor fijo
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.mutex.Lock()
			w.lastHeartbeat = time.Now()
			w.mutex.Unlock()
		}
	}
}

func (w *Worker) Stop() {
	w.mutex.Lock()
	w.isActive = false
	w.mutex.Unlock()

	w.logger.Info("Stopping enhanced worker gracefully...")
	close(w.stopCh)
}

func (w *Worker) GetStats() WorkerStats {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.stats
}

// HELPER METHODS

func (p *WorkerPool) getQueueLength() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	length, err := p.engine.queueRepo.GetQueueLength(ctx, "workflow:queue")
	if err != nil {
		return 0
	}
	return length
}

func (p *WorkerPool) calculateAverageLoad() float64 {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	if len(p.workers) == 0 {
		return 0
	}

	totalLoad := 0
	for _, worker := range p.workers {
		worker.mutex.RLock()
		if !worker.stats.IsIdle {
			totalLoad++
		}
		worker.mutex.RUnlock()
	}

	return float64(totalLoad) / float64(len(p.workers))
}

func (p *WorkerPool) calculateErrorRate() float64 {
	var totalTasks, totalErrors int64

	p.workersMutex.RLock()
	for _, worker := range p.workers {
		worker.mutex.RLock()
		totalTasks += worker.stats.TasksProcessed
		totalErrors += worker.stats.TasksFailed
		worker.mutex.RUnlock()
	}
	p.workersMutex.RUnlock()

	if totalTasks == 0 {
		return 0
	}
	return float64(totalErrors) / float64(totalTasks)
}

func (p *WorkerPool) findIdleWorker() int {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	for id, worker := range p.workers {
		worker.mutex.RLock()
		isIdle := worker.stats.IsIdle
		timeSinceLastTask := time.Since(worker.stats.LastTaskTime)
		worker.mutex.RUnlock()

		// Solo considerar workers que han estado idle por un tiempo
		if isIdle && timeSinceLastTask > 2*time.Minute {
			return id
		}
	}
	return -1
}

func (p *WorkerPool) getNextWorkerID() int {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	maxID := -1
	for id := range p.workers {
		if id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}

// EMERGENCY MODE METHODS

func (p *WorkerPool) enterEmergencyMode(reason string) {
	if !p.emergencyMode {
		p.emergencyMode = true
		p.emergencyReason = reason
		p.logger.Warn("Entering emergency mode", zap.String("reason", reason))
	}
}

func (p *WorkerPool) exitEmergencyMode() {
	if p.emergencyMode {
		p.emergencyMode = false
		p.emergencyReason = ""
		p.logger.Info("Exiting emergency mode")
	}
}

func (p *WorkerPool) recordScalingEvent(action string, fromWorkers, toWorkers int, reason string) {
	event := ScalingEvent{
		Timestamp:   time.Now(),
		Action:      action,
		FromWorkers: fromWorkers,
		ToWorkers:   toWorkers,
		Reason:      reason,
		QueueLength: p.getQueueLength(),
		AvgLoad:     p.calculateAverageLoad(),
	}

	p.scalingHistory = append(p.scalingHistory, event)

	// Mantener solo los últimos 100 eventos
	if len(p.scalingHistory) > 100 {
		p.scalingHistory = p.scalingHistory[1:]
	}

	p.metrics.ScalingEvents++
}

// PUBLIC API METHODS

func (p *WorkerPool) GetWorkerCount() int {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()
	return len(p.workers)
}

func (p *WorkerPool) GetActiveWorkerCount() int {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	activeCount := 0
	for _, worker := range p.workers {
		worker.mutex.RLock()
		if worker.isActive && !worker.stats.IsIdle {
			activeCount++
		}
		worker.mutex.RUnlock()
	}
	return activeCount
}

func (p *WorkerPool) GetIdleWorkerCount() int {
	totalWorkers := p.GetWorkerCount()
	activeWorkers := p.GetActiveWorkerCount()
	return totalWorkers - activeWorkers
}

func (p *WorkerPool) IsHealthy() bool {
	p.healthChecker.mutex.RLock()
	defer p.healthChecker.mutex.RUnlock()

	return p.isRunning &&
		len(p.workers) >= p.minWorkers &&
		p.healthChecker.healthStatus.IsHealthy
}

func (p *WorkerPool) IsRunning() bool {
	return p.isRunning
}

func (p *WorkerPool) GetHealthStatus() HealthStatus {
	p.healthChecker.mutex.RLock()
	defer p.healthChecker.mutex.RUnlock()
	return p.healthChecker.healthStatus
}

func (p *WorkerPool) GetScalingHistory() []ScalingEvent {
	return p.scalingHistory
}

func (p *WorkerPool) GetMetrics() *PoolMetrics {
	return p.metrics
}

// GetStats mantiene compatibilidad con el método existente del engine
func (p *WorkerPool) GetStats() map[string]interface{} {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	stats := map[string]interface{}{
		"total_workers":  len(p.workers),
		"min_workers":    p.minWorkers,
		"max_workers":    p.maxWorkers,
		"current_load":   atomic.LoadInt64(&p.currentLoad),
		"average_load":   p.calculateAverageLoad(),
		"queue_length":   p.getQueueLength(),
		"is_running":     p.isRunning,
		"worker_details": p.getDetailedWorkerStats(),
		"advanced": map[string]interface{}{
			"emergency_mode":   p.emergencyMode,
			"emergency_reason": p.emergencyReason,
			"error_rate":       p.calculateErrorRate(),
			"throughput":       p.calculateThroughput(),
			"last_scale_time":  p.lastScaleTime,
			"scaling_events":   len(p.scalingHistory),
		},
		"health":  p.GetHealthStatus(),
		"metrics": p.metrics,
	}

	return stats
}

func (p *WorkerPool) getDetailedWorkerStats() []map[string]interface{} {
	stats := make([]map[string]interface{}, 0, len(p.workers))

	for id, worker := range p.workers {
		worker.mutex.RLock()
		workerStats := map[string]interface{}{
			"id":               id,
			"is_active":        worker.isActive,
			"is_healthy":       worker.isHealthy,
			"tasks_processed":  worker.stats.TasksProcessed,
			"tasks_succeeded":  worker.stats.TasksSucceeded,
			"tasks_failed":     worker.stats.TasksFailed,
			"is_idle":          worker.stats.IsIdle,
			"last_task_time":   worker.stats.LastTaskTime,
			"total_run_time":   worker.stats.TotalRunTime,
			"start_time":       worker.startTime,
			"last_heartbeat":   worker.lastHeartbeat,
			"uptime":           time.Since(worker.startTime),
			"channel_usage":    len(worker.tasksCh),
			"channel_capacity": cap(worker.tasksCh),
			"last_error":       worker.stats.LastError,
			"last_error_time":  worker.stats.LastErrorTime,
		}

		// Calcular métricas derivadas
		if worker.stats.TasksProcessed > 0 {
			workerStats["average_task_time"] = worker.stats.TotalRunTime / time.Duration(worker.stats.TasksProcessed)
			workerStats["success_rate"] = float64(worker.stats.TasksSucceeded) / float64(worker.stats.TasksProcessed)
			workerStats["error_rate"] = float64(worker.stats.TasksFailed) / float64(worker.stats.TasksProcessed)

			uptime := time.Since(worker.startTime)
			if uptime.Seconds() > 0 {
				workerStats["tasks_per_second"] = float64(worker.stats.TasksProcessed) / uptime.Seconds()
			}
		}

		worker.mutex.RUnlock()
		stats = append(stats, workerStats)
	}

	return stats
}

func (p *WorkerPool) calculateThroughput() float64 {
	uptime := time.Since(p.metrics.LastResetTime)
	if uptime.Seconds() == 0 {
		return 0
	}
	return float64(p.metrics.TotalTasksProcessed) / uptime.Seconds()
}

// UTILITY FUNCTIONS

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
