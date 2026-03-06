package llm

import (
	"context"
)

// Client defines the interface for LLM clients
type Client interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteStream(ctx context.Context, prompt string) (<-chan string, error)
}
