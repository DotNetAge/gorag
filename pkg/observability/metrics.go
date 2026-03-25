package observability

import (
	"context"
	"time"
)

// Collector defines the interface for collecting metrics (Deprecated: use core.Metrics instead).
type Collector interface {
	// RecordDuration records the duration of an operation.
	RecordDuration(operation string, duration time.Duration, labels map[string]string)

	// RecordCount records the count of an operation (success/failure).
	RecordCount(operation string, status string, labels map[string]string)

	// RecordValue records a custom metric value.
	RecordValue(metricName string, value float64, labels map[string]string)
}

// noopCollector is a no-op implementation for testing and default usage.
type noopCollector struct{}

// DefaultNoopCollector creates a no-op collector that discards all metrics.
func DefaultNoopCollector() Collector {
	return &noopCollector{}
}

func (c *noopCollector) RecordDuration(string, time.Duration, map[string]string) {
	// No-op
}

func (c *noopCollector) RecordCount(string, string, map[string]string) {
	// No-op
}

func (c *noopCollector) RecordValue(string, float64, map[string]string) {
	// No-op
}

// NoopMetrics is a no-op implementation of core.Metrics.
type NoopMetrics struct{}

func (m *NoopMetrics) RecordSearchDuration(string, any)                  {}
func (m *NoopMetrics) RecordSearchResult(string, int)                    {}
func (m *NoopMetrics) RecordSearchError(string, error)                   {}
func (m *NoopMetrics) RecordIndexingDuration(string, any)                {}
func (m *NoopMetrics) RecordIndexingResult(string, int)                  {}
func (m *NoopMetrics) RecordEmbeddingCount(int)                         {}
func (m *NoopMetrics) RecordVectorStoreOperations(string, int)           {}
func (m *NoopMetrics) RecordQueryCount(string)                           {}
func (m *NoopMetrics) RecordLLMTokenUsage(string, int, int)              {}
func (m *NoopMetrics) RecordRAGEvaluation(string, float32)                {}

// Tracer defines the interface for distributed tracing.
type Tracer interface {
	// StartSpan starts a new span/tracing context.
	StartSpan(ctx context.Context, operationName string) (context.Context, Span)

	// GetSpan retrieves the current span from context.
	GetSpan(ctx context.Context) Span
}

// Span represents a single operation in a trace.
type Span interface {
	// SetTag sets a tag on the span.
	SetTag(key string, value interface{})

	// LogEvent logs an event within the span.
	LogEvent(eventName string, fields map[string]interface{})

	// End ends the span.
	End()
}

// noopTracer is a no-op implementation.
type noopTracer struct{}

// DefaultNoopTracer creates a no-op tracer.
func DefaultNoopTracer() Tracer {
	return &noopTracer{}
}

func (t *noopTracer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return ctx, &noopSpan{}
}

func (t *noopTracer) GetSpan(context.Context) Span {
	return &noopSpan{}
}

type noopSpan struct{}

func (s *noopSpan) SetTag(string, interface{})              {}
func (s *noopSpan) LogEvent(string, map[string]interface{}) {}
func (s *noopSpan) End()                                    {}
