// internal/config/cache.go
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TTLConfig configuración de TTL por dominio
type TTLConfig struct {
	Dashboard time.Duration
	Workflow  time.Duration
	User      time.Duration
	Metrics   time.Duration
	Queue     time.Duration
	System    time.Duration
	Auth      time.Duration
}

// CacheStrategy define estrategias de caché
type CacheStrategy string

const (
	StrategyWriteThrough CacheStrategy = "write_through"
	StrategyWriteBack    CacheStrategy = "write_back"
	StrategyWriteAround  CacheStrategy = "write_around"
	StrategyRefreshAhead CacheStrategy = "refresh_ahead"
)

// CachePolicy política de caché por tipo de datos
type CachePolicy struct {
	Strategy    CacheStrategy `json:"strategy"`
	TTL         time.Duration `json:"ttl"`
	MaxSize     int64         `json:"max_size"`
	Compression bool          `json:"compression"`
	Persistence bool          `json:"persistence"`
}

// CacheWarmupConfig configuración de warmup del caché
type CacheWarmupConfig struct {
	Enabled     bool
	Timeout     time.Duration
	Concurrency int
}

// CacheInvalidationConfig configuración de invalidación del caché
type CacheInvalidationConfig struct {
	Enabled bool
	Buffer  int
	Timeout time.Duration
}

// SimpleCacheConfig configuración simplificada del caché (sin dependencias externas)
type SimpleCacheConfig struct {
	Enabled          bool          `json:"enabled"`
	DefaultTTL       time.Duration `json:"default_ttl"`
	CleanupInterval  time.Duration `json:"cleanup_interval"`
	MaxMemory        int64         `json:"max_memory"`
	CompressionLevel int           `json:"compression_level"`
	Serializer       string        `json:"serializer"`
}

// GetCacheConfig obtiene la configuración de caché del config principal
func (c *Config) GetCacheConfig() *SimpleCacheConfig {
	maxMemory := c.parseMemorySize(c.Cache.MaxMemory)

	return &SimpleCacheConfig{
		Enabled:          c.Cache.Enabled,
		DefaultTTL:       c.Cache.DefaultTTL,
		CleanupInterval:  c.Cache.CleanupInterval,
		MaxMemory:        maxMemory,
		CompressionLevel: c.Cache.CompressionLevel,
		Serializer:       c.Cache.Serializer,
	}
}

// GetTTLConfig obtiene configuración de TTL por dominio
func (c *Config) GetTTLConfig() *TTLConfig {
	return &TTLConfig{
		Dashboard: c.Cache.DashboardTTL,
		Workflow:  c.Cache.WorkflowTTL,
		User:      c.Cache.UserTTL,
		Metrics:   c.Cache.MetricsTTL,
		Queue:     c.Cache.QueueTTL,
		System:    c.Cache.SystemTTL,
		Auth:      c.Cache.AuthTTL,
	}
}

// GetCachePolicies retorna políticas de caché predefinidas
func (c *Config) GetCachePolicies() map[string]*CachePolicy {
	return map[string]*CachePolicy{
		"dashboard": {
			Strategy:    StrategyWriteThrough,
			TTL:         c.Cache.DashboardTTL,
			MaxSize:     1000,
			Compression: false,
			Persistence: true,
		},
		"workflow": {
			Strategy:    StrategyWriteBack,
			TTL:         c.Cache.WorkflowTTL,
			MaxSize:     5000,
			Compression: true,
			Persistence: true,
		},
		"user": {
			Strategy:    StrategyWriteThrough,
			TTL:         c.Cache.UserTTL,
			MaxSize:     10000,
			Compression: false,
			Persistence: true,
		},
		"metrics": {
			Strategy:    StrategyWriteAround,
			TTL:         c.Cache.MetricsTTL,
			MaxSize:     2000,
			Compression: true,
			Persistence: false,
		},
		"queue": {
			Strategy:    StrategyRefreshAhead,
			TTL:         c.Cache.QueueTTL,
			MaxSize:     500,
			Compression: false,
			Persistence: false,
		},
		"system": {
			Strategy:    StrategyWriteThrough,
			TTL:         c.Cache.SystemTTL,
			MaxSize:     100,
			Compression: false,
			Persistence: true,
		},
		"auth": {
			Strategy:    StrategyWriteBack,
			TTL:         c.Cache.AuthTTL,
			MaxSize:     20000,
			Compression: false,
			Persistence: false,
		},
	}
}

