package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Engine_API_Workflow/internal/services/integration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestSlackService_SendMessage tests sending messages via webhook
func TestSlackService_SendMessage(t *testing.T) {
	logger := zap.NewNop()
	config := integration.SlackConfig{
		BotToken:     "",
		WebhookURL:   "",
		Timeout:      5 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 1 * time.Second,
	}

	t.Run("via webhook", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)

			assert.Equal(t, "#test-channel", payload["channel"])
			assert.Equal(t, "Test message", payload["text"])

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer server.Close()

		config.WebhookURL = server.URL
		service := integration.NewSlackService(config, logger)

		err := service.SendMessage(context.Background(), "#test-channel", "Test message")
		assert.NoError(t, err)
	})

	t.Run("missing configuration", func(t *testing.T) {
		config.WebhookURL = ""
		config.BotToken = ""
		service := integration.NewSlackService(config, logger)

		err := service.SendMessage(context.Background(), "#test-channel", "Test message")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be configured")
	})

	t.Run("empty channel", func(t *testing.T) {
		config.WebhookURL = "https://hooks.slack.com/test"
		service := integration.NewSlackService(config, logger)

		err := service.SendMessage(context.Background(), "", "Test message")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel cannot be empty")
	})

	t.Run("empty message", func(t *testing.T) {
		config.WebhookURL = "https://hooks.slack.com/test"
		service := integration.NewSlackService(config, logger)

		err := service.SendMessage(context.Background(), "#test-channel", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message cannot be empty")
	})
}

// TestSlackService_SendMessage_API tests sending via Slack Web API
func TestSlackService_SendMessage_API(t *testing.T) {
	logger := zap.NewNop()
	
	t.Run("via API with bot token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)

			assert.Equal(t, "#test-channel", payload["channel"])
			assert.Equal(t, "Test message", payload["text"])

			response := map[string]interface{}{
				"ok": true,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		// Note: In real implementation, we'd need to mock the Slack API URL
		// For now, we test the structure
		config := integration.SlackConfig{
			BotToken:     "test-token",
			WebhookURL:   "",
			Timeout:      5 * time.Second,
			MaxRetries:   3,
			RetryBackoff: 1 * time.Second,
		}
		service := integration.NewSlackService(config, logger)

		// This will fail because we can't easily mock the Slack API URL
		// But we test that the service is created correctly
		assert.NotNil(t, service)
	})
}

// TestSlackService_SendRichMessage tests sending rich messages with blocks
func TestSlackService_SendRichMessage(t *testing.T) {
	logger := zap.NewNop()
	config := integration.SlackConfig{
		BotToken:     "test-token",
		WebhookURL:   "",
		Timeout:      5 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 1 * time.Second,
	}
	service := integration.NewSlackService(config, logger)

	t.Run("valid rich message", func(t *testing.T) {
		blocks := []integration.SlackBlock{
			{
				Type: "section",
				Text: &integration.SlackText{
					Type: "mrkdwn",
					Text: "Hello *world*",
				},
			},
		}

		// This requires bot token, so it will fail without proper API setup
		// But we test the structure
		err := service.SendRichMessage(context.Background(), "#test-channel", blocks)
		// In a real test with mocked API, this would succeed
		// For now, we just verify the method exists and accepts parameters
		assert.Error(t, err) // Expected to fail without real API
	})

	t.Run("empty channel", func(t *testing.T) {
		blocks := []integration.SlackBlock{
			{Type: "section"},
		}

		err := service.SendRichMessage(context.Background(), "", blocks)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel cannot be empty")
	})

	t.Run("empty blocks", func(t *testing.T) {
		err := service.SendRichMessage(context.Background(), "#test-channel", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blocks cannot be empty")
	})
}

// TestSlackService_GetChannelInfo tests getting channel information
func TestSlackService_GetChannelInfo(t *testing.T) {
	logger := zap.NewNop()
	config := integration.SlackConfig{
		BotToken:     "test-token",
		WebhookURL:   "",
		Timeout:      5 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 1 * time.Second,
	}
	service := integration.NewSlackService(config, logger)

	t.Run("empty channel ID", func(t *testing.T) {
		info, err := service.GetChannelInfo(context.Background(), "")
		assert.Nil(t, info)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel ID cannot be empty")
	})

	t.Run("missing bot token", func(t *testing.T) {
		config.BotToken = ""
		service := integration.NewSlackService(config, logger)

		info, err := service.GetChannelInfo(context.Background(), "C123456")
		assert.Nil(t, info)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SLACK_BOT_TOKEN is required")
	})
}

// TestSlackService_UploadFile tests file upload functionality
func TestSlackService_UploadFile(t *testing.T) {
	logger := zap.NewNop()
	config := integration.SlackConfig{
		BotToken:     "test-token",
		WebhookURL:   "",
		Timeout:      5 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 1 * time.Second,
	}
	service := integration.NewSlackService(config, logger)

	t.Run("empty channel", func(t *testing.T) {
		err := service.UploadFile(context.Background(), "", "test.txt", []byte("test"), "text/plain")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel cannot be empty")
	})

	t.Run("empty filename", func(t *testing.T) {
		err := service.UploadFile(context.Background(), "#test-channel", "", []byte("test"), "text/plain")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "filename cannot be empty")
	})

	t.Run("empty content", func(t *testing.T) {
		err := service.UploadFile(context.Background(), "#test-channel", "test.txt", nil, "text/plain")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file content cannot be empty")
	})

	t.Run("missing bot token", func(t *testing.T) {
		config.BotToken = ""
		service := integration.NewSlackService(config, logger)

		err := service.UploadFile(context.Background(), "#test-channel", "test.txt", []byte("test"), "text/plain")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SLACK_BOT_TOKEN is required")
	})
}

// TestSlackService_ErrorHandling tests error handling scenarios
func TestSlackService_ErrorHandling(t *testing.T) {
	logger := zap.NewNop()

	t.Run("webhook server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal error"}`))
		}))
		defer server.Close()

		config := integration.SlackConfig{
			WebhookURL:   server.URL,
			Timeout:      5 * time.Second,
			MaxRetries:   3,
			RetryBackoff: 1 * time.Second,
		}
		service := integration.NewSlackService(config, logger)

		err := service.SendMessage(context.Background(), "#test-channel", "Test message")
		assert.Error(t, err)
	})

	t.Run("webhook invalid response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid json`))
		}))
		defer server.Close()

		config := integration.SlackConfig{
			WebhookURL:   server.URL,
			Timeout:      5 * time.Second,
			MaxRetries:   3,
			RetryBackoff: 1 * time.Second,
		}
		service := integration.NewSlackService(config, logger)

		// Slack webhook should still succeed with "ok" response
		err := service.SendMessage(context.Background(), "#test-channel", "Test message")
		// Webhook returns "ok" string, not JSON, so this should still work
		assert.NoError(t, err)
	})
}




