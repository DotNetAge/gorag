package observability

import (
	"context"
)

// Tracer defines the interface for distributed tracing
//
// This interface defines methods for creating and managing distributed traces.
// Tracing allows you to track the flow of requests through the system and
// identify performance bottlenecks.
//
// Example:
//
//     type CustomTracer struct {
//         // Custom tracer implementation
//     }
//
//     func (t *CustomTracer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
//         // Start a new span
//     }
//
//     func (t *CustomTracer) Extract(ctx context.Context) (Span, bool) {
//         // Extract span from context
//     }
type Tracer interface {
	// StartSpan starts a new span
	//
	// Parameters:
	// - ctx: Context for cancellation
	// - name: Name of the span
	//
	// Returns:
	// - context.Context: New context with the span
	// - Span: New span instance
	StartSpan(ctx context.Context, name string) (context.Context, Span)
	
	// Extract extracts a span from a context
	//
	// Parameters:
	// - ctx: Context containing the span
	//
	// Returns:
	// - Span: Extracted span
	// - bool: True if a span was found
	Extract(ctx context.Context) (Span, bool)
}

// Span defines the interface for a tracing span
//
// A span represents a single operation within a trace. It has a name,
// start time, end time, and attributes.
//
// Example:
//
//     ctx, span := tracer.StartSpan(ctx, "query")
//     defer span.End()
//     
//     span.SetAttribute("query", "What is GoRAG?")
//     
//     // Perform operation...
//     
//     if err != nil {
//         span.SetError(err)
//     }
type Span interface {
	// End ends the span
	End()
	
	// SetAttribute sets an attribute on the span
	//
	// Parameters:
	// - key: Attribute key
	// - value: Attribute value
	SetAttribute(key string, value interface{})
	
	// SetError sets an error on the span
	//
	// Parameters:
	// - err: Error to set
	SetError(err error)
}

// NoopTracer implements a no-op tracer
//
// This implementation provides a no-operation tracer that does nothing.
// It's useful when tracing is not needed or during testing.
//
// Example:
//
//     tracer := NewNoopTracer()
//     ctx, span := tracer.StartSpan(ctx, "operation")
//     defer span.End()
//     // No tracing will be performed

type NoopTracer struct{}

// NewNoopTracer creates a new no-op tracer
//
// Returns:
// - *NoopTracer: New no-op tracer instance
func NewNoopTracer() *NoopTracer {
	return &NoopTracer{}
}

// StartSpan starts a new span
//
// Parameters:
// - ctx: Context for cancellation
// - name: Name of the span
//
// Returns:
// - context.Context: Original context
// - Span: No-op span
func (t *NoopTracer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return ctx, &NoopSpan{}
}

// Extract extracts a span from a context
//
// Parameters:
// - ctx: Context containing the span
//
// Returns:
// - Span: No-op span
// - bool: False (no span found)
func (t *NoopTracer) Extract(ctx context.Context) (Span, bool) {
	return &NoopSpan{}, false
}

// NoopSpan implements a no-op span
//
// This implementation provides a no-operation span that does nothing.
// It's used by the NoopTracer.
type NoopSpan struct{}

// End ends the span
func (s *NoopSpan) End() {}

// SetAttribute sets an attribute on the span
//
// Parameters:
// - key: Attribute key
// - value: Attribute value
func (s *NoopSpan) SetAttribute(key string, value interface{}) {}

// SetError sets an error on the span
//
// Parameters:
// - err: Error to set
func (s *NoopSpan) SetError(err error) {}

// SpanContext represents the context of a span
//
// This struct contains the trace and span IDs for a span.
//
// Example:
//
//     spanCtx := SpanContext{
//         TraceID: "trace-123",
//         SpanID:  "span-456",
//     }
type SpanContext struct {
	TraceID string // Trace ID
	SpanID  string // Span ID
}
