package vector

import (
	"context"
	"errors"
	"testing"

	gchat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockVectorStore struct {
	vectors []*core.Vector
	scores  []float32
	err     error
}

func (m *mockVectorStore) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.vectors, m.scores, nil
}

func (m *mockVectorStore) Upsert(ctx context.Context, vectors []*core.Vector) error {
	return nil
}

func (m *mockVectorStore) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockVectorStore) Close(ctx context.Context) error {
	return nil
}

type mockEmbedder struct {
	embeddings [][]float32
	dimension  int
	err        error
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.embeddings, nil
}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.embeddings[0], nil
}

func (m *mockEmbedder) Dimension() int {
	return m.dimension
}

func TestSearchStep_Name(t *testing.T) {
	step := Search(nil, nil, SearchOptions{})
	assert.Equal(t, "VectorSearch", step.Name())
}

func TestSearchStep_Execute_DefensiveNilStoreOrEmbedder(t *testing.T) {
	step := Search(nil, nil, SearchOptions{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestSearchStep_Execute_SingleQuery(t *testing.T) {
	store := &mockVectorStore{
		vectors: []*core.Vector{
			{ID: "v1", Metadata: map[string]any{"content": "test1"}},
			{ID: "v2", Metadata: map[string]any{"content": "test2"}},
		},
		scores: []float32{0.9, 0.8},
	}
	embedder := &mockEmbedder{
		embeddings: [][]float32{{1.0, 2.0, 3.0}},
	}
	step := Search(store, embedder, SearchOptions{TopK: 10})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test query", nil),
		Agentic: &core.AgenticContext{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.RetrievedChunks)
}

func TestSearchStep_Execute_WithSubQueries(t *testing.T) {
	store := &mockVectorStore{
		vectors: []*core.Vector{
			{ID: "v1", Metadata: map[string]any{"content": "result"}},
		},
		scores: []float32{0.9},
	}
	embedder := &mockEmbedder{
		embeddings: [][]float32{{1.0, 2.0, 3.0}},
	}
	step := Search(store, embedder, SearchOptions{TopK: 5})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{
			SubQueries: []string{"sub1", "sub2"},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestSearchStep_Execute_EmbedderError(t *testing.T) {
	store := &mockVectorStore{}
	embedder := &mockEmbedder{err: errors.New("embed error")}
	step := Search(store, embedder, SearchOptions{TopK: 10})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{},
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding failed")
}

func TestSearchStep_Execute_SearchError(t *testing.T) {
	store := &mockVectorStore{err: errors.New("search error")}
	embedder := &mockEmbedder{
		embeddings: [][]float32{{1.0, 2.0, 3.0}},
	}
	step := Search(store, embedder, SearchOptions{TopK: 10})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{},
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vector search failed")
}

func TestSearchStep_Execute_Concurrency(t *testing.T) {
	store := &mockVectorStore{
		vectors: []*core.Vector{{ID: "v1", Metadata: map[string]any{"content": "test"}}},
		scores:  []float32{0.9},
	}
	embedder := &mockEmbedder{
		embeddings: [][]float32{{1.0, 2.0, 3.0}},
	}
	step := Search(store, embedder, SearchOptions{TopK: 5, Concurrency: 4})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestSearchStep_Execute_DefaultConcurrency(t *testing.T) {
	store := &mockVectorStore{
		vectors: []*core.Vector{{ID: "v1"}},
		scores:  []float32{0.9},
	}
	embedder := &mockEmbedder{
		embeddings: [][]float32{{1.0, 2.0, 3.0}},
	}
	step := Search(store, embedder, SearchOptions{TopK: 5, Concurrency: 0})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestSearchStep_Execute_ScoreInMetadata(t *testing.T) {
	store := &mockVectorStore{
		vectors: []*core.Vector{
			{ID: "v1", Metadata: map[string]any{"content": "test1"}},
		},
		scores: []float32{0.95},
	}
	embedder := &mockEmbedder{
		embeddings: [][]float32{{1.0, 2.0, 3.0}},
	}
	step := Search(store, embedder, SearchOptions{TopK: 10})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
}

func TestSearchOptions_Defaults(t *testing.T) {
	opts := SearchOptions{}

	assert.Equal(t, 0, opts.TopK)
	assert.Nil(t, opts.Filters)
	assert.Equal(t, 0, opts.Concurrency)
}

var _ embedding.Provider = (*mockEmbedder)(nil)
var _ core.VectorStore = (*mockVectorStore)(nil)
var _ gchat.Client = (*mockLLM)(nil)
var _ core.IntentClassifier = (*intentRouter)(nil)

type mockLLM struct {
	response *gchat.Response
	err      error
}

func (m *mockLLM) Chat(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLM) ChatStream(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Stream, error) {
	return nil, nil
}

type intentRouter struct {
	llm            gchat.Client
	promptTemplate string
	defaultIntent  core.IntentType
	minConfidence  float32
}

func (r *intentRouter) Classify(ctx context.Context, query *core.Query) (*core.IntentResult, error) {
	return nil, nil
}
