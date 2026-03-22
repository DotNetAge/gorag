package resilience

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

type mockStep struct {
	executeErr error
	executed   bool
}

func (m *mockStep) Name() string {
	return "MockStep"
}

func (m *mockStep) Execute(ctx context.Context, state *core.RetrievalContext) error {
	m.executed = true
	return m.executeErr
}

func TestRateLimiterWrapper_Name(t *testing.T) {
	step := &mockStep{}
	wrapper := WithRateLimiter(step, rate.Limit(10), 5)
	assert.Equal(t, "RateLimiterStepWrapper", wrapper.Name())
}

func TestRateLimiterWrapper_Execute_Success(t *testing.T) {
	step := &mockStep{}
	wrapper := WithRateLimiter(step, rate.Limit(100), 10)
	ctx := context.Background()
	state := &core.RetrievalContext{}

	err := wrapper.Execute(ctx, state)

	assert.NoError(t, err)
	assert.True(t, step.executed)
}

func TestRateLimiterWrapper_Execute_WithContextCancellation(t *testing.T) {
	step := &mockStep{}
	wrapper := WithRateLimiter(step, rate.Limit(0.001), 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state := &core.RetrievalContext{}

	err := wrapper.Execute(ctx, state)

	assert.Error(t, err)
}

func TestCircuitBreakerWrapper_Name(t *testing.T) {
	step := &mockStep{}
	wrapper := WithCircuitBreakerAndFallback(step, nil, BreakerOptions{})
	assert.Equal(t, "CircuitBreakerStepWrapper", wrapper.Name())
}

func TestCircuitBreakerWrapper_Execute_Success(t *testing.T) {
	step := &mockStep{}
	wrapper := WithCircuitBreakerAndFallback(step, nil, BreakerOptions{ErrorThreshold: 3, Timeout: 0})
	ctx := context.Background()
	state := &core.RetrievalContext{}

	err := wrapper.Execute(ctx, state)

	assert.NoError(t, err)
	assert.True(t, step.executed)
}

func TestCircuitBreakerWrapper_Execute_ErrorWithFallback(t *testing.T) {
	fallback := &mockStep{}
	step := &mockStep{executeErr: errors.New("primary error")}
	wrapper := WithCircuitBreakerAndFallback(step, fallback, BreakerOptions{ErrorThreshold: 3, Timeout: 0})
	ctx := context.Background()
	state := &core.RetrievalContext{}

	err := wrapper.Execute(ctx, state)

	assert.NoError(t, err)
	assert.True(t, step.executed)
	assert.True(t, fallback.executed)
}

func TestCircuitBreakerWrapper_Execute_ErrorThresholdReached(t *testing.T) {
	step := &mockStep{executeErr: errors.New("error")}
	wrapper := WithCircuitBreakerAndFallback(step, nil, BreakerOptions{ErrorThreshold: 3, Timeout: 0})
	ctx := context.Background()
	state := &core.RetrievalContext{}

	for i := 0; i < 3; i++ {
		wrapper.Execute(ctx, state)
	}

	assert.GreaterOrEqual(t, wrapper.consecutiveErr.Load(), int32(3))
}

func TestCircuitBreakerWrapper_Execute_ResetOnSuccess(t *testing.T) {
	step := &mockStep{}
	wrapper := WithCircuitBreakerAndFallback(step, nil, BreakerOptions{ErrorThreshold: 3, Timeout: 0})
	ctx := context.Background()
	state := &core.RetrievalContext{}

	wrapper.Execute(ctx, state)
	assert.Equal(t, int32(0), wrapper.consecutiveErr.Load())

	step.executeErr = errors.New("error")
	wrapper.Execute(ctx, state)
	wrapper.Execute(ctx, state)
	assert.GreaterOrEqual(t, wrapper.consecutiveErr.Load(), int32(2))

	step.executeErr = nil
	err := wrapper.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), wrapper.consecutiveErr.Load())
}

func TestCircuitBreakerWrapper_DefaultErrorThreshold(t *testing.T) {
	step := &mockStep{}
	wrapper := WithCircuitBreakerAndFallback(step, nil, BreakerOptions{})
	assert.Equal(t, 3, wrapper.options.ErrorThreshold)
}

func TestCircuitBreakerWrapper_CircuitOpenFastFail(t *testing.T) {
	step := &mockStep{executeErr: errors.New("error")}
	wrapper := WithCircuitBreakerAndFallback(step, nil, BreakerOptions{ErrorThreshold: 1, Timeout: 0})
	ctx := context.Background()
	state := &core.RetrievalContext{}

	err := wrapper.Execute(ctx, state)
	assert.Error(t, err)

	_ = state
}
