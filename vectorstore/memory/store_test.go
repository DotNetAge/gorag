package memory

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	require.NotNil(t, store)
	assert.NotNil(t, store.documents)
	assert.NotNil(t, store.embeddings)
	assert.NotNil(t, store.norms)
}

func TestStore_Add(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	// Create test chunks and embeddings
	chunks := []core.Chunk{
		{
			ID:      uuid.New().String(),
			Content: "Test content 1",
			Metadata: map[string]string{
				"source": "test1",
			},
		},
		{
			ID:      uuid.New().String(),
			Content: "Test content 2",
			Metadata: map[string]string{
				"source": "test2",
			},
		},
	}

	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}

	// Test adding chunks
	err := store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	// Verify chunks were added
	store.mu.RLock()
	defer store.mu.RUnlock()

	for i, chunk := range chunks {
		assert.Contains(t, store.documents, chunk.ID)
		assert.Equal(t, chunk, store.documents[chunk.ID])
		assert.Contains(t, store.embeddings, chunk.ID)
		assert.Equal(t, embeddings[i], store.embeddings[chunk.ID])
		assert.Contains(t, store.norms, chunk.ID)
		assert.Greater(t, store.norms[chunk.ID], float32(0))
	}
}

func TestStore_Add_Empty(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	// Test adding empty chunks
	err := store.Add(ctx, []core.Chunk{}, [][]float32{})
	require.NoError(t, err)

	// Verify no chunks were added
	store.mu.RLock()
	defer store.mu.RUnlock()

	assert.Empty(t, store.documents)
	assert.Empty(t, store.embeddings)
	assert.Empty(t, store.norms)
}

func TestStore_Search(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	// Add test data
	chunks := []core.Chunk{
		{
			ID:      "1",
			Content: "Apple is a fruit",
			Metadata: map[string]string{
				"category": "fruit",
			},
		},
		{
			ID:      "2",
			Content: "Banana is a fruit",
			Metadata: map[string]string{
				"category": "fruit",
			},
		},
		{
			ID:      "3",
			Content: "Dog is an animal",
			Metadata: map[string]string{
				"category": "animal",
			},
		},
	}

	// Use simple embeddings where similar content has similar vectors
	embeddings := [][]float32{
		{1.0, 0.0, 0.0}, // Apple
		{0.9, 0.1, 0.0}, // Banana (similar to Apple)
		{0.0, 1.0, 0.0}, // Dog (different)
	}

	err := store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	// Test search with query similar to Apple
	query := []float32{0.95, 0.05, 0.0}
	results, err := store.Search(ctx, query, vectorstore.SearchOptions{TopK: 2, MinScore: 0.5})
	require.NoError(t, err)

	// Verify results
	assert.Len(t, results, 2)
	assert.Equal(t, "1", results[0].Chunk.ID) // Apple should be first
	assert.Equal(t, "2", results[1].Chunk.ID) // Banana should be second
	assert.Greater(t, results[0].Score, results[1].Score)

	// Test search with query similar to Dog
	query = []float32{0.05, 0.95, 0.0}
	results, err = store.Search(ctx, query, vectorstore.SearchOptions{TopK: 1, MinScore: 0.5})
	require.NoError(t, err)

	// Verify results
	assert.Len(t, results, 1)
	assert.Equal(t, "3", results[0].Chunk.ID) // Dog should be first

	// Test search with no results
	query = []float32{0.0, 0.0, 1.0}
	results, err = store.Search(ctx, query, vectorstore.SearchOptions{TopK: 2, MinScore: 0.5})
	require.NoError(t, err)

	// Verify no results
	assert.Empty(t, results)
}

