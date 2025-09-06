package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// TokenBlacklist maneja la lista negra de tokens JWT revocados
type TokenBlacklist struct {
	redis  *redis.Client
	prefix string // Prefijo para las keys de Redis (ej: "blacklist:")
}

// BlacklistConfig configuración para el sistema de blacklist
type BlacklistConfig struct {
	RedisClient *redis.Client
	KeyPrefix   string
}

// NewTokenBlacklist crea una nueva instancia del blacklist
func NewTokenBlacklist(config BlacklistConfig) *TokenBlacklist {
	prefix := config.KeyPrefix
	if prefix == "" {
		prefix = "jwt_blacklist:"
	}

	return &TokenBlacklist{
		redis:  config.RedisClient,
		prefix: prefix,
	}
}

// BlacklistToken agrega un token a la lista negra hasta su expiración natural
func (tb *TokenBlacklist) BlacklistToken(ctx context.Context, tokenString string) error {
	if tokenString == "" {
		return fmt.Errorf("token string cannot be empty")
	}

	// Parsear el token sin verificar la firma (solo necesitamos los claims)
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return fmt.Errorf("failed to parse token for blacklisting: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return fmt.Errorf("invalid token claims type")
	}

	// Verificar que el token tenga tiempo de expiración
	if claims.ExpiresAt == nil {
		return fmt.Errorf("token does not have expiration time")
	}

	// Calcular TTL hasta la expiración natural del token
	expiration := claims.ExpiresAt.Time
	ttl := time.Until(expiration)

	// Solo agregar a blacklist si el token no ha expirado
	if ttl > 0 {
		key := tb.getBlacklistKey(tokenString)

		// Almacenar información del token revocado
		tokenInfo := map[string]interface{}{
			"user_id":    claims.UserID,
			"email":      claims.Email,
			"type":       claims.Type,
			"revoked_at": time.Now().Unix(),
			"expires_at": expiration.Unix(),
		}

		// Usar HSET para almacenar información estructurada
		pipe := tb.redis.Pipeline()
		pipe.HMSet(ctx, key, tokenInfo)
		pipe.Expire(ctx, key, ttl)

		_, err := pipe.Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to blacklist token in Redis: %w", err)
		}
	}

	return nil
}

// BlacklistTokenWithReason agrega un token a la lista negra con una razón específica
func (tb *TokenBlacklist) BlacklistTokenWithReason(ctx context.Context, tokenString, reason string) error {
	if reason == "" {
		return tb.BlacklistToken(ctx, tokenString)
	}

	// Similar a BlacklistToken pero con razón adicional
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return fmt.Errorf("failed to parse token for blacklisting: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return fmt.Errorf("invalid token claims type")
	}

	if claims.ExpiresAt == nil {
		return fmt.Errorf("token does not have expiration time")
	}

	expiration := claims.ExpiresAt.Time
	ttl := time.Until(expiration)

	if ttl > 0 {
		key := tb.getBlacklistKey(tokenString)

		tokenInfo := map[string]interface{}{
			"user_id":    claims.UserID,
			"email":      claims.Email,
			"type":       claims.Type,
			"reason":     reason, // Razón adicional
			"revoked_at": time.Now().Unix(),
			"expires_at": expiration.Unix(),
		}

		pipe := tb.redis.Pipeline()
		pipe.HMSet(ctx, key, tokenInfo)
		pipe.Expire(ctx, key, ttl)

		_, err := pipe.Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to blacklist token in Redis: %w", err)
		}
	}

	return nil
}

// IsBlacklisted verifica si un token está en la lista negra
func (tb *TokenBlacklist) IsBlacklisted(ctx context.Context, tokenString string) bool {
	if tokenString == "" {
		return false
	}

	key := tb.getBlacklistKey(tokenString)
	result := tb.redis.Exists(ctx, key)

	// Si hay error al consultar Redis, ser conservador y considerar el token válido
	// En producción podrías querer logear este error
	if result.Err() != nil {
		return false
	}

	return result.Val() > 0
}

