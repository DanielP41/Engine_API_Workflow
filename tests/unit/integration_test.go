package unit

import (
	"testing"
	"time"

	"Engine_API_Workflow/internal/services/integration"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestWebhookService_Config tests configuration defaults
func TestWebhookService_Config(t *testing.T) {
	logger := zap.NewNop()

	t.Run("default values", func(t *testing.T) {
		config := integration.WebhookConfig{}
		service := integration.NewWebhookService(config, logger)
		assert.NotNil(t, service)
	})

	t.Run("custom timeout", func(t *testing.T) {
		config := integration.WebhookConfig{
			Timeout: 60 * time.Second,
		}
		service := integration.NewWebhookService(config, logger)
		assert.NotNil(t, service)
	})

	t.Run("insecure SSL", func(t *testing.T) {
		config := integration.WebhookConfig{
			InsecureSSL: true,
		}
		service := integration.NewWebhookService(config, logger)
		assert.NotNil(t, service)
	})
}

// TestSlackService_Config tests Slack service configuration
func TestSlackService_Config(t *testing.T) {
	logger := zap.NewNop()

	t.Run("default values", func(t *testing.T) {
		config := integration.SlackConfig{}
		service := integration.NewSlackService(config, logger)
		assert.NotNil(t, service)
	})

	t.Run("with webhook URL", func(t *testing.T) {
		config := integration.SlackConfig{
			WebhookURL: "https://hooks.slack.com/test",
		}
		service := integration.NewSlackService(config, logger)
		assert.NotNil(t, service)
	})

	t.Run("with bot token", func(t *testing.T) {
		config := integration.SlackConfig{
			BotToken: "xoxb-test-token",
		}
		service := integration.NewSlackService(config, logger)
		assert.NotNil(t, service)
	})
}

// TestWebhookService_Validation tests URL validation edge cases
func TestWebhookService_Validation(t *testing.T) {
	logger := zap.NewNop()
	config := integration.WebhookConfig{
		Timeout: 30 * time.Second,
	}
	service := integration.NewWebhookService(config, logger)

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "URL with query parameters",
			url:     "https://example.com/webhook?key=value",
			wantErr: false,
		},
		{
			name:    "URL with port",
			url:     "https://example.com:8080/webhook",
			wantErr: false,
		},
		{
			name:    "URL with path",
			url:     "https://example.com/api/v1/webhook",
			wantErr: false,
		},
		{
			name:    "URL with fragment",
			url:     "https://example.com/webhook#section",
			wantErr: false,
		},
		{
			name:    "IP address",
			url:     "http://192.168.1.1/webhook",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateWebhookURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSlackBlock_Structure tests Slack block structure
func TestSlackBlock_Structure(t *testing.T) {
	t.Run("section block", func(t *testing.T) {
		block := integration.SlackBlock{
			Type: "section",
			Text: &integration.SlackText{
				Type: "mrkdwn",
				Text: "Test text",
			},
		}
		assert.Equal(t, "section", block.Type)
		assert.NotNil(t, block.Text)
		assert.Equal(t, "Test text", block.Text.Text)
	})

	t.Run("block with fields", func(t *testing.T) {
		block := integration.SlackBlock{
			Type: "section",
			Fields: []integration.SlackField{
				{
					Type:  "mrkdwn",
					Text:  "Field 1",
					Short: true,
				},
			},
		}
		assert.Len(t, block.Fields, 1)
		assert.True(t, block.Fields[0].Short)
	})
}

// TestWebhookService_Headers tests custom headers
func TestWebhookService_Headers(t *testing.T) {
	logger := zap.NewNop()
	config := integration.WebhookConfig{
		Timeout: 5 * time.Second,
		DefaultHeaders: map[string]string{
			"X-Default-Header": "default-value",
		},
	}
	service := integration.NewWebhookService(config, logger)

	// This test would require a mock server to verify headers
	// For now, we just verify the service is created with default headers
	assert.NotNil(t, service)
}

// TestSlackService_BlockTypes tests different Slack block types
func TestSlackService_BlockTypes(t *testing.T) {
	t.Run("image block", func(t *testing.T) {
		block := integration.SlackBlock{
			Type:     "image",
			ImageURL: "https://example.com/image.png",
			AltText:  "Test image",
		}
		assert.Equal(t, "image", block.Type)
		assert.NotEmpty(t, block.ImageURL)
	})

	t.Run("block with accessory", func(t *testing.T) {
		block := integration.SlackBlock{
			Type: "section",
			Accessory: &integration.SlackAccessory{
				Type: "button",
				Text: &integration.SlackText{
					Type: "plain_text",
					Text: "Click me",
				},
			},
		}
		assert.NotNil(t, block.Accessory)
		assert.Equal(t, "button", block.Accessory.Type)
	})
}
