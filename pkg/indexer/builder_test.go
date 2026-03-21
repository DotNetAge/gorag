package indexer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultIndexer_Init(t *testing.T) {
	idxIface, err := DefaultIndexer(t.TempDir(), 
		WithConcurrency(true),
		WithWorkers(5),
	)
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)

	err = idx.Init()
	assert.NoError(t, err)
	assert.NotNil(t, idx.pipeline)
	assert.True(t, idx.config.Concurrency)
	assert.Equal(t, 5, idx.config.Workers)
}

func TestDefaultIndexer_WithAllParsers(t *testing.T) {
	idxIface, err := DefaultIndexer(t.TempDir(), WithAllParsers())
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)

	assert.True(t, len(idx.parsers) > 0)
}

type mockParser struct {
	core.Parser
}

func (m *mockParser) GetSupportedTypes() []string {
	return []string{".mock"}
}

func TestDefaultIndexer_WithParsers(t *testing.T) {
	mock := &mockParser{}
	idxIface, err := DefaultIndexer(t.TempDir(), WithParsers(mock))
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)

	assert.Equal(t, 1, len(idx.parsers))
	assert.Equal(t, mock, idx.parsers[0])
}

func TestDefaultIndexer_IndexFile_Init(t *testing.T) {
	idxIface, err := DefaultIndexer(t.TempDir())
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)
	
	// Since we now Init inside DefaultIndexer, pipeline is not nil
	assert.NotNil(t, idx.pipeline)

	// Will fail because no parsers/steps are meaningful without real file
	_, err = idx.IndexFile(context.Background(), "test.txt")
	assert.Error(t, err) 
}
