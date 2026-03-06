package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{{
		name: "valid config",
		config: Config{
			APIKey:      "test-key",
			Model:       "claude-3-opus-20240229",
			Timeout:     30 * time.Second,
			MaxRetries:  3,
			Temperature: 0.7,
			MaxTokens:   1000,
		},
		wantErr: false,
	}, {
		name: "empty api key",
		config: Config{
			APIKey: "",
			Model:  "claude-3-opus-20240229",
		},
		wantErr: true,
	}, {
		name: "invalid model",
		config: Config{
			APIKey: "test-key",
			Model:  "invalid-model",
		},
		wantErr: false,
	}, {
		name: "default values",
		config: Config{
			APIKey: "test-key",
		},
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				// Verify default values are set
				if tt.config.Model == "" {
					assert.Equal(t, "claude-3-opus-20240229", client.config.Model)
				}
				if tt.config.BaseURL == "" {
					assert.Equal(t, "https://api.anthropic.com", client.config.BaseURL)
				}
				if tt.config.Timeout == 0 {
					assert.Equal(t, 30*time.Second, client.config.Timeout)
				}
				if tt.config.MaxRetries == 0 {
					assert.Equal(t, 3, client.config.MaxRetries)
				}
				if tt.config.Temperature == 0 {
					assert.Equal(t, 0.7, client.config.Temperature)
				}
			}
		})
	}
}

func TestClient_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

		var reqBody ChatCompletionRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "claude-3-opus-20240229", reqBody.Model)
		assert.Len(t, reqBody.Messages, 1)
		assert.Equal(t, "user", reqBody.Messages[0].Role)
		assert.Equal(t, "test prompt", reqBody.Messages[0].Content)

		response := ChatCompletionResponse{
			ID:      "msg_abc123",
			Object:  "message",
			Created: 1699000000,
			Model:   "claude-3-opus-20240229",
			Choices: []Choice{{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "test response",
				},
				FinishReason: "stop",
			}},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := New(Config{
		APIKey:  "test-key",
		Model:   "claude-3-opus-20240229",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	response, err := client.Complete(context.Background(), "test prompt")
	require.NoError(t, err)
	assert.Equal(t, "test response", response)
}

func TestClient_CompleteStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody ChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&reqBody)

		assert.True(t, reqBody.Stream)

		flusher, _ := w.(http.Flusher)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		chunks := []string{
			`data: {"id":"msg_abc123","object":"message","created":1699000000,"model":"claude-3-opus-20240229","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"msg_abc123","object":"message","created":1699000000,"model":"claude-3-opus-20240229","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"msg_abc123","object":"message","created":1699000000,"model":"claude-3-opus-20240229","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	client, err := New(Config{
		APIKey:  "test-key",
		Model:   "claude-3-opus-20240229",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	stream, err := client.CompleteStream(context.Background(), "test prompt")
	require.NoError(t, err)

	var result strings.Builder
	for chunk := range stream {
		result.WriteString(chunk)
	}

	assert.Equal(t, "Hello world", result.String())
}

func TestClient_Complete_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		wantErr    string
	}{{
		name:       "invalid api key",
		statusCode: 401,
		response:   `{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`,
		wantErr:    "invalid_request_error",
	}, {
		name:       "rate limit",
		statusCode: 429,
		response:   `{"error": {"message": "Rate limit exceeded", "type": "rate_limit_error"}}`,
		wantErr:    "rate_limit_error",
	}, {
		name:       "server error",
		statusCode: 500,
		response:   `{"error": {"message": "Internal server error", "type": "server_error"}}`,
		wantErr:    "server_error",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client, err := New(Config{
				APIKey:     "test-key",
				Model:      "claude-3-opus-20240229",
				BaseURL:    server.URL,
				MaxRetries: 0,
			})
			require.NoError(t, err)

			_, err = client.Complete(context.Background(), "test")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestClient_Complete_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client, err := New(Config{
		APIKey:     "test-key",
		Model:      "claude-3-opus-20240229",
		BaseURL:    server.URL,
		Timeout:    100 * time.Millisecond,
		MaxRetries: 0,
	})
	require.NoError(t, err)

	_, err = client.Complete(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestClient_Complete_EmptyPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody ChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&reqBody)

		response := ChatCompletionResponse{
			ID:     "msg_abc123",
			Object: "message",
			Choices: []Choice{{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "",
				},
				FinishReason: "stop",
			}},
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := New(Config{
		APIKey:  "test-key",
		Model:   "claude-3-opus-20240229",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	response, err := client.Complete(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "", response)
}

func TestClient_CompleteStream_ErrorInStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		w.Write([]byte(`data: {"id":"msg_abc123","object":"message","created":1699000000,"model":"claude-3-opus-20240229","choices":[{"index":0,"delta":{"content":"Hello"}}]}` + "\n\n"))
		w.Write([]byte(`data: {"id":"msg_abc123","object":"message","created":1699000000,"model":"claude-3-opus-20240229","choices":[{"index":0,"delta":{"content":" world"}}]}` + "\n\n"))
		w.Write([]byte(`data: {"id":"msg_abc123","object":"message","created":1699000000,"model":"claude-3-opus-20240229","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n"))
	}))
	defer server.Close()

	client, err := New(Config{
		APIKey:  "test-key",
		Model:   "claude-3-opus-20240229",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	stream, err := client.CompleteStream(context.Background(), "test")
	require.NoError(t, err)

	chunks := []string{}
	for chunk := range stream {
		chunks = append(chunks, chunk)
	}

	assert.GreaterOrEqual(t, len(chunks), 2)
	result := strings.Join(chunks, "")
	assert.Equal(t, "Hello world", result)
}

func TestParseStreamChunk(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    *StreamChunk
		wantErr bool
	}{{
		name: "valid chunk",
		line: `data: {"id":"msg_abc123","object":"message","created":1699000000,"model":"claude-3-opus-20240229","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
		want: &StreamChunk{
			ID:      "msg_abc123",
			Object:  "message",
			Created: 1699000000,
			Model:   "claude-3-opus-20240229",
			Choices: []StreamChoice{{
				Index: 0,
				Delta: &Delta{
					Content: "Hello",
				},
			}},
		},
		wantErr: false,
	}, {
		name:    "done signal",
		line:    `data: [DONE]`,
		want:    nil,
		wantErr: false,
	}, {
		name:    "empty line",
		line:    "",
		want:    nil,
		wantErr: false,
	}, {
		name:    "invalid json",
		line:    `data: {invalid}`,
		want:    nil,
		wantErr: true,
	}, {
		name:    "not data line",
		line:    `event: ping`,
		want:    nil,
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := parseStreamChunk(tt.line)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, chunk)
				} else {
					assert.Equal(t, tt.want.ID, chunk.ID)
					assert.Equal(t, tt.want.Choices[0].Delta.Content, chunk.Choices[0].Delta.Content)
				}
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{{
		name:     "rate limit error",
		err:      fmt.Errorf("rate limit exceeded"),
		expected: true,
	}, {
		name:     "timeout error",
		err:      fmt.Errorf("context deadline exceeded"),
		expected: true,
	}, {
		name:     "server error",
		err:      fmt.Errorf("internal server error"),
		expected: true,
	}, {
		name:     "connection error",
		err:      fmt.Errorf("connection reset by peer"),
		expected: true,
	}, {
		name:     "invalid api key",
		err:      fmt.Errorf("invalid api key"),
		expected: false,
	}, {
		name:     "context canceled",
		err:      context.Canceled,
		expected: false,
	}, {
		name:     "nil error",
		err:      nil,
		expected: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
