// pkg/cache/interfaces.go
package cache

import (
	"context"
	"time"
)

// CacheService define la interfaz principal para operaciones de caché
type CacheService interface {
	// Operaciones básicas
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) bool

	// Operaciones de lote
	GetMany(ctx context.Context, keys []string) (map[string]interface{}, error)
	SetMany(ctx context.Context, items map[string]CacheItem) error
	DeleteMany(ctx context.Context, keys []string) error

	// Operaciones de patrón
	DeletePattern(ctx context.Context, pattern string) error
	GetKeys(ctx context.Context, pattern string) ([]string, error)

	// Operaciones con TTL
	GetTTL(ctx context.Context, key string) (time.Duration, error)
	UpdateTTL(ctx context.Context, key string, ttl time.Duration) error

	// Operaciones atómicas
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	Decrement(ctx context.Context, key string, delta int64) (int64, error)

	// Operaciones de cache con callback
	GetOrSet(ctx context.Context, key string, ttl time.Duration, fetchFunc func() (interface{}, error)) (interface{}, error)

	// Health y estadísticas
	Ping(ctx context.Context) error
	Stats(ctx context.Context) (*CacheStats, error)
	Clear(ctx context.Context) error

	// Close connection
	Close() error
}

// CacheItem representa un item en el caché con metadata
type CacheItem struct {
	Key       string        `json:"key"`
	Value     interface{}   `json:"value"`
	TTL       time.Duration `json:"ttl"`
	CreatedAt time.Time     `json:"created_at"`
}

// CacheStats representa estadísticas del caché
type CacheStats struct {
	HitCount    int64     `json:"hit_count"`
	MissCount   int64     `json:"miss_count"`
	HitRate     float64   `json:"hit_rate"`
	TotalKeys   int64     `json:"total_keys"`
	UsedMemory  int64     `json:"used_memory_bytes"`
	Uptime      int64     `json:"uptime_seconds"`
	LastUpdated time.Time `json:"last_updated"`
}

// CacheConfig configuración del sistema de caché
type CacheConfig struct {
	Enabled          bool          `json:"enabled"`
	DefaultTTL       time.Duration `json:"default_ttl"`
	CleanupInterval  time.Duration `json:"cleanup_interval"`
	MaxMemory        int64         `json:"max_memory_bytes"`
	CompressionLevel int           `json:"compression_level"`
	Serializer       string        `json:"serializer"` // "json", "gob", "msgpack"
}

// CacheError tipos de errores del caché
type CacheError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Key     string `json:"key,omitempty"`
}

func (e *CacheError) Error() string {
	if e.Key != "" {
		return e.Type + ": " + e.Message + " (key: " + e.Key + ")"
	}
	return e.Type + ": " + e.Message
}

// Errores predefinidos
var (
	ErrCacheDisabled  = &CacheError{Type: "CACHE_DISABLED", Message: "cache service is disabled"}
	ErrKeyNotFound    = &CacheError{Type: "KEY_NOT_FOUND", Message: "key not found in cache"}
	ErrKeyExpired     = &CacheError{Type: "KEY_EXPIRED", Message: "key has expired"}
	ErrInvalidValue   = &CacheError{Type: "INVALID_VALUE", Message: "value cannot be serialized"}
	ErrConnectionLost = &CacheError{Type: "CONNECTION_LOST", Message: "connection to cache backend lost"}
)

// Constantes para TTL predefinidos
const (
	TTLVeryShort   = 10 * time.Second     // Health checks, real-time data
	TTLShort       = 30 * time.Second     // Dashboard data, queue stats
	TTLMedium      = 5 * time.Minute      // User profiles, configurations
	TTLLong        = 30 * time.Minute     // Workflow definitions, system settings
	TTLVeryLong    = 2 * time.Hour        // Static data, templates
	TTLNeverExpire = 24 * time.Hour * 365 // Persistent cache items
)

// Prefijos de caché organizados por dominio
const (
	PrefixDashboard = "dashboard:"
	PrefixWorkflow  = "workflow:"
	PrefixUser      = "user:"
	PrefixMetrics   = "metrics:"
	PrefixQueue     = "queue:"
	PrefixSystem    = "system:"
	PrefixAuth      = "auth:"
	PrefixBackup    = "backup:"
)

// CacheKeyBuilder helper para construir claves de caché consistentes
type CacheKeyBuilder struct {
	prefix    string
	separator string
}

func NewCacheKeyBuilder(prefix string) *CacheKeyBuilder {
	return &CacheKeyBuilder{
		prefix:    prefix,
		separator: ":",
	}
}

func (kb *CacheKeyBuilder) Build(parts ...string) string {
	key := kb.prefix
	for _, part := range parts {
		if part != "" {
			key += kb.separator + part
		}
	}
	return key
}

func (kb *CacheKeyBuilder) BuildWithID(id string, suffix ...string) string {
	parts := append([]string{id}, suffix...)
	return kb.Build(parts...)
}

// Helpers para construcción de claves comunes
var (
	DashboardKeys = NewCacheKeyBuilder(PrefixDashboard)
	WorkflowKeys  = NewCacheKeyBuilder(PrefixWorkflow)
	UserKeys      = NewCacheKeyBuilder(PrefixUser)
	MetricsKeys   = NewCacheKeyBuilder(PrefixMetrics)
	QueueKeys     = NewCacheKeyBuilder(PrefixQueue)
	SystemKeys    = NewCacheKeyBuilder(PrefixSystem)
	AuthKeys      = NewCacheKeyBuilder(PrefixAuth)
	BackupKeys    = NewCacheKeyBuilder(PrefixBackup)
)
