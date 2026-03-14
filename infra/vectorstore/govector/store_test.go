package govector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestGoVectorStore(t *testing.T) {
	// Create a temporary database for testing
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_vectors.db")
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Initialize the store
	store, err := NewStore(ctx,
		WithDBPath(dbPath),
		WithDimension(3),
		WithCollection("test_col"),
		WithHNSW(false), // use flat index for simple testing
	)
	assert.NoError(t, err)
	defer store.Close(ctx)

	// 1. Add vectors
	vectors := []*entity.Vector{
		entity.NewVector("1", []float32{1.0, 0.0, 0.0}, "chunk1", map[string]any{"source": "doc1"}),
		entity.NewVector("2", []float32{0.0, 1.0, 0.0}, "chunk2", map[string]any{"source": "doc2"}),
	}

	err = store.AddBatch(ctx, vectors)
	assert.NoError(t, err)

	// 2. Search
	query := []float32{0.9, 0.1, 0.0} // Close to Go
	
	results, scores, err := store.Search(ctx, query, 1, nil)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "1", results[0].ID)
	assert.Equal(t, "chunk1", results[0].ChunkID)
	assert.Equal(t, "doc1", results[0].Metadata["source"])
	assert.Len(t, scores, 1)

	// 3. Search by Metadata (Filter)
	filter := map[string]any{"source": "doc2"}
	// A dummy zero query works as long as there is a filter in a flat scan
	resultsFilter, _, err := store.Search(ctx, []float32{0, 0, 0}, 1, filter)
	assert.NoError(t, err)
	assert.Len(t, resultsFilter, 1)
	assert.Equal(t, "2", resultsFilter[0].ID)

	// 4. Delete
	err = store.Delete(ctx, "1")
	assert.NoError(t, err)

	// Verify deletion
	results, _, err = store.Search(ctx, query, 5, nil)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "2", results[0].ID) // Only 2 should remain
}
