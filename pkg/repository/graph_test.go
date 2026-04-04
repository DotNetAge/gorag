package repository

import (
	"context"
	"os"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/store/graph/gograph"
	"github.com/DotNetAge/gorag/pkg/store/vector/govector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphRepository_CreateNode(t *testing.T) {
	ctx := context.Background()
	
	// Setup temporary storage
	tmpDir, err := os.MkdirTemp("", "gorag_graph_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	// Create graph store
	graphStore, err := gograph.NewGraphStore(tmpDir + "/graph.db")
	require.NoError(t, err)
	defer graphStore.Close(ctx)
	
	// Create vector store
	vecStore, err := govector.NewStore(
		govector.WithDBPath(tmpDir+"/vectors.db"),
		govector.WithDimension(128),
		govector.WithCollection("test"),
	)
	require.NoError(t, err)
	defer vecStore.Close(ctx)
	
	// Create repository
	repo := NewGraphRepository(
		graphStore,
		vecStore,
		nil,
		&mockEmbedder{},
		logging.DefaultNoopLogger(),
	)
	require.NotNil(t, repo)
	
	// Test CreateNode
	node := &core.Node{
		ID:   "node-1",
		Type: "PERSON",
		Properties: map[string]any{
			"name":    "Alice",
			"content": "Alice is a software engineer",
		},
	}
	
	err = repo.CreateNode(ctx, node)
	assert.NoError(t, err)
}

func TestGraphRepository_ReadNode(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_graph_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	graphStore, err := gograph.NewGraphStore(tmpDir + "/graph.db")
	require.NoError(t, err)
	defer graphStore.Close(ctx)
	
	vecStore, err := govector.NewStore(
		govector.WithDBPath(tmpDir+"/vectors.db"),
		govector.WithDimension(128),
		govector.WithCollection("test"),
	)
	require.NoError(t, err)
	defer vecStore.Close(ctx)
	
	repo := NewGraphRepository(
		graphStore,
		vecStore,
		nil,
		&mockEmbedder{},
		logging.DefaultNoopLogger(),
	)
	
	// Create node first
	node := &core.Node{
		ID:   "node-read-1",
		Type: "PERSON",
		Properties: map[string]any{
			"name": "Bob",
		},
	}
	err = repo.CreateNode(ctx, node)
	require.NoError(t, err)
	
	// Read node
	readNode, err := repo.ReadNode(ctx, "node-read-1")
	assert.NoError(t, err)
	assert.Equal(t, "node-read-1", readNode.ID)
	assert.Equal(t, "PERSON", readNode.Type)
}

func TestGraphRepository_DeleteNode(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_graph_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	graphStore, err := gograph.NewGraphStore(tmpDir + "/graph.db")
	require.NoError(t, err)
	defer graphStore.Close(ctx)
	
	vecStore, err := govector.NewStore(
		govector.WithDBPath(tmpDir+"/vectors.db"),
		govector.WithDimension(128),
		govector.WithCollection("test"),
	)
	require.NoError(t, err)
	defer vecStore.Close(ctx)
	
	repo := NewGraphRepository(
		graphStore,
		vecStore,
		nil,
		&mockEmbedder{},
		logging.DefaultNoopLogger(),
	)
	
	// Create node
	node := &core.Node{
		ID:   "node-delete-1",
		Type: "PERSON",
		Properties: map[string]any{
			"name": "Charlie",
		},
	}
	err = repo.CreateNode(ctx, node)
	require.NoError(t, err)
	
	// Delete node
	err = repo.DeleteNode(ctx, "node-delete-1")
	assert.NoError(t, err)
	
	// Verify deletion
	_, err = repo.ReadNode(ctx, "node-delete-1")
	assert.Error(t, err)
}

func TestGraphRepository_CreateEdge(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_graph_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	graphStore, err := gograph.NewGraphStore(tmpDir + "/graph.db")
	require.NoError(t, err)
	defer graphStore.Close(ctx)
	
	vecStore, err := govector.NewStore(
		govector.WithDBPath(tmpDir+"/vectors.db"),
		govector.WithDimension(128),
		govector.WithCollection("test"),
	)
	require.NoError(t, err)
	defer vecStore.Close(ctx)
	
	repo := NewGraphRepository(
		graphStore,
		vecStore,
		nil,
		&mockEmbedder{},
		logging.DefaultNoopLogger(),
	)
	
	// Create nodes first
	node1 := &core.Node{ID: "edge-node-1", Type: "PERSON"}
	node2 := &core.Node{ID: "edge-node-2", Type: "COMPANY"}
	
	err = repo.CreateNode(ctx, node1)
	require.NoError(t, err)
	err = repo.CreateNode(ctx, node2)
	require.NoError(t, err)
	
	// Create edge
	edge := &core.Edge{
		ID:     "edge-1",
		Type:   "WORKS_FOR",
		Source: "edge-node-1",
		Target: "edge-node-2",
	}
	
	err = repo.CreateEdge(ctx, edge)
	assert.NoError(t, err)
}

func TestGraphRepository_GetNeighbors(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_graph_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	graphStore, err := gograph.NewGraphStore(tmpDir + "/graph.db")
	require.NoError(t, err)
	defer graphStore.Close(ctx)
	
	vecStore, err := govector.NewStore(
		govector.WithDBPath(tmpDir+"/vectors.db"),
		govector.WithDimension(128),
		govector.WithCollection("test"),
	)
	require.NoError(t, err)
	defer vecStore.Close(ctx)
	
	repo := NewGraphRepository(
		graphStore,
		vecStore,
		nil,
		&mockEmbedder{},
		logging.DefaultNoopLogger(),
	)
	
	// Create nodes
	node1 := &core.Node{ID: "neighbor-1", Type: "PERSON"}
	node2 := &core.Node{ID: "neighbor-2", Type: "COMPANY"}
	
	err = repo.CreateNode(ctx, node1)
	require.NoError(t, err)
	err = repo.CreateNode(ctx, node2)
	require.NoError(t, err)
	
	// Create edge
	edge := &core.Edge{
		ID:     "neighbor-edge-1",
		Type:   "KNOWS",
		Source: "neighbor-1",
		Target: "neighbor-2",
	}
	err = repo.CreateEdge(ctx, edge)
	require.NoError(t, err)
	
	// Get neighbors
	nodes, edges, err := repo.GetNeighbors(ctx, "neighbor-1", 1, 10)
	assert.NoError(t, err)
	assert.NotEmpty(t, nodes)
	assert.NotEmpty(t, edges)
}

func TestGraphRepository_SyncMode(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_graph_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	graphStore, err := gograph.NewGraphStore(tmpDir + "/graph.db")
	require.NoError(t, err)
	defer graphStore.Close(ctx)
	
	vecStore, err := govector.NewStore(
		govector.WithDBPath(tmpDir+"/vectors.db"),
		govector.WithDimension(128),
		govector.WithCollection("test"),
	)
	require.NoError(t, err)
	defer vecStore.Close(ctx)
	
	repo := NewGraphRepository(
		graphStore,
		vecStore,
		nil,
		&mockEmbedder{},
		logging.DefaultNoopLogger(),
	)
	
	// Cast to concrete type to access WithSyncMode
	if graphRepo, ok := repo.(*graphRepository); ok {
		graphRepo.WithSyncMode(SyncGraphWithVector)
	}
	
	// Create node with vector sync
	node := &core.Node{
		ID:   "sync-node-1",
		Type: "PERSON",
		Properties: map[string]any{
			"content": "This node will be vectorized",
		},
	}
	
	err = repo.CreateNode(ctx, node)
	assert.NoError(t, err)
}
