package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInMemorySemanticCache_New(t *testing.T) {
	// Test creating a new semantic cache
	cache := NewInMemorySemanticCache()
	assert.NotNil(t, cache)
	assert.Empty(t, cache.entries)
}

func TestInMemorySemanticCache_SetAndGet(t *testing.T) {
	// Create a new semantic cache
	cache := NewInMemorySemanticCache()

	// Create test embeddings and response
	embedding1 := []float32{0.1, 0.2, 0.3}
	response1 := "Response 1"
	embedding2 := []float32{0.4, 0.5, 0.6}
	response2 := "Response 2"

	// Set the first entry
	ctx := context.Background()
	err := cache.Set(ctx, embedding1, response1)
	assert.NoError(t, err)

	// Set the second entry
	err = cache.Set(ctx, embedding2, response2)
	assert.NoError(t, err)

	// Check that entries were added
	assert.Len(t, cache.entries, 2)

	// Test getting with exact match
	response, found, err := cache.Get(ctx, embedding1, 0.9)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, response1, response)

	// Test getting with similar embedding
	similarEmbedding := []float32{0.11, 0.21, 0.31} // Very similar to embedding1
	response, found, err = cache.Get(ctx, similarEmbedding, 0.9)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, response1, response)

	// Test getting with dissimilar embedding
	dissimilarEmbedding := []float32{-0.9, -0.8, -0.7} // Very different (negative values)
	response, found, err = cache.Get(ctx, dissimilarEmbedding, 0.9)
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, response)

	// Test getting with lower threshold
	// First, let's calculate the actual similarity
	sim1 := cosineSimilarity(dissimilarEmbedding, embedding1)
	sim2 := cosineSimilarity(dissimilarEmbedding, embedding2)
	highestSim := sim1
	if sim2 > sim1 {
		highestSim = sim2
	}
	
	// Set a threshold lower than the highest similarity
	threshold := highestSim - 0.1
	response, found, err = cache.Get(ctx, dissimilarEmbedding, threshold)
	assert.NoError(t, err)
	assert.True(t, found)
	// Should return the most similar entry
	if sim1 > sim2 {
		assert.Equal(t, response1, response)
	} else {
		assert.Equal(t, response2, response)
	}
}

func TestInMemorySemanticCache_Get_EmptyCache(t *testing.T) {
	// Create a new semantic cache
	cache := NewInMemorySemanticCache()

	// Test getting from empty cache
	embedding := []float32{0.1, 0.2, 0.3}
	response, found, err := cache.Get(context.Background(), embedding, 0.9)
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, response)
}

func TestCosineSimilarity(t *testing.T) {
	// Test cosine similarity with identical vectors
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0, 3.0}
	sim := cosineSimilarity(a, b)
	assert.InDelta(t, 1.0, sim, 0.000001)

	// Test cosine similarity with different vectors
	a = []float32{1.0, 0.0, 0.0}
	b = []float32{0.0, 1.0, 0.0}
	sim = cosineSimilarity(a, b)
	assert.InDelta(t, 0.0, sim, 0.000001)

	// Test cosine similarity with empty vectors
	a = []float32{}
	b = []float32{}
	sim = cosineSimilarity(a, b)
	assert.Equal(t, float32(0.0), sim)

	// Test cosine similarity with vectors of different lengths
	a = []float32{1.0, 2.0}
	b = []float32{1.0, 2.0, 3.0}
	sim = cosineSimilarity(a, b)
	assert.Equal(t, float32(0.0), sim)

	// Test cosine similarity with one zero vector
	a = []float32{1.0, 2.0, 3.0}
	b = []float32{0.0, 0.0, 0.0}
	sim = cosineSimilarity(a, b)
	assert.Equal(t, float32(0.0), sim)
}
