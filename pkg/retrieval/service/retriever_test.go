package service

import (
	"context"
	"errors"
	"testing"
	"time"

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
	err        error
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.embeddings[0], nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = m.embeddings[0]
	}
	return embeddings, nil
}

func (m *mockEmbedder) Dimension() int {
	if len(m.embeddings) == 0 {
		return 0
	}
	return len(m.embeddings[0])
}

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLogger) Info(msg string, fields ...map[string]any)             {}
func (m *mockLogger) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLogger) Error(msg string, err error, fields ...map[string]any) {}

type mockCollector struct{}

func (m *mockCollector) RecordCount(name, value string, labels map[string]string) {}
func (m *mockCollector) RecordDuration(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockCollector) RecordValue(name string, value float64, labels map[string]string) {}

func TestNew_DefaultValues(t *testing.T) {
	vs := &mockVectorStore{}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}
	retriever := New(vs, emb)

	assert.NotNil(t, retriever)
}

func TestNew_WithOptions(t *testing.T) {
	vs := &mockVectorStore{}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}
	logger := &mockLogger{}
	collector := &mockCollector{}

	retriever := New(vs, emb,
		WithTopK(10),
		WithLogger(logger),
		WithCollector(collector),
	)

	assert.NotNil(t, retriever)
}

func TestWithTopK_ValidValue(t *testing.T) {
	vs := &mockVectorStore{}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}
	retriever := New(vs, emb, WithTopK(20))
	assert.NotNil(t, retriever)
}

func TestWithTopK_InvalidValue(t *testing.T) {
	vs := &mockVectorStore{}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}
	retriever := New(vs, emb, WithTopK(-5))
	assert.NotNil(t, retriever)
}

func TestWithLogger_NilLogger(t *testing.T) {
	vs := &mockVectorStore{}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}
	retriever := New(vs, emb, WithLogger(nil))
	assert.NotNil(t, retriever)
}

func TestWithCollector_NilCollector(t *testing.T) {
	vs := &mockVectorStore{}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}
	retriever := New(vs, emb, WithCollector(nil))
	assert.NotNil(t, retriever)
}

func TestRetrieve_SingleQuery_Success(t *testing.T) {
	vs := &mockVectorStore{
		vectors: []*core.Vector{
			{ID: "v1", Metadata: map[string]any{"content": "test1"}},
			{ID: "v2", Metadata: map[string]any{"content": "test2"}},
		},
		scores: []float32{0.9, 0.8},
	}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}

	retriever := New(vs, emb, WithLogger(&mockLogger{}), WithCollector(&mockCollector{}))

	results, err := retriever.Retrieve(context.Background(), []string{"test query"}, 5)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Len(t, results[0].Chunks, 2)
	assert.Len(t, results[0].Scores, 2)
	assert.Equal(t, float32(0.9), results[0].Scores[0])
}

func TestRetrieve_MultipleQueries_Success(t *testing.T) {
	vs := &mockVectorStore{
		vectors: []*core.Vector{
			{ID: "v1", Metadata: map[string]any{"content": "test1"}},
		},
		scores: []float32{0.9},
	}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}

	retriever := New(vs, emb, WithLogger(&mockLogger{}), WithCollector(&mockCollector{}))

	results, err := retriever.Retrieve(context.Background(), []string{"query1", "query2"}, 5)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestRetrieve_EmptyQueries(t *testing.T) {
	vs := &mockVectorStore{}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}

	retriever := New(vs, emb)

	results, err := retriever.Retrieve(context.Background(), []string{}, 5)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "queries required")
}

func TestRetrieve_NegativeTopK(t *testing.T) {
	vs := &mockVectorStore{
		vectors: []*core.Vector{{ID: "v1"}},
		scores:  []float32{0.9},
	}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}

	retriever := New(vs, emb, WithTopK(5))

	results, err := retriever.Retrieve(context.Background(), []string{"test"}, -1)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestRetrieve_EmbedderError(t *testing.T) {
	vs := &mockVectorStore{}
	emb := &mockEmbedder{err: errors.New("embed error")}

	retriever := New(vs, emb, WithLogger(&mockLogger{}), WithCollector(&mockCollector{}))

	results, err := retriever.Retrieve(context.Background(), []string{"test"}, 5)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "embed failed")
}

func TestRetrieve_VectorStoreError(t *testing.T) {
	vs := &mockVectorStore{err: errors.New("search error")}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}

	retriever := New(vs, emb, WithLogger(&mockLogger{}), WithCollector(&mockCollector{}))

	results, err := retriever.Retrieve(context.Background(), []string{"test"}, 5)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "Search failed")
}

func TestRetrieve_SkipsErrorsInParallel(t *testing.T) {
	vs := &mockVectorStore{
		vectors: []*core.Vector{{ID: "v1"}},
		scores:  []float32{0.9},
	}
	emb := &mockEmbedder{embeddings: [][]float32{{1.0, 2.0, 3.0}}}

	retriever := New(vs, emb, WithLogger(&mockLogger{}), WithCollector(&mockCollector{}))

	results, err := retriever.Retrieve(context.Background(), []string{"query1", "query2"}, 5)

	assert.NoError(t, err)
	assert.NotNil(t, results)
}

func TestRetrieveResult_Structure(t *testing.T) {
	result := retrieveResult{
		query:  "test query",
		chunks: []*core.Chunk{{ID: "c1", Content: "content"}},
		scores: []float32{0.9},
		err:    nil,
	}

	assert.Equal(t, "test query", result.query)
	assert.Len(t, result.chunks, 1)
	assert.Len(t, result.scores, 1)
	assert.Nil(t, result.err)
}
