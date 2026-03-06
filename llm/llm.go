package llm

import (
	"context"
)

// Client defines the interface for LLM clients
type Client interface {
	// Complete generates a completion for the given prompt
	Complete(ctx context.Context, prompt string) (string, error)

	// CompleteStream generates a completion for the given prompt and returns a channel for streaming responses
	CompleteStream(ctx context.Context, prompt string) (<-chan string, error)
}

// Config defines common configuration for LLM clients
type Config struct {
	APIKey      string
	Model       string
	BaseURL     string
	Temperature float64
	MaxTokens   int
}
