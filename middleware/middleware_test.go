package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChain(t *testing.T) {
	m1 := NewBaseMiddleware("m1")
	m2 := NewBaseMiddleware("m2")

	chain := NewChain(m1, m2)
	assert.NotNil(t, chain)
	assert.Len(t, chain.middlewares, 2)
}

func TestChain_Add(t *testing.T) {
	chain := NewChain()
	assert.Len(t, chain.middlewares, 0)

	m1 := NewBaseMiddleware("m1")
	chain.Add(m1)
	assert.Len(t, chain.middlewares, 1)

	m2 := NewBaseMiddleware("m2")
	chain.Add(m2)
	assert.Len(t, chain.middlewares, 2)
}

func TestChain_BeforeIndex(t *testing.T) {
	ctx := context.Background()
	source := &Source{Type: "text", Content: "test"}

	// Test with no middleware
	chain := NewChain()
	err := chain.BeforeIndex(ctx, source)
	assert.NoError(t, err)

	// Test with middleware
	m := NewBaseMiddleware("test")
	chain.Add(m)
	err = chain.BeforeIndex(ctx, source)
	assert.NoError(t, err)
}

func TestChain_AfterIndex(t *testing.T) {
	ctx := context.Background()
	chunks := []core.Chunk{
		{ID: "1", Content: "test"},
	}

	chain := NewChain()
	err := chain.AfterIndex(ctx, chunks)
	assert.NoError(t, err)
}

func TestChain_BeforeQuery(t *testing.T) {
	ctx := context.Background()
	query := &Query{Question: "test"}

	chain := NewChain()
	err := chain.BeforeQuery(ctx, query)
	assert.NoError(t, err)
}

func TestChain_AfterQuery(t *testing.T) {
	ctx := context.Background()
	response := &Response{Answer: "test"}

	chain := NewChain()
	err := chain.AfterQuery(ctx, response)
	assert.NoError(t, err)
}

func TestBaseMiddleware(t *testing.T) {
	m := NewBaseMiddleware("test")
	assert.Equal(t, "test", m.Name())

	ctx := context.Background()

	// All methods should be no-op
	err := m.BeforeIndex(ctx, &Source{})
	assert.NoError(t, err)

	err = m.AfterIndex(ctx, []core.Chunk{})
	assert.NoError(t, err)

	err = m.BeforeQuery(ctx, &Query{})
	assert.NoError(t, err)

	err = m.AfterQuery(ctx, &Response{})
	assert.NoError(t, err)
}

// Mock logger for testing
type mockLogger struct {
	infoCalls  int
	debugCalls int
	errorCalls int
}

func (m *mockLogger) Info(ctx context.Context, message string, fields map[string]interface{}) {
	m.infoCalls++
}

func (m *mockLogger) Debug(ctx context.Context, message string, fields map[string]interface{}) {
	m.debugCalls++
}

func (m *mockLogger) Error(ctx context.Context, message string, err error, fields map[string]interface{}) {
	m.errorCalls++
}

func TestLoggingMiddleware(t *testing.T) {
	logger := &mockLogger{}
	m := NewLoggingMiddleware(logger)

	assert.Equal(t, "logging", m.Name())

	ctx := context.Background()

	// Test BeforeIndex
	err := m.BeforeIndex(ctx, &Source{Type: "text", Path: "test.txt"})
	assert.NoError(t, err)
	assert.Equal(t, 1, logger.infoCalls)

	// Test AfterIndex
	err = m.AfterIndex(ctx, []core.Chunk{{ID: "1"}})
	assert.NoError(t, err)
	assert.Equal(t, 2, logger.infoCalls)

	// Test BeforeQuery
	err = m.BeforeQuery(ctx, &Query{Question: "test"})
	assert.NoError(t, err)
	assert.Equal(t, 3, logger.infoCalls)

	// Test AfterQuery
	err = m.AfterQuery(ctx, &Response{Answer: "test"})
	assert.NoError(t, err)
	assert.Equal(t, 4, logger.infoCalls)
}

// Mock metrics for testing
type mockMetrics struct {
	indexLatencyCalls int
	indexCountCalls   int
	queryLatencyCalls int
	queryCountCalls   int
}

func (m *mockMetrics) RecordIndexLatency(ctx context.Context, duration time.Duration) {
	m.indexLatencyCalls++
}

func (m *mockMetrics) RecordIndexCount(ctx context.Context, status string) {
	m.indexCountCalls++
}

func (m *mockMetrics) RecordQueryLatency(ctx context.Context, duration time.Duration) {
	m.queryLatencyCalls++
}

func (m *mockMetrics) RecordQueryCount(ctx context.Context, status string) {
	m.queryCountCalls++
}

func TestMetricsMiddleware(t *testing.T) {
	metrics := &mockMetrics{}
	m := NewMetricsMiddleware(metrics)

	assert.Equal(t, "metrics", m.Name())

	ctx := context.Background()

	// Test index metrics
	err := m.BeforeIndex(ctx, &Source{})
	assert.NoError(t, err)

	err = m.AfterIndex(ctx, []core.Chunk{})
	assert.NoError(t, err)
	assert.Equal(t, 1, metrics.indexCountCalls)

	// Test query metrics
	err = m.BeforeQuery(ctx, &Query{})
	assert.NoError(t, err)

	err = m.AfterQuery(ctx, &Response{})
	assert.NoError(t, err)
	assert.Equal(t, 1, metrics.queryCountCalls)
}

// Mock cache for testing
type mockCache struct {
	data map[string]*Response
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string]*Response),
	}
}

func (c *mockCache) Get(ctx context.Context, key string) (*Response, bool) {
	resp, found := c.data[key]
	return resp, found
}

