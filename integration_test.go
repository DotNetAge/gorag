package gorag

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/vectorstore/memory"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmbedder is a simple mock embedder for testing
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i := range texts {
		// Simple mock embedding: just return a vector of 3 zeros
		results[i] = []float32{0.1, 0.2, 0.3}
	}
	return results, nil
}

func (m *mockEmbedder) Dimension() int {
	return 3
}

// mockLLM is a simple mock LLM for testing
type mockLLM struct{}

func (m *mockLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return "This is a mock response based on the context", nil
}

func (m *mockLLM) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "This is a mock stream response"
	close(ch)
	return ch, nil
}

func TestEndToEndFlow(t *testing.T) {
	// Create components
	parser := text.NewParser()
	embedder := &mockEmbedder{}
	vectorStore := memory.NewStore()
	llmClient := &mockLLM{}

	// Create RAG engine
	engine, err := rag.New(
		rag.WithParser(parser),
		rag.WithEmbedder(embedder),
		rag.WithVectorStore(vectorStore),
		rag.WithLLM(llmClient),
	)
	require.NoError(t, err)
	require.NotNil(t, engine)

	// Test indexing a document
	source := rag.Source{
		Type:    "text",
		Content: "GoRAG is a retrieval-augmented generation framework for Go.",
	}

	err = engine.Index(context.Background(), source)
	require.NoError(t, err)

	// Test querying the document
	query := "What is GoRAG?"
	response, err := engine.Query(context.Background(), query, rag.QueryOptions{
		TopK: 5,
	})
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response
	assert.NotEmpty(t, response.Answer)
	assert.Len(t, response.Sources, 1)
	assert.Contains(t, response.Sources[0].Content, "GoRAG")
}

func TestEndToEndFlow_WithMultipleDocuments(t *testing.T) {
	// Create components
	parser := text.NewParser()
	embedder := &mockEmbedder{}
	vectorStore := memory.NewStore()
	llmClient := &mockLLM{}

	// Create RAG engine
	engine, err := rag.New(
		rag.WithParser(parser),
		rag.WithEmbedder(embedder),
		rag.WithVectorStore(vectorStore),
		rag.WithLLM(llmClient),
	)
	require.NoError(t, err)
	require.NotNil(t, engine)

	// Index multiple documents
	sources := []rag.Source{
		{
			Type:    "text",
			Content: "GoRAG is a retrieval-augmented generation framework for Go.",
		},
		{
			Type:    "text",
			Content: "Go is a statically typed, compiled programming language.",
		},
		{
			Type:    "text",
			Content: "RAG combines retrieval and generation to improve LLM performance.",
		},
	}

	for _, source := range sources {
		err = engine.Index(context.Background(), source)
		require.NoError(t, err)
	}

	// Test querying
	query := "What is RAG?"
	response, err := engine.Query(context.Background(), query, rag.QueryOptions{
		TopK: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response
	assert.NotEmpty(t, response.Answer)
	assert.GreaterOrEqual(t, len(response.Sources), 1)
}

func TestEndToEndFlow_WithStream(t *testing.T) {
	// Create components
	parser := text.NewParser()
	embedder := &mockEmbedder{}
	vectorStore := memory.NewStore()
	llmClient := &mockLLM{}

	// Create RAG engine
	engine, err := rag.New(
		rag.WithParser(parser),
		rag.WithEmbedder(embedder),
		rag.WithVectorStore(vectorStore),
		rag.WithLLM(llmClient),
	)
	require.NoError(t, err)
	require.NotNil(t, engine)

	// Index a document
	source := rag.Source{
		Type:    "text",
		Content: "GoRAG supports streaming responses for better user experience.",
	}

	err = engine.Index(context.Background(), source)
	require.NoError(t, err)

	// Test streaming query
	query := "Does GoRAG support streaming?"
	response, err := engine.Query(context.Background(), query, rag.QueryOptions{
		TopK:   1,
		Stream: true,
	})
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response
	assert.NotEmpty(t, response.Answer)
}

func TestEndToEndFlow_WithCustomPromptTemplate(t *testing.T) {
	// Create components
	parser := text.NewParser()
	embedder := &mockEmbedder{}
	vectorStore := memory.NewStore()
	llmClient := &mockLLM{}

	// Create RAG engine
	engine, err := rag.New(
		rag.WithParser(parser),
		rag.WithEmbedder(embedder),
		rag.WithVectorStore(vectorStore),
		rag.WithLLM(llmClient),
	)
	require.NoError(t, err)
	require.NotNil(t, engine)

	// Index a document
	source := rag.Source{
		Type:    "text",
		Content: "Custom prompt templates allow for more control over the LLM prompt.",
	}

	err = engine.Index(context.Background(), source)
	require.NoError(t, err)

	// Test with custom prompt template
	query := "What are custom prompt templates?"
	customTemplate := "Answer the following question based on the context: {question}\n\nContext: {context}"
	response, err := engine.Query(context.Background(), query, rag.QueryOptions{
		TopK:           1,
		PromptTemplate: customTemplate,
	})
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response
	assert.NotEmpty(t, response.Answer)
}
