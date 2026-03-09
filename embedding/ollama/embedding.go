package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/DotNetAge/gorag/internal/retry"
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
		if !retry.IsRetryableError(err) {
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
	// Use a semaphore to limit concurrent requests
	maxConcurrent := 5
	sem := make(chan struct{}, maxConcurrent)
	var mu sync.Mutex
	var allEmbeddings [][]float32
	var firstErr error

	var wg sync.WaitGroup
	for i, text := range texts {
		wg.Add(1)
		go func(index int, txt string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				mu.Unlock()
				return
			}

			reqBody := EmbeddingRequest{
				Model: p.config.Model,
				Input: txt,
			}

			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to marshal request: %w", err)
				}
				mu.Unlock()
				return
			}

			url := fmt.Sprintf("%s/api/embed", p.config.BaseURL)
			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to create request: %w", err)
				}
				mu.Unlock()
				return
			}

			req.Header.Set("Content-Type", "application/json")

			resp, err := p.httpClient.Do(req)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to send request: %w", err)
				}
				mu.Unlock()
				return
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to read response: %w", err)
				}
				mu.Unlock()
				return
			}

			if resp.StatusCode != http.StatusOK {
				var errResp ErrorResponse
				if err := json.Unmarshal(body, &errResp); err == nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("error: %s", errResp.Error)
					}
					mu.Unlock()
					return
				}

				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
				}
				mu.Unlock()
				return
			}

			// Check if response is empty
			if len(body) == 0 {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("empty response body")
				}
				mu.Unlock()
				return
			}

			var respData EmbeddingResponse
			if err := json.Unmarshal(body, &respData); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to unmarshal response: %w, response: %s", err, string(body))
				}
				mu.Unlock()
				return
			}

			// Check if embeddings array is empty
			if len(respData.Embeddings) == 0 {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("no embeddings returned in response: %s", string(body))
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			// Ensure allEmbeddings has enough capacity
			for len(allEmbeddings) <= index {
				allEmbeddings = append(allEmbeddings, nil)
			}
			allEmbeddings[index] = respData.Embeddings[0]
			mu.Unlock()
		}(i, text)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return allEmbeddings, nil
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