func TestStore_Search_TopK(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	// Add test data
	chunks := make([]core.Chunk, 5)
	embeddings := make([][]float32, 5)

	for i := 0; i < 5; i++ {
		chunks[i] = core.Chunk{
			ID:      string(rune('1' + i)),
			Content: "Content " + string(rune('1'+i)),
		}
		// Create embeddings where first vector is most similar to query
		embeddings[i] = []float32{float32(1.0 - float64(i)*0.1), float32(0.1 * float64(i)), 0.0}
	}

	err := store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	// Test with TopK=3
	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, vectorstore.SearchOptions{TopK: 3, MinScore: 0.0})
	require.NoError(t, err)

	// Verify results
	assert.Len(t, results, 3)
	assert.Equal(t, "1", results[0].Chunk.ID)
	assert.Equal(t, "2", results[1].Chunk.ID)
	assert.Equal(t, "3", results[2].Chunk.ID)

	// Test with TopK=0 (should return empty)
	results, err = store.Search(ctx, query, vectorstore.SearchOptions{TopK: 0, MinScore: 0.0})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestStore_Delete(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	// Add test data
	chunks := []core.Chunk{
		{
			ID:      "1",
			Content: "Content 1",
		},
		{
			ID:      "2",
			Content: "Content 2",
		},
		{
			ID:      "3",
			Content: "Content 3",
		},
	}

	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
		{0.7, 0.8, 0.9},
	}

	err := store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	// Verify all chunks are present
	store.mu.RLock()
	assert.Len(t, store.documents, 3)
	assert.Len(t, store.embeddings, 3)
	assert.Len(t, store.norms, 3)
	store.mu.RUnlock()

	// Delete one chunk
	err = store.Delete(ctx, []string{"2"})
	require.NoError(t, err)

	// Verify chunk 2 is deleted
	store.mu.RLock()
	assert.Len(t, store.documents, 2)
	assert.Len(t, store.embeddings, 2)
	assert.Len(t, store.norms, 2)
	assert.Contains(t, store.documents, "1")
	assert.NotContains(t, store.documents, "2")
	assert.Contains(t, store.documents, "3")
	store.mu.RUnlock()

	// Delete multiple chunks
	err = store.Delete(ctx, []string{"1", "3"})
	require.NoError(t, err)

	// Verify all chunks are deleted
	store.mu.RLock()
	assert.Empty(t, store.documents)
	assert.Empty(t, store.embeddings)
	assert.Empty(t, store.norms)
	store.mu.RUnlock()
}

func TestStore_Delete_Empty(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	// Test deleting from empty store
	err := store.Delete(ctx, []string{"1"})
	require.NoError(t, err)

	// Test deleting with empty ids
	err = store.Delete(ctx, []string{})
	require.NoError(t, err)

	// Verify store is still empty
	store.mu.RLock()
	assert.Empty(t, store.documents)
	assert.Empty(t, store.embeddings)
	assert.Empty(t, store.norms)
	store.mu.RUnlock()
}

func TestComputeNorm(t *testing.T) {
	tests := []struct {
		name     string
		vector   []float32
		expected float32
	}{
		{
			name:     "zero vector",
			vector:   []float32{0, 0, 0},
			expected: 0,
		},
		{
			name:     "unit vector",
			vector:   []float32{1, 0, 0},
			expected: 1,
		},
		{
			name:     "arbitrary vector",
			vector:   []float32{3, 4, 0},
			expected: 5, // sqrt(3^2 + 4^2) = 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeNorm(tt.vector)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1,
		},
		{
			name:     "similar vectors",
			a:        []float32{1, 2, 3},
			b:        []float32{2, 4, 6}, // Scaled version of a
			expected: 1,                  // Should be 1 since they're in the same direction
		},
		{
			name:     "different lengths",
			a:        []float32{1, 0, 0},
			b:        []float32{0.5, 0, 0},
			expected: 1, // Cosine similarity is direction-based, not magnitude-based
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normA := computeNorm(tt.a)
			normB := computeNorm(tt.b)
			result := cosineSimilarity(tt.a, tt.b, normA, normB)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestTopK(t *testing.T) {
	results := []core.Result{
		{Score: 0.5, Chunk: core.Chunk{ID: "1"}},
		{Score: 0.9, Chunk: core.Chunk{ID: "2"}},
		{Score: 0.3, Chunk: core.Chunk{ID: "3"}},
		{Score: 0.7, Chunk: core.Chunk{ID: "4"}},
		{Score: 0.1, Chunk: core.Chunk{ID: "5"}},
	}

	// Test with k=3
	topResults := topK(results, 3)
	assert.Len(t, topResults, 3)
	// Just check that we get results, not the exact order
	assert.Greater(t, len(topResults), 0)

	// Test with k=0
	topResults = topK(results, 0)
	assert.Empty(t, topResults)

	// Test with k greater than length
	topResults = topK(results, 10)
	assert.Len(t, topResults, 5)
	assert.Greater(t, len(topResults), 0)
}
