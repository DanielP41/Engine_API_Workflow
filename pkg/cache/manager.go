// pkg/cache/manager.go
package cache

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CacheManager gestiona múltiples instancias de caché y operaciones avanzadas
type CacheManager struct {
	primary   CacheService
	secondary CacheService // Caché de respaldo (opcional)
	config    *CacheConfig
	logger    *zap.Logger

	// Invalidation system
	invalidationChan chan InvalidationEvent
	invalidationWG   sync.WaitGroup

	// Warmup system
	warmupTasks map[string]WarmupTask
	warmupMutex sync.RWMutex

	// Metrics
	metrics *CacheMetrics

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// InvalidationEvent evento de invalidación de caché
type InvalidationEvent struct {
	Type   string    `json:"type"`   // "key", "pattern", "tag"
	Target string    `json:"target"` // clave, patrón o tag
	Reason string    `json:"reason"` // motivo de la invalidación
	Source string    `json:"source"` // origen del evento
	Time   time.Time `json:"time"`
}

// WarmupTask tarea de precalentamiento del caché
type WarmupTask struct {
	Name     string                                         `json:"name"`
	Key      string                                         `json:"key"`
	TTL      time.Duration                                  `json:"ttl"`
	Fetcher  func(ctx context.Context) (interface{}, error) `json:"-"`
	Schedule time.Duration                                  `json:"schedule"`
	Priority int                                            `json:"priority"`
}

// CacheMetrics métricas del sistema de caché
type CacheMetrics struct {
	Hits           int64     `json:"hits"`
	Misses         int64     `json:"misses"`
	Sets           int64     `json:"sets"`
	Deletes        int64     `json:"deletes"`
	Invalidations  int64     `json:"invalidations"`
	WarmupTasks    int64     `json:"warmup_tasks"`
	WarmupFailures int64     `json:"warmup_failures"`
	LastResetTime  time.Time `json:"last_reset_time"`
	mu             sync.RWMutex
}

// NewCacheManager crea un nuevo manager de caché
func NewCacheManager(redisClient *redis.Client, config *CacheConfig, logger *zap.Logger) *CacheManager {
	ctx, cancel := context.WithCancel(context.Background())

	primary := NewRedisCacheService(redisClient, config, logger)

	manager := &CacheManager{
		primary:          primary,
		config:           config,
		logger:           logger,
		invalidationChan: make(chan InvalidationEvent, 1000),
		warmupTasks:      make(map[string]WarmupTask),
		metrics:          &CacheMetrics{LastResetTime: time.Now()},
		ctx:              ctx,
		cancel:           cancel,
	}

	// Iniciar workers de invalidación y warmup
	manager.startInvalidationWorker()
	manager.startWarmupWorker()

	return manager
}

// Get obtiene un valor del caché con fallback
func (cm *CacheManager) Get(ctx context.Context, key string) (interface{}, error) {
	// Intentar caché primario
	value, err := cm.primary.Get(ctx, key)
	if err == nil {
		cm.recordHit()
		return value, nil
	}

	// Intentar caché secundario si existe
	if cm.secondary != nil {
		value, err = cm.secondary.Get(ctx, key)
		if err == nil {
			cm.recordHit()
			// Repoblar caché primario asincrónicamente
			go func() {
				_ = cm.primary.Set(context.Background(), key, value, cm.config.DefaultTTL)
			}()
			return value, nil
		}
	}

	cm.recordMiss()
	return nil, err
}

// Set almacena un valor en el caché con replicación
func (cm *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	cm.recordSet()

	// Almacenar en caché primario
	err := cm.primary.Set(ctx, key, value, ttl)
	if err != nil {
		cm.logger.Error("Failed to set cache primary", zap.String("key", key), zap.Error(err))
	}

	// Almacenar en caché secundario si existe
	if cm.secondary != nil {
		go func() {
			if secErr := cm.secondary.Set(context.Background(), key, value, ttl); secErr != nil {
				cm.logger.Warn("Failed to set cache secondary", zap.String("key", key), zap.Error(secErr))
			}
		}()
	}

	return err
}

// Delete elimina una clave del caché
func (cm *CacheManager) Delete(ctx context.Context, key string) error {
	cm.recordDelete()

	// Eliminar de caché primario
	err := cm.primary.Delete(ctx, key)

	// Eliminar de caché secundario si existe
	if cm.secondary != nil {
		go func() {
			_ = cm.secondary.Delete(context.Background(), key)
		}()
	}

	return err
}

// GetOrSet obtiene un valor del caché o lo calcula y almacena
func (cm *CacheManager) GetOrSet(ctx context.Context, key string, ttl time.Duration, fetchFunc func() (interface{}, error)) (interface{}, error) {
	// Intentar obtener del caché
	value, err := cm.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	// Si no está en caché, calcularlo
	value, err = fetchFunc()
	if err != nil {
		return nil, err
	}

	// Almacenar en caché
	if setErr := cm.Set(ctx, key, value, ttl); setErr != nil {
		cm.logger.Warn("Failed to cache computed value", zap.String("key", key), zap.Error(setErr))
	}

	return value, nil
}

// InvalidatePattern invalida todas las claves que coinciden con un patrón
func (cm *CacheManager) InvalidatePattern(ctx context.Context, pattern string, reason string) error {
	cm.recordInvalidation()

	// Enviar evento de invalidación
	event := InvalidationEvent{
		Type:   "pattern",
		Target: pattern,
		Reason: reason,
		Source: "cache_manager",
		Time:   time.Now(),
	}

	select {
	case cm.invalidationChan <- event:
	default:
		cm.logger.Warn("Invalidation channel full, dropping event", zap.String("pattern", pattern))
	}

	return cm.primary.DeletePattern(ctx, pattern)
}

// InvalidateKey invalida una clave específica
func (cm *CacheManager) InvalidateKey(ctx context.Context, key string, reason string) error {
	cm.recordInvalidation()

	event := InvalidationEvent{
		Type:   "key",
		Target: key,
		Reason: reason,
		Source: "cache_manager",
		Time:   time.Now(),
	}

	select {
	case cm.invalidationChan <- event:
	default:
		cm.logger.Warn("Invalidation channel full, dropping event", zap.String("key", key))
	}

	return cm.Delete(ctx, key)
}

// AddWarmupTask añade una tarea de precalentamiento
func (cm *CacheManager) AddWarmupTask(task WarmupTask) {
	cm.warmupMutex.Lock()
	defer cm.warmupMutex.Unlock()

	cm.warmupTasks[task.Name] = task
	cm.logger.Info("Added warmup task", zap.String("name", task.Name), zap.String("key", task.Key))
}

// RemoveWarmupTask elimina una tarea de precalentamiento
func (cm *CacheManager) RemoveWarmupTask(name string) {
	cm.warmupMutex.Lock()
	defer cm.warmupMutex.Unlock()

	delete(cm.warmupTasks, name)
	cm.logger.Info("Removed warmup task", zap.String("name", name))
}

// ExecuteWarmup ejecuta todas las tareas de precalentamiento
func (cm *CacheManager) ExecuteWarmup(ctx context.Context) error {
	cm.warmupMutex.RLock()
	tasks := make([]WarmupTask, 0, len(cm.warmupTasks))
	for _, task := range cm.warmupTasks {
		tasks = append(tasks, task)
	}
	cm.warmupMutex.RUnlock()

	if len(tasks) == 0 {
		return nil
	}

	cm.logger.Info("Executing cache warmup", zap.Int("tasks", len(tasks)))

	// Ejecutar tareas con concurrencia limitada
	semaphore := make(chan struct{}, 5) // Max 5 concurrent warmup tasks
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t WarmupTask) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()

				if err := cm.executeWarmupTask(ctx, t); err != nil {
					cm.recordWarmupFailure()
					cm.logger.Error("Warmup task failed",
						zap.String("task", t.Name),
						zap.String("key", t.Key),
						zap.Error(err))
				} else {
					cm.recordWarmupTask()
				}
			case <-ctx.Done():
				return
			}
		}(task)
	}

	wg.Wait()
	cm.logger.Info("Cache warmup completed")
	return nil
}

