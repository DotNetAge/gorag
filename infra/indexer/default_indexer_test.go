package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/infra/vectorstore"
	"github.com/stretchr/testify/assert"
)

func TestDefaultIndexer(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create vector store
	dbPath := filepath.Join(tmpDir, "govector")
	store, err := vectorstore.DefaultVectorStore(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, store)

	// Create indexer with options
	indexer := DefaultIndexer(
		WithAllParsers(),
		WithWatchDir(tmpDir),
		WithStore(store),
	)

	assert.NotNil(t, indexer)

	// Test Init
	defaultIdx, ok := indexer.(*defaultIndexer)
	assert.True(t, ok)

	err = defaultIdx.Init()
	assert.NoError(t, err)
	assert.NotNil(t, defaultIdx.logger)
	assert.NotNil(t, defaultIdx.embedder)
	assert.NotNil(t, defaultIdx.vectorStore)
}

func TestWithParsers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexer := DefaultIndexer(
		WithParsers(), // Empty parsers
		WithWatchDir(tmpDir),
	)

	defaultIdx, ok := indexer.(*defaultIndexer)
	assert.True(t, ok)
	assert.Nil(t, defaultIdx.parser) // Should be nil when no parsers provided
}

func TestWithWatchDir(t *testing.T) {
	tmpDir1, err := os.MkdirTemp("", "indexer-test-1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir1)

	tmpDir2, err := os.MkdirTemp("", "indexer-test-2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir2)

	indexer := DefaultIndexer(
		WithAllParsers(),
		WithWatchDir(tmpDir1, tmpDir2),
	)

	defaultIdx, ok := indexer.(*defaultIndexer)
	assert.True(t, ok)
	assert.Equal(t, 2, len(defaultIdx.watchDirs))
	assert.Contains(t, defaultIdx.watchDirs, tmpDir1)
	assert.Contains(t, defaultIdx.watchDirs, tmpDir2)
}

func TestWithEmbedding(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "indexer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	embedder, err := newTestEmbedder()
	assert.NoError(t, err)

	indexer := DefaultIndexer(
		WithAllParsers(),
		WithWatchDir(tmpDir),
		WithEmbedding(embedder),
	)

	defaultIdx, ok := indexer.(*defaultIndexer)
	assert.True(t, ok)
	assert.Equal(t, embedder, defaultIdx.embedder)
}

// testEmbedder is a simple embedder for testing
type testEmbedder struct{}

func newTestEmbedder() (*testEmbedder, error) {
	return &testEmbedder{}, nil
}

func (e *testEmbedder) Embed(ctx context.Context, docs []string) ([][]float32, error) {
	result := make([][]float32, len(docs))
	for i := range docs {
		result[i] = []float32{1.0, 2.0, 3.0}
	}
	return result, nil
}

func (e *testEmbedder) EmbedDocuments(ctx context.Context, docs []string) ([][]float32, error) {
	result := make([][]float32, len(docs))
	for i := range docs {
		result[i] = []float32{1.0, 2.0, 3.0}
	}
	return result, nil
}

func (e *testEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return []float32{1.0, 2.0, 3.0}, nil
}

func (e *testEmbedder) Dimension() int {
	return 3
}
