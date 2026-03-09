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
type Config struct {
	EnableReducedFeatures bool
	EnableMinimal         bool
	EnableFallback        bool
	ErrorThreshold        int
	LatencyThreshold      time.Duration
}

// DefaultConfig returns default configuration
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
type DegradationManager struct {
	config       Config
	currentLevel Level
	errorCount   int
	mu           sync.RWMutex
	lastError    time.Time
}

// New creates a new degradation manager
func New(config Config) *DegradationManager {
	return &DegradationManager{
		config:       config,
		currentLevel: LevelNormal,
	}
}

// RecordError records an error and potentially triggers degradation
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
func (dm *DegradationManager) RecordLatency(duration time.Duration) {
	if duration > dm.config.LatencyThreshold {
		// Treat high latency as an error
		dm.RecordError(errors.New("high latency"))
	}
}

// GetLevel returns the current degradation level
func (dm *DegradationManager) GetLevel() Level {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentLevel
}

// Reset resets the degradation level to normal
func (dm *DegradationManager) Reset() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.currentLevel = LevelNormal
	dm.errorCount = 0
}

// ShouldDegrade checks if a feature should be degraded
func (dm *DegradationManager) ShouldDegrade(featureLevel Level) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentLevel >= featureLevel
}

// Execute executes a function with degradation handling
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
type DegradableService interface {
	ExecuteNormal(ctx context.Context) error
	ExecuteReduced(ctx context.Context) error
	ExecuteMinimal(ctx context.Context) error
	ExecuteFallback(ctx context.Context) error
}

// ServiceWrapper wraps a service with degradation support
type ServiceWrapper struct {
	manager *DegradationManager
	service DegradableService
}

// NewServiceWrapper creates a new service wrapper
func NewServiceWrapper(service DegradableService, config Config) *ServiceWrapper {
	return &ServiceWrapper{
		manager: New(config),
		service: service,
	}
}

// Execute executes the service with degradation
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
func (sw *ServiceWrapper) Status() DegradationStatus {
	return DegradationStatus{
		Level:      sw.manager.GetLevel(),
		ErrorCount: sw.manager.errorCount,
	}
}

// DegradationStatus represents the degradation status
type DegradationStatus struct {
	Level      Level
	ErrorCount int
}

// String returns a string representation of the status
func (s DegradationStatus) String() string {
	return fmt.Sprintf("Level: %s, Errors: %d", s.Level, s.ErrorCount)
}
