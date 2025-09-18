// pkg/cache/redis_cache.go
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// redisCacheService implementa CacheService usando Redis
type redisCacheService struct {
	client *redis.Client
	config *CacheConfig
	logger *zap.Logger
	stats  *cacheStats
	mu     sync.RWMutex
}

// cacheStats mantiene estadísticas en memoria
type cacheStats struct {
	hitCount  int64
	missCount int64
	startTime time.Time
	mu        sync.RWMutex
}

// NewRedisCacheService crea una nueva instancia del servicio de caché Redis
func NewRedisCacheService(client *redis.Client, config *CacheConfig, logger *zap.Logger) CacheService {
	service := &redisCacheService{
		client: client,
		config: config,
		logger: logger,
		stats: &cacheStats{
			startTime: time.Now(),
		},
	}

	// Inicializar estadísticas desde Redis si existen
	service.initializeStats()

	return service
}

// Get obtiene un valor del caché
func (r *redisCacheService) Get(ctx context.Context, key string) (interface{}, error) {
	if !r.config.Enabled {
		return nil, ErrCacheDisabled
	}

	result := r.client.Get(ctx, key)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			r.recordMiss()
			return nil, &CacheError{Type: "KEY_NOT_FOUND", Message: "key not found", Key: key}
		}
		r.logger.Error("Cache get error", zap.String("key", key), zap.Error(result.Err()))
		return nil, fmt.Errorf("cache get error: %w", result.Err())
	}

	// Deserializar el valor
	var value interface{}
	if err := json.Unmarshal([]byte(result.Val()), &value); err != nil {
		r.logger.Error("Cache unmarshal error", zap.String("key", key), zap.Error(err))
		return nil, fmt.Errorf("cache unmarshal error: %w", err)
	}

	r.recordHit()
	r.logger.Debug("Cache hit", zap.String("key", key))
	return value, nil
}