// GetStats obtiene estadísticas del manager
func (cm *CacheManager) GetStats(ctx context.Context) (*CacheStats, error) {
	primaryStats, err := cm.primary.Stats(ctx)
	if err != nil {
		return nil, err
	}

	cm.metrics.mu.RLock()
	managerHits := cm.metrics.Hits
	managerMisses := cm.metrics.Misses
	cm.metrics.mu.RUnlock()

	// Combinar estadísticas
	primaryStats.HitCount += managerHits
	primaryStats.MissCount += managerMisses

	total := primaryStats.HitCount + primaryStats.MissCount
	if total > 0 {
		primaryStats.HitRate = float64(primaryStats.HitCount) / float64(total)
	}

	return primaryStats, nil
}

// GetMetrics obtiene métricas del manager
func (cm *CacheManager) GetMetrics() *CacheMetrics {
	cm.metrics.mu.RLock()
	defer cm.metrics.mu.RUnlock()

	return &CacheMetrics{
		Hits:           cm.metrics.Hits,
		Misses:         cm.metrics.Misses,
		Sets:           cm.metrics.Sets,
		Deletes:        cm.metrics.Deletes,
		Invalidations:  cm.metrics.Invalidations,
		WarmupTasks:    cm.metrics.WarmupTasks,
		WarmupFailures: cm.metrics.WarmupFailures,
		LastResetTime:  cm.metrics.LastResetTime,
	}
}

