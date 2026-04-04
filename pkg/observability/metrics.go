// Package observability provides metrics collection and distributed tracing capabilities.
// It offers interfaces and implementations for monitoring RAG system performance,
// tracking operations, and integrating with observability platforms.
//
// The package provides:
//   - Metrics collection for search, indexing, and LLM operations
//   - Distributed tracing support for debugging and performance analysis
//   - No-op implementations for testing and default usage
//
// Example usage:
//
//	// Create a metrics collector
//	metrics := observability.NewPrometheusMetrics()
//	metrics.RecordSearchDuration("vector", time.Second)
//	metrics.RecordQueryCount("hybrid")
//
//	// Create a tracer
//	tracer := observability.NewOpenTelemetryTracer()
//	ctx, span := tracer.StartSpan(context.Background(), "search")
//	defer span.End()
package observability

import (
	"context"
	"time"
)

// Collector defines the interface for collecting metrics.
// Deprecated: Use core.Metrics instead for more comprehensive RAG-specific metrics.
//
// This interface provides basic metric collection capabilities for:
//   - Recording operation durations
//   - Counting operations (success/failure)
//   - Recording custom metric values
type Collector interface {
	// RecordDuration records the duration of an operation.
	// Use this to track performance metrics like search latency.
	//
	// Parameters:
	//   - operation: Name of the operation (e.g., "search", "index")
	//   - duration: How long the operation took
	//   - labels: Additional labels for filtering and grouping
	RecordDuration(operation string, duration time.Duration, labels map[string]string)

	// RecordCount records the count of an operation.
	// Use this to track operation frequency and success rates.
	//
	// Parameters:
	//   - operation: Name of the operation
	//   - status: Status of the operation (e.g., "success", "failure")
	//   - labels: Additional labels for filtering and grouping
	RecordCount(operation string, status string, labels map[string]string)

	// RecordValue records a custom metric value.
	// Use this for domain-specific metrics that don't fit other categories.
	//
	// Parameters:
	//   - metricName: Name of the metric
	//   - value: The metric value
	//   - labels: Additional labels for filtering and grouping
	RecordValue(metricName string, value float64, labels map[string]string)
}

// noopCollector is a no-op implementation of Collector.
// It discards all metrics and is useful for testing or when metrics are disabled.
type noopCollector struct{}

// DefaultNoopCollector creates a no-op collector that discards all metrics.
// Use this when metrics collection is not needed or during testing.
//
// Returns:
//   - Collector: A collector that does nothing
func DefaultNoopCollector() Collector {
	return &noopCollector{}
}

func (c *noopCollector) RecordDuration(string, time.Duration, map[string]string) {
}

func (c *noopCollector) RecordCount(string, string, map[string]string) {
}

func (c *noopCollector) RecordValue(string, float64, map[string]string) {
}

// NoopMetrics is a no-op implementation of core.Metrics.
// It implements all metrics methods as no-ops, useful for testing or when
// metrics collection is disabled.
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
// Distributed tracing helps track requests across multiple services and components,
// making it easier to debug performance issues and understand system behavior.
//
// Implementations should integrate with tracing backends like:
//   - OpenTelemetry
//   - Jaeger
//   - Zipkin
type Tracer interface {
	// StartSpan starts a new span within a trace.
	// A span represents a unit of work in the system.
	//
	// Parameters:
	//   - ctx: Parent context (may contain existing trace information)
	//   - operationName: Name of the operation being traced
	//
	// Returns:
	//   - context.Context: New context containing the span
	//   - Span: The created span that should be ended when complete
	//
	// Example:
	//
	//	ctx, span := tracer.StartSpan(ctx, "vector_search")
	//	defer span.End()
	//	// ... perform search ...
	StartSpan(ctx context.Context, operationName string) (context.Context, Span)

	// GetSpan retrieves the current span from context.
	// Returns a no-op span if no span is found in the context.
	//
	// Parameters:
	//   - ctx: Context that may contain span information
	//
	// Returns:
	//   - Span: The current span or a no-op span
	GetSpan(ctx context.Context) Span
}

// Span represents a single operation within a distributed trace.
// Spans can have tags and events attached for debugging and analysis.
type Span interface {
	// SetTag sets a key-value tag on the span.
	// Tags are used to filter and query traces.
	//
	// Parameters:
	//   - key: Tag name
	//   - value: Tag value (can be any type)
	//
	// Example:
	//
	//	span.SetTag("user_id", 123)
	//	span.SetTag("operation", "search")
	SetTag(key string, value interface{})

	// LogEvent logs an event within the span.
	// Events represent point-in-time occurrences during the span's lifetime.
	//
	// Parameters:
	//   - eventName: Name of the event
	//   - fields: Additional structured data about the event
	//
	// Example:
	//
	//	span.LogEvent("cache_hit", map[string]interface{}{
	//	    "key": "query_123",
	//	    "latency_ms": 5,
	//	})
	LogEvent(eventName string, fields map[string]interface{})

	// End completes the span.
	// This should be called when the operation is finished.
	// After End() is called, the span should not be modified.
	End()
}

// noopTracer is a no-op implementation of Tracer.
// It creates spans that do nothing, useful for testing or when tracing is disabled.
type noopTracer struct{}

// DefaultNoopTracer creates a no-op tracer that discards all trace data.
// Use this when distributed tracing is not needed or during testing.
//
// Returns:
//   - Tracer: A tracer that creates no-op spans
func DefaultNoopTracer() Tracer {
	return &noopTracer{}
}

func (t *noopTracer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return ctx, &noopSpan{}
}

func (t *noopTracer) GetSpan(context.Context) Span {
	return &noopSpan{}
}

// noopSpan is a no-op implementation of Span.
type noopSpan struct{}

func (s *noopSpan) SetTag(string, interface{})              {}
func (s *noopSpan) LogEvent(string, map[string]interface{}) {}
func (s *noopSpan) End()                                    {}
