package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	// Create circuit breaker
	breaker := New(Config{
		MaxFailures: 2,
		Timeout:     100 * time.Millisecond,
		HalfOpenMax: 1,
	})

	ctx := context.Background()

	// Test 1: Initially closed
	state := breaker.State()
	if state != StateClosed {
		t.Errorf("Expected initial state to be StateClosed, got %v", state)
	}

	// Test 2: Trip the breaker (2 failures)
	err := breaker.Execute(ctx, func() error {
		return errors.New("test error")
	})
	if err == nil {
		t.Error("Expected error from first failure")
	}

	err = breaker.Execute(ctx, func() error {
		return errors.New("test error")
	})
	if err == nil {
		t.Error("Expected error from second failure")
	}

	// Check if breaker is open
	state = breaker.State()
	if state != StateOpen {
		t.Errorf("Expected state to be StateOpen after 2 failures, got %v", state)
	}

	// Test 3: Try to execute while open (should fail fast)
	err = breaker.Execute(ctx, func() error {
		return nil
	})
	if err == nil {
		t.Error("Expected error from open circuit")
	}

	// Test 4: Wait for timeout and check half-open
	time.Sleep(150 * time.Millisecond)
	// Execute a request to trigger state transition
	err = breaker.Execute(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected success after timeout, got error: %v", err)
	}
	state = breaker.State()
	if state != StateHalfOpen && state != StateClosed {
		t.Errorf("Expected state to be StateHalfOpen or StateClosed after timeout and success, got %v", state)
	}

	// Test 5: Success in half-open should close the circuit
	err = breaker.Execute(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected success in half-open state, got error: %v", err)
	}

	state = breaker.State()
	if state != StateClosed {
		t.Errorf("Expected state to be StateClosed after success, got %v", state)
	}

	// Test 7: Reset
	breaker.Reset()
	state = breaker.State()
	if state != StateClosed {
		t.Errorf("Expected state to be StateClosed after reset, got %v", state)
	}
}

func TestCircuitBreakerWithSuccesses(t *testing.T) {
	// Create circuit breaker
	breaker := New(Config{
		MaxFailures: 2,
		Timeout:     100 * time.Millisecond,
		HalfOpenMax: 1,
	})

	ctx := context.Background()

	// Test multiple successes
	for i := 0; i < 5; i++ {
		err := breaker.Execute(ctx, func() error {
			return nil
		})
		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}
	}

	// State should still be closed
	state := breaker.State()
	if state != StateClosed {
		t.Errorf("Expected state to be StateClosed after multiple successes, got %v", state)
	}
}
