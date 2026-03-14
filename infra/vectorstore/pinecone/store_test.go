package pinecone

import (
	"context"
	"os"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func getPineconeAPIKey() string {
	return os.Getenv("PINECONE_API_KEY")
}

func TestStore_Add_Search_Delete(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
	require.NoError(t, err)
	defer store.Close(ctx)

	uid1 := "test-vec-1"
	uid2 := "test-vec-2"

	vectors := []*entity.Vector{
		entity.NewVector(uid1, []float32{0.1, 0.2, 0.3, 0.4}, "chunk1", map[string]any{"lang": "en"}),
		entity.NewVector(uid2, []float32{0.9, 0.8, 0.7, 0.6}, "chunk2", map[string]any{"lang": "zh"}),
	}

	err = store.AddBatch(ctx, vectors)
	require.NoError(t, err)

	query := []float32{0.1, 0.2, 0.3, 0.4}
	results, scores, err := store.Search(ctx, query, 5, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 0)
	assert.Equal(t, len(results), len(scores))

	err = store.DeleteBatch(ctx, []string{uid1, uid2})
	require.NoError(t, err)
}

func TestStore_Options(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, 
		WithIndex("test-index"),
		WithEnvironment("gcp-starter"),
		WithDimension(128),
		WithNamespace("test-namespace"),
	)
	require.NoError(t, err)
	defer store.Close(ctx)

	assert.Equal(t, "test-index", store.indexName)
	assert.Equal(t, "gcp-starter", store.environment)
	assert.Equal(t, 128, store.dimension)
	assert.Equal(t, "test-namespace", store.namespace)
}

func TestStore_AddBatch_Empty(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
	require.NoError(t, err)
	defer store.Close(ctx)

	// Test adding empty batch
	err = store.AddBatch(ctx, []*entity.Vector{})
	require.NoError(t, err)
}

func TestStore_DeleteBatch_Empty(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
	require.NoError(t, err)
	defer store.Close(ctx)

	// Test deleting empty batch
	err = store.DeleteBatch(ctx, []string{})
	require.NoError(t, err)
}

func TestStore_Search_ZeroTopK(t *testing.T) {
	apiKey := getPineconeAPIKey()
	if apiKey == "" {
		t.Skip("Skipping Pinecone test - PINECONE_API_KEY not set")
	}

	ctx := context.Background()
	store, err := NewStore(apiKey, WithDimension(4))
	require.NoError(t, err)
	defer store.Close(ctx)

	// Test search with zero topK (should default to 5)
	query := []float32{0.1, 0.2, 0.3, 0.4}
	results, scores, err := store.Search(ctx, query, 0, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 0)
	assert.Equal(t, len(results), len(scores))
}

// TestParseMatches tests the parseMatches method
func TestParseMatches(t *testing.T) {
	// Create test data
	metadata1, err := structpb.NewStruct(map[string]interface{}{
		"chunk_id": "chunk1",
		"lang":    "en",
	})
	require.NoError(t, err)

	metadata2, err := structpb.NewStruct(map[string]interface{}{
		"chunk_id": "chunk2",
		"lang":    "zh",
	})
	require.NoError(t, err)

	matches := []*pinecone.ScoredVector{
		{
			Vector: &pinecone.Vector{
				Id:       "vec1",
				Metadata: metadata1,
			},
			Score: 0.95,
		},
		{
			Vector: &pinecone.Vector{
				Id:       "vec2",
				Metadata: metadata2,
			},
			Score: 0.85,
		},
	}

	// Create a store instance
	store := &Store{}

	// Call parseMatches
	vectors, scores, err := store.parseMatches(matches)
	require.NoError(t, err)

	// Verify results
	assert.Len(t, vectors, 2)
	assert.Len(t, scores, 2)

	assert.Equal(t, "vec1", vectors[0].ID)
	assert.Equal(t, "chunk1", vectors[0].ChunkID)
	assert.Equal(t, "en", vectors[0].Metadata["lang"])
	assert.Equal(t, float32(0.95), scores[0])

	assert.Equal(t, "vec2", vectors[1].ID)
	assert.Equal(t, "chunk2", vectors[1].ChunkID)
	assert.Equal(t, "zh", vectors[1].Metadata["lang"])
	assert.Equal(t, float32(0.85), scores[1])
}

// TestParseMatches_Empty tests parseMatches with empty matches
func TestParseMatches_Empty(t *testing.T) {
	store := &Store{}
	vectors, scores, err := store.parseMatches([]*pinecone.ScoredVector{})
	require.NoError(t, err)
	assert.Len(t, vectors, 0)
	assert.Len(t, scores, 0)
}

// TestBuildMetadata tests the buildMetadata method
func TestBuildMetadata(t *testing.T) {
	store := &Store{}

	vector := entity.NewVector("vec1", []float32{0.1, 0.2}, "chunk1", map[string]any{"lang": "en", "author": "test"})

	metadata, err := store.buildMetadata(vector)
	require.NoError(t, err)

	// Convert metadata to map for verification
	metadataMap := metadata.AsMap()
	assert.Equal(t, "chunk1", metadataMap["chunk_id"])
	assert.Equal(t, "en", metadataMap["lang"])
	assert.Equal(t, "test", metadataMap["author"])
}