// parseMemorySize convierte string de memoria a bytes
func (c *Config) parseMemorySize(size string) int64 {
	if size == "" {
		return 100 * 1024 * 1024 // 100MB por defecto
	}

	// Parseo simple (en producción usar library como go-units)
	switch {
	case strings.HasSuffix(size, "GB"):
		if val, err := strconv.ParseFloat(strings.TrimSuffix(size, "GB"), 64); err == nil {
			return int64(val * 1024 * 1024 * 1024)
		}
	case strings.HasSuffix(size, "MB"):
		if val, err := strconv.ParseFloat(strings.TrimSuffix(size, "MB"), 64); err == nil {
			return int64(val * 1024 * 1024)
		}
	case strings.HasSuffix(size, "KB"):
		if val, err := strconv.ParseFloat(strings.TrimSuffix(size, "KB"), 64); err == nil {
			return int64(val * 1024)
		}
	}

	// Fallback a 100MB
	return 100 * 1024 * 1024
}

// IsCacheEnabled verifica si el caché está habilitado
func (c *Config) IsCacheEnabled() bool {
	return c.Cache.Enabled
}

// GetCacheWarmupConfig obtiene configuración de warmup
func (c *Config) GetCacheWarmupConfig() *CacheWarmupConfig {
	return &CacheWarmupConfig{
		Enabled:     c.Cache.WarmupEnabled,
		Timeout:     c.Cache.WarmupTimeout,
		Concurrency: c.Cache.WarmupConcurrency,
	}
}

// GetCacheInvalidationConfig obtiene configuración de invalidación
func (c *Config) GetCacheInvalidationConfig() *CacheInvalidationConfig {
	return &CacheInvalidationConfig{
		Enabled: c.Cache.InvalidationEnabled,
		Buffer:  c.Cache.InvalidationBuffer,
		Timeout: c.Cache.InvalidationTimeout,
	}
}

// ValidateCacheConfig valida la configuración de caché
func (c *Config) ValidateCacheConfig() error {
	if !c.Cache.Enabled {
		return nil // No validar si está deshabilitado
	}

	if c.Cache.DefaultTTL <= 0 {
		return fmt.Errorf("cache default TTL must be positive")
	}

	if c.Cache.CleanupInterval <= 0 {
		return fmt.Errorf("cache cleanup interval must be positive")
	}

	if c.Cache.CompressionLevel < 0 || c.Cache.CompressionLevel > 9 {
		return fmt.Errorf("cache compression level must be between 0 and 9")
	}

	validSerializers := []string{"json", "gob", "msgpack"}
	isValidSerializer := false
	for _, s := range validSerializers {
		if c.Cache.Serializer == s {
			isValidSerializer = true
			break
		}
	}
	if !isValidSerializer {
		return fmt.Errorf("cache serializer must be one of: %s", strings.Join(validSerializers, ", "))
	}

	return nil
}

// GetCacheBackupConfig obtiene configuración de backup del caché
func (c *Config) GetCacheBackupConfig() *CacheBackupConfig {
	return &CacheBackupConfig{
		Enabled:  c.Cache.BackupEnabled,
		Interval: c.Cache.BackupInterval,
		Path:     c.Cache.BackupPath,
	}
}

// CacheBackupConfig configuración de backup del caché
type CacheBackupConfig struct {
	Enabled  bool
	Interval time.Duration
	Path     string
}
