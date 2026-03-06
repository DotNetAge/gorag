package weaviate

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	// This test would typically require a real Weaviate connection
	// For now, we'll skip it and focus on unit tests for other methods
	t.Skip("Skipping Weaviate connection test - requires running Weaviate server")
}

func TestStore_Add(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test with empty chunks
	err := store.Add(context.Background(), []vectorstore.Chunk{}, [][]float32{})
	require.NoError(t, err)

	// Test with mismatched lengths
	err = store.Add(context.Background(), []vectorstore.Chunk{{ID: "1", Content: "test"}}, [][]float32{})
	require.NoError(t, err)

	// Test with valid data requires a real Weaviate connection
	// Skip this test as it needs a real client
	t.Skip("Skipping test with valid data - requires running Weaviate server")
}

func TestStore_Search(t *testing.T) {
	// Test search requires a real Weaviate connection
	// Skip this test as it needs a real client
	t.Skip("Skipping search test - requires running Weaviate server")
}

func TestStore_Delete(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test with empty IDs
	err := store.Delete(context.Background(), []string{})
	require.NoError(t, err)

	// Test with valid IDs requires a real Weaviate connection
	// Skip this test as it needs a real client
	t.Skip("Skipping test with valid IDs - requires running Weaviate server")
}

func TestStore_Close(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test close
	err := store.Close()
	require.NoError(t, err)
}

func TestStore_ensureCollectionExists(t *testing.T) {
	// Test ensureCollectionExists requires a real Weaviate connection
	// Skip this test as it needs a real client
	t.Skip("Skipping ensureCollectionExists test - requires running Weaviate server")
}

func TestOptions(t *testing.T) {
	// Test WithCollection option
	store := &Store{collection: "default"}
	WithCollection("test")(store)
	assert.Equal(t, "test", store.collection)

	// Test WithDimension option
	store = &Store{dimension: 1536}
	WithDimension(768)(store)
	assert.Equal(t, 768, store.dimension)
}
