package vectorstore

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/integration_test/testcontainers"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/DotNetAge/gorag/vectorstore/milvus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMilvusStore tests the Milvus vector store integration
func TestMilvusStore(t *testing.T) {
	// Skip integration test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Create Milvus container
	container, err := testcontainers.NewMilvusContainer(t)
	require.NoError(t, err)
	defer container.Terminate(context.Background())

	// Initialize Milvus store with dimension 3 for testing
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := milvus.NewStore(ctx,
		container.Host+":"+container.Port,
		milvus.WithCollection("test"),
		milvus.WithDimension(3),
	)
	require.NoError(t, err)
	defer store.Close()

	// Test adding vectors
	chunks := []vectorstore.Chunk{
		{ID: "1", Content: "test content 1"},
		{ID: "2", Content: "test content 2"},
	}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}

	err = store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	// Test searching - wait a bit for data to be indexed
	time.Sleep(2 * time.Second)

	results, err := store.Search(ctx,
		[]float32{0.1, 0.2, 0.3},
		vectorstore.SearchOptions{TopK: 2},
	)
	require.NoError(t, err)
	t.Logf("Search results: %d items", len(results))
	for i, r := range results {
		t.Logf("  Result %d: ID=%s, Content=%s, Score=%f", i, r.ID, r.Content, r.Score)
	}
	assert.GreaterOrEqual(t, len(results), 1)

	// Test deleting
	err = store.Delete(ctx, []string{"1"})
	require.NoError(t, err)
}
