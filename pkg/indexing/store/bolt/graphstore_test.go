package bolt

import (
	"context"
	"os"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

func TestBoltGraphStore(t *testing.T) {
	dbPath := "test_graph.bolt"
	defer os.Remove(dbPath)

	store, err := NewGraphStore(dbPath)
	assert.NoError(t, err)
	defer store.Close(context.Background())

	ctx := context.Background()

	// Test UpsertNodes
	nodes := []*core.Node{
		{ID: "1", Type: "PERSON", Properties: map[string]any{"name": "Alice"}},
		{ID: "2", Type: "PERSON", Properties: map[string]any{"name": "Bob"}},
		{ID: "3", Type: "ORGANIZATION", Properties: map[string]any{"name": "TechCorp"}},
	}
	err = store.UpsertNodes(ctx, nodes)
	assert.NoError(t, err)

	// Test GetNode
	node, err := store.GetNode(ctx, "1")
	assert.NoError(t, err)
	assert.NotNil(t, node)
	assert.Equal(t, "Alice", node.Properties["name"])

	// Test UpsertEdges
	edges := []*core.Edge{
		{ID: "e1", Type: "KNOWS", Source: "1", Target: "2", Properties: map[string]any{"since": "2020"}},
		{ID: "e2", Type: "WORKS_AT", Source: "1", Target: "3", Properties: map[string]any{"role": "Engineer"}},
	}
	err = store.UpsertEdges(ctx, edges)
	assert.NoError(t, err)

	// Test GetNeighbors
	neighborNodes, neighborEdges, err := store.GetNeighbors(ctx, "1", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, neighborNodes, 2)
	assert.Len(t, neighborEdges, 2)

	// Verify neighbors contain Bob and TechCorp
	foundBob := false
	foundTechCorp := false
	for _, n := range neighborNodes {
		if n.ID == "2" {
			foundBob = true
		}
		if n.ID == "3" {
			foundTechCorp = true
		}
	}
	assert.True(t, foundBob)
	assert.True(t, foundTechCorp)

	// Test depth 2
	nodes2 := []*core.Node{
		{ID: "4", Type: "PERSON", Properties: map[string]any{"name": "Charlie"}},
	}
	store.UpsertNodes(ctx, nodes2)
	edges2 := []*core.Edge{
		{ID: "e3", Type: "KNOWS", Source: "2", Target: "4", Properties: map[string]any{"since": "2021"}},
	}
	store.UpsertEdges(ctx, edges2)

	neighborNodes2, _, err := store.GetNeighbors(ctx, "1", 2, 10)
	assert.NoError(t, err)
	assert.Len(t, neighborNodes2, 3) // Bob, TechCorp, Charlie

	foundCharlie := false
	for _, n := range neighborNodes2 {
		if n.ID == "4" {
			foundCharlie = true
		}
	}
	assert.True(t, foundCharlie)
}
