package rag

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/vectorstore/memory"
)

// MockEmbeddingProvider is a mock embedding provider for testing
type MockEmbeddingProvider struct{}

func (m *MockEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embedding := make([]float32, 3)
		for j := range embedding {
			embedding[j] = float32(i*10 + j)
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

func (m *MockEmbeddingProvider) Dimension() int {
	return 3
}

// MockLLMClient is a mock LLM client for testing
type MockLLMClient struct{}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "Mock response to: " + prompt, nil
}

func (m *MockLLMClient) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		defer close(ch)
		ch <- "Mock stream response to: " + prompt
	}()
	return ch, nil
}

func TestRAGEngineWithAllFeatures(t *testing.T) {
	// Create vector store
	store := memory.NewStore()

	// Create embedding provider
	embedder := &MockEmbeddingProvider{}

	// Create LLM client
	llmClient := &MockLLMClient{}

	// Create RAG engine with all features
	eg, err := New(
		WithVectorStore(store),
		WithEmbedder(embedder),
		WithLLM(llmClient),
		WithConnectionPool(5, 2, 30*time.Second),
		WithQueryCache(5*time.Minute, 100),
		WithBatchProcessor(10, 5, 10*time.Millisecond),
		WithCircuitBreaker(3, 30*time.Second, 1),
	)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	ctx := context.Background()

	// Test 1: Add document
	source := Source{
		Type:    "txt",
		Content: "This is a test document",
	}
	err = eg.Index(ctx, source)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Test 2: Query
	question := "What is the test document about?"
	response, err := eg.Query(ctx, question, QueryOptions{})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if response.Answer == "" {
		t.Error("Expected answer to be non-empty")
	}

	// Test 3: Query again (should use cache)
	response2, err := eg.Query(ctx, question, QueryOptions{})
	if err != nil {
		t.Fatalf("Failed to query again: %v", err)
	}

	if response2.Answer != response.Answer {
		t.Error("Expected cached answer to be the same")
	}
}
