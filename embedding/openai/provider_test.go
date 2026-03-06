package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				APIKey:     "test-key",
				Model:      "text-embedding-3-small",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
				BatchSize:  100,
			},
			wantErr: false,
		},
		{
			name: "empty api key",
			config: Config{
				APIKey: "",
				Model:  "text-embedding-3-small",
			},
			wantErr: true,
		},
		{
			name: "invalid model",
			config: Config{
				APIKey: "test-key",
				Model:  "invalid-model",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestProvider_Dimension(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		{
			name:     "text-embedding-3-small",
			model:    "text-embedding-3-small",
			expected: 1536,
		},
		{
			name:     "text-embedding-3-large",
			model:    "text-embedding-3-large",
			expected: 3072,
		},
		{
			name:     "text-embedding-ada-002",
			model:    "text-embedding-ada-002",
			expected: 1536,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(Config{
				APIKey: "test-key",
				Model:  tt.model,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, provider.Dimension())
		})
	}
}

func TestProvider_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/embeddings", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var reqBody EmbeddingRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "text-embedding-3-small", reqBody.Model)
		assert.Equal(t, []string{"test text"}, reqBody.Input)

		response := EmbeddingResponse{
			Data: []EmbeddingData{
				{
					Embedding: make([]float32, 1536),
					Index:     0,
				},
			},
			Model: "text-embedding-3-small",
			Usage: Usage{
				PromptTokens: 3,
				TotalTokens:  3,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := New(Config{
		APIKey:  "test-key",
		Model:   "text-embedding-3-small",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	embeddings, err := provider.Embed(context.Background(), []string{"test text"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 1)
	assert.Len(t, embeddings[0], 1536)
}

func TestProvider_Embed_Batching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var reqBody EmbeddingRequest
		json.NewDecoder(r.Body).Decode(&reqBody)

		response := EmbeddingResponse{
			Data: make([]EmbeddingData, len(reqBody.Input)),
			Usage: Usage{
				PromptTokens: len(reqBody.Input),
				TotalTokens:  len(reqBody.Input),
			},
		}

		for i := range response.Data {
			response.Data[i] = EmbeddingData{
				Embedding: make([]float32, 1536),
				Index:     i,
			}
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := New(Config{
		APIKey:    "test-key",
		Model:     "text-embedding-3-small",
		BaseURL:   server.URL,
		BatchSize: 2,
	})
	require.NoError(t, err)

	texts := []string{"text1", "text2", "text3", "text4", "text5"}
	embeddings, err := provider.Embed(context.Background(), texts)
	require.NoError(t, err)
	assert.Len(t, embeddings, 5)
	assert.Equal(t, 3, callCount)
}

func TestProvider_Embed_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		wantErr    string
	}{
		{
			name:       "invalid api key",
			statusCode: 401,
			response:   `{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`,
			wantErr:    "invalid",
		},
		{
			name:       "rate limit",
			statusCode: 429,
			response:   `{"error": {"message": "Rate limit exceeded", "type": "rate_limit_error"}}`,
			wantErr:    "rate",
		},
		{
			name:       "server error",
			statusCode: 500,
			response:   `{"error": {"message": "Internal server error", "type": "server_error"}}`,
			wantErr:    "server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			provider, err := New(Config{
				APIKey:     "test-key",
				Model:      "text-embedding-3-small",
				BaseURL:    server.URL,
				MaxRetries: 0,
			})
			require.NoError(t, err)

			_, err = provider.Embed(context.Background(), []string{"test"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestProvider_Embed_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	provider, err := New(Config{
		APIKey:     "test-key",
		Model:      "text-embedding-3-small",
		BaseURL:    server.URL,
		Timeout:    100 * time.Millisecond,
		MaxRetries: 0,
	})
	require.NoError(t, err)

	_, err = provider.Embed(context.Background(), []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestProvider_Embed_EmptyInput(t *testing.T) {
	provider, err := New(Config{
		APIKey: "test-key",
		Model:  "text-embedding-3-small",
	})
	require.NoError(t, err)

	embeddings, err := provider.Embed(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Empty(t, embeddings)
}

func TestParseEmbeddingResponse(t *testing.T) {
	responseBody := `{
		"object": "list",
		"data": [
			{
				"object": "embedding",
				"embedding": [0.0023, -0.0052, 0.0012],
				"index": 0
			}
		],
		"model": "text-embedding-3-small",
		"usage": {
			"prompt_tokens": 8,
			"total_tokens": 8
		}
	}`

	var resp EmbeddingResponse
	err := json.Unmarshal([]byte(responseBody), &resp)
	require.NoError(t, err)

	assert.Equal(t, "list", resp.Object)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, float32(0.0023), resp.Data[0].Embedding[0])
	assert.Equal(t, 8, resp.Usage.PromptTokens)
}

func TestReadAll(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid input",
			input:   "test content",
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := io.NopCloser(strings.NewReader(tt.input))
			content, err := io.ReadAll(r)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.input, string(content))
			}
		})
	}
}
