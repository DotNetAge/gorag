package llmutil

import (
	"context"

	"github.com/DotNetAge/gorag/circuitbreaker"
	gochatcore "github.com/DotNetAge/gochat/pkg/core"
)

// CircuitBreakerClient wraps an LLM Client with circuit breaker protection
type CircuitBreakerClient struct {
	client  gochatcore.Client
	breaker *circuitbreaker.CircuitBreaker
}

// NewCircuitBreakerClient creates a new circuit breaker client
func NewCircuitBreakerClient(client gochatcore.Client, breaker *circuitbreaker.CircuitBreaker) *CircuitBreakerClient {
	return &CircuitBreakerClient{
		client:  client,
		breaker: breaker,
	}
}

// Chat generates a completion with circuit breaker protection
func (c *CircuitBreakerClient) Chat(ctx context.Context, messages []gochatcore.Message, opts ...gochatcore.Option) (*gochatcore.Response, error) {
	var result *gochatcore.Response
	err := c.breaker.Execute(ctx, func() error {
		var err error
		result, err = c.client.Chat(ctx, messages, opts...)
		return err
	})
	return result, err
}

// ChatStream generates a completion stream with circuit breaker protection
func (c *CircuitBreakerClient) ChatStream(ctx context.Context, messages []gochatcore.Message, opts ...gochatcore.Option) (*gochatcore.Stream, error) {
	var resultStream *gochatcore.Stream
	err := c.breaker.Execute(ctx, func() error {
		var err error
		resultStream, err = c.client.ChatStream(ctx, messages, opts...)
		return err
	})
	return resultStream, err
}
