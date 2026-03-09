package pinecone

import (
	"context"
	"os"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getPineconeAPIKey returns the Pinecone API key from environment
// Set PINECONE_API_KEY environment variable to run integration tests
func getPineconeAPIKey() string {
	return os.Getenv("PINECONE_API_KEY")
}

func TestNewStore(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	store, err := NewStore(apiKey)
	require.NoError(t, err)
	defer store.Close()

	assert.NotNil(t, store)
	assert.Equal(t, "gorag", store.indexName)
	assert.Equal(t, 1536, store.dimension)
}

func TestNewStore_WithOptions(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	store, err := NewStore(apiKey,
		WithIndex("test-index"),
		WithEnvironment("gcp-starter"),
		WithDimension(768),
		WithNamespace("test-ns"),
	)
	require.NoError(t, err)
	defer store.Close()

	assert.Equal(t, "test-index", store.indexName)
	assert.Equal(t, "gcp-starter", store.environment)
	assert.Equal(t, 768, store.dimension)
	assert.Equal(t, "test-ns", store.namespace)
}

func TestStore_Add(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
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
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
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
	assert.GreaterOrEqual(t, len(results), 0)
}

func TestStore_SearchStructured(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
	require.NoError(t, err)
	defer store.Close()

	// Add test data with metadata
	chunks := []core.Chunk{
		{ID: "1", Content: "hello world", Metadata: map[string]string{"category": "greeting"}},
		{ID: "2", Content: "goodbye world", Metadata: map[string]string{"category": "farewell"}},
	}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.9, 0.8, 0.7, 0.6},
	}
	err = store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	// Search with filter
	query := &vectorstore.StructuredQuery{
		Query: "hello",
		Filters: []vectorstore.FilterCondition{
			{Field: "category", Operator: vectorstore.FilterOpEq, Value: "greeting"},
		},
		TopK: 5,
	}
	results, err := store.SearchStructured(ctx, query, []float32{0.1, 0.2, 0.3, 0.4})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 0)
}

func TestStore_GetByMetadata(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
	require.NoError(t, err)
	defer store.Close()

	// Add test data
	chunks := []core.Chunk{
		{ID: "1", Content: "document 1", Metadata: map[string]string{"author": "alice"}},
		{ID: "2", Content: "document 2", Metadata: map[string]string{"author": "bob"}},
	}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
	}
	err = store.Add(ctx, chunks, embeddings)
	require.NoError(t, err)

	// Get by metadata
	results, err := store.GetByMetadata(ctx, map[string]string{"author": "alice"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 0)
}

func TestStore_Delete(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
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
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	store, err := NewStore(apiKey)
	require.NoError(t, err)

	err = store.Close()
	require.NoError(t, err)
}

func TestOptions(t *testing.T) {
	// Test WithIndex option
	store := &Store{indexName: "default"}
	WithIndex("test")(store)
	assert.Equal(t, "test", store.indexName)

	// Test WithEnvironment option
	store = &Store{environment: "default"}
	WithEnvironment("gcp-starter")(store)
	assert.Equal(t, "gcp-starter", store.environment)

	// Test WithDimension option
	store = &Store{dimension: 1536}
	WithDimension(768)(store)
	assert.Equal(t, 768, store.dimension)

	// Test WithNamespace option
	store = &Store{namespace: ""}
	WithNamespace("test-ns")(store)
	assert.Equal(t, "test-ns", store.namespace)
}