// Set almacena un valor en el caché
func (r *redisCacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !r.config.Enabled {
		return ErrCacheDisabled
	}

	// Serializar el valor
	data, err := json.Marshal(value)
	if err != nil {
		r.logger.Error("Cache marshal error", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("cache marshal error: %w", err)
	}

	// Usar TTL por defecto si no se especifica
	if ttl <= 0 {
		ttl = r.config.DefaultTTL
	}

	result := r.client.Set(ctx, key, data, ttl)
	if result.Err() != nil {
		r.logger.Error("Cache set error", zap.String("key", key), zap.Error(result.Err()))
		return fmt.Errorf("cache set error: %w", result.Err())
	}

	r.logger.Debug("Cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

// Delete elimina una clave del caché
func (r *redisCacheService) Delete(ctx context.Context, key string) error {
	if !r.config.Enabled {
		return ErrCacheDisabled
	}

	result := r.client.Del(ctx, key)
	if result.Err() != nil {
		r.logger.Error("Cache delete error", zap.String("key", key), zap.Error(result.Err()))
		return fmt.Errorf("cache delete error: %w", result.Err())
	}

	r.logger.Debug("Cache delete", zap.String("key", key))
	return nil
}

// Exists verifica si una clave existe en el caché
func (r *redisCacheService) Exists(ctx context.Context, key string) bool {
	if !r.config.Enabled {
		return false
	}

	result := r.client.Exists(ctx, key)
	exists := result.Val() > 0

	r.logger.Debug("Cache exists check", zap.String("key", key), zap.Bool("exists", exists))
	return exists
}

// GetMany obtiene múltiples valores del caché
func (r *redisCacheService) GetMany(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if !r.config.Enabled {
		return nil, ErrCacheDisabled
	}

	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	result := r.client.MGet(ctx, keys...)
	if result.Err() != nil {
		r.logger.Error("Cache mget error", zap.Strings("keys", keys), zap.Error(result.Err()))
		return nil, fmt.Errorf("cache mget error: %w", result.Err())
	}

	values := make(map[string]interface{})
	for i, val := range result.Val() {
		if val != nil {
			var value interface{}
			if err := json.Unmarshal([]byte(val.(string)), &value); err != nil {
				r.logger.Warn("Cache unmarshal error for key", zap.String("key", keys[i]), zap.Error(err))
				continue
			}
			values[keys[i]] = value
			r.recordHit()
		} else {
			r.recordMiss()
		}
	}

	r.logger.Debug("Cache mget", zap.Int("requested", len(keys)), zap.Int("found", len(values)))
	return values, nil
}

// SetMany almacena múltiples valores en el caché
func (r *redisCacheService) SetMany(ctx context.Context, items map[string]CacheItem) error {
	if !r.config.Enabled {
		return ErrCacheDisabled
	}

	if len(items) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()

	for key, item := range items {
		data, err := json.Marshal(item.Value)
		if err != nil {
			r.logger.Error("Cache marshal error for key", zap.String("key", key), zap.Error(err))
			continue
		}

		ttl := item.TTL
		if ttl <= 0 {
			ttl = r.config.DefaultTTL
		}

		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		r.logger.Error("Cache pipeline error", zap.Error(err))
		return fmt.Errorf("cache pipeline error: %w", err)
	}

	r.logger.Debug("Cache mset", zap.Int("count", len(items)))
	return nil
}

// DeleteMany elimina múltiples claves del caché
func (r *redisCacheService) DeleteMany(ctx context.Context, keys []string) error {
	if !r.config.Enabled {
		return ErrCacheDisabled
	}

	if len(keys) == 0 {
		return nil
	}

	result := r.client.Del(ctx, keys...)
	if result.Err() != nil {
		r.logger.Error("Cache del error", zap.Strings("keys", keys), zap.Error(result.Err()))
		return fmt.Errorf("cache del error: %w", result.Err())
	}

	r.logger.Debug("Cache delete many", zap.Int("count", len(keys)), zap.Int64("deleted", result.Val()))
	return nil
}

// DeletePattern elimina todas las claves que coinciden con un patrón
func (r *redisCacheService) DeletePattern(ctx context.Context, pattern string) error {
	if !r.config.Enabled {
		return ErrCacheDisabled
	}

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		r.logger.Error("Cache keys error", zap.String("pattern", pattern), zap.Error(err))
		return fmt.Errorf("cache keys error: %w", err)
	}

	if len(keys) == 0 {
		r.logger.Debug("No keys found for pattern", zap.String("pattern", pattern))
		return nil
	}

	result := r.client.Del(ctx, keys...)
	if result.Err() != nil {
		r.logger.Error("Cache del pattern error", zap.String("pattern", pattern), zap.Error(result.Err()))
		return fmt.Errorf("cache del pattern error: %w", result.Err())
	}

	r.logger.Debug("Cache delete pattern", zap.String("pattern", pattern), zap.Int("keys", len(keys)))
	return nil
}

// GetKeys obtiene todas las claves que coinciden con un patrón
func (r *redisCacheService) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	if !r.config.Enabled {
		return nil, ErrCacheDisabled
	}

	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		r.logger.Error("Cache keys error", zap.String("pattern", pattern), zap.Error(err))
		return nil, fmt.Errorf("cache keys error: %w", err)
	}

	return keys, nil
}

// GetTTL obtiene el tiempo de vida restante de una clave
func (r *redisCacheService) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	if !r.config.Enabled {
		return 0, ErrCacheDisabled
	}

	result := r.client.TTL(ctx, key)
	if result.Err() != nil {
		return 0, fmt.Errorf("cache ttl error: %w", result.Err())
	}

	return result.Val(), nil
}

// UpdateTTL actualiza el tiempo de vida de una clave
func (r *redisCacheService) UpdateTTL(ctx context.Context, key string, ttl time.Duration) error {
	if !r.config.Enabled {
		return ErrCacheDisabled
	}

	result := r.client.Expire(ctx, key, ttl)
	if result.Err() != nil {
		return fmt.Errorf("cache expire error: %w", result.Err())
	}

	return nil
}

// Increment incrementa un valor numérico en el caché
func (r *redisCacheService) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if !r.config.Enabled {
		return 0, ErrCacheDisabled
	}

	result := r.client.IncrBy(ctx, key, delta)
	if result.Err() != nil {
		return 0, fmt.Errorf("cache incr error: %w", result.Err())
	}

	return result.Val(), nil
}

