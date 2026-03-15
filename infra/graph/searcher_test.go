package graph

import (
	"context"
	"testing"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
)

// MockGraphStore is a mock implementation of abstraction.GraphStore
type MockGraphStore struct {
	getNeighborsFn          func(ctx context.Context, nodeID string, limit int) ([]*abstraction.Node, error)
	getCommunitySummariesFn func(ctx context.Context, limit int) ([]map[string]any, error)
}

func (m *MockGraphStore) CreateNode(ctx context.Context, node *abstraction.Node) error {
	return nil
}

func (m *MockGraphStore) CreateEdge(ctx context.Context, edge *abstraction.Edge) error {
	return nil
}

func (m *MockGraphStore) GetNode(ctx context.Context, id string) (*abstraction.Node, error) {
	return nil, nil
}

func (m *MockGraphStore) GetEdge(ctx context.Context, id string) (*abstraction.Edge, error) {
	return nil, nil
}

func (m *MockGraphStore) DeleteNode(ctx context.Context, id string) error {
	return nil
}

func (m *MockGraphStore) DeleteEdge(ctx context.Context, id string) error {
	return nil
}

func (m *MockGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	return nil, nil
}

func (m *MockGraphStore) GetNeighbors(ctx context.Context, nodeID string, limit int) ([]*abstraction.Node, error) {
	if m.getNeighborsFn != nil {
		return m.getNeighborsFn(ctx, nodeID, limit)
	}
	return []*abstraction.Node{
		{
			ID:   "Bob",
			Type: "PERSON",
			Properties: map[string]any{
				"name": "Bob",
			},
		},
		{
			ID:   "Charlie",
			Type: "PERSON",
			Properties: map[string]any{
				"name": "Charlie",
			},
		},
	}, nil
}

func (m *MockGraphStore) GetCommunitySummaries(ctx context.Context, limit int) ([]map[string]any, error) {
	if m.getCommunitySummariesFn != nil {
		return m.getCommunitySummariesFn(ctx, limit)
	}
	return []map[string]any{
		{
			"community_id": 1,
			"summary":      "This community contains information about people.",
		},
		{
			"community_id": 2,
			"summary":      "This community contains information about places.",
		},
	}, nil
}

func (m *MockGraphStore) UpsertNodes(ctx context.Context, nodes []*abstraction.Node) error {
	return nil
}

func (m *MockGraphStore) UpsertEdges(ctx context.Context, edges []*abstraction.Edge) error {
	return nil
}

func (m *MockGraphStore) Close(ctx context.Context) error {
	return nil
}

func TestLocalSearcher_Search(t *testing.T) {
	// Create a mock graph store
	mockStore := &MockGraphStore{}

	// Create a local searcher
	searcher := NewLocalSearcher(mockStore)

	// Test Search method
	ctx := context.Background()
	result, err := searcher.Search(ctx, []string{"Alice"}, 2, 5)

	// Check for errors
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	// Check results
	if result == "" {
		t.Errorf("Expected non-empty result, got empty string")
	}
}

func TestGlobalSearcher_Search(t *testing.T) {
	// Create a mock graph store
	mockStore := &MockGraphStore{}

	// Create a mock LLM client
	mockLLM := &MockLLMClient{
		chatFn: func(ctx context.Context, messages []core.Message, options ...core.Option) (*core.Response, error) {
			return &core.Response{Content: "Global answer based on community summaries"}, nil
		},
	}

	// Create a global searcher
	searcher := NewGlobalSearcher(mockStore, mockLLM)

	// Test Search method
	ctx := context.Background()
	result, err := searcher.Search(ctx, "What are the main communities?", 2)

	// Check for errors
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	// Check results
	if result == "" {
		t.Errorf("Expected non-empty result, got empty string")
	}
}

func TestGlobalSearcher_Search_EmptySummaries(t *testing.T) {
	// Create a mock graph store that returns empty summaries
	mockStore := &MockGraphStore{
		getCommunitySummariesFn: func(ctx context.Context, limit int) ([]map[string]any, error) {
			return []map[string]any{}, nil
		},
	}

	// Create a mock LLM client
	mockLLM := &MockLLMClient{}

	// Create a global searcher
	searcher := NewGlobalSearcher(mockStore, mockLLM)

	// Test Search method
	ctx := context.Background()
	result, err := searcher.Search(ctx, "What are the main communities?", 2)

	// Check for errors
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	// Check results
	expected := "No global community data available."
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}
