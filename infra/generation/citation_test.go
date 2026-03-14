package generation

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockLLMClient is a mock implementation of SimpleLLMClient
 type MockLLMClient struct {
	generateFn func(ctx context.Context, prompt string) (string, error)
 }

 func (m *MockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, prompt)
	}
	return "Test answer", nil
}

func TestCitationGenerator_New(t *testing.T) {
	// Create a mock LLM client
	mockLLM := &MockLLMClient{}

	// Test creating a new CitationGenerator
	generator := NewCitationGenerator(mockLLM)
	assert.NotNil(t, generator)
	assert.NotNil(t, generator.llm)
}

func TestCitationGenerator_GenerateWithCitations(t *testing.T) {
	// Create a mock LLM client that returns a specific response
	mockLLM := &MockLLMClient{
		generateFn: func(ctx context.Context, prompt string) (string, error) {
			// Verify that the prompt contains the expected structure
			assert.Contains(t, prompt, "[Documents]")
			assert.Contains(t, prompt, "[Question]")
			assert.Contains(t, prompt, "[doc_1]")
			assert.Contains(t, prompt, "[doc_2]")
			assert.Contains(t, prompt, "Paris is the capital of France")
			assert.Contains(t, prompt, "France is a country in Europe")
			assert.Contains(t, prompt, "What is the capital of France?")
			return "The capital of France is Paris [doc_1]", nil
		},
	}

	// Create a CitationGenerator
	generator := NewCitationGenerator(mockLLM)

	// Create test data
	query := "What is the capital of France?"
	chunks := []*entity.Chunk{
		{
			ID:      "chunk1",
			Content: "Paris is the capital of France and the largest city in the country.",
		},
		{
			ID:      "chunk2",
			Content: "France is a country in Europe with a rich history and culture.",
		},
	}

	// Test GenerateWithCitations method
	ctx := context.Background()
	answer, err := generator.GenerateWithCitations(ctx, query, chunks)

	// Check for errors
	assert.NoError(t, err)

	// Check the result
	assert.Equal(t, "The capital of France is Paris [doc_1]", answer)
}

func TestCitationGenerator_GenerateWithCitations_EmptyChunks(t *testing.T) {
	// Create a mock LLM client
	mockLLM := &MockLLMClient{
		generateFn: func(ctx context.Context, prompt string) (string, error) {
			// Verify that the prompt contains the expected structure even with empty chunks
			assert.Contains(t, prompt, "[Documents]")
			assert.Contains(t, prompt, "[Question]")
			assert.Contains(t, prompt, "What is the capital of France?")
			return "I don't have enough information.", nil
		},
	}

	// Create a CitationGenerator
	generator := NewCitationGenerator(mockLLM)

	// Create test data with empty chunks
	query := "What is the capital of France?"
	chunks := []*entity.Chunk{}

	// Test GenerateWithCitations method
	ctx := context.Background()
	answer, err := generator.GenerateWithCitations(ctx, query, chunks)

	// Check for errors
	assert.NoError(t, err)

	// Check the result
	assert.Equal(t, "I don't have enough information.", answer)
}

func TestCitationGenerator_GenerateWithCitations_SingleChunk(t *testing.T) {
	// Create a mock LLM client
	mockLLM := &MockLLMClient{
		generateFn: func(ctx context.Context, prompt string) (string, error) {
			// Verify that the prompt contains the expected structure with a single chunk
			assert.Contains(t, prompt, "[doc_1]")
			return "Paris is the capital of France [doc_1]", nil
		},
	}

	// Create a CitationGenerator
	generator := NewCitationGenerator(mockLLM)

	// Create test data with a single chunk
	query := "What is the capital of France?"
	chunks := []*entity.Chunk{
		{
			ID:      "chunk1",
			Content: "Paris is the capital of France.",
		},
	}

	// Test GenerateWithCitations method
	ctx := context.Background()
	answer, err := generator.GenerateWithCitations(ctx, query, chunks)

	// Check for errors
	assert.NoError(t, err)

	// Check the result
	assert.Equal(t, "Paris is the capital of France [doc_1]", answer)
}
