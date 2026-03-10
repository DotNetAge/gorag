// Package degradation implements graceful degradation for GoRAG
//
// This package provides functionality for graceful degradation of services
// when they encounter errors or high latency. It allows services to
// automatically switch to degraded modes with reduced functionality
// to maintain availability.
//
// Degradation levels:
// - LevelNormal: Full functionality
// - LevelReducedFeatures: Reduced feature set
// - LevelMinimal: Minimal essential functionality
// - LevelFallback: Fallback to basic functionality
//
// Example:
//
//     manager := degradation.New(degradation.DefaultConfig())
//     
//     err := manager.Execute(ctx, 
//         func() error { return normalOperation() },
//         map[degradation.Level]func() error{
//             degradation.LevelReducedFeatures: func() error { return reducedOperation() },
//             degradation.LevelMinimal: func() error { return minimalOperation() },
//             degradation.LevelFallback: func() error { return fallbackOperation() },
//         },
//     )
package degradation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Level represents the degradation level
type Level int

const (
	LevelNormal Level = iota
	LevelReducedFeatures
	LevelMinimal
	LevelFallback
)

func (l Level) String() string {
	switch l {
	case LevelNormal:
		return "normal"
	case LevelReducedFeatures:
		return "reduced-features"
	case LevelMinimal:
		return "minimal"
	case LevelFallback:
		return "fallback"
	default:
		return "unknown"
	}
}

// Config configures graceful degradation
//
// This struct defines the configuration parameters for the degradation manager,
// including which degradation levels to enable and thresholds for triggering degradation.
//
// Example:
//
//     config := Config{
//         EnableReducedFeatures: true,
//         EnableMinimal:         true,
//         EnableFallback:        true,
//         ErrorThreshold:        10,  // Trigger after 10 errors
//         LatencyThreshold:      2 * time.Second, // Treat latency > 2s as error
//     }
type Config struct {
	EnableReducedFeatures bool          // Enable reduced features degradation level
	EnableMinimal         bool          // Enable minimal functionality degradation level
	EnableFallback        bool          // Enable fallback functionality degradation level
	ErrorThreshold        int           // Number of errors before triggering degradation
	LatencyThreshold      time.Duration // Latency threshold to treat as error
}

// DefaultConfig returns default configuration
//
// Returns a configuration with sensible defaults:
// - EnableReducedFeatures: true
// - EnableMinimal: true
// - EnableFallback: true
// - ErrorThreshold: 5
// - LatencyThreshold: 5 seconds
func DefaultConfig() Config {
	return Config{
		EnableReducedFeatures: true,
		EnableMinimal:         true,
		EnableFallback:        true,
		ErrorThreshold:        5,
		LatencyThreshold:      5 * time.Second,
	}
}

// DegradationManager manages graceful degradation
//
// The DegradationManager monitors errors and latency to automatically
// adjust the service degradation level. It supports multiple degradation levels
// and can execute functions with fallback options for each level.
//
// Example:
//
//     manager := New(DefaultConfig())
//     
//     err := manager.Execute(ctx, 
//         func() error { return normalOperation() },
//         map[Level]func() error{
//             LevelReducedFeatures: func() error { return reducedOperation() },
//             LevelMinimal: func() error { return minimalOperation() },
//             LevelFallback: func() error { return fallbackOperation() },
//         },
//     )
type DegradationManager struct {
	config       Config
	currentLevel Level
	errorCount   int
	mu           sync.RWMutex
	lastError    time.Time
}

// New creates a new degradation manager
//
// Parameters:
// - config: Configuration for the degradation manager
//
// Returns:
// - *DegradationManager: New degradation manager instance
func New(config Config) *DegradationManager {
	return &DegradationManager{
		config:       config,
		currentLevel: LevelNormal,
	}
}

// RecordError records an error and potentially triggers degradation
//
// This method records an error and checks if degradation should be triggered
// based on the error threshold and current degradation level.
//
// Parameters:
// - err: Error to record
func (dm *DegradationManager) RecordError(err error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.errorCount++
	dm.lastError = time.Now()

	// Check if we should degrade
	switch dm.currentLevel {
	case LevelNormal:
		if dm.errorCount >= dm.config.ErrorThreshold && dm.config.EnableReducedFeatures {
			dm.currentLevel = LevelReducedFeatures
		}
	case LevelReducedFeatures:
		if dm.errorCount >= dm.config.ErrorThreshold*2 && dm.config.EnableMinimal {
			dm.currentLevel = LevelMinimal
		}
	case LevelMinimal:
		if dm.errorCount >= dm.config.ErrorThreshold*3 && dm.config.EnableFallback {
			dm.currentLevel = LevelFallback
		}
	}
}

// RecordLatency records a latency measurement
//
// This method records a latency measurement and treats high latency
// (exceeding the configured threshold) as an error.
//
// Parameters:
// - duration: Duration to record
func (dm *DegradationManager) RecordLatency(duration time.Duration) {
	if duration > dm.config.LatencyThreshold {
		// Treat high latency as an error
		dm.RecordError(errors.New("high latency"))
	}
}

// GetLevel returns the current degradation level
//
// Returns:
// - Level: Current degradation level
func (dm *DegradationManager) GetLevel() Level {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentLevel
}

// Reset resets the degradation level to normal
//
// This method resets the degradation manager to its initial state,
// setting the degradation level back to normal and clearing the error count.
func (dm *DegradationManager) Reset() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.currentLevel = LevelNormal
	dm.errorCount = 0
}

