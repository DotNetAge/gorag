package gograph

import (
	"context"
	"os"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*gographStore, func()) {
	tmpPath := "/tmp/gograph_store_test_" + t.Name()
	store, err := NewGraphStore(tmpPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	cleanup := func() {
		store.Close(context.Background())
		os.RemoveAll(tmpPath)
	}
	return store.(*gographStore), cleanup
}

func TestNewGraphStore(t *testing.T) {
	tmpPath := "/tmp/gograph_new_store_test"
	defer os.RemoveAll(tmpPath)

	store, err := NewGraphStore(tmpPath)
	require.NoError(t, err)
	require.NotNil(t, store)

	err = store.Close(context.Background())
	require.NoError(t, err)
}

func TestDefaultGraphStore(t *testing.T) {
	tmpPath := "/tmp/gograph_default_test"
	defer os.RemoveAll(tmpPath)

	store, err := DefaultGraphStore(WithPath(tmpPath))
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close(context.Background())
}

func TestUpsertNodes(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	nodes := []*core.Node{
		{
			ID:         "node1",
			Type:       "Person",
			Properties: map[string]any{"name": "Alice", "age": 30},
		},
		{
			ID:         "node2",
			Type:       "Organization",
			Properties: map[string]any{"name": "Acme Corp"},
		},
	}

	err := store.UpsertNodes(ctx, nodes)
	require.NoError(t, err)

	retrievedNode, err := store.GetNode(ctx, "node1")
	require.NoError(t, err)
	require.NotNil(t, retrievedNode)
	assert.Equal(t, "node1", retrievedNode.ID)
	assert.Equal(t, "Person", retrievedNode.Type)
	assert.Equal(t, "Alice", retrievedNode.Properties["name"])
}

func TestUpsertNodesEmpty(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	err := store.UpsertNodes(ctx, []*core.Node{})
	require.NoError(t, err)
}

func TestUpsertNodesWithEmptyType(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	nodes := []*core.Node{
		{
			ID:         "node1",
			Type:       "",
			Properties: map[string]any{"name": "Test"},
		},
	}

	err := store.UpsertNodes(ctx, nodes)
	require.NoError(t, err)

	retrievedNode, err := store.GetNode(ctx, "node1")
	require.NoError(t, err)
	require.NotNil(t, retrievedNode)
	assert.Equal(t, "node1", retrievedNode.ID)
}

func TestUpsertEdges(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	node1 := &core.Node{ID: "node1", Type: "Person", Properties: map[string]any{"name": "Alice"}}
	node2 := &core.Node{ID: "node2", Type: "Person", Properties: map[string]any{"name": "Bob"}}
	err := store.UpsertNodes(ctx, []*core.Node{node1, node2})
	require.NoError(t, err)

	edge := &core.Edge{
		ID:         "edge1",
		Type:       "KNOWS",
		Source:     "node1",
		Target:     "node2",
		Properties: map[string]any{"since": 2020},
	}

	err = store.UpsertEdges(ctx, []*core.Edge{edge})
	require.NoError(t, err)

	neighbors, edges, err := store.GetNeighbors(ctx, "node1", 1, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, neighbors)
	assert.NotEmpty(t, edges)
}

func TestUpsertEdgesEmpty(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	err := store.UpsertEdges(ctx, []*core.Edge{})
	require.NoError(t, err)
}

func TestGetNode(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	nodes := []*core.Node{
		{
			ID:         "testNode",
			Type:       "TestType",
			Properties: map[string]any{"key": "value", "number": 42},
		},
	}

	err := store.UpsertNodes(ctx, nodes)
	require.NoError(t, err)

	retrieved, err := store.GetNode(ctx, "testNode")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "testNode", retrieved.ID)
	assert.Equal(t, "TestType", retrieved.Type)
	assert.Equal(t, "value", retrieved.Properties["key"])
	assert.Equal(t, 42, retrieved.Properties["number"])
}

func TestGetNodeNotFound(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	retrieved, err := store.GetNode(ctx, "nonexistent")
	require.NoError(t, err)
	require.Nil(t, retrieved)
}

