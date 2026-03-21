package indexer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultIndexer_Init(t *testing.T) {
	idxIface, err := DefaultIndexer(
		WithConcurrency(true),
		WithWorkers(5),
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

func TestDefaultIndexer_WithParsers(t *testing.T) {
	mock := &mockParser{}
	// Fast test with specific parsers
	idxIface, err := DefaultIndexer(WithParsers(mock))
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)

	assert.Equal(t, 1, len(idx.parsers))
}

func TestDefaultIndexer_IndexFile_Init(t *testing.T) {
	idxIface, err := DefaultIndexer()
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)
	
	assert.NotNil(t, idx.pipeline)

	_, err = idx.IndexFile(context.Background(), "non-existent.txt")
	assert.Error(t, err) 
}
