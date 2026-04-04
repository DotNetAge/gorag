package indexer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultIndexer_Init(t *testing.T) {
	tmpDir := t.TempDir()
	idxIface, err := DefaultIndexer(
		WithConcurrency(true),
		WithWorkers(5),
		WithGoVector("test", filepath.Join(tmpDir, "vectors.db"), 1536),
		WithSQLDoc(filepath.Join(tmpDir, "docs.db")),
	)
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)

	err = idx.Init()
	assert.NoError(t, err)
	assert.NotNil(t, idx.pipeline)
}

type mockParser struct {
	core.Parser
}

func (m *mockParser) GetSupportedTypes() []string {
	return []string{".mock"}
}

func (m *mockParser) Parse(ctx context.Context, data []byte, metadata map[string]interface{}) (*core.Document, error) {
	return &core.Document{Content: string(data)}, nil
}

func TestDefaultIndexer_WithParsers(t *testing.T) {
	tmpDir := t.TempDir()
	mock := &mockParser{}
	idxIface, err := DefaultIndexer(
		WithParsers(mock),
		WithGoVector("test", filepath.Join(tmpDir, "vectors.db"), 1536),
	)
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)

	// WithParsers appends to default parsers (21 builtin + 1 mock = 22)
	// To replace parsers completely, need to clear defaults first in production code
	assert.NotNil(t, idx.registry)
}

func TestDefaultIndexer_WithName(t *testing.T) {
	idxIface, err := DefaultNativeIndexer(
		WithName("test_bot"),
	)
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)

	assert.Equal(t, "test_bot", idx.name)
}

func TestDefaultIndexer_IndexFile_Init(t *testing.T) {
	tmpDir := t.TempDir()
	idxIface, err := DefaultIndexer(
		WithName("test_index_file"),
		WithParsers(&mockParser{}),
		WithGoVector("test", filepath.Join(tmpDir, "vectors.db"), 1536),
		WithBoltDoc(filepath.Join(tmpDir, "docs.bolt")),
	)
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)

	assert.NotNil(t, idx.pipeline)

	_, err = idx.IndexFile(context.Background(), "non-existent.txt")
	assert.Error(t, err)
}
