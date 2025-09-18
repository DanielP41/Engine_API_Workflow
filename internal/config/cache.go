// internal/config/cache.go
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"Engine_API_Workflow/pkg/cache"
)

// CacheConfiguration configuración específica del caché
type CacheConfiguration struct {
	// Configuración básica
	Enabled          bool          `env:"CACHE_ENABLED" envDefault:"true"`
	DefaultTTL       time.Duration `env:"CACHE_DEFAULT_TTL" envDefault:"5m"`
	CleanupInterval  time.Duration `env:"CACHE_CLEANUP_INTERVAL" envDefault:"1h"`
	MaxMemory        string        `env:"CACHE_MAX_MEMORY" envDefault:"100MB"`
	CompressionLevel int           `env:"CACHE_COMPRESSION_LEVEL" envDefault:"1"`
	Serializer       string        `env:"CACHE_SERIALIZER" envDefault:"json"`

	// Configuración de TTL por dominio
	DashboardTTL time.Duration `env:"CACHE_DASHBOARD_TTL" envDefault:"30s"`
	WorkflowTTL  time.Duration `env:"CACHE_WORKFLOW_TTL" envDefault:"5m"`
	UserTTL      time.Duration `env:"CACHE_USER_TTL" envDefault:"15m"`
	MetricsTTL   time.Duration `env:"CACHE_METRICS_TTL" envDefault:"1m"`
	QueueTTL     time.Duration `env:"CACHE_QUEUE_TTL" envDefault:"10s"`
	SystemTTL    time.Duration `env:"CACHE_SYSTEM_TTL" envDefault:"5m"`
	AuthTTL      time.Duration `env:"CACHE_AUTH_TTL" envDefault:"10m"`

	// Configuración avanzada
	EnableCompression  bool `env:"CACHE_ENABLE_COMPRESSION" envDefault:"false"`
	EnableMetrics      bool `env:"CACHE_ENABLE_METRICS" envDefault:"true"`
	EnableDistribution bool `env:"CACHE_ENABLE_DISTRIBUTION" envDefault:"false"`
	EnablePersistence  bool `env:"CACHE_ENABLE_PERSISTENCE" envDefault:"true"`
	EnableWarmup       bool `env:"CACHE_ENABLE_WARMUP" envDefault:"true"`

	// Configuración de warmup
	WarmupEnabled     bool          `env:"CACHE_WARMUP_ENABLED" envDefault:"true"`
	WarmupTimeout     time.Duration `env:"CACHE_WARMUP_TIMEOUT" envDefault:"30s"`
	WarmupConcurrency int           `env:"CACHE_WARMUP_CONCURRENCY" envDefault:"5"`

	// Configuración de invalidación
	InvalidationEnabled bool          `env:"CACHE_INVALIDATION_ENABLED" envDefault:"true"`
	InvalidationBuffer  int           `env:"CACHE_INVALIDATION_BUFFER" envDefault:"1000"`
	InvalidationTimeout time.Duration `env:"CACHE_INVALIDATION_TIMEOUT" envDefault:"5s"`

	// Configuración de backup del caché
	BackupEnabled  bool          `env:"CACHE_BACKUP_ENABLED" envDefault:"false"`
	BackupInterval time.Duration `env:"CACHE_BACKUP_INTERVAL" envDefault:"1h"`
	BackupPath     string        `env:"CACHE_BACKUP_PATH" envDefault:"./cache_backups"`
}

// GetCacheConfig obtiene la configuración de caché del config principal
func (c *Config) GetCacheConfig() *cache.CacheConfig {
	maxMemory := c.parseMemorySize(c.Cache.MaxMemory)

	return &cache.CacheConfig{
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

// CacheWarmupConfig configuración de warmup del caché
type CacheWarmupConfig struct {
	Enabled     bool
	Timeout     time.Duration
	Concurrency int
}

// GetCacheInvalidationConfig obtiene configuración de invalidación
func (c *Config) GetCacheInvalidationConfig() *CacheInvalidationConfig {
	return &CacheInvalidationConfig{
		Enabled: c.Cache.InvalidationEnabled,
		Buffer:  c.Cache.InvalidationBuffer,
		Timeout: c.Cache.InvalidationTimeout,
	}
}

// CacheInvalidationConfig configuración de invalidación del caché
type CacheInvalidationConfig struct {
	Enabled bool
	Buffer  int
	Timeout time.Duration
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
