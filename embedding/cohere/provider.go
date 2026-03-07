package cohere

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var (
	supportedModels = map[string]int{
		"embed-english-v3.0": 1024,
		"embed-english-light-v3.0": 1024,
		"embed-multilingual-v3.0": 1024,
		"embed-multilingual-light-v3.0": 1024,
	}
)

type Config struct {
	APIKey     string
	Model      string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
	BatchSize  int
}

type Provider struct {
	config     Config
	httpClient *http.Client
	dimension  int
}

type EmbeddingRequest struct {
	Model string   `json:"model"`
	Texts []string `json:"texts"`
	InputType string `json:"input_type,omitempty"`
	Truncate string `json:"truncate,omitempty"`
}

type EmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingResponse struct {
	ID           string           `json:"id"`
	Embeddings   []EmbeddingData  `json:"embeddings"`
	Model        string           `json:"model"`
	Texts        []string         `json:"texts"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func New(config Config) (*Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	if config.Model == "" {
		config.Model = "embed-english-v3.0"
	}

	dimension, ok := supportedModels[config.Model]
	if !ok {
		return nil, fmt.Errorf("unsupported model: %s", config.Model)
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.cohere.ai"
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.BatchSize == 0 {
		config.BatchSize = 96 // Cohere's default batch size
	}

	return &Provider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		dimension: dimension,
	}, nil
}

func (p *Provider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += p.config.BatchSize {
		end := i + p.config.BatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := p.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to embed batch: %w", err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

func (p *Provider) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		embeddings, err := p.doEmbed(ctx, texts)
		if err == nil {
			return embeddings, nil
		}

		lastErr = err
		if !isRetryableError(err) {
			break
		}
	}

	return nil, lastErr
}

func (p *Provider) doEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := EmbeddingRequest{
		Model:     p.config.Model,
		Texts:     texts,
		InputType: "search_query",
		Truncate:  "END",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/embed", p.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("Cohere-Version", "2022-12-06")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("error %d: %s", errResp.Code, errResp.Message)
		}
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var respData EmbeddingResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	embeddings := make([][]float32, len(respData.Embeddings))
	for _, data := range respData.Embeddings {
		if data.Index < len(embeddings) {
			embeddings[data.Index] = data.Embedding
		}
	}

	return embeddings, nil
}

func (p *Provider) Dimension() int {
	return p.dimension
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return contains(errStr, "rate limit") ||
		contains(errStr, "timeout") ||
		contains(errStr, "server error") ||
		contains(errStr, "connection") ||
		contains(errStr, "500") ||
		contains(errStr, "502") ||
		contains(errStr, "503")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