// Decrement decrementa un valor numérico en el caché
func (r *redisCacheService) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	if !r.config.Enabled {
		return 0, ErrCacheDisabled
	}

	result := r.client.DecrBy(ctx, key, delta)
	if result.Err() != nil {
		return 0, fmt.Errorf("cache decr error: %w", result.Err())
	}

	return result.Val(), nil
}

// GetOrSet obtiene un valor del caché o lo calcula y almacena si no existe
func (r *redisCacheService) GetOrSet(ctx context.Context, key string, ttl time.Duration, fetchFunc func() (interface{}, error)) (interface{}, error) {
	if !r.config.Enabled {
		return fetchFunc()
	}

	// Intentar obtener del caché primero
	value, err := r.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	// Si no está en caché, calcularlo
	value, err = fetchFunc()
	if err != nil {
		return nil, err
	}

	// Almacenar en caché (no fallar si hay error de caché)
	if setErr := r.Set(ctx, key, value, ttl); setErr != nil {
		r.logger.Warn("Failed to cache value", zap.String("key", key), zap.Error(setErr))
	}

	return value, nil
}

// Ping verifica la conectividad con Redis
func (r *redisCacheService) Ping(ctx context.Context) error {
	result := r.client.Ping(ctx)
	if result.Err() != nil {
		return fmt.Errorf("cache ping error: %w", result.Err())
	}
	return nil
}

// Stats obtiene estadísticas del caché
func (r *redisCacheService) Stats(ctx context.Context) (*CacheStats, error) {
	r.stats.mu.RLock()
	hitCount := r.stats.hitCount
	missCount := r.stats.missCount
	startTime := r.stats.startTime
	r.stats.mu.RUnlock()

	totalRequests := hitCount + missCount
	var hitRate float64
	if totalRequests > 0 {
		hitRate = float64(hitCount) / float64(totalRequests)
	}

	// Obtener información de Redis
	info := r.client.Info(ctx, "memory", "stats")
	var usedMemory int64
	var totalKeys int64

	if info.Err() == nil {
		lines := strings.Split(info.Val(), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "used_memory:") {
				if val, err := strconv.ParseInt(strings.TrimPrefix(line, "used_memory:"), 10, 64); err == nil {
					usedMemory = val
				}
			}
		}
	}

	// Contar claves totales
	dbSize := r.client.DBSize(ctx)
	if dbSize.Err() == nil {
		totalKeys = dbSize.Val()
	}

	return &CacheStats{
		HitCount:    hitCount,
		MissCount:   missCount,
		HitRate:     hitRate,
		TotalKeys:   totalKeys,
		UsedMemory:  usedMemory,
		Uptime:      int64(time.Since(startTime).Seconds()),
		LastUpdated: time.Now(),
	}, nil
}

// Clear limpia todo el caché
func (r *redisCacheService) Clear(ctx context.Context) error {
	if !r.config.Enabled {
		return ErrCacheDisabled
	}

	result := r.client.FlushDB(ctx)
	if result.Err() != nil {
		return fmt.Errorf("cache clear error: %w", result.Err())
	}

	r.logger.Info("Cache cleared")
	return nil
}

// Close cierra la conexión
func (r *redisCacheService) Close() error {
	return r.client.Close()
}

// Métodos privados para estadísticas
func (r *redisCacheService) recordHit() {
	r.stats.mu.Lock()
	r.stats.hitCount++
	r.stats.mu.Unlock()
}

func (r *redisCacheService) recordMiss() {
	r.stats.mu.Lock()
	r.stats.missCount++
	r.stats.mu.Unlock()
}

func (r *redisCacheService) initializeStats() {
	// Intentar cargar estadísticas desde Redis
	ctx := context.Background()

	if hitStr, err := r.client.Get(ctx, "cache:stats:hits").Result(); err == nil {
		if hits, err := strconv.ParseInt(hitStr, 10, 64); err == nil {
			r.stats.hitCount = hits
		}
	}

	if missStr, err := r.client.Get(ctx, "cache:stats:misses").Result(); err == nil {
		if misses, err := strconv.ParseInt(missStr, 10, 64); err == nil {
			r.stats.missCount = misses
		}
	}
}
