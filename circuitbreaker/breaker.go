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
type Config struct {
	MaxFailures  int
	Timeout      time.Duration
	HalfOpenMax  int
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		MaxFailures:  5,
		Timeout:      30 * time.Second,
		HalfOpenMax:  3,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config      Config
	state       State
	failures    int
	successes   int
	lastFailure time.Time
	mu          sync.RWMutex
}

// New creates a new circuit breaker
func New(config Config) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute executes the given function with circuit breaker protection
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
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
}
