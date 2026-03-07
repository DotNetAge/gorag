package llm

import (
	"context"
)

// Client defines the interface for LLM clients
//
// This interface is implemented by all LLM providers
// (OpenAI, Anthropic, Azure OpenAI, Ollama, Compatible)
// and allows the RAG engine to generate responses using LLMs.
//
// Example implementation:
//
//     type OpenAIClient struct {
//         client *openai.Client
//         config Config
//     }
//
//     func (c *OpenAIClient) Complete(ctx context.Context, prompt string) (string, error) {
//         // Call OpenAI API to generate completion
//     }
//
//     func (c *OpenAIClient) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
//         // Call OpenAI API for streaming completion
//     }
type Client interface {
	// Complete generates a completion for the given prompt
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - prompt: Prompt text to generate completion for
	//
	// Returns:
	// - string: Generated completion
	// - error: Error if completion generation fails
	Complete(ctx context.Context, prompt string) (string, error)

	// CompleteStream generates a completion for the given prompt and returns a channel for streaming responses
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - prompt: Prompt text to generate completion for
	//
	// Returns:
	// - <-chan string: Channel for streaming completion chunks
	// - error: Error if streaming completion fails
	CompleteStream(ctx context.Context, prompt string) (<-chan string, error)
}

// Config defines common configuration for LLM clients
//
// This struct contains configuration options that are common
// across all LLM providers.
//
// Example:
//
//     config := Config{
//         APIKey:      "your-api-key",
//         Model:       "gpt-4",
//         Temperature: 0.7,
//         MaxTokens:   1000,
//     }
type Config struct {
	APIKey      string  // API key for the LLM provider
	Model       string  // Model name to use
	BaseURL     string  // Base URL for API requests (optional)
	Temperature float64 // Temperature for generation (0.0-1.0)
	MaxTokens   int     // Maximum tokens to generate
}
