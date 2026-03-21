package gorag

import (
	"context"
	"testing"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/di"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEmbedder struct {
	dimension int
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i := range texts {
		results[i] = make([]float32, m.dimension)
	}
	return results, nil
}

func (m *mockEmbedder) Dimension() int { return m.dimension }

type mockParser struct {
	core.Parser
}

func (m *mockParser) GetSupportedTypes() []string {
	return []string{".txt"}
}

func (m *mockParser) Parse(ctx context.Context, data []byte, metadata map[string]interface{}) (*core.Document, error) {
	return &core.Document{
		ID:       "test-doc",
		Content:  string(data),
		Metadata: metadata,
	}, nil
}

func TestRAG_DI_Injection(t *testing.T) {
	tmpDir := t.TempDir()
	
	ctr := di.New()
	ctr.RegisterInstance((*embedding.Provider)(nil), &mockEmbedder{dimension: 384})
	
	app, err := DefaultNativeRAG(
		WithWorkDir(tmpDir),
		WithDimension(384),
		WithContainer(ctr),
		WithParsers(&mockParser{}),
	)
	require.NoError(t, err)

	assert.NotNil(t, app)
	assert.Equal(t, ctr, app.Container())
}
