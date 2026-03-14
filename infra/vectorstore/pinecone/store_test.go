package pinecone

import (
	"context"
	"os"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
