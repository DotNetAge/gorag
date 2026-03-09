package weaviate

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcweaviate "github.com/testcontainers/testcontainers-go/modules/weaviate"
)

func setupWeaviateContainer(t *testing.T) (string, func()) {
	ctx := context.Background()

	container, err := tcweaviate.Run(ctx, "semitechnologies/weaviate:latest")
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "8080")
	require.NoError(t, err)

	endpoint := host + ":" + port.Port()

	cleanup := func() {
		container.Terminate(ctx)
	}

	return endpoint, cleanup
}

func TestNewStore(t *testing.T) {
	endpoint, cleanup := setupWeaviateContainer(t)
	defer cleanup()

	store, err := NewStore(endpoint, "")
	require.NoError(t, err)
	defer store.Close()

	assert.NotNil(t, store)
	assert.Equal(t, "GoRAG", store.collection)
	assert.Equal(t, 1536, store.dimension)
}

func TestNewStore_WithOptions(t *testing.T) {
	endpoint, cleanup := setupWeaviateContainer(t)
	defer cleanup()

	store, err := NewStore(endpoint, "",
		WithCollection("TestCollection"),
		WithDimension(768),
	)
	require.NoError(t, err)
	defer store.Close()

	assert.Equal(t, "TestCollection", store.collection)
	assert.Equal(t, 768, store.dimension)
}

func TestStore_Add(t *testing.T) {
	endpoint, cleanup := setupWeaviateContainer(t)
	defer cleanup()

	ctx := context.Background()
	store, err := NewStore(endpoint, "", WithDimension(4))
	require.NoError(t, err)
	defer store.Close()

	// Test with empty chunks
	err = store.Add(ctx, []core.Chunk{}, [][]float32{})
	require.NoError(t, err)

	// Test with mismatched lengths
	err = store.Add(ctx, []core.Chunk{{ID: "1", Content: "test"}}, [][]float32{})
	require.NoError(t, err)

	// Test with valid data
	chunks := []core.Chunk{
		{ID: "1", Content: "hello world", Metadata: map[string]string{"lang": "en"}},
		{ID: "2", Content: "你好世界", Metadata: map[string]string{"lang": "zh"}},
	}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
	}
	err = store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)
}

func TestStore_Search(t *testing.T) {
	endpoint, cleanup := setupWeaviateContainer(t)
	defer cleanup()

	ctx := context.Background()
	store, err := NewStore(endpoint, "", WithDimension(4))
	require.NoError(t, err)
	defer store.Close()

	// Add test data
	chunks := []core.Chunk{
		{ID: "1", Content: "hello world"},
		{ID: "2", Content: "goodbye world"},
	}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.9, 0.8, 0.7, 0.6},
	}
	err = store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	// Search
	query := []float32{0.1, 0.2, 0.3, 0.4}
	results, err := store.Search(ctx, query, vectorstore.SearchOptions{TopK: 5})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}

func TestStore_Delete(t *testing.T) {
	endpoint, cleanup := setupWeaviateContainer(t)
	defer cleanup()

	ctx := context.Background()
	store, err := NewStore(endpoint, "", WithDimension(4))
	require.NoError(t, err)
	defer store.Close()

	// Test with empty IDs
	err = store.Delete(ctx, []string{})
	require.NoError(t, err)

	// Add and delete
	chunks := []core.Chunk{{ID: "1", Content: "test"}}
	embeddings := [][]float32{{0.1, 0.2, 0.3, 0.4}}
	err = store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	err = store.Delete(ctx, []string{"1"})
	require.NoError(t, err)
}

func TestStore_Close(t *testing.T) {
	endpoint, cleanup := setupWeaviateContainer(t)
	defer cleanup()

	store, err := NewStore(endpoint, "")
	require.NoError(t, err)

	err = store.Close()
	require.NoError(t, err)
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
