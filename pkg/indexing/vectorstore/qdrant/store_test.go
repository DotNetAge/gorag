package qdrant

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/qdrant"
)

func setupQdrantContainer(t *testing.T) (string, func()) {
	ctx := context.Background()

	container, err := qdrant.Run(ctx, "qdrant/qdrant:latest")
	if err != nil {
		t.Skip("Skipping Qdrant test - Docker/Testcontainers not available: ", err)
		return "", func() {}
	}

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6334")
	require.NoError(t, err)

	endpoint := host + ":" + port.Port()

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate qdrant container: %v", err)
		}
	}

	return endpoint, cleanup
}

func TestStore_Add_Search_Delete(t *testing.T) {
	endpoint, cleanup := setupQdrantContainer(t)
	defer cleanup()
	if endpoint == "" {
		return // Skipped
	}

	ctx := context.Background()
	// Create a 4-dimensional collection
	store, err := NewStore(ctx, endpoint, WithDimension(4))
	require.NoError(t, err)
	defer store.Close(ctx)

	// Qdrant requires properly formatted UUIDs when using IDUUID
	uid1 := "00000000-0000-0000-0000-000000000001"
	uid2 := "00000000-0000-0000-0000-000000000002"

	vectors := []*core.Vector{
		core.NewVector(uid1, []float32{0.1, 0.2, 0.3, 0.4}, "chunk1", map[string]any{"lang": "en"}),
		core.NewVector(uid2, []float32{0.9, 0.8, 0.7, 0.6}, "chunk2", map[string]any{"lang": "zh"}),
	}

	// Test AddBatch
	err = store.Upsert(ctx, vectors)
	require.NoError(t, err)

	// Test Search (without filter)
	query := []float32{0.1, 0.2, 0.3, 0.4}
	results, scores, err := store.Search(ctx, query, 5, nil)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(results), 1)
	if len(results) > 0 {
		assert.Equal(t, uid1, results[0].ID)
		assert.Equal(t, "chunk1", results[0].ChunkID)
		assert.Equal(t, "en", results[0].Metadata["lang"])
	}
	assert.GreaterOrEqual(t, len(scores), 1)

	// Test Search (with filter)
	filter := map[string]any{"lang": "zh"}
	resultsFilter, _, err := store.Search(ctx, query, 5, filter)
	require.NoError(t, err)
	assert.Len(t, resultsFilter, 1)
	assert.Equal(t, uid2, resultsFilter[0].ID)

	// Test Delete
	err = store.Delete(ctx, uid1)
	require.NoError(t, err)

	// Verify deletion
	resultsAfterDelete, _, _ := store.Search(ctx, query, 5, nil)
	// It shouldn't find uid1 anymore
	for _, res := range resultsAfterDelete {
		assert.NotEqual(t, uid1, res.ID)
	}
}
