package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Config struct {
	Model      string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
	Dimension  int
}

type Provider struct {
	config     Config
	httpClient *http.Client
}

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func New(config Config) (*Provider, error) {
	if config.Model == "" {
		config.Model = "qwen3-embedding:0.6b"
	}

	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}

	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.Dimension == 0 {
		// Default dimension for common embedding models
		config.Dimension = 1536
	}

	return &Provider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

func (p *Provider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	var lastErr error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		response, err := p.doEmbed(ctx, texts)
		if err == nil {
			return response, nil
		}

		lastErr = err
		if !isRetryableError(err) {
			break
		}
	}

	return nil, lastErr
}

func (p *Provider) Dimension() int {
	return p.config.Dimension
}

func (p *Provider) doEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	// For Ollama, we need to handle each text individually
	// because the Ollama API doesn't support batch embedding
	var allEmbeddings [][]float32

	for _, text := range texts {
		reqBody := EmbeddingRequest{
			Model: p.config.Model,
			Input: text,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		url := fmt.Sprintf("%s/api/embed", p.config.BaseURL)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			var errResp ErrorResponse
			if err := json.Unmarshal(body, &errResp); err == nil {
				return nil, fmt.Errorf("error: %s", errResp.Error)
			}

			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}

		// Check if response is empty
		if len(body) == 0 {
			return nil, fmt.Errorf("empty response body")
		}

		var respData EmbeddingResponse
		if err := json.Unmarshal(body, &respData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w, response: %s", err, string(body))
		}

		// Check if embeddings array is empty
		if len(respData.Embeddings) == 0 {
			return nil, fmt.Errorf("no embeddings returned in response: %s", string(body))
		}

		allEmbeddings = append(allEmbeddings, respData.Embeddings...)
	}

	return allEmbeddings, nil
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return contains(errStr, "rate limit") ||
		contains(errStr, "timeout") ||
		contains(errStr, "deadline exceeded") ||
		contains(errStr, "server error") ||
		contains(errStr, "connection")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
