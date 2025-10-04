// internal/api/handlers/cache_handler.go
package handlers

import (
	"fmt"
	"strconv"
	"time"

	"Engine_API_Workflow/internal/utils"
	"Engine_API_Workflow/pkg/cache"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type CacheHandler struct {
	cacheManager *cache.CacheManager
	logger       *zap.Logger
}

func NewCacheHandler(cacheManager *cache.CacheManager, logger *zap.Logger) *CacheHandler {
	return &CacheHandler{
		cacheManager: cacheManager,
		logger:       logger,
	}
}

func (h *CacheHandler) GetStats(c *fiber.Ctx) error {
	stats, err := h.cacheManager.GetStats(c.Context())
	if err != nil {
		h.logger.Error("Failed to get cache stats", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get cache statistics", err)
	}

	managerMetrics := h.cacheManager.GetMetrics()

	enrichedStats := map[string]interface{}{
		"redis_stats":     stats,
		"manager_metrics": managerMetrics,
		"performance": map[string]interface{}{
			"hit_rate_percent": stats.HitRate * 100,
			"total_requests":   stats.HitCount + stats.MissCount,
			"cache_efficiency": h.calculateCacheEfficiency(stats),
			"memory_usage_mb":  float64(stats.UsedMemory) / (1024 * 1024),
		},
		"timestamp": time.Now(),
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache statistics retrieved successfully", enrichedStats)
}

func (h *CacheHandler) GetHealth(c *fiber.Ctx) error {
	start := time.Now()

	err := h.cacheManager.Ping(c.Context())
	latency := time.Since(start)

	health := map[string]interface{}{
		"status":     "healthy",
		"latency_ms": latency.Milliseconds(),
		"timestamp":  time.Now(),
	}

	if err != nil {
		health["status"] = "unhealthy"
		health["error"] = err.Error()

		h.logger.Error("Cache health check failed", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Cache is unhealthy", err)
	}

	if latency > 100*time.Millisecond {
		health["status"] = "degraded"
		health["warning"] = "High latency detected"
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache health check completed", health)
}

func (h *CacheHandler) GetKeys(c *fiber.Ctx) error {
	pattern := c.Query("pattern", "*")
	limitStr := c.Query("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 1000 {
		return utils.BadRequestResponse(c, "Invalid limit parameter (1-1000)", nil)
	}

	h.logger.Info("Getting cache keys",
		zap.String("pattern", pattern),
		zap.Int("limit", limit),
		zap.String("user_id", getUserID(c)))

	keys, err := h.cacheManager.GetKeys(c.Context(), pattern)
	if err != nil {
		h.logger.Error("Failed to get cache keys",
			zap.String("pattern", pattern),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get cache keys", err)
	}

	if len(keys) > limit {
		keys = keys[:limit]
	}

	result := map[string]interface{}{
		"keys":        keys,
		"total_found": len(keys),
		"pattern":     pattern,
		"limited":     len(keys) == limit,
		"timestamp":   time.Now(),
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache keys retrieved successfully", result)
}

func (h *CacheHandler) ClearCache(c *fiber.Ctx) error {
	h.logger.Warn("CRITICAL: Full cache clear initiated",
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)),
		zap.String("ip_address", c.IP()),
		zap.String("user_agent", c.Get("User-Agent")))

	if err := h.cacheManager.Clear(c.Context()); err != nil {
		h.logger.Error("Failed to clear cache",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to clear cache", err)
	}

	h.logger.Info("Cache cleared successfully", zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache cleared successfully", map[string]interface{}{
		"cleared_at": time.Now(),
		"cleared_by": getUserID(c),
	})
}

func (h *CacheHandler) ClearPattern(c *fiber.Ctx) error {
	pattern := c.Params("pattern")
	if pattern == "" {
		return utils.BadRequestResponse(c, "Pattern parameter is required", nil)
	}

	dangerousPatterns := []string{"*", "**", ""}
	for _, dangerous := range dangerousPatterns {
		if pattern == dangerous {
			return utils.BadRequestResponse(c, "Dangerous pattern not allowed. Use /clear for full cache clear", nil)
		}
	}

	h.logger.Warn("Cache pattern clear initiated",
		zap.String("pattern", pattern),
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)),
		zap.String("ip_address", c.IP()))

	if err := h.cacheManager.InvalidatePattern(c.Context(), pattern, "admin_clear"); err != nil {
		h.logger.Error("Failed to clear cache pattern",
			zap.String("pattern", pattern),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to clear cache pattern", err)
	}

	h.logger.Info("Cache pattern cleared successfully",
		zap.String("pattern", pattern),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache pattern cleared successfully", map[string]interface{}{
		"pattern":    pattern,
		"cleared_at": time.Now(),
		"cleared_by": getUserID(c),
	})
}

func (h *CacheHandler) ExecuteWarmup(c *fiber.Ctx) error {
	start := time.Now()

	h.logger.Info("Manual cache warmup initiated",
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)))

	if err := h.cacheManager.ExecuteWarmup(c.Context()); err != nil {
		h.logger.Error("Failed to execute cache warmup",
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to execute cache warmup", err)
	}

	duration := time.Since(start)

	h.logger.Info("Cache warmup completed successfully",
		zap.Duration("duration", duration),
		zap.String("user_id", getUserID(c)))

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache warmup executed successfully", map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
		"executed_at": time.Now(),
		"executed_by": getUserID(c),
	})
}

