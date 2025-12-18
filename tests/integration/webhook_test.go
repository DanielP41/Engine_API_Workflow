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

// TestWebhookService_ValidateWebhookURL tests URL validation
func TestWebhookService_ValidateWebhookURL(t *testing.T) {
	logger := zap.NewNop()
	config := integration.WebhookConfig{
		Timeout:         30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   1 * time.Second,
		InsecureSSL:    false,
		FollowRedirects: true,
		MaxRedirects:   5,
		UserAgent:      "Test-Agent/1.0",
	}
	service := integration.NewWebhookService(config, logger)

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid HTTPS URL",
			url:     "https://example.com/webhook",
			wantErr: false,
		},
		{
			name:    "valid HTTP URL",
			url:     "http://example.com/webhook",
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid URL format",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			url:     "ftp://example.com/webhook",
			wantErr: true,
		},
		{
			name:    "localhost URL",
			url:     "http://localhost:8080/webhook",
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

// TestWebhookService_SendWebhook tests webhook sending with mock server
func TestWebhookService_SendWebhook(t *testing.T) {
	logger := zap.NewNop()
	config := integration.WebhookConfig{
		Timeout:         5 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   100 * time.Millisecond,
		InsecureSSL:    false,
		FollowRedirects: true,
		MaxRedirects:   5,
		UserAgent:      "Test-Agent/1.0",
	}

	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		payload        interface{}
		headers        map[string]string
		wantErr        bool
		setupServer    func() *httptest.Server
	}{
		{
			name:         "successful POST request",
			statusCode:   http.StatusOK,
			responseBody: `{"success": true}`,
			payload: map[string]interface{}{
				"message": "test",
				"data":   "test data",
			},
			headers: map[string]string{
				"X-Custom-Header": "test-value",
			},
			wantErr: false,
		},
		{
			name:         "server error",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"error": "internal server error"}`,
			payload:      map[string]string{"error": "test"},
			wantErr:      true,
		},
		{
			name:         "not found",
			statusCode:   http.StatusNotFound,
			responseBody: `{"error": "not found"}`,
			payload:      map[string]string{"test": "data"},
			wantErr:      true,
		},
		{
			name:         "empty payload",
			statusCode:   http.StatusOK,
			responseBody: `{"success": true}`,
			payload:      nil,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				assert.Equal(t, http.MethodPost, r.Method)
				
				// Verify headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, config.UserAgent, r.Header.Get("User-Agent"))
				
				if tt.headers != nil {
					for key, value := range tt.headers {
						assert.Equal(t, value, r.Header.Get(key))
					}
				}
				
				// Verify payload if present
				if tt.payload != nil {
					var receivedPayload map[string]interface{}
					err := json.NewDecoder(r.Body).Decode(&receivedPayload)
					require.NoError(t, err)
					
					expectedPayload, _ := tt.payload.(map[string]interface{})
					if expectedPayload != nil {
						for key, value := range expectedPayload {
							assert.Equal(t, value, receivedPayload[key])
						}
					}
				}
				
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			service := integration.NewWebhookService(config, logger)
			ctx := context.Background()

			err := service.SendWebhook(ctx, server.URL, tt.payload, tt.headers)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWebhookService_SendWebhookWithMethod tests different HTTP methods
func TestWebhookService_SendWebhookWithMethod(t *testing.T) {
	logger := zap.NewNop()
	config := integration.WebhookConfig{
		Timeout:         5 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   100 * time.Millisecond,
		InsecureSSL:    false,
		FollowRedirects: true,
		MaxRedirects:   5,
		UserAgent:      "Test-Agent/1.0",
	}
	service := integration.NewWebhookService(config, logger)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch}
	
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, method, r.Method)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"success": true}`))
			}))
			defer server.Close()

			payload := map[string]string{"test": "data"}
			err := service.SendWebhookWithMethod(context.Background(), method, server.URL, payload, nil)
			assert.NoError(t, err)
		})
	}

	// Test invalid method
	t.Run("invalid method", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer server.Close()

		err := service.SendWebhookWithMethod(context.Background(), "GET", server.URL, nil, nil)
		assert.Error(t, err)
	})
}

// TestWebhookService_RetryWebhook tests retry functionality
func TestWebhookService_RetryWebhook(t *testing.T) {
	logger := zap.NewNop()
	config := integration.WebhookConfig{
		Timeout:         2 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   100 * time.Millisecond,
		InsecureSSL:    false,
		FollowRedirects: true,
		MaxRedirects:   5,
		UserAgent:      "Test-Agent/1.0",
	}

	t.Run("success after retries", func(t *testing.T) {
		attempt := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempt++
			if attempt < 3 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"success": true}`))
			}
		}))
		defer server.Close()

		service := integration.NewWebhookService(config, logger)
		payload := map[string]string{"test": "data"}
		
		err := service.RetryWebhook(context.Background(), server.URL, payload, nil, 3)
		assert.NoError(t, err)
		assert.Equal(t, 3, attempt)
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		service := integration.NewWebhookService(config, logger)
		payload := map[string]string{"test": "data"}
		
		err := service.RetryWebhook(context.Background(), server.URL, payload, nil, 2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed after")
	})
}

// TestWebhookService_Timeout tests timeout handling
func TestWebhookService_Timeout(t *testing.T) {
	logger := zap.NewNop()
	config := integration.WebhookConfig{
		Timeout:         100 * time.Millisecond,
		MaxRetries:     1,
		RetryBackoff:   50 * time.Millisecond,
		InsecureSSL:    false,
		FollowRedirects: true,
		MaxRedirects:   5,
		UserAgent:      "Test-Agent/1.0",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := integration.NewWebhookService(config, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := service.SendWebhook(ctx, server.URL, map[string]string{"test": "data"}, nil)
	assert.Error(t, err)
}


