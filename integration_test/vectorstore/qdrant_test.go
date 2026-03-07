package vectorstore

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/integration_test/testcontainers"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/DotNetAge/gorag/vectorstore/qdrant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQdrantStore tests the Qdrant vector store integration
func TestQdrantStore(t *testing.T) {
	// Skip integration test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Create Qdrant container
	container, err := testcontainers.NewQdrantContainer(t)
	require.NoError(t, err)
	defer container.Terminate(context.Background())

	// Initialize Qdrant store with dimension 3 for testing
	grpcPort, err := strconv.Atoi(container.GRPCPort)
	require.NoError(t, err)

	store, err := qdrant.NewStore(context.Background(),
		container.Host,
		qdrant.WithCollection("test"),
		qdrant.WithDimension(3),
		qdrant.WithPort(grpcPort),
	)
	require.NoError(t, err)
	defer store.Close()

	// Test adding vectors
	chunks := []core.Chunk{
		{ID: "00000000-0000-0000-0000-000000000001", Content: "test content 1"},
		{ID: "00000000-0000-0000-0000-000000000002", Content: "test content 2"},
	}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}

	err = store.Add(context.Background(), chunks, embeddings)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Test searching
	results, err := store.Search(context.Background(),
		[]float32{0.1, 0.2, 0.3},
		vectorstore.SearchOptions{TopK: 2},
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)

	// Test deleting
	err = store.Delete(context.Background(), []string{"00000000-0000-0000-0000-000000000001"})
	require.NoError(t, err)
}
