// Package circuitbreaker implements the circuit breaker pattern
//
// This package provides a circuit breaker implementation to protect
// services from overloading and to improve system resilience by
// temporarily stopping requests to failing services.
//
// The circuit breaker has three states:
// - Closed: All requests are allowed
// - Open: No requests are allowed, returns an error immediately
// - Half-Open: A limited number of requests are allowed to test if the service has recovered
//
// Example:
//
//     cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
//     
//     err := cb.Execute(ctx, func() error {
//         // Call external service
//         return externalService.Call()
//     })
//     
//     if err != nil {
//         // Handle error (may be circuit breaker error)
//     }
package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State represents the state of the circuit breaker
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures the circuit breaker
//
// This struct defines the configuration parameters for the circuit breaker,
// including failure threshold, timeout, and half-open state limits.
//
// Example:
//
//     config := Config{
//         MaxFailures:  10,  // Open circuit after 10 failures
//         Timeout:      1 * time.Minute, // Wait 1 minute before trying again
//         HalfOpenMax:  5,   // Allow 5 requests in half-open state
//     }
type Config struct {
	MaxFailures  int
	Timeout      time.Duration
	HalfOpenMax  int
}

// DefaultConfig returns a default configuration
//
// Returns a configuration with sensible defaults:
// - MaxFailures: 5
// - Timeout: 30 seconds
// - HalfOpenMax: 3
func DefaultConfig() Config {
	return Config{
		MaxFailures:  5,
		Timeout:      30 * time.Second,
		HalfOpenMax:  3,
	}
}

// CircuitBreaker implements the circuit breaker pattern
//
// The circuit breaker protects services from overloading by temporarily
// stopping requests to failing services. It has three states:
// - Closed: All requests are allowed
// - Open: No requests are allowed, returns an error immediately
// - Half-Open: A limited number of requests are allowed to test if the service has recovered
//
// Example:
//
//     cb := New(DefaultConfig())
//     
//     err := cb.Execute(ctx, func() error {
//         // Call external service
//         return externalService.Call()
//     })
//     
//     if err != nil {
//         // Handle error (may be circuit breaker error)
//     }
type CircuitBreaker struct {
	config      Config
	state       State
	failures    int
	successes   int
	lastFailure time.Time
	mu          sync.RWMutex
}

// New creates a new circuit breaker
//
// Parameters:
// - config: Configuration for the circuit breaker
//
// Returns:
// - *CircuitBreaker: New circuit breaker instance
func New(config Config) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute executes the given function with circuit breaker protection
//
// This method wraps the execution of a function with circuit breaker logic:
// 1. Checks if execution is allowed based on the current state
// 2. Executes the function
// 3. Records the result and updates the circuit breaker state
//
// Parameters:
// - ctx: Context for cancellation
// - fn: Function to execute
//
// Returns:
// - error: Error from the function or circuit breaker error
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if err := cb.canExecute(); err != nil {
		return err
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// canExecute checks if execution is allowed
func (cb *CircuitBreaker) canExecute() error {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.failures = 0
			cb.successes = 0
			cb.mu.Unlock()
			cb.mu.RLock()
			return nil
		}
		return errors.New("circuit breaker is open")
	case StateHalfOpen:
		if cb.successes >= cb.config.HalfOpenMax {
			return errors.New("circuit breaker half-open limit reached")
		}
		return nil
	default:
		return nil
	}
}

// recordResult records the result of an execution
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		switch cb.state {
		case StateClosed:
			if cb.failures >= cb.config.MaxFailures {
				cb.state = StateOpen
			}
		case StateHalfOpen:
			cb.state = StateOpen
		}
	} else {
		switch cb.state {
		case StateHalfOpen:
			cb.successes++
			if cb.successes >= cb.config.HalfOpenMax {
				cb.state = StateClosed
				cb.failures = 0
				cb.successes = 0
			}
		case StateClosed:
			cb.failures = 0
		}
	}
}

// State returns the current state
//
// Returns:
// - State: Current circuit breaker state (Closed, Open, or HalfOpen)
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state
//
// This method resets the circuit breaker to its initial closed state,
// clearing all failure and success counters.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
}