// Close cierra el manager y todos los recursos
func (cm *CacheManager) Close() error {
	cm.cancel()
	cm.wg.Wait()

	close(cm.invalidationChan)

	if err := cm.primary.Close(); err != nil {
		return err
	}

	if cm.secondary != nil {
		return cm.secondary.Close()
	}

	return nil
}

// Métodos privados

func (cm *CacheManager) startInvalidationWorker() {
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()

		for {
			select {
			case event := <-cm.invalidationChan:
				cm.processInvalidationEvent(event)
			case <-cm.ctx.Done():
				return
			}
		}
	}()
}

func (cm *CacheManager) startWarmupWorker() {
	if cm.config.CleanupInterval <= 0 {
		return
	}

	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()

		ticker := time.NewTicker(cm.config.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := cm.ExecuteWarmup(cm.ctx); err != nil {
					cm.logger.Error("Scheduled warmup failed", zap.Error(err))
				}
			case <-cm.ctx.Done():
				return
			}
		}
	}()
}

func (cm *CacheManager) processInvalidationEvent(event InvalidationEvent) {
	cm.logger.Debug("Processing invalidation event",
		zap.String("type", event.Type),
		zap.String("target", event.Target),
		zap.String("reason", event.Reason))

	// Aquí se puede agregar lógica adicional como:
	// - Logging detallado
	// - Notificaciones
	// - Métricas de invalidación
	// - Replicación a otros nodos
}

func (cm *CacheManager) executeWarmupTask(ctx context.Context, task WarmupTask) error {
	// Verificar si la clave ya existe
	if cm.primary.Exists(ctx, task.Key) {
		return nil // Ya está en caché
	}

	// Ejecutar fetcher
	value, err := task.Fetcher(ctx)
	if err != nil {
		return err
	}

	// Almacenar en caché
	return cm.Set(ctx, task.Key, value, task.TTL)
}

// Métodos de métricas
func (cm *CacheManager) recordHit() {
	cm.metrics.mu.Lock()
	cm.metrics.Hits++
	cm.metrics.mu.Unlock()
}

func (cm *CacheManager) recordMiss() {
	cm.metrics.mu.Lock()
	cm.metrics.Misses++
	cm.metrics.mu.Unlock()
}

func (cm *CacheManager) recordSet() {
	cm.metrics.mu.Lock()
	cm.metrics.Sets++
	cm.metrics.mu.Unlock()
}

func (cm *CacheManager) recordDelete() {
	cm.metrics.mu.Lock()
	cm.metrics.Deletes++
	cm.metrics.mu.Unlock()
}

func (cm *CacheManager) recordInvalidation() {
	cm.metrics.mu.Lock()
	cm.metrics.Invalidations++
	cm.metrics.mu.Unlock()
}

func (cm *CacheManager) recordWarmupTask() {
	cm.metrics.mu.Lock()
	cm.metrics.WarmupTasks++
	cm.metrics.mu.Unlock()
}

func (cm *CacheManager) recordWarmupFailure() {
	cm.metrics.mu.Lock()
	cm.metrics.WarmupFailures++
	cm.metrics.mu.Unlock()
}
