package observability

import (
	"context"
)

// Tracer defines the interface for distributed tracing
type Tracer interface {
	// StartSpan starts a new span
	StartSpan(ctx context.Context, name string) (context.Context, Span)
	// Extract extracts a span from a context
	Extract(ctx context.Context) (Span, bool)
}

// Span defines the interface for a tracing span
type Span interface {
	// End ends the span
	End()
	// SetAttribute sets an attribute on the span
	SetAttribute(key string, value interface{})
	// SetError sets an error on the span
	SetError(err error)
}

// NoopTracer implements a no-op tracer
type NoopTracer struct{}

// NewNoopTracer creates a new no-op tracer
func NewNoopTracer() *NoopTracer {
	return &NoopTracer{}
}

// StartSpan starts a new span
func (t *NoopTracer) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return ctx, &NoopSpan{}
}

// Extract extracts a span from a context
func (t *NoopTracer) Extract(ctx context.Context) (Span, bool) {
	return &NoopSpan{}, false
}

// NoopSpan implements a no-op span
type NoopSpan struct{}

// End ends the span
func (s *NoopSpan) End() {}

// SetAttribute sets an attribute on the span
func (s *NoopSpan) SetAttribute(key string, value interface{}) {}

// SetError sets an error on the span
func (s *NoopSpan) SetError(err error) {}

// SpanContext represents the context of a span
type SpanContext struct {
	TraceID string
	SpanID  string
}
