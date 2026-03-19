package resilience

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"sync/atomic"
	"time"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"golang.org/x/time/rate"
)

// RateLimiterStepWrapper wraps any pipeline Step with a Token Bucket rate limiter.
type RateLimiterStepWrapper struct {
	BaseStep pipeline.Step[*core.State]
	limiter  *rate.Limiter
}

func WithRateLimiter(base pipeline.Step[*core.State], limit rate.Limit, burst int) *RateLimiterStepWrapper {
	return &RateLimiterStepWrapper{
		BaseStep: base,
		limiter:  rate.NewLimiter(limit, burst),
	}
}

func (w *RateLimiterStepWrapper) Name() string {
	return "RateLimiterStepWrapper"
}

func (w *RateLimiterStepWrapper) Execute(ctx context.Context, state *core.State) error {
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
type CircuitBreakerStepWrapper struct {
	BaseStep       pipeline.Step[*core.State]
	FallbackStep   pipeline.Step[*core.State]
	options        BreakerOptions
	consecutiveErr atomic.Int32
	lastErrorTime  atomic.Int64 // Unix nanoseconds, protected by atomic operations
}

func WithCircuitBreakerAndFallback(base pipeline.Step[*core.State], fallback pipeline.Step[*core.State], opts BreakerOptions) *CircuitBreakerStepWrapper {
	if opts.ErrorThreshold <= 0 {
		opts.ErrorThreshold = 3
	}
	return &CircuitBreakerStepWrapper{
		BaseStep:     base,
		FallbackStep: fallback,
		options:      opts,
	}
}

func (w *CircuitBreakerStepWrapper) Name() string {
	return "CircuitBreakerStepWrapper"
}

func (w *CircuitBreakerStepWrapper) Execute(ctx context.Context, state *core.State) error {
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
