package govector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/vectorstore"
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
	defer store.Close()

	// 1. Add vectors
	chunks := []core.Chunk{
		{
			ID:      "1",
			Content: "Go programming language",
			Metadata: map[string]string{
				"source": "doc1",
			},
		},
		{
			ID:      "2",
			Content: "Python programming language",
			Metadata: map[string]string{
				"source": "doc2",
			},
		},
	}

	embeddings := [][]float32{
		{1.0, 0.0, 0.0},
		{0.0, 1.0, 0.0},
	}

	err = store.Add(ctx, chunks, embeddings)
	assert.NoError(t, err)

	// 2. Search
	query := []float32{0.9, 0.1, 0.0} // Close to Go
	opts := vectorstore.SearchOptions{
		TopK:     1,
		MinScore: 0.5,
	}

	results, err := store.Search(ctx, query, opts)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "1", results[0].Chunk.ID)
	assert.Equal(t, "Go programming language", results[0].Chunk.Content)
	assert.Equal(t, "doc1", results[0].Chunk.Metadata["source"])

	// 3. Search by Metadata
	meta := map[string]string{"source": "doc2"}
	chunksResult, err := store.SearchByMetadata(ctx, meta)
	assert.NoError(t, err)
	assert.Len(t, chunksResult, 1)
	assert.Equal(t, "2", chunksResult[0].ID)
	assert.Equal(t, "Python programming language", chunksResult[0].Content)

	// 4. Delete
	err = store.Delete(ctx, []string{"1"})
	assert.NoError(t, err)

	// Verify deletion
	opts.TopK = 5
	opts.MinScore = 0.0 // Allow everything to match
	results, err = store.Search(ctx, query, opts)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "2", results[0].Chunk.ID) // Only 2 should remain
}
