package resilience

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"golang.org/x/time/rate"
)

// RateLimiterStepWrapper wraps any pipeline Step with a Token Bucket rate limiter.
type RateLimiterStepWrapper[T any] struct {
	BaseStep pipeline.Step[T]
	limiter  *rate.Limiter
}

// WithRateLimiter creates a new rate limiter wrapper for any step type.
func WithRateLimiter[T any](base pipeline.Step[T], limit rate.Limit, burst int) *RateLimiterStepWrapper[T] {
	return &RateLimiterStepWrapper[T]{
		BaseStep: base,
		limiter:  rate.NewLimiter(limit, burst),
	}
}

func (w *RateLimiterStepWrapper[T]) Name() string {
	return "RateLimiterStepWrapper"
}

func (w *RateLimiterStepWrapper[T]) Execute(ctx context.Context, state T) error {
	// Wait for a token before allowing the underlying step to execute
	if err := w.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter rejected request: %w", err)
	}
	return w.BaseStep.Execute(ctx, state)
}

// BreakerOptions configures the Circuit Breaker behavior.
type BreakerOptions struct {
	ErrorThreshold int
	Timeout        time.Duration
}

// CircuitBreakerStepWrapper wraps a generation/retrieval step to provide
// fast failure and elegant fallback when the primary service degrades.
type CircuitBreakerStepWrapper[T any] struct {
	BaseStep       pipeline.Step[T]
	FallbackStep   pipeline.Step[T]
	options        BreakerOptions
	consecutiveErr atomic.Int32
	lastErrorTime  atomic.Int64 // Unix nanoseconds, protected by atomic operations
}

// WithCircuitBreakerAndFallback creates a new circuit breaker wrapper for any step type.
func WithCircuitBreakerAndFallback[T any](base pipeline.Step[T], fallback pipeline.Step[T], opts BreakerOptions) *CircuitBreakerStepWrapper[T] {
	if opts.ErrorThreshold <= 0 {
		opts.ErrorThreshold = 3
	}
	return &CircuitBreakerStepWrapper[T]{
		BaseStep:     base,
		FallbackStep: fallback,
		options:      opts,
	}
}

func (w *CircuitBreakerStepWrapper[T]) Name() string {
	return "CircuitBreakerStepWrapper"
}

func (w *CircuitBreakerStepWrapper[T]) Execute(ctx context.Context, state T) error {
	// Check if the circuit is "Open" (failing)
	if w.consecutiveErr.Load() >= int32(w.options.ErrorThreshold) {
		// Check if timeout has expired to "Half-Open" and retry
		lastErrNano := w.lastErrorTime.Load()
		if lastErrNano > 0 && time.Since(time.Unix(0, lastErrNano)) < w.options.Timeout {
			// Circuit is OPEN, fast fail to fallback
			if w.FallbackStep != nil {
				return w.FallbackStep.Execute(ctx, state)
			}
			return fmt.Errorf("circuit breaker is OPEN: fast failing request")
		}
	}

	// Attempt to execute the primary step
	err := w.BaseStep.Execute(ctx, state)

	if err != nil {
		w.lastErrorTime.Store(time.Now().UnixNano())
		w.consecutiveErr.Add(1)

		// If it just broke, or we have a fallback, try fallback immediately
		if w.FallbackStep != nil {
			return w.FallbackStep.Execute(ctx, state)
		}
		return err
	}

	// Success, reset the breaker (circuit "Closed")
	w.consecutiveErr.Store(0)
	return nil
}
