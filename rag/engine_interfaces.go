package rag

import (
	"context"
	"time"

	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/rag/query"
)

// Metrics interface implementations for Engine

// RecordErrorCount records error metrics
func (e *Engine) RecordErrorCount(ctx context.Context, errorType string) {
	if e.metrics != nil {
		e.metrics.RecordErrorCount(ctx, errorType)
	}
}

// RecordIndexLatency records index latency metrics
func (e *Engine) RecordIndexLatency(ctx context.Context, duration time.Duration) {
	if e.metrics != nil {
		e.metrics.RecordIndexLatency(ctx, duration)
	}
}

// RecordIndexCount records index count metrics
func (e *Engine) RecordIndexCount(ctx context.Context, status string) {
	if e.metrics != nil {
		e.metrics.RecordIndexCount(ctx, status)
	}
}

// RecordQueryLatency records query latency metrics
func (e *Engine) RecordQueryLatency(ctx context.Context, duration time.Duration) {
	if e.metrics != nil {
		e.metrics.RecordQueryLatency(ctx, duration)
	}
}

// RecordQueryCount records query count metrics
func (e *Engine) RecordQueryCount(ctx context.Context, status string) {
	if e.metrics != nil {
		e.metrics.RecordQueryCount(ctx, status)
	}
}

// Logger interface implementations for Engine

// Info logs info level message
func (e *Engine) Info(ctx context.Context, message string, fields map[string]interface{}) {
	if e.logger != nil {
		e.logger.Info(ctx, message, fields)
	}
}

// Debug logs debug level message
func (e *Engine) Debug(ctx context.Context, message string, fields map[string]interface{}) {
	if e.logger != nil {
		e.logger.Debug(ctx, message, fields)
	}
}

// Error logs error level message
func (e *Engine) Error(ctx context.Context, message string, err error, fields map[string]interface{}) {
	if e.logger != nil {
		e.logger.Error(ctx, message, err, fields)
	}
}

// Warn logs warn level message
func (e *Engine) Warn(ctx context.Context, message string, fields map[string]interface{}) {
	if e.logger != nil {
		e.logger.Warn(ctx, message, fields)
	}
}

// Tracer interface implementations for Engine

// StartSpan starts a new trace span
func (e *Engine) StartSpan(ctx context.Context, name string) (context.Context, observability.Span) {
	if e.tracer != nil {
		return e.tracer.StartSpan(ctx, name)
	}
	return ctx, nil
}

// Extract extracts span from context
func (e *Engine) Extract(ctx context.Context) (observability.Span, bool) {
	if e.tracer != nil {
		return e.tracer.Extract(ctx)
	}
	return nil, false
}

// Cache interface implementations for Engine

// Get retrieves a cached response
func (e *Engine) Get(ctx context.Context, key string) (*query.Response, bool) {
	if e.cache == nil {
		return nil, false
	}
	resp, found := e.cache.Get(ctx, key)
	if !found {
		return nil, false
	}
	return &query.Response{
		Answer:  resp.Answer,
		Sources: resp.Sources,
	}, true
}

// Set stores a response in cache
func (e *Engine) Set(ctx context.Context, key string, value *query.Response, expiration time.Duration) {
	if e.cache == nil {
		return
	}
	e.cache.Set(ctx, key, &Response{
		Answer:  value.Answer,
		Sources: value.Sources,
	}, expiration)
}
