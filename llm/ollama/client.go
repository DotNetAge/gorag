package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/DotNetAge/gorag/internal/retry"
)

type Config struct {
	Model       string
	BaseURL     string
	Timeout     time.Duration
	MaxRetries  int
	Temperature float64
	MaxTokens   int
}

type Client struct {
	config     Config
	httpClient *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type ChatCompletionResponse struct {
	Model              string  `json:"model"`
	CreatedAt          string  `json:"created_at"`
	Message            Message `json:"message"`
	Done               bool    `json:"done"`
	DoneReason         string  `json:"done_reason"`
	TotalDuration      int64   `json:"total_duration"`
	LoadDuration       int64   `json:"load_duration"`
	PromptEvalCount    int     `json:"prompt_eval_count"`
	PromptEvalDuration int64   `json:"prompt_eval_duration"`
	EvalCount          int     `json:"eval_count"`
	EvalDuration       int64   `json:"eval_duration"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Delta struct {
	Content string `json:"content,omitempty"`
}

type StreamChoice struct {
	Index        int    `json:"index"`
	Delta        *Delta `json:"delta,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
}

type StreamChunk struct {
	Model              string   `json:"model"`
	CreatedAt          string   `json:"created_at"`
	Message            *Message `json:"message,omitempty"`
	Delta              *Delta   `json:"delta,omitempty"`
	Done               bool     `json:"done"`
	DoneReason         string   `json:"done_reason,omitempty"`
	TotalDuration      int64    `json:"total_duration,omitempty"`
	LoadDuration       int64    `json:"load_duration,omitempty"`
	PromptEvalCount    int      `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64    `json:"prompt_eval_duration,omitempty"`
	EvalCount          int      `json:"eval_count,omitempty"`
	EvalDuration       int64    `json:"eval_duration,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func New(config Config) (*Client, error) {
	if config.Model == "" {
		config.Model = "llama2"
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
			return response.Message.Content, nil
		}

		lastErr = err
		if !retry.IsRetryableError(err) {
			break
		}
	}

	return "", lastErr
}

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

	url := fmt.Sprintf("%s/api/chat", c.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
			return nil, fmt.Errorf("error: %s", errResp.Error)
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

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				select {
				case ch <- "ERROR: " + err.Error():
				case <-ctx.Done():
				}
				return
			}

			if chunk.Delta != nil && chunk.Delta.Content != "" {
				select {
				case ch <- chunk.Delta.Content:
				case <-ctx.Done():
					return
				}
			} else if chunk.Message != nil && chunk.Message.Content != "" {
				select {
				case ch <- chunk.Message.Content:
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

	url := fmt.Sprintf("%s/api/chat", c.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
			return nil, fmt.Errorf("error: %s", errResp.Error)
		}

		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	defer resp.Body.Close()

	// Read response line by line
	scanner := bufio.NewScanner(resp.Body)
	var fullContent string
	var lastChunk StreamChunk

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return nil, fmt.Errorf("failed to unmarshal chunk: %w", err)
		}

		lastChunk = chunk

		if chunk.Delta != nil && chunk.Delta.Content != "" {
			fullContent += chunk.Delta.Content
		} else if chunk.Message != nil && chunk.Message.Content != "" {
			fullContent += chunk.Message.Content
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Create final response
	response := &ChatCompletionResponse{
		Model:     lastChunk.Model,
		CreatedAt: lastChunk.CreatedAt,
		Message: Message{
			Role:    "assistant",
			Content: fullContent,
		},
		Done:               lastChunk.Done,
		DoneReason:         lastChunk.DoneReason,
		TotalDuration:      lastChunk.TotalDuration,
		LoadDuration:       lastChunk.LoadDuration,
		PromptEvalCount:    lastChunk.PromptEvalCount,
		PromptEvalDuration: lastChunk.PromptEvalDuration,
		EvalCount:          lastChunk.EvalCount,
		EvalDuration:       lastChunk.EvalDuration,
	}

	return response, nil
}
