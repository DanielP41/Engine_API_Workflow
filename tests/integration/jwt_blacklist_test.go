package integration

import (
	"context"
	"testing"
	"time"

	"Engine_API_Workflow/pkg/jwt"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestJWTBlacklistIntegration(t *testing.T) {
	// Setup Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Check if Redis is up
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Logf("Redis not available on localhost:6379: %v", err)
		t.Skip("Redis is not available on localhost:6379, skipping integration test. Make sure 'docker-compose up redis' is running.")
	}

	// JWT Config (32 chars secret required)
	cfg := jwt.Config{
		SecretKey:       "integration-test-secret-key-32-chars-long-2024",
		AccessTokenTTL:  1 * time.Minute,
		RefreshTokenTTL: 5 * time.Minute,
		Issuer:          "test-issuer",
		Audience:        "test-audience",
	}

	jwtService := jwt.NewJWTServiceWithRedis(cfg, redisClient)
	userID := primitive.NewObjectID()
	email := "test@example.com"
	role := "user"

	t.Run("Revoke_AccessToken_Successfully", func(t *testing.T) {
		// 1. Generate token
		tokens, err := jwtService.GenerateTokens(userID, email, role)
		assert.NoError(t, err)
		assert.NotEmpty(t, tokens.AccessToken)

		// 2. Validate token (should be valid)
		claims, err := jwtService.ValidateToken(tokens.AccessToken)
		assert.NoError(t, err)
		assert.NotNil(t, claims)

		// 3. Revoke token
		err = jwtService.RevokeToken(tokens.AccessToken)
		assert.NoError(t, err)

		// 4. Validate again (should be revoked)
		_, err = jwtService.ValidateToken(tokens.AccessToken)
		assert.Error(t, err)
		assert.Equal(t, jwt.ErrTokenRevoked, err)
	})

	t.Run("Revoke_RefreshToken_Successfully", func(t *testing.T) {
		// 1. Generate tokens
		tokens, err := jwtService.GenerateTokens(userID, email, role)
		assert.NoError(t, err)
		assert.NotEmpty(t, tokens.RefreshToken)

		// 2. Validate refresh token (indirectly via RefreshToken method)
		_, err = jwtService.RefreshToken(tokens.RefreshToken)
		assert.NoError(t, err)

		// 3. Revoke refresh token
		err = jwtService.RevokeToken(tokens.RefreshToken)
		assert.NoError(t, err)

		// 4. Try to use it again (should fail)
		_, err = jwtService.RefreshToken(tokens.RefreshToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), jwt.ErrTokenRevoked.Error())
	})

	t.Run("Check_Redis_Key_Persistence", func(t *testing.T) {
		tokens, _ := jwtService.GenerateTokens(userID, email, role)
		claims, _ := jwtService.GetTokenClaims(tokens.AccessToken)

		err := jwtService.RevokeToken(tokens.AccessToken)
		assert.NoError(t, err)

		// Check Redis key directly
		key := "blacklist:token:" + claims.ID
		exists, err := redisClient.Exists(context.Background(), key).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), exists)

		// Check value
		val, err := redisClient.Get(context.Background(), key).Result()
		assert.NoError(t, err)
		assert.Equal(t, "revoked", val)
	})
}