func TestGetNeighbors(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	nodes := []*core.Node{
		{ID: "center", Type: "Person", Properties: map[string]any{"name": "Alice"}},
		{ID: "neighbor1", Type: "Person", Properties: map[string]any{"name": "Bob"}},
		{ID: "neighbor2", Type: "Person", Properties: map[string]any{"name": "Charlie"}},
	}
	err := store.UpsertNodes(ctx, nodes)
	require.NoError(t, err)

	edges := []*core.Edge{
		{ID: "edge1", Type: "KNOWS", Source: "center", Target: "neighbor1"},
		{ID: "edge2", Type: "KNOWS", Source: "center", Target: "neighbor2"},
	}
	err = store.UpsertEdges(ctx, edges)
	require.NoError(t, err)

	neighbors, returnedEdges, err := store.GetNeighbors(ctx, "center", 1, 10)
	require.NoError(t, err)
	assert.Len(t, neighbors, 2)
	assert.Len(t, returnedEdges, 2)
}

func TestGetNeighborsDepthZero(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	neighbors, edges, err := store.GetNeighbors(ctx, "nonexistent", 0, 10)
	require.NoError(t, err)
	assert.Empty(t, neighbors)
	assert.Empty(t, edges)
}

func TestQuery(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	nodes := []*core.Node{
		{ID: "node1", Type: "Person", Properties: map[string]any{"name": "Alice", "age": 30}},
		{ID: "node2", Type: "Person", Properties: map[string]any{"name": "Bob", "age": 25}},
	}
	err := store.UpsertNodes(ctx, nodes)
	require.NoError(t, err)

	results, err := store.Query(ctx, "MATCH (n:Person) RETURN n", nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestQueryEmptyResult(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	results, err := store.Query(ctx, "MATCH (n:NonExistent) RETURN n", nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestGetCommunitySummaries(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	summaries, err := store.GetCommunitySummaries(ctx, 1)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestClose(t *testing.T) {
	tmpPath := "/tmp/gograph_close_test"
	defer os.RemoveAll(tmpPath)

	store, err := NewGraphStore(tmpPath)
	require.NoError(t, err)

	err = store.Close(context.Background())
	require.NoError(t, err)
}

func TestInterfaceCompliance(t *testing.T) {
	var store interface {
		UpsertNodes(ctx context.Context, nodes []*core.Node) error
		UpsertEdges(ctx context.Context, edges []*core.Edge) error
		GetNode(ctx context.Context, id string) (*core.Node, error)
		GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error)
		Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)
		GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error)
		Close(ctx context.Context) error
	}

	gs, cleanup := setupTestDB(t)
	defer cleanup()

	store = gs

	ctx := context.Background()

	nodes := []*core.Node{
		{ID: "test1", Type: "Test", Properties: map[string]any{"key": "value"}},
	}
	err := store.UpsertNodes(ctx, nodes)
	require.NoError(t, err)

	_, err = store.GetNode(ctx, "test1")
	require.NoError(t, err)

	_, _, err = store.GetNeighbors(ctx, "test1", 1, 10)
	require.NoError(t, err)

	_, err = store.Query(ctx, "MATCH (n) RETURN n", nil)
	require.NoError(t, err)

	_, err = store.GetCommunitySummaries(ctx, 0)
	require.NoError(t, err)

	err = store.Close(ctx)
	require.NoError(t, err)
}

func TestPropertyTypes(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	nodes := []*core.Node{
		{
			ID:   "props",
			Type: "Test",
			Properties: map[string]any{
				"string": "hello",
				"int":    42,
			},
		},
	}

	err := store.UpsertNodes(ctx, nodes)
	require.NoError(t, err)

	retrieved, err := store.GetNode(ctx, "props")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "hello", retrieved.Properties["string"])
	assert.Equal(t, 42, retrieved.Properties["int"])
}

func TestEdgeProperties(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	node1 := &core.Node{ID: "n1", Type: "A", Properties: nil}
	node2 := &core.Node{ID: "n2", Type: "B", Properties: nil}
	err := store.UpsertNodes(ctx, []*core.Node{node1, node2})
	require.NoError(t, err)

	edge := &core.Edge{
		ID:         "rel1",
		Type:       "REL",
		Source:     "n1",
		Target:     "n2",
		Properties: map[string]any{"weight": 1.5, "active": true},
	}
	err = store.UpsertEdges(ctx, []*core.Edge{edge})
	require.NoError(t, err)

	_, edges, err := store.GetNeighbors(ctx, "n1", 1, 10)
	require.NoError(t, err)
	require.NotEmpty(t, edges)

	found := false
	for _, e := range edges {
		if e.ID == "rel1" {
			found = true
			assert.Equal(t, "REL", e.Type)
			assert.Equal(t, 1.5, e.Properties["weight"])
			assert.Equal(t, true, e.Properties["active"])
			break
		}
	}
	assert.True(t, found, "Edge rel1 not found")
}
