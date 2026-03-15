package enhancer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockLLMClient is a mock implementation of core.Client for testing
type MockLLMClient struct {
	chatFn func(ctx context.Context, messages []core.Message, options ...core.Option) (*core.Response, error)
}

func (m *MockLLMClient) Chat(ctx context.Context, messages []core.Message, options ...core.Option) (*core.Response, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, messages, options...)
	}
	// Default mock response
	return &core.Response{Content: `{"year": 2023, "author": "test"}`}, nil
}

func (m *MockLLMClient) ChatStream(ctx context.Context, messages []core.Message, options ...core.Option) (*core.Stream, error) {
	// For this test, we don't need streaming support
	// Return nil stream and no error (will not be used in tests)
	return nil, nil
}

func TestFilterExtractor_ExtractFilters(t *testing.T) {
	// Create a mock LLM client
	mockLLM := &MockLLMClient{
		chatFn: func(ctx context.Context, messages []core.Message, options ...core.Option) (*core.Response, error) {
			return &core.Response{Content: `{"year": 2023, "author": "John Doe"}`}, nil
		},
	}

	// Create a filter extractor
	extractor := NewFilterExtractor(mockLLM)

	// Create a test query
	query := entity.NewQuery("test-query", "Find documents by John Doe from 2023", map[string]any{})

	// Test ExtractFilters method
	ctx := context.Background()
	filters, err := extractor.ExtractFilters(ctx, query)

	// Check for errors
	assert.NoError(t, err)

	// Check that filters were extracted
	assert.Equal(t, 2023.0, filters["year"])
	assert.Equal(t, "John Doe", filters["author"])
}

func TestFilterExtractor_ExtractFilters_EmptyFilters(t *testing.T) {
	// Create a mock LLM client that returns empty filters
	mockLLM := &MockLLMClient{
		chatFn: func(ctx context.Context, messages []core.Message, options ...core.Option) (*core.Response, error) {
			return &core.Response{Content: `{}`}, nil
		},
	}

	// Create a filter extractor
	extractor := NewFilterExtractor(mockLLM)

	// Create a test query
	query := entity.NewQuery("test-query", "Find documents", map[string]any{})

	// Test ExtractFilters method
	ctx := context.Background()
	filters, err := extractor.ExtractFilters(ctx, query)

	// Check for errors
	assert.NoError(t, err)

	// Check that empty filters were returned
	assert.Empty(t, filters)
}

func TestFilterExtractor_ExtractFilters_InvalidJSON(t *testing.T) {
	// Create a mock LLM client that returns invalid JSON
	mockLLM := &MockLLMClient{
		chatFn: func(ctx context.Context, messages []core.Message, options ...core.Option) (*core.Response, error) {
			return &core.Response{Content: "invalid json"}, nil
		},
	}

	// Create a filter extractor
	extractor := NewFilterExtractor(mockLLM)

	// Create a test query
	query := entity.NewQuery("test-query", "Find documents by John Doe", map[string]any{})

	// Test ExtractFilters method
	ctx := context.Background()
	filters, err := extractor.ExtractFilters(ctx, query)

	// Check for errors
	assert.NoError(t, err)

	// Check that empty filters were returned as fallback
	assert.Empty(t, filters)
}