func (c *mockCache) Set(ctx context.Context, key string, response *Response, ttl time.Duration) error {
	c.data[key] = response
	return nil
}

func TestCacheMiddleware(t *testing.T) {
	cache := newMockCache()
	m := NewCacheMiddleware(cache)

	assert.Equal(t, "cache", m.Name())

	ctx := context.Background()

	// Test cache miss
	query := &Query{Question: "test"}
	err := m.BeforeQuery(ctx, query)
	assert.NoError(t, err)

	// Test cache set
	response := &Response{Answer: "test answer"}
	err = m.AfterQuery(ctx, response)
	assert.NoError(t, err)

	// Verify cache was set
	cached, found := cache.Get(ctx, "test answer")
	assert.True(t, found)
	assert.Equal(t, "test answer", cached.Answer)
}

func TestValidationMiddleware(t *testing.T) {
	m := NewValidationMiddleware(100, 1024*1024)

	assert.Equal(t, "validation", m.Name())

	ctx := context.Background()

	tests := []struct {
		name      string
		source    *Source
		query     *Query
		wantError bool
	}{
		{
			name:      "valid source",
			source:    &Source{Type: "text", Content: "test"},
			wantError: false,
		},
		{
			name:      "missing type",
			source:    &Source{Content: "test"},
			wantError: true,
		},
		{
			name:      "missing content",
			source:    &Source{Type: "text"},
			wantError: true,
		},
		{
			name:      "valid query",
			query:     &Query{Question: "test"},
			wantError: false,
		},
		{
			name:      "empty query",
			query:     &Query{Question: ""},
			wantError: true,
		},
		{
			name:      "query too long",
			query:     &Query{Question: string(make([]byte, 200))},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.source != nil {
				err = m.BeforeIndex(ctx, tt.source)
			} else if tt.query != nil {
				err = m.BeforeQuery(ctx, tt.query)
			}

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Mock rate limiter for testing
type mockRateLimiter struct {
	allowed bool
}

func (r *mockRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return r.allowed, nil
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := &mockRateLimiter{allowed: true}
	m := NewRateLimitMiddleware(limiter)

	assert.Equal(t, "rate_limit", m.Name())

	ctx := context.Background()
	query := &Query{Question: "test"}

	// Test allowed
	err := m.BeforeQuery(ctx, query)
	assert.NoError(t, err)

	// Test rate limited
	limiter.allowed = false
	err = m.BeforeQuery(ctx, query)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

func TestTransformMiddleware(t *testing.T) {
	queryCalled := false
	responseCalled := false

	queryTransformer := func(ctx context.Context, query *Query) error {
		queryCalled = true
		query.Question = "transformed: " + query.Question
		return nil
	}

	responseTransformer := func(ctx context.Context, response *Response) error {
		responseCalled = true
		response.Answer = "transformed: " + response.Answer
		return nil
	}

	m := NewTransformMiddleware(queryTransformer, responseTransformer)

	assert.Equal(t, "transform", m.Name())

	ctx := context.Background()

	// Test query transformation
	query := &Query{Question: "test"}
	err := m.BeforeQuery(ctx, query)
	assert.NoError(t, err)
	assert.True(t, queryCalled)
	assert.Equal(t, "transformed: test", query.Question)

	// Test response transformation
	response := &Response{Answer: "test"}
	err = m.AfterQuery(ctx, response)
	assert.NoError(t, err)
	assert.True(t, responseCalled)
	assert.Equal(t, "transformed: test", response.Answer)
}

func TestChain_ExecutionOrder(t *testing.T) {
	var executionOrder []string

	m1 := &testMiddleware{
		name: "m1",
		beforeQuery: func(ctx context.Context, query *Query) error {
			executionOrder = append(executionOrder, "m1-before")
			return nil
		},
		afterQuery: func(ctx context.Context, response *Response) error {
			executionOrder = append(executionOrder, "m1-after")
			return nil
		},
	}

	m2 := &testMiddleware{
		name: "m2",
		beforeQuery: func(ctx context.Context, query *Query) error {
			executionOrder = append(executionOrder, "m2-before")
			return nil
		},
		afterQuery: func(ctx context.Context, response *Response) error {
			executionOrder = append(executionOrder, "m2-after")
			return nil
		},
	}

	chain := NewChain(m1, m2)

	ctx := context.Background()
	query := &Query{Question: "test"}
	response := &Response{Answer: "test"}

	err := chain.BeforeQuery(ctx, query)
	require.NoError(t, err)

	err = chain.AfterQuery(ctx, response)
	require.NoError(t, err)

	// Verify execution order
	expected := []string{"m1-before", "m2-before", "m1-after", "m2-after"}
	assert.Equal(t, expected, executionOrder)
}

// testMiddleware is a helper for testing execution order
type testMiddleware struct {
	name        string
	beforeQuery func(ctx context.Context, query *Query) error
	afterQuery  func(ctx context.Context, response *Response) error
}

func (m *testMiddleware) Name() string {
	return m.name
}

func (m *testMiddleware) BeforeIndex(ctx context.Context, source *Source) error {
	return nil
}

func (m *testMiddleware) AfterIndex(ctx context.Context, chunks []core.Chunk) error {
	return nil
}

func (m *testMiddleware) BeforeQuery(ctx context.Context, query *Query) error {
	if m.beforeQuery != nil {
		return m.beforeQuery(ctx, query)
	}
	return nil
}

func (m *testMiddleware) AfterQuery(ctx context.Context, response *Response) error {
	if m.afterQuery != nil {
		return m.afterQuery(ctx, response)
	}
	return nil
}
