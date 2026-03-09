package azureopenai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DotNetAge/gorag/internal/retry"
)

// Config represents the configuration for Azure OpenAI client
type Config struct {
	APIKey     string
	Model      string
	Endpoint   string
	APIVersion string
	Timeout    time.Duration
	MaxRetries int
	Temperature float64
	MaxTokens  int
}

// Client represents the Azure OpenAI client
type Client struct {
	config     Config
	httpClient *http.Client
}

// Message represents a message in the chat completion request
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents the request for chat completion
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Choice represents a choice in the chat completion response
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ChatCompletionResponse represents the response from chat completion
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Usage represents the usage information in the response
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Delta represents a delta in the stream response
type Delta struct {
	Content string `json:"content,omitempty"`
}

// StreamChoice represents a choice in the stream response
type StreamChoice struct {
	Index        int    `json:"index"`
	Delta        *Delta `json:"delta,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// StreamChunk represents a chunk in the stream response
type StreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// Error represents an error in the response
type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error Error `json:"error"`
}

// New creates a new Azure OpenAI client
func New(config Config) (*Client, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	if config.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if config.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	if config.APIVersion == "" {
		config.APIVersion = "2024-03-01-preview"
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.Temperature == 0 {
		config.Temperature = 0.7
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// Complete generates a completion for the given prompt
func (c *Client) Complete(ctx context.Context, prompt string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		response, err := c.doComplete(ctx, prompt, false)
		if err == nil {
			if len(response.Choices) > 0 {
				return response.Choices[0].Message.Content, nil
			}
			return "", nil
		}

		lastErr = err
		if !retry.IsRetryableError(err) {
			break
		}
	}

	return "", lastErr
}

// CompleteStream generates a streaming completion for the given prompt
func (c *Client) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
	reqBody := ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
		Stream:      true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		c.config.Endpoint, c.config.Model, c.config.APIVersion)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
		}

		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan string, 10)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			chunk, err := parseStreamChunk(line)
			if err != nil {
				select {
				case ch <- "ERROR: " + err.Error():
				case <-ctx.Done():
				}
				return
			}

			if chunk != nil && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
				select {
				case ch <- chunk.Choices[0].Delta.Content:
				case <-ctx.Done():
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case ch <- "ERROR: " + err.Error():
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// doComplete performs the actual completion request
func (c *Client) doComplete(ctx context.Context, prompt string, stream bool) (*ChatCompletionResponse, error) {
	reqBody := ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
		Stream:      stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		c.config.Endpoint, c.config.Model, c.config.APIVersion)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
		}

		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var respData ChatCompletionResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &respData, nil
}

// readStream reads the stream response
func readStream(body io.Reader) ([]StreamChunk, error) {
	scanner := bufio.NewScanner(body)
	var chunks []StreamChunk

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		chunk, err := parseStreamChunk(line)
		if err != nil {
			return nil, err
		}

		if chunk != nil {
			chunks = append(chunks, *chunk)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read stream: %w", err)
	}

	return chunks, nil
}

// parseStreamChunk parses a stream chunk
func parseStreamChunk(line string) (*StreamChunk, error) {
	if !strings.HasPrefix(line, "data: ") {
		return nil, nil
	}

	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return nil, nil
	}

	var chunk StreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chunk: %w", err)
	}

	return &chunk, nil
}
