package enhancer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockSimpleLLMClient is a mock implementation of SimpleLLMClient
type MockSimpleLLMClient struct {
	generateFn func(ctx context.Context, prompt string) (string, error)
}

func (m *MockSimpleLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, prompt)
	}
	// Return a dummy response
	return `{"year": 2023, "author": "test"}`, nil
}

func TestFilterExtractor_ExtractFilters(t *testing.T) {
	// Create a mock LLM client
	mockLLM := &MockSimpleLLMClient{
		generateFn: func(ctx context.Context, prompt string) (string, error) {
			return `{"year": 2023, "author": "John Doe"}`, nil
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
	mockLLM := &MockSimpleLLMClient{
		generateFn: func(ctx context.Context, prompt string) (string, error) {
			return `{}`, nil
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
	mockLLM := &MockSimpleLLMClient{
		generateFn: func(ctx context.Context, prompt string) (string, error) {
			return "invalid json", nil
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
