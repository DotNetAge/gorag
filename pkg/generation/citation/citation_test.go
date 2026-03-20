package generation

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"testing"
	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/stretchr/testify/assert"
)

// MockLLMClient is a mock implementation of chat.Client for testing
type MockLLMClient struct {
	chatFn func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error)
}

func (m *MockLLMClient) Chat(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, messages, options...)
	}
	return &chat.Response{Content: "Test answer"}, nil
}

func (m *MockLLMClient) ChatStream(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Stream, error) {
	return nil, nil
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
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			content := messages[0].Content[0].Text
			// Verify that the prompt contains the expected structure
			assert.Contains(t, content, "[Documents]")
			assert.Contains(t, content, "[Question]")
			assert.Contains(t, content, "[doc_1]")
			assert.Contains(t, content, "[doc_2]")
			assert.Contains(t, content, "Paris is the capital of France")
			assert.Contains(t, content, "France is a country in Europe")
			assert.Contains(t, content, "What is the capital of France?")
			return &chat.Response{Content: "The capital of France is Paris [doc_1]"}, nil
		},
	}

	// Create a CitationGenerator
	generator := NewCitationGenerator(mockLLM)

	// Create test data
	query := "What is the capital of France?"
	chunks := []*core.Chunk{
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
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			content := messages[0].Content[0].Text
			// Verify that the prompt contains the expected structure even with empty chunks
			assert.Contains(t, content, "[Documents]")
			assert.Contains(t, content, "[Question]")
			assert.Contains(t, content, "What is the capital of France?")
			return &chat.Response{Content: "I don't have enough information."}, nil
		},
	}

	// Create a CitationGenerator
	generator := NewCitationGenerator(mockLLM)

	// Create test data with empty chunks
	query := "What is the capital of France?"
	chunks := []*core.Chunk{}

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
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			content := messages[0].Content[0].Text
			// Verify that the prompt contains the expected structure with a single chunk
			assert.Contains(t, content, "[doc_1]")
			return &chat.Response{Content: "Paris is the capital of France [doc_1]"}, nil
		},
	}

	// Create a CitationGenerator
	generator := NewCitationGenerator(mockLLM)

	// Create test data with a single chunk
	query := "What is the capital of France?"
	chunks := []*core.Chunk{
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
