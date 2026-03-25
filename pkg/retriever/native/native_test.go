package native

import (
	"context"
	"testing"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockVectorStoreForNative struct {
	vectors []*core.Vector
	scores  []float32
	err     error
}

func (m *mockVectorStoreForNative) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.vectors, m.scores, nil
}

func (m *mockVectorStoreForNative) Upsert(ctx context.Context, vectors []*core.Vector) error {
	return nil
}

func (m *mockVectorStoreForNative) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockVectorStoreForNative) Close(ctx context.Context) error {
	return nil
}

type mockEmbedderForNative struct {
	embeddings [][]float32
	dimension  int
	err        error
}

func (m *mockEmbedderForNative) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.embeddings, nil
}

func (m *mockEmbedderForNative) Dimension() int {
	return m.dimension
}

func TestWithVectorStore(t *testing.T) {
	store := &mockVectorStoreForNative{}
	opt := WithVectorStore(store)

	options := &Options{}
	opt(options)

	assert.Equal(t, store, options.VectorStore)
}

func TestWithDocStore(t *testing.T) {
	opt := WithDocStore(nil)

	options := &Options{}
	opt(options)

	assert.Nil(t, options.DocStore)
}

func TestWithWorkDir(t *testing.T) {
	opt := WithWorkDir("/tmp/test")

	options := &Options{}
	opt(options)

	assert.Equal(t, "/tmp/test", options.WorkDir)
}

func TestWithLogger(t *testing.T) {
	opt := WithLogger(nil)

	options := &Options{}
	opt(options)

	assert.Nil(t, options.Logger)
}

func TestWithTracer(t *testing.T) {
	opt := WithTracer(nil)

	options := &Options{}
	opt(options)

	assert.Nil(t, options.Tracer)
}

func TestWithEmbedder(t *testing.T) {
	emb := &mockEmbedderForNative{dimension: 128}
	opt := WithEmbedder(emb)

	options := &Options{}
	opt(options)

	assert.Equal(t, emb, options.Embedder)
}

func TestWithTopK(t *testing.T) {
	opt := WithTopK(10)

	options := &Options{}
	opt(options)

	assert.Equal(t, 10, options.TopK)
}

func TestWithName(t *testing.T) {
	opt := WithName("test_bot")

	options := &Options{}
	opt(options)

	assert.Equal(t, "test_bot", options.Name)
}

func TestOptions_Defaults(t *testing.T) {
	options := &Options{}

	assert.Nil(t, options.Logger)
	assert.Nil(t, options.Tracer)
	assert.Nil(t, options.Embedder)
	assert.Nil(t, options.LLM)
	assert.Equal(t, 0, options.TopK)
	assert.Equal(t, "", options.WorkDir)
	assert.Nil(t, options.VectorStore)
	assert.Nil(t, options.DocStore)
}

var _ embedding.Provider = (*mockEmbedderForNative)(nil)
var _ core.VectorStore = (*mockVectorStoreForNative)(nil)
