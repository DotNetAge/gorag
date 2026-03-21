package indexer

import (
	"context"
	"testing"
	"path/filepath"

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

	assert.Equal(t, 1, len(idx.parsers))
}

func TestDefaultIndexer_IndexFile_Init(t *testing.T) {
	tmpDir := t.TempDir()
	idxIface, err := DefaultIndexer(
		WithParsers(&mockParser{}),
		WithGoVector("test", filepath.Join(tmpDir, "vectors.db"), 1536),
	)
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)
	
	assert.NotNil(t, idx.pipeline)

	_, err = idx.IndexFile(context.Background(), "non-existent.txt")
	assert.Error(t, err) 
}
