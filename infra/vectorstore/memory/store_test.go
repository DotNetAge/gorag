package memory

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	require.NotNil(t, store)
	assert.NotNil(t, store.vectors)
	assert.NotNil(t, store.norms)
}

func TestStore_AddBatch(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	vectors := []*entity.Vector{
		entity.NewVector(uuid.New().String(), []float32{0.1, 0.2, 0.3}, "chunk1", map[string]any{"source": "test1"}),
		entity.NewVector(uuid.New().String(), []float32{0.4, 0.5, 0.6}, "chunk2", map[string]any{"source": "test2"}),
	}

	err := store.AddBatch(ctx, vectors)
	require.NoError(t, err)

	store.mu.RLock()
	defer store.mu.RUnlock()

	for _, v := range vectors {
		assert.Contains(t, store.vectors, v.ID)
		assert.Equal(t, v, store.vectors[v.ID])
		assert.Contains(t, store.norms, v.ID)
		assert.Greater(t, store.norms[v.ID], float32(0))
	}
}

func TestStore_Search(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	vectors := []*entity.Vector{
		entity.NewVector("1", []float32{1.0, 0.0, 0.0}, "chunk1", map[string]any{"category": "fruit"}),
		entity.NewVector("2", []float32{0.9, 0.1, 0.0}, "chunk2", map[string]any{"category": "fruit"}),
		entity.NewVector("3", []float32{0.0, 1.0, 0.0}, "chunk3", map[string]any{"category": "animal"}),
	}

	err := store.AddBatch(ctx, vectors)
	require.NoError(t, err)

	// 1. Basic Similarity Search
	query := []float32{0.95, 0.05, 0.0}
	results, scores, err := store.Search(ctx, query, 2, nil)
	require.NoError(t, err)

	assert.Len(t, results, 2)
	assert.Len(t, scores, 2)
	assert.Equal(t, "1", results[0].ID) // Apple should be first
	assert.Equal(t, "2", results[1].ID) // Banana should be second
	assert.Greater(t, scores[0], scores[1])

	// 2. Search with Metadata Filter
	filter := map[string]any{"category": "animal"}
	results, scores, err = store.Search(ctx, query, 2, filter)
	require.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "3", results[0].ID) // Only Dog has category=animal
}

func TestStore_Delete(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	vectors := []*entity.Vector{
		entity.NewVector("1", []float32{0.1, 0.2}, "c1", nil),
		entity.NewVector("2", []float32{0.4, 0.5}, "c2", nil),
		entity.NewVector("3", []float32{0.7, 0.8}, "c3", nil),
	}

	err := store.AddBatch(ctx, vectors)
	require.NoError(t, err)

	// Delete single
	err = store.Delete(ctx, "2")
	require.NoError(t, err)

	store.mu.RLock()
	assert.Len(t, store.vectors, 2)
	assert.NotContains(t, store.vectors, "2")
	store.mu.RUnlock()

	// Delete batch
	err = store.DeleteBatch(ctx, []string{"1", "3"})
	require.NoError(t, err)

	store.mu.RLock()
	assert.Empty(t, store.vectors)
	store.mu.RUnlock()
}

func TestComputeNorm(t *testing.T) {
	result := computeNorm([]float32{3, 4, 0})
	assert.InDelta(t, 5.0, result, 0.0001) // sqrt(3^2 + 4^2) = 5
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	normA := computeNorm(a)
	normB := computeNorm(b)
	
	result := cosineSimilarity(a, b, normA, normB)
	assert.InDelta(t, 1.0, result, 0.0001)
}
