package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gorag/core"
)

// LoggingMiddleware logs indexing and query operations
type LoggingMiddleware struct {
	*BaseMiddleware
	logger Logger
}

// Logger defines the logging interface
type Logger interface {
	Info(ctx context.Context, message string, fields map[string]interface{})
	Debug(ctx context.Context, message string, fields map[string]interface{})
	Error(ctx context.Context, message string, err error, fields map[string]interface{})
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		BaseMiddleware: NewBaseMiddleware("logging"),
		logger:         logger,
	}
}

// BeforeIndex logs before indexing
func (m *LoggingMiddleware) BeforeIndex(ctx context.Context, source *Source) error {
	if m.logger != nil {
		m.logger.Info(ctx, "Starting document indexing", map[string]interface{}{
			"type": source.Type,
			"path": source.Path,
		})
	}
	return nil
}

// AfterIndex logs after indexing
func (m *LoggingMiddleware) AfterIndex(ctx context.Context, chunks []core.Chunk) error {
	if m.logger != nil {
		m.logger.Info(ctx, "Document indexed successfully", map[string]interface{}{
			"chunks_count": len(chunks),
		})
	}
	return nil
}

// BeforeQuery logs before query
func (m *LoggingMiddleware) BeforeQuery(ctx context.Context, query *Query) error {
	if m.logger != nil {
		m.logger.Info(ctx, "Starting query", map[string]interface{}{
			"question": query.Question,
		})
	}
	return nil
}

// AfterQuery logs after query
func (m *LoggingMiddleware) AfterQuery(ctx context.Context, response *Response) error {
	if m.logger != nil {
		m.logger.Info(ctx, "Query completed", map[string]interface{}{
			"sources_count": len(response.Sources),
		})
	}
	return nil
}

// MetricsMiddleware collects metrics for indexing and query operations
type MetricsMiddleware struct {
	*BaseMiddleware
	metrics Metrics
}

