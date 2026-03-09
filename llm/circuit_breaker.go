package llm

import (
	"context"

	"github.com/DotNetAge/gorag/circuitbreaker"
)

// CircuitBreakerClient wraps an LLM Client with circuit breaker protection
type CircuitBreakerClient struct {
	client  Client
	breaker *circuitbreaker.CircuitBreaker
}

// NewCircuitBreakerClient creates a new circuit breaker client
func NewCircuitBreakerClient(client Client, breaker *circuitbreaker.CircuitBreaker) *CircuitBreakerClient {
	return &CircuitBreakerClient{
		client:  client,
		breaker: breaker,
	}
}

// Complete generates a completion with circuit breaker protection
// Implements the Client interface
func (c *CircuitBreakerClient) Complete(ctx context.Context, prompt string) (string, error) {
	var result string
	err := c.breaker.Execute(ctx, func() error {
		var err error
		result, err = c.client.Complete(ctx, prompt)
		return err
	})
	return result, err
}

// CompleteStream generates a completion stream with circuit breaker protection
// Implements the Client interface
func (c *CircuitBreakerClient) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
	var resultChan <-chan string
	err := c.breaker.Execute(ctx, func() error {
		var err error
		resultChan, err = c.client.CompleteStream(ctx, prompt)
		return err
	})
	return resultChan, err
}
