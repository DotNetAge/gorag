package resilience

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// MockStep is a mock implementation of pipeline.Step[*entity.PipelineState]
type MockStep struct {
	name     string
	executeFn func(ctx context.Context, state *entity.PipelineState) error
}

func (m *MockStep) Name() string {
	return m.name
}

func (m *MockStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if m.executeFn != nil {
		return m.executeFn(ctx, state)
	}
	return nil
}

func TestRateLimiterStepWrapper(t *testing.T) {
	// Create a mock step
	mockStep := &MockStep{
		name: "MockStep",
	}

	// Create the rate limiter wrapper
	wrapper := WithRateLimiter(mockStep, 1, 1)

	// Test that the wrapper implements the Step interface
	var _ pipeline.Step[*entity.PipelineState] = wrapper

	// Test Name method
	if wrapper.Name() != "RateLimiterStepWrapper" {
		t.Errorf("Expected name 'RateLimiterStepWrapper', got '%s'", wrapper.Name())
	}

	// Test Execute method (basic functionality)
	ctx := context.Background()
	state := entity.NewPipelineState()

	err := wrapper.Execute(ctx, state)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	// Test rate limiting
	time.Sleep(time.Second) // Wait for the rate limiter to reset
	err = wrapper.Execute(ctx, state)
	if err != nil {
		t.Errorf("Expected no error after waiting, got '%v'", err)
	}
}

func TestCircuitBreakerStepWrapper(t *testing.T) {
	// Create mock steps
	mockStep := &MockStep{
		name: "MockStep",
		executeFn: func(ctx context.Context, state *entity.PipelineState) error {
			return nil
		},
	}

	fallbackStep := &MockStep{
		name: "FallbackStep",
	}

	// Create circuit breaker options
	opts := BreakerOptions{
		ErrorThreshold: 3,
		Timeout:        time.Second,
	}

	// Create the circuit breaker wrapper
	wrapper := WithCircuitBreakerAndFallback(mockStep, fallbackStep, opts)

	// Test that the wrapper implements the Step interface
	var _ pipeline.Step[*entity.PipelineState] = wrapper

	// Test Name method
	if wrapper.Name() != "CircuitBreakerStepWrapper" {
		t.Errorf("Expected name 'CircuitBreakerStepWrapper', got '%s'", wrapper.Name())
	}

	// Test Execute method with successful execution
	ctx := context.Background()
	state := entity.NewPipelineState()

	err := wrapper.Execute(ctx, state)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	// Test with failing step
	failingStep := &MockStep{
		name: "FailingStep",
		executeFn: func(ctx context.Context, state *entity.PipelineState) error {
			return fmt.Errorf("test error")
		},
	}

	// Create a circuit breaker without fallback to test error counting
	failingWrapper := WithCircuitBreakerAndFallback(failingStep, nil, opts)

	// Execute multiple times to trip the circuit
	for i := 0; i < 3; i++ {
		err := failingWrapper.Execute(ctx, state)
		if err == nil {
			t.Errorf("Expected error for failing step, got nil")
		}
	}

	// Test that circuit is open (should return error since no fallback)
	err = failingWrapper.Execute(ctx, state)
	if err == nil {
		t.Errorf("Expected error when circuit is open, got nil")
	}
}