// ShouldDegrade checks if a feature should be degraded
//
// This method checks if a feature with the given degradation level
// should be degraded based on the current degradation state.
//
// Parameters:
// - featureLevel: Degradation level of the feature
//
// Returns:
// - bool: True if the feature should be degraded
func (dm *DegradationManager) ShouldDegrade(featureLevel Level) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentLevel >= featureLevel
}

// Execute executes a function with degradation handling
//
// This method executes a function with degradation handling, trying the normal
// function first and then falling back to degraded functions based on the
// current degradation level.
//
// Parameters:
// - ctx: Context for cancellation
// - normalFn: Normal function to execute
// - degradedFns: Map of degradation levels to fallback functions
//
// Returns:
// - error: Error if all execution attempts failed
func (dm *DegradationManager) Execute(
	ctx context.Context,
	normalFn func() error,
	degradedFns map[Level]func() error,
) error {
	level := dm.GetLevel()

	// Try normal execution first
	if level == LevelNormal {
		start := time.Now()
		err := normalFn()
		dm.RecordLatency(time.Since(start))
		if err == nil {
			return nil
		}
		dm.RecordError(err)
	}

	// Try degraded execution
	for _, degLevel := range []Level{LevelReducedFeatures, LevelMinimal, LevelFallback} {
		if level <= degLevel {
			if fn, exists := degradedFns[degLevel]; exists {
				if err := fn(); err == nil {
					return nil
				}
			}
		}
	}

	return errors.New("all degradation levels failed")
}

// DegradableService represents a service that supports graceful degradation
//
// This interface defines a service that can operate at different degradation levels.
// Implementations should provide different levels of functionality based on
// the current degradation state.
//
// Example:
//
//     type MyService struct {
//         // Service fields
//     }
//
//     func (s *MyService) ExecuteNormal(ctx context.Context) error {
//         // Full functionality
//     }
//
//     func (s *MyService) ExecuteReduced(ctx context.Context) error {
//         // Reduced functionality
//     }
//
//     func (s *MyService) ExecuteMinimal(ctx context.Context) error {
//         // Minimal functionality
//     }
//
//     func (s *MyService) ExecuteFallback(ctx context.Context) error {
//         // Fallback functionality
//     }
type DegradableService interface {
	// ExecuteNormal executes the service with full functionality
	ExecuteNormal(ctx context.Context) error
	// ExecuteReduced executes the service with reduced functionality
	ExecuteReduced(ctx context.Context) error
	// ExecuteMinimal executes the service with minimal functionality
	ExecuteMinimal(ctx context.Context) error
	// ExecuteFallback executes the service with fallback functionality
	ExecuteFallback(ctx context.Context) error
}

// ServiceWrapper wraps a service with degradation support
//
// The ServiceWrapper wraps a DegradableService and provides automatic
// degradation handling based on the configured thresholds.
//
// Example:
//
//     service := &MyService{}
//     wrapper := NewServiceWrapper(service, DefaultConfig())
//     
//     err := wrapper.Execute(ctx)
//     if err != nil {
//         log.Fatal(err)
//     }
//     
//     status := wrapper.Status()
//     fmt.Println("Current status:", status)
type ServiceWrapper struct {
	manager *DegradationManager
	service DegradableService
}

// NewServiceWrapper creates a new service wrapper
//
// Parameters:
// - service: Degradable service to wrap
// - config: Configuration for the degradation manager
//
// Returns:
// - *ServiceWrapper: New service wrapper instance
func NewServiceWrapper(service DegradableService, config Config) *ServiceWrapper {
	return &ServiceWrapper{
		manager: New(config),
		service: service,
	}
}

// Execute executes the service with degradation
//
// This method executes the wrapped service with automatic degradation handling,
// trying the normal execution first and then falling back to degraded modes
// if needed.
//
// Parameters:
// - ctx: Context for cancellation
//
// Returns:
// - error: Error if all execution attempts failed
func (sw *ServiceWrapper) Execute(ctx context.Context) error {
	return sw.manager.Execute(ctx,
		func() error { return sw.service.ExecuteNormal(ctx) },
		map[Level]func() error{
			LevelReducedFeatures: func() error { return sw.service.ExecuteReduced(ctx) },
			LevelMinimal:         func() error { return sw.service.ExecuteMinimal(ctx) },
			LevelFallback:        func() error { return sw.service.ExecuteFallback(ctx) },
		},
	)
}

// Status returns the current degradation status
//
// Returns:
// - DegradationStatus: Current degradation status
func (sw *ServiceWrapper) Status() DegradationStatus {
	return DegradationStatus{
		Level:      sw.manager.GetLevel(),
		ErrorCount: sw.manager.errorCount,
	}
}

// DegradationStatus represents the degradation status
//
// This struct represents the current degradation status of a service,
// including the current degradation level and error count.
//
// Example:
//
//     status := serviceWrapper.Status()
//     fmt.Printf("Degradation level: %s\n", status.Level)
//     fmt.Printf("Error count: %d\n", status.ErrorCount)
type DegradationStatus struct {
	Level      Level // Current degradation level
	ErrorCount int   // Number of errors recorded
}

// String returns a string representation of the status
//
// Returns:
// - string: String representation of the status
func (s DegradationStatus) String() string {
	return fmt.Sprintf("Level: %s, Errors: %d", s.Level, s.ErrorCount)
}
