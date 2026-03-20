package indexer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

func TestDefaultIndexer_Init(t *testing.T) {
	idx := DefaultIndexer(
		WithConcurrency(true),
		WithWorkers(5),
	).(*defaultIndexer)

	err := idx.Init()
	assert.NoError(t, err)
	assert.NotNil(t, idx.pipeline)
	assert.True(t, idx.config.Concurrency)
	assert.Equal(t, 5, idx.config.Workers)
}

func TestDefaultIndexer_WithAllParsers(t *testing.T) {
	idx := DefaultIndexer(
		WithAllParsers(),
	).(*defaultIndexer)

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
	idx := DefaultIndexer(
		WithParsers(mock),
	).(*defaultIndexer)

	assert.Equal(t, 1, len(idx.parsers))
	assert.Equal(t, mock, idx.parsers[0])
}

func TestDefaultIndexer_IndexFile_Init(t *testing.T) {
	idx := DefaultIndexer().(*defaultIndexer)
	assert.Nil(t, idx.pipeline)

	// Should not panic, should auto-init (though it will fail because no parsers/steps are meaningful without config)
	_, _ = idx.IndexFile(context.Background(), "test.txt")
	assert.NotNil(t, idx.pipeline)
}