// GetBlacklistInfo obtiene información detallada sobre un token en blacklist
func (tb *TokenBlacklist) GetBlacklistInfo(ctx context.Context, tokenString string) (map[string]string, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token string cannot be empty")
	}

	key := tb.getBlacklistKey(tokenString)
	result := tb.redis.HGetAll(ctx, key)

	if result.Err() != nil {
		return nil, fmt.Errorf("failed to get blacklist info: %w", result.Err())
	}

	data := result.Val()
	if len(data) == 0 {
		return nil, fmt.Errorf("token not found in blacklist")
	}

	return data, nil
}

// BlacklistAllUserTokens revoca todos los tokens de un usuario específico
func (tb *TokenBlacklist) BlacklistAllUserTokens(ctx context.Context, userID string, reason string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	// Usar un patrón para encontrar todos los tokens del usuario
	// Nota: Esto requiere que almacenemos tokens por usuario (implementación adicional).-
	userTokensKey := fmt.Sprintf("%suser:%s:*", tb.prefix, userID)

	keys, err := tb.redis.Keys(ctx, userTokensKey).Result()
	if err != nil {
		return fmt.Errorf("failed to find user tokens: %w", err)
	}

	if len(keys) == 0 {
		return nil // No hay tokens para revocar
	}

	// Pipeline para revocar todos los tokens del usuario
	pipe := tb.redis.Pipeline()
	for _, key := range keys {
		pipe.HSet(ctx, key, "revoked_at", time.Now().Unix(), "reason", reason)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to blacklist user tokens: %w", err)
	}

	return nil
}

// CleanupExpiredTokens limpia tokens expirados de la blacklist (opcional, Redis lo hace automáticamente)
func (tb *TokenBlacklist) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	// Esta función es opcional ya que Redis expira las keys automáticamente
	// Pero puede ser útil para estadísticas o limpieza manual

	pattern := tb.prefix + "*"
	iter := tb.redis.Scan(ctx, 0, pattern, 100).Iterator()

	var expiredKeys []string
	now := time.Now().Unix()

	for iter.Next(ctx) {
		key := iter.Val()

		// Verificar si el token ha expirado
		expiresAt, err := tb.redis.HGet(ctx, key, "expires_at").Int64()
		if err != nil {
			continue
		}

		if expiresAt < now {
			expiredKeys = append(expiredKeys, key)
		}
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("failed to scan blacklist keys: %w", err)
	}

	if len(expiredKeys) > 0 {
		deleted := tb.redis.Del(ctx, expiredKeys...)
		return deleted.Val(), deleted.Err()
	}

	return 0, nil
}

// GetStats obtiene estadísticas del blacklist
func (tb *TokenBlacklist) GetStats(ctx context.Context) (map[string]interface{}, error) {
	pattern := tb.prefix + "*"

	// Contar total de tokens en blacklist
	keys, err := tb.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get blacklist keys: %w", err)
	}

	totalBlacklisted := len(keys)

	// Estadísticas básicas
	stats := map[string]interface{}{
		"total_blacklisted": totalBlacklisted,
		"generated_at":      time.Now().Unix(),
	}

	// Contar por tipo de token si hay suficientes
	if totalBlacklisted > 0 && totalBlacklisted < 1000 { // Evitar operaciones costosas
		accessCount := 0
		refreshCount := 0

		pipe := tb.redis.Pipeline()
		for _, key := range keys {
			pipe.HGet(ctx, key, "type")
		}

		results, err := pipe.Exec(ctx)
		if err == nil {
			for _, result := range results {
				if cmd, ok := result.(*redis.StringCmd); ok {
					tokenType, _ := cmd.Result()
					switch tokenType {
					case "access":
						accessCount++
					case "refresh":
						refreshCount++
					}
				}
			}
		}

		stats["access_tokens"] = accessCount
		stats["refresh_tokens"] = refreshCount
	}

	return stats, nil
}

// getBlacklistKey genera la key de Redis para un token específico
func (tb *TokenBlacklist) getBlacklistKey(tokenString string) string {
	// Usar hash del token para key más corta y consistente
	// En producción podrías usar SHA256 para mayor seguridad
	return fmt.Sprintf("%s%x", tb.prefix, []byte(tokenString))
}

// HealthCheck verifica que Redis esté disponible para blacklisting
func (tb *TokenBlacklist) HealthCheck(ctx context.Context) error {
	// Ping simple a Redis
	return tb.redis.Ping(ctx).Err()
}
