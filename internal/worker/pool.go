package worker

import (
	"context"
	"fmt"
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
	stats    PoolWorkerStats // CORREGIDO: usar PoolWorkerStats
	logger   *zap.Logger

	// Funcionalidades avanzadas
	startTime     time.Time
	lastHeartbeat time.Time
	isHealthy     bool
	taskTimeout   time.Duration
}

// PoolWorkerStats estadísticas de un worker del pool - CORREGIDO: renombrado para evitar conflicto
type PoolWorkerStats struct {
	TasksProcessed  int64 // CORREGIDO: campo requerido
	TasksSucceeded  int64 // CORREGIDO: campo requerido
	TasksFailed     int64 // CORREGIDO: campo requerido
	LastTaskTime    time.Time
	TotalRunTime    time.Duration // CORREGIDO: campo requerido
	IsIdle          bool          // CORREGIDO: campo requerido
	AverageTaskTime time.Duration
	TasksPerSecond  float64
	ErrorRate       float64 // CORREGIDO: campo requerido
	LastErrorTime   time.Time
	LastError       string
}

// PoolConfig configuración específica del pool (separada de WorkerConfig)
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
		stats:         PoolWorkerStats{IsIdle: true}, // CORREGIDO: usar PoolWorkerStats
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
	avgLoad := p.calculateAverageLoad()
	queueLength := p.getQueueLength()

	// Detectar modo de emergencia
	if queueLength > p.config.EmergencyThreshold {
		p.activateEmergencyMode("high_queue_length")
	}

	// Lógica de escalamiento
	if avgLoad > p.config.ScalingThreshold && currentWorkers < p.maxWorkers {
		// Scale up
		newWorkerID := p.getNextWorkerID()
		if err := p.createWorker(newWorkerID); err == nil {
			p.lastScaleTime = time.Now()
			p.logger.Info("Scaled up workers",
				zap.Float64("avg_load", avgLoad),
				zap.Int64("queue_length", queueLength),
				zap.Int("new_workers", currentWorkers+1))
		}
	} else if avgLoad < p.config.DownscaleThreshold && currentWorkers > p.minWorkers {
		// Scale down
		oldestWorkerID := p.getOldestWorkerID()
		if oldestWorkerID >= 0 {
			p.removeWorker(oldestWorkerID)
			p.lastScaleTime = time.Now()
			p.logger.Info("Scaled down workers",
				zap.Float64("avg_load", avgLoad),
				zap.Int64("queue_length", queueLength),
				zap.Int("new_workers", currentWorkers-1))
		}
	}

	// Desactivar modo de emergencia si es necesario
	if queueLength < p.config.EmergencyThreshold/2 && p.emergencyMode {
		p.deactivateEmergencyMode()
	}
}

// healthMonitor monitorea la salud de los workers
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

// performHealthCheck realiza chequeos de salud en todos los workers
func (p *WorkerPool) performHealthCheck() {
	p.healthChecker.mutex.Lock()
	defer p.healthChecker.mutex.Unlock()

	p.healthChecker.lastCheck = time.Now()
	p.healthChecker.healthStatus.LastCheckTime = time.Now()
	p.healthChecker.healthStatus.Issues = make([]string, 0)

	unhealthyCount := 0

	p.workersMutex.RLock()
	for id, worker := range p.workers {
		worker.mutex.RLock()

		// Verificar heartbeat
		if time.Since(worker.lastHeartbeat) > p.config.MaxIdleTime {
			unhealthyCount++
			p.healthChecker.healthStatus.Issues = append(
				p.healthChecker.healthStatus.Issues,
				fmt.Sprintf("Worker %d: heartbeat timeout", id),
			)
			worker.mutex.RUnlock()
			continue
		}

		// Verificar errores recientes
		if worker.stats.ErrorRate > 0.5 { // 50% error rate
			unhealthyCount++
			p.healthChecker.healthStatus.Issues = append(
				p.healthChecker.healthStatus.Issues,
				fmt.Sprintf("Worker %d: high error rate (%.2f%%)", id, worker.stats.ErrorRate*100),
			)
		}

		worker.mutex.RUnlock()
	}
	p.workersMutex.RUnlock()

	p.healthChecker.healthStatus.UnhealthyCount = unhealthyCount
	p.healthChecker.healthStatus.IsHealthy = unhealthyCount == 0

	if !p.healthChecker.healthStatus.IsHealthy {
		p.logger.Warn("Health check detected issues",
			zap.Int("unhealthy_workers", unhealthyCount),
			zap.Strings("issues", p.healthChecker.healthStatus.Issues))
	}
}

