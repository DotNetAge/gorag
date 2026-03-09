package anthropic

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

// Config configures the Anthropic client
type Config struct {
	APIKey      string
	Model       string
	BaseURL     string
	Timeout     time.Duration
	MaxRetries  int
	Temperature float64
	MaxTokens   int
}

// Client implements an LLM client for Anthropic
type Client struct {
	config     Config
	httpClient *http.Client
}

// Message represents a message in the conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents a chat completion request
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

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Usage represents usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Delta represents a delta in the streaming response
type Delta struct {
	Content string `json:"content,omitempty"`
}

// StreamChoice represents a choice in the streaming response
type StreamChoice struct {
	Index        int    `json:"index"`
	Delta        *Delta `json:"delta,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// StreamChunk represents a chunk in the streaming response
type StreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// Error represents an error response
type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error Error `json:"error"`
}

// New creates a new Anthropic client
func New(config Config) (*Client, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	if config.Model == "" {
		config.Model = "claude-3-opus-20240229"
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.anthropic.com"
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

// Complete performs a non-streaming completion
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

// CompleteStream performs a streaming completion
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

	url := fmt.Sprintf("%s/v1/messages", c.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
			return nil, fmt.Errorf("%s: %s", errResp.Error.Type, errResp.Error.Message)
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

// doComplete performs a completion request
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

	url := fmt.Sprintf("%s/v1/messages", c.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
			return nil, fmt.Errorf("%s: %s", errResp.Error.Type, errResp.Error.Message)
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

// readStream reads the streaming response
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
