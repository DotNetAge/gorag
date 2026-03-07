package pinecone

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	// This test would typically require a real Pinecone API key
	// For now, we'll skip it and focus on unit tests for other methods
	t.Skip("Skipping Pinecone connection test - requires API key")
}

func TestStore_Add(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test with empty chunks
	err := store.Add(context.Background(), []core.Chunk{}, [][]float32{})
	require.NoError(t, err)

	// Test with mismatched lengths
	err = store.Add(context.Background(), []core.Chunk{{ID: "1", Content: "test"}}, [][]float32{})
	require.NoError(t, err)

	// Test with valid data
	err = store.Add(context.Background(), 
		[]core.Chunk{{ID: "1", Content: "test"}}, 
		[][]float32{{0.1, 0.2}})
	require.NoError(t, err)
}

func TestStore_Search(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test search
	results, err := store.Search(context.Background(), []float32{0.1, 0.2}, vectorstore.SearchOptions{})
	require.NoError(t, err)
	assert.Empty(t, results)

	// Test search with custom options
	results, err = store.Search(context.Background(), []float32{0.1, 0.2}, vectorstore.SearchOptions{TopK: 10})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestStore_Delete(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test with empty IDs
	err := store.Delete(context.Background(), []string{})
	require.NoError(t, err)

	// Test with valid IDs
	err = store.Delete(context.Background(), []string{"1", "2"})
	require.NoError(t, err)
}

func TestStore_Close(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test close
	err := store.Close()
	require.NoError(t, err)
}

func TestOptions(t *testing.T) {
	// Test WithIndex option
	store := &Store{index: "default"}
	WithIndex("test")(store)
	assert.Equal(t, "test", store.index)

	// Test WithEnvironment option
	store = &Store{environment: "default"}
	WithEnvironment("gcp-starter")(store)
	assert.Equal(t, "gcp-starter", store.environment)

	// Test WithDimension option
	store = &Store{dimension: 1536}
	WithDimension(768)(store)
	assert.Equal(t, 768, store.dimension)
}