// enhancedTaskDispatcher distribuye tareas de manera inteligente
func (p *WorkerPool) enhancedTaskDispatcher(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		default:
			// Obtener tarea de la cola
			task, err := p.engine.queueRepo.Dequeue(ctx)
			if err != nil {
				// No hay tareas o error, esperar un poco
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Distribuir tarea al worker menos cargado
			if !p.dispatchToOptimalWorker(task) {
				// Si no se puede despachar, reintentarlo después
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

// dispatchToOptimalWorker encuentra el worker óptimo para la tarea
func (p *WorkerPool) dispatchToOptimalWorker(task *models.QueueTask) bool {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	var bestWorker *Worker
	lowestLoad := int(^uint(0) >> 1) // MaxInt

	// Encontrar worker con menor carga
	for _, worker := range p.workers {
		worker.mutex.RLock()
		load := len(worker.tasksCh)
		isActive := worker.isActive
		isHealthy := worker.isHealthy
		worker.mutex.RUnlock()

		if isActive && isHealthy && load < lowestLoad {
			bestWorker = worker
			lowestLoad = load
		}
	}

	if bestWorker == nil {
		return false
	}

	// Intentar enviar tarea sin bloquear
	select {
	case bestWorker.tasksCh <- task:
		atomic.AddInt64(&p.engine.currentLoad, 1)
		return true
	default:
		return false
	}
}

// metricsCollector recolecta métricas del pool
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
			p.updateMetrics()
		}
	}
}

// updateMetrics actualiza las métricas del pool
func (p *WorkerPool) updateMetrics() {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	var totalProcessed, totalSucceeded, totalFailed int64
	var totalRunTime time.Duration

	for _, worker := range p.workers {
		worker.mutex.RLock()
		totalProcessed += worker.stats.TasksProcessed
		totalSucceeded += worker.stats.TasksSucceeded
		totalFailed += worker.stats.TasksFailed
		totalRunTime += worker.stats.TotalRunTime
		worker.mutex.RUnlock()
	}

	p.metrics.TotalTasksProcessed = totalProcessed
	p.metrics.TotalTasksSucceeded = totalSucceeded
	p.metrics.TotalTasksFailed = totalFailed
	p.metrics.UpTime = time.Since(p.metrics.LastResetTime)

	// Calcular métricas derivadas
	if totalProcessed > 0 {
		p.metrics.AverageProcessTime = totalRunTime / time.Duration(totalProcessed)
		p.metrics.ThroughputPerSecond = float64(totalProcessed) / p.metrics.UpTime.Seconds()
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
		case <-time.After(30 * time.Minute): // Usar timeout fijo
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
			zap.String("task_id", task.ID))
	}
	w.mutex.Unlock()
}

// heartbeatLoop mantiene el heartbeat del worker
func (w *Worker) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second) // Usar valor fijo en lugar de config
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

// Stop detiene el worker
func (w *Worker) Stop() {
	w.mutex.Lock()
	w.isActive = false
	w.mutex.Unlock()

	close(w.stopCh)
}

// UTILITY METHODS

func (p *WorkerPool) calculateAverageLoad() float64 {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	if len(p.workers) == 0 {
		return 0
	}

	totalLoad := 0
	for _, worker := range p.workers {
		totalLoad += len(worker.tasksCh)
	}

	return float64(totalLoad) / float64(len(p.workers))
}

func (p *WorkerPool) getQueueLength() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	length, err := p.engine.queueRepo.Length(ctx, "workflow_queue")
	if err != nil {
		return 0
	}
	return length
}

func (p *WorkerPool) getNextWorkerID() int {
	maxID := -1
	for id := range p.workers {
		if id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}

func (p *WorkerPool) getOldestWorkerID() int {
	var oldestID = -1
	var oldestTime time.Time

	for id, worker := range p.workers {
		if oldestID == -1 || worker.startTime.Before(oldestTime) {
			oldestID = id
			oldestTime = worker.startTime
		}
	}

	return oldestID
}

func (p *WorkerPool) activateEmergencyMode(reason string) {
	if !p.emergencyMode {
		p.emergencyMode = true
		p.emergencyReason = reason
		p.logger.Warn("Emergency mode activated", zap.String("reason", reason))
	}
}

func (p *WorkerPool) deactivateEmergencyMode() {
	if p.emergencyMode {
		p.emergencyMode = false
		p.emergencyReason = ""
		p.logger.Info("Emergency mode deactivated")
	}
}

func (p *WorkerPool) recordScalingEvent(action string, from, to int, reason string) {
	event := ScalingEvent{
		Timestamp:   time.Now(),
		Action:      action,
		FromWorkers: from,
		ToWorkers:   to,
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

func (p *WorkerPool) calculateErrorRate() float64 {
	if p.metrics.TotalTasksProcessed == 0 {
		return 0
	}
	return float64(p.metrics.TotalTasksFailed) / float64(p.metrics.TotalTasksProcessed)
}

// PUBLIC API METHODS

// GetWorkerCount retorna el número de workers
func (p *WorkerPool) GetWorkerCount() int {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()
	return len(p.workers)
}

// AddTask agrega una tarea al pool
func (p *WorkerPool) AddTask(task *models.QueueTask) bool {
	return p.dispatchToOptimalWorker(task)
}

// GetStats retorna estadísticas del pool
func (p *WorkerPool) GetStats() map[string]interface{} {
	p.workersMutex.RLock()
	defer p.workersMutex.RUnlock()

	stats := map[string]interface{}{
		"total_workers":  len(p.workers),
		"min_workers":    p.minWorkers,
		"max_workers":    p.maxWorkers,
		"current_load":   atomic.LoadInt64(&p.engine.currentLoad),
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

// GetHealthStatus retorna el estado de salud del pool
func (p *WorkerPool) GetHealthStatus() HealthStatus {
	p.healthChecker.mutex.RLock()
	defer p.healthChecker.mutex.RUnlock()
	return p.healthChecker.healthStatus
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