// Metrics defines the metrics collection interface
type Metrics interface {
	RecordIndexLatency(ctx context.Context, duration time.Duration)
	RecordIndexCount(ctx context.Context, status string)
	RecordQueryLatency(ctx context.Context, duration time.Duration)
	RecordQueryCount(ctx context.Context, status string)
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(metrics Metrics) *MetricsMiddleware {
	return &MetricsMiddleware{
		BaseMiddleware: NewBaseMiddleware("metrics"),
		metrics:        metrics,
	}
}

// BeforeIndex records index start time
func (m *MetricsMiddleware) BeforeIndex(ctx context.Context, source *Source) error {
	// Store start time in context
	ctx = context.WithValue(ctx, "index_start_time", time.Now())
	return nil
}

// AfterIndex records index metrics
func (m *MetricsMiddleware) AfterIndex(ctx context.Context, chunks []core.Chunk) error {
	if m.metrics != nil {
		if startTime, ok := ctx.Value("index_start_time").(time.Time); ok {
			duration := time.Since(startTime)
			m.metrics.RecordIndexLatency(ctx, duration)
		}
		m.metrics.RecordIndexCount(ctx, "success")
	}
	return nil
}

// BeforeQuery records query start time
func (m *MetricsMiddleware) BeforeQuery(ctx context.Context, query *Query) error {
	// Store start time in context
	ctx = context.WithValue(ctx, "query_start_time", time.Now())
	return nil
}

// AfterQuery records query metrics
func (m *MetricsMiddleware) AfterQuery(ctx context.Context, response *Response) error {
	if m.metrics != nil {
		if startTime, ok := ctx.Value("query_start_time").(time.Time); ok {
			duration := time.Since(startTime)
			m.metrics.RecordQueryLatency(ctx, duration)
		}
		m.metrics.RecordQueryCount(ctx, "success")
	}
	return nil
}

// CacheMiddleware provides caching for query results
type CacheMiddleware struct {
	*BaseMiddleware
	cache Cache
}

// Cache defines the cache interface
type Cache interface {
	Get(ctx context.Context, key string) (*Response, bool)
	Set(ctx context.Context, key string, response *Response, ttl time.Duration) error
}

// NewCacheMiddleware creates a new cache middleware
func NewCacheMiddleware(cache Cache) *CacheMiddleware {
	return &CacheMiddleware{
		BaseMiddleware: NewBaseMiddleware("cache"),
		cache:          cache,
	}
}

// BeforeQuery checks cache before query
func (m *CacheMiddleware) BeforeQuery(ctx context.Context, query *Query) error {
	if m.cache != nil {
		if response, found := m.cache.Get(ctx, query.Question); found {
			// Store cached response in context
			ctx = context.WithValue(ctx, "cached_response", response)
		}
	}
	return nil
}

// AfterQuery stores response in cache
func (m *CacheMiddleware) AfterQuery(ctx context.Context, response *Response) error {
	if m.cache != nil {
		// Check if this was a cache hit
		if _, found := ctx.Value("cached_response").(*Response); !found {
			// Store in cache with 1 hour TTL
			if err := m.cache.Set(ctx, response.Answer, response, time.Hour); err != nil {
				// Log error but don't fail the request
				return nil
			}
		}
	}
	return nil
}

// ValidationMiddleware validates input before processing
type ValidationMiddleware struct {
	*BaseMiddleware
	maxQueryLength int
	maxFileSize    int64
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(maxQueryLength int, maxFileSize int64) *ValidationMiddleware {
	return &ValidationMiddleware{
		BaseMiddleware: NewBaseMiddleware("validation"),
		maxQueryLength: maxQueryLength,
		maxFileSize:    maxFileSize,
	}
}

// BeforeIndex validates source before indexing
func (m *ValidationMiddleware) BeforeIndex(ctx context.Context, source *Source) error {
	if source.Type == "" {
		return fmt.Errorf("source type is required")
	}

	if source.Content == "" && source.Path == "" && source.Reader == nil {
		return fmt.Errorf("source content, path, or reader is required")
	}

	// Add more validation as needed
	return nil
}

// BeforeQuery validates query before execution
func (m *ValidationMiddleware) BeforeQuery(ctx context.Context, query *Query) error {
	if query.Question == "" {
		return fmt.Errorf("query question is required")
	}

	if m.maxQueryLength > 0 && len(query.Question) > m.maxQueryLength {
		return fmt.Errorf("query question exceeds maximum length of %d characters", m.maxQueryLength)
	}

	return nil
}

// RateLimitMiddleware implements rate limiting
type RateLimitMiddleware struct {
	*BaseMiddleware
	limiter RateLimiter
}

// RateLimiter defines the rate limiting interface
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(limiter RateLimiter) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		BaseMiddleware: NewBaseMiddleware("rate_limit"),
		limiter:        limiter,
	}
}

// BeforeQuery checks rate limit before query
func (m *RateLimitMiddleware) BeforeQuery(ctx context.Context, query *Query) error {
	if m.limiter != nil {
		allowed, err := m.limiter.Allow(ctx, "query")
		if err != nil {
			return fmt.Errorf("rate limit check failed: %w", err)
		}
		if !allowed {
			return fmt.Errorf("rate limit exceeded")
		}
	}
	return nil
}

// TransformMiddleware transforms queries and responses
type TransformMiddleware struct {
	*BaseMiddleware
	queryTransformer    func(ctx context.Context, query *Query) error
	responseTransformer func(ctx context.Context, response *Response) error
}

// NewTransformMiddleware creates a new transform middleware
func NewTransformMiddleware(
	queryTransformer func(ctx context.Context, query *Query) error,
	responseTransformer func(ctx context.Context, response *Response) error,
) *TransformMiddleware {
	return &TransformMiddleware{
		BaseMiddleware:      NewBaseMiddleware("transform"),
		queryTransformer:    queryTransformer,
		responseTransformer: responseTransformer,
	}
}

// BeforeQuery transforms query
func (m *TransformMiddleware) BeforeQuery(ctx context.Context, query *Query) error {
	if m.queryTransformer != nil {
		return m.queryTransformer(ctx, query)
	}
	return nil
}

// AfterQuery transforms response
func (m *TransformMiddleware) AfterQuery(ctx context.Context, response *Response) error {
	if m.responseTransformer != nil {
		return m.responseTransformer(ctx, response)
	}
	return nil
}
