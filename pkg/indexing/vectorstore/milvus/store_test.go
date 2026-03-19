package milvus

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/milvus"
)

func setupMilvusContainer(t *testing.T) (string, func()) {
	ctx := context.Background()

	container, err := milvus.Run(ctx, "milvusdb/milvus:latest")
	if err != nil {
		t.Skip("Skipping Milvus test - Docker/Testcontainers not available: ", err)
		return "", func() {}
	}

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "19530")
	require.NoError(t, err)

	endpoint := host + ":" + port.Port()

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate milvus container: %v", err)
		}
	}

	return endpoint, cleanup
}

func TestStore_Add_Search_Delete(t *testing.T) {
	endpoint, cleanup := setupMilvusContainer(t)
	defer cleanup()
	if endpoint == "" {
		return // Skipped
	}

	ctx := context.Background()
	store, err := NewStore(ctx, endpoint, WithDimension(4))
	require.NoError(t, err)
	defer store.Close(ctx)

	uid1 := uuid.New().String()
	uid2 := uuid.New().String()

	vectors := []*core.Vector{
		core.NewVector(uid1, []float32{0.1, 0.2, 0.3, 0.4}, "chunk1", map[string]any{"lang": "en"}),
		core.NewVector(uid2, []float32{0.9, 0.8, 0.7, 0.6}, "chunk2", map[string]any{"lang": "zh"}),
	}

	// Test AddBatch
	err = store.AddBatch(ctx, vectors)
	require.NoError(t, err)

	// Test Search
	query := []float32{0.1, 0.2, 0.3, 0.4}
	results, scores, err := store.Search(ctx, query, 5, nil)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(results), 1)
	if len(results) > 0 {
		// Milvus returns results ordered by distance (L2 by default, smaller is better)
		assert.Equal(t, uid1, results[0].ID)
		assert.Equal(t, "chunk1", results[0].ChunkID)
	}
	assert.GreaterOrEqual(t, len(scores), 1)

	// Test Delete
	err = store.Delete(ctx, uid1)
	require.NoError(t, err)
}
