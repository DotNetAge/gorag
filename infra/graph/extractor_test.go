package graph

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// MockSimpleLLMClient is a mock implementation of SimpleLLMClient
type MockSimpleLLMClient struct {
	generateFn func(ctx context.Context, prompt string) (string, error)
}

func (m *MockSimpleLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, prompt)
	}
	return `{
		"nodes": [
			{
				"id": "Alice",
				"type": "PERSON",
				"properties": {}
			},
			{
				"id": "Bob",
				"type": "PERSON",
				"properties": {}
			}
		],
		"edges": [
			{
				"id": "1",
				"type": "KNOWS",
				"source": "Alice",
				"target": "Bob",
				"properties": {}
			}
		]
	}`, nil
}

func TestGraphExtractor_Extract(t *testing.T) {
	// Create a mock LLM client
	mockLLM := &MockSimpleLLMClient{}

	// Create a graph extractor
	extractor := NewGraphExtractor(mockLLM)

	// Create a test chunk
	chunk := &entity.Chunk{
		ID:      "test-chunk-1",
		Content: "Alice knows Bob.",
		Metadata: map[string]any{
			"source": "test.txt",
		},
	}

	// Test Extract method
	ctx := context.Background()
	nodes, edges, err := extractor.Extract(ctx, chunk)

	// Check for errors
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	// Check results
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}

	if len(edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(edges))
	}

	// Check node properties
	for _, node := range nodes {
		if node.Properties == nil {
			t.Errorf("Expected node properties to be set")
		}
		if node.Properties["source_chunk_id"] != chunk.ID {
			t.Errorf("Expected source_chunk_id to be '%s', got '%v'", chunk.ID, node.Properties["source_chunk_id"])
		}
	}

	// Check edge properties
	for _, edge := range edges {
		if edge.Properties == nil {
			t.Errorf("Expected edge properties to be set")
		}
		if edge.Properties["source_chunk_id"] != chunk.ID {
			t.Errorf("Expected source_chunk_id to be '%s', got '%v'", chunk.ID, edge.Properties["source_chunk_id"])
		}
	}
}

func TestGraphExtractor_Extract_InvalidJSON(t *testing.T) {
	// Create a mock LLM client that returns invalid JSON
	mockLLM := &MockSimpleLLMClient{
		generateFn: func(ctx context.Context, prompt string) (string, error) {
			return "invalid json", nil
		},
	}

	// Create a graph extractor
	extractor := NewGraphExtractor(mockLLM)

	// Create a test chunk
	chunk := &entity.Chunk{
		ID:      "test-chunk-1",
		Content: "Alice knows Bob.",
	}

	// Test Extract method
	ctx := context.Background()
	nodes, edges, err := extractor.Extract(ctx, chunk)

	// Check for errors
	if err == nil {
		t.Errorf("Expected error for invalid JSON, got nil")
	}

	// Check results
	if nodes != nil {
		t.Errorf("Expected nodes to be nil, got %v", nodes)
	}

	if edges != nil {
		t.Errorf("Expected edges to be nil, got %v", edges)
	}
}