func (h *CacheHandler) GetMetrics(c *fiber.Ctx) error {
	managerMetrics := h.cacheManager.GetMetrics()

	redisStats, err := h.cacheManager.GetStats(c.Context())
	if err != nil {
		h.logger.Error("Failed to get Redis stats for metrics", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get cache metrics", err)
	}

	detailedMetrics := map[string]interface{}{
		"manager": managerMetrics,
		"redis":   redisStats,
		"performance": map[string]interface{}{
			"hit_rate":            redisStats.HitRate,
			"miss_rate":           1.0 - redisStats.HitRate,
			"requests_per_second": h.calculateRequestsPerSecond(managerMetrics),
			"cache_efficiency":    h.calculateCacheEfficiency(redisStats),
		},
		"operations": map[string]interface{}{
			"total_hits":    managerMetrics.Hits,
			"total_misses":  managerMetrics.Misses,
			"total_sets":    managerMetrics.Sets,
			"total_deletes": managerMetrics.Deletes,
			"invalidations": managerMetrics.Invalidations,
		},
		"warmup": map[string]interface{}{
			"successful_tasks": managerMetrics.WarmupTasks,
			"failed_tasks":     managerMetrics.WarmupFailures,
			"success_rate":     h.calculateWarmupSuccessRate(managerMetrics),
		},
		"memory": map[string]interface{}{
			"used_bytes":   redisStats.UsedMemory,
			"used_mb":      float64(redisStats.UsedMemory) / (1024 * 1024),
			"total_keys":   redisStats.TotalKeys,
			"avg_key_size": h.calculateAverageKeySize(redisStats),
		},
		"uptime": map[string]interface{}{
			"seconds":        redisStats.Uptime,
			"human_readable": h.formatUptime(redisStats.Uptime),
		},
		"timestamp": time.Now(),
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Detailed cache metrics retrieved successfully", detailedMetrics)
}

func (h *CacheHandler) calculateCacheEfficiency(stats *cache.CacheStats) float64 {
	total := stats.HitCount + stats.MissCount
	if total == 0 {
		return 0
	}

	hitRate := float64(stats.HitCount) / float64(total)

	memoryEfficiency := 1.0
	if stats.TotalKeys > 0 && stats.UsedMemory > 0 {
		avgKeySize := float64(stats.UsedMemory) / float64(stats.TotalKeys)
		if avgKeySize > 0 {
			memoryEfficiency = 1.0 / (1.0 + avgKeySize/1024)
		}
	}

	return (hitRate * 0.8) + (memoryEfficiency * 0.2)
}

func (h *CacheHandler) calculateRequestsPerSecond(metrics *cache.CacheMetrics) float64 {
	uptime := time.Since(metrics.LastResetTime).Seconds()
	if uptime <= 0 {
		return 0
	}

	totalRequests := float64(metrics.Hits + metrics.Misses)
	return totalRequests / uptime
}

func (h *CacheHandler) calculateWarmupSuccessRate(metrics *cache.CacheMetrics) float64 {
	total := metrics.WarmupTasks + metrics.WarmupFailures
	if total == 0 {
		return 0
	}

	return float64(metrics.WarmupTasks) / float64(total)
}

func (h *CacheHandler) calculateAverageKeySize(stats *cache.CacheStats) float64 {
	if stats.TotalKeys == 0 {
		return 0
	}

	return float64(stats.UsedMemory) / float64(stats.TotalKeys)
}

func (h *CacheHandler) formatUptime(seconds int64) string {
	duration := time.Duration(seconds) * time.Second

	days := int64(duration.Hours()) / 24
	hours := int64(duration.Hours()) % 24
	minutes := int64(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

func getUserID(c *fiber.Ctx) string {
	if userID := c.Locals("user_id"); userID != nil {
		return userID.(string)
	}
	return "unknown"
}

func getUserEmail(c *fiber.Ctx) string {
	if userEmail := c.Locals("user_email"); userEmail != nil {
		return userEmail.(string)
	}
	return "unknown"
}

type CacheKeyRequest struct {
	Key   string      `json:"key" validate:"required"`
	Value interface{} `json:"value,omitempty"`
	TTL   int         `json:"ttl,omitempty"`
}

func (h *CacheHandler) SetCacheKey(c *fiber.Ctx) error {
	var req CacheKeyRequest

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid JSON format", nil)
	}

	if req.Key == "" {
		return utils.BadRequestResponse(c, "Key is required", nil)
	}

	ttl := cache.TTLMedium
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}

	h.logger.Info("Manual cache key set",
		zap.String("key", req.Key),
		zap.Duration("ttl", ttl),
		zap.String("user_id", getUserID(c)))

	if err := h.cacheManager.Set(c.Context(), req.Key, req.Value, ttl); err != nil {
		h.logger.Error("Failed to set cache key",
			zap.String("key", req.Key),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to set cache key", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache key set successfully", map[string]interface{}{
		"key":    req.Key,
		"ttl":    ttl.String(),
		"set_at": time.Now(),
		"set_by": getUserID(c),
	})
}

func (h *CacheHandler) GetCacheKey(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.BadRequestResponse(c, "Key parameter is required", nil)
	}

	value, err := h.cacheManager.Get(c.Context(), key)
	if err != nil {
		if err.Error() == "KEY_NOT_FOUND: key not found" {
			return utils.NotFoundResponse(c, "Cache key not found")
		}

		h.logger.Error("Failed to get cache key",
			zap.String("key", key),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get cache key", err)
	}

	ttl, _ := h.cacheManager.GetTTL(c.Context(), key)

	result := map[string]interface{}{
		"key":           key,
		"value":         value,
		"ttl_remaining": ttl.String(),
		"retrieved_at":  time.Now(),
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache key retrieved successfully", result)
}

func (h *CacheHandler) DeleteCacheKey(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return utils.BadRequestResponse(c, "Key parameter is required", nil)
	}

	h.logger.Info("Manual cache key deletion",
		zap.String("key", key),
		zap.String("user_id", getUserID(c)),
		zap.String("user_email", getUserEmail(c)))

	if err := h.cacheManager.Delete(c.Context(), key); err != nil {
		h.logger.Error("Failed to delete cache key",
			zap.String("key", key),
			zap.String("user_id", getUserID(c)),
			zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to delete cache key", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache key deleted successfully", map[string]interface{}{
		"key":        key,
		"deleted_at": time.Now(),
		"deleted_by": getUserID(c),
	})
}

func (h *CacheHandler) GetCacheInfo(c *fiber.Ctx) error {
	stats, err := h.cacheManager.GetStats(c.Context())
	if err != nil {
		h.logger.Error("Failed to get cache info", zap.Error(err))
		return utils.InternalServerErrorResponse(c, "Failed to get cache information", err)
	}

	info := map[string]interface{}{
		"system": map[string]interface{}{
			"enabled": true,
			"backend": "redis",
			"version": "2.0.0",
			"uptime":  stats.Uptime,
		},
		"configuration": map[string]interface{}{
			"ttl_policies": map[string]string{
				"dashboard": cache.TTLShort.String(),
				"workflow":  cache.TTLLong.String(),
				"user":      cache.TTLMedium.String(),
				"metrics":   cache.TTLVeryShort.String(),
				"queue":     cache.TTLVeryShort.String(),
				"system":    cache.TTLMedium.String(),
			},
		},
		"current_status": map[string]interface{}{
			"total_keys":   stats.TotalKeys,
			"memory_usage": fmt.Sprintf("%.2f MB", float64(stats.UsedMemory)/(1024*1024)),
			"hit_rate":     fmt.Sprintf("%.2f%%", stats.HitRate*100),
			"health":       "healthy",
		},
		"features": []string{
			"automatic_warmup",
			"pattern_invalidation",
			"ttl_management",
			"statistics_tracking",
			"admin_operations",
		},
		"timestamp": time.Now(),
	}

	return utils.SuccessResponse(c, fiber.StatusOK, "Cache information retrieved successfully", info)
}
