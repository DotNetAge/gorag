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
type RateLimiterStepWrapper struct {
	BaseStep pipeline.Step
	limiter  *rate.Limiter
}

func WithRateLimiter(base pipeline.Step, limit rate.Limit, burst int) *RateLimiterStepWrapper {
	return &RateLimiterStepWrapper{
		BaseStep: base,
		limiter:  rate.NewLimiter(limit, burst),
	}
}

func (w *RateLimiterStepWrapper) Execute(ctx context.Context, state *pipeline.State) error {
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
	BaseStep      pipeline.Step
	FallbackStep  pipeline.Step
	options       BreakerOptions
	consecutiveErr int32
	lastErrorTime time.Time
}

func WithCircuitBreakerAndFallback(base pipeline.Step, fallback pipeline.Step, opts BreakerOptions) *CircuitBreakerStepWrapper {
	if opts.ErrorThreshold <= 0 {
		opts.ErrorThreshold = 3
	}
	return &CircuitBreakerStepWrapper{
		BaseStep:     base,
		FallbackStep: fallback,
		options:      opts,
	}
}

func (w *CircuitBreakerStepWrapper) Execute(ctx context.Context, state *pipeline.State) error {
	// Check if the circuit is "Open" (failing)
	if atomic.LoadInt32(&w.consecutiveErr) >= int32(w.options.ErrorThreshold) {
		// Check if timeout has expired to "Half-Open" and retry
		if time.Since(w.lastErrorTime) < w.options.Timeout {
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
		w.lastErrorTime = time.Now()
		atomic.AddInt32(&w.consecutiveErr, 1)
		
		// If it just broke, or we have a fallback, try fallback immediately
		if w.FallbackStep != nil {
			return w.FallbackStep.Execute(ctx, state)
		}
		return err
	}

	// Success, reset the breaker (circuit "Closed")
	atomic.StoreInt32(&w.consecutiveErr, 0)
	return nil
}
