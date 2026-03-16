package service

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/stretchr/testify/assert"
)

// mockEmbedder implements embedding.Provider for testing
type mockEmbedder struct {
	embedFn     func(ctx context.Context, texts []string) ([][]float32, error)
	embedOneFn  func(ctx context.Context, text string) ([]float32, error)
	dimensionFn func() int
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedFn != nil {
		return m.embedFn(ctx, texts)
	}
	results := make([][]float32, len(texts))
	for i := range texts {
		results[i] = []float32{0.1, 0.2, 0.3}
	}
	return results, nil
}

func (m *mockEmbedder) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	if m.embedOneFn != nil {
		return m.embedOneFn(ctx, text)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbedder) Dimension() int {
	if m.dimensionFn != nil {
		return m.dimensionFn()
	}
	return 3
}

// mockVectorStore implements abstraction.VectorStore for testing
type mockVectorStore struct {
	addFn         func(ctx context.Context, vector *entity.Vector) error
	addBatchFn    func(ctx context.Context, vectors []*entity.Vector) error
	searchFn      func(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error)
	deleteFn      func(ctx context.Context, id string) error
	deleteBatchFn func(ctx context.Context, ids []string) error
	resetFn       func(ctx context.Context) error
	closeFn       func(ctx context.Context) error
}

func (m *mockVectorStore) Add(ctx context.Context, vector *entity.Vector) error {
	if m.addFn != nil {
		return m.addFn(ctx, vector)
	}
	return nil
}

func (m *mockVectorStore) AddBatch(ctx context.Context, vectors []*entity.Vector) error {
	if m.addBatchFn != nil {
		return m.addBatchFn(ctx, vectors)
	}
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, topK, filter)
	}
	return []*entity.Vector{}, []float32{}, nil
}

func (m *mockVectorStore) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockVectorStore) DeleteBatch(ctx context.Context, ids []string) error {
	if m.deleteBatchFn != nil {
		return m.deleteBatchFn(ctx, ids)
	}
	return nil
}

func (m *mockVectorStore) Reset(ctx context.Context) error {
	if m.resetFn != nil {
		return m.resetFn(ctx)
	}
	return nil
}

func (m *mockVectorStore) Close(ctx context.Context) error {
	if m.closeFn != nil {
		return m.closeFn(ctx)
	}
	return nil
}

func TestNewRetriever(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		mockEmbedder := &mockEmbedder{}
		mockVectorStore := &mockVectorStore{}

		ret := NewRetriever(mockVectorStore, mockEmbedder)

		assert.NotNil(t, ret)
		assert.Equal(t, 5, ret.defaultTopK)
	})

	t.Run("with custom topK", func(t *testing.T) {
		mockEmbedder := &mockEmbedder{}
		mockVectorStore := &mockVectorStore{}

		ret := NewRetriever(mockVectorStore, mockEmbedder, WithTopK(10))

		assert.NotNil(t, ret)
		assert.Equal(t, 10, ret.defaultTopK)
	})

	t.Run("with logger and collector", func(t *testing.T) {
		mockEmbedder := &mockEmbedder{}
		mockVectorStore := &mockVectorStore{}
		logger := logging.NewNoopLogger()
		collector := observability.NewNoopCollector()

		ret := NewRetriever(
			mockVectorStore,
			mockEmbedder,
			WithRetrieverLogger(logger),
			WithRetrieverCollector(collector),
		)

		assert.NotNil(t, ret)
		assert.Equal(t, logger, ret.logger)
		assert.Equal(t, collector, ret.collector)
	})
}

func TestRetriever_Retrieve_Success(t *testing.T) {
	mockEmbedder := &mockEmbedder{
		embedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			results := make([][]float32, len(texts))
			for i := range texts {
				results[i] = []float32{0.1, 0.2, 0.3}
			}
			return results, nil
		},
	}

	mockVectorStore := &mockVectorStore{
		searchFn: func(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
			return []*entity.Vector{
				{ID: "v1", Metadata: map[string]any{"content": "relevant content 1"}},
				{ID: "v2", Metadata: map[string]any{"content": "relevant content 2"}},
			}, []float32{0.9, 0.8}, nil
		},
	}

	ret := NewRetriever(mockVectorStore, mockEmbedder, WithTopK(2))
	ctx := context.Background()

	results, err := ret.Retrieve(ctx, []string{"test query"}, 2)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Contains(t, results[0].Chunks, "relevant content 1")
	assert.Contains(t, results[1].Chunks, "relevant content 2")
}

func TestRetriever_Retrieve_EmbeddingError(t *testing.T) {
	mockEmbedder := &mockEmbedder{
		embedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			return nil, assert.AnError
		},
	}

	mockVectorStore := &mockVectorStore{}

	ret := NewRetriever(mockVectorStore, mockEmbedder)
	ctx := context.Background()

	results, err := ret.Retrieve(ctx, []string{"test query"}, 2)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "embedding failed")
}

func TestRetriever_Retrieve_VectorStoreError(t *testing.T) {
	mockEmbedder := &mockEmbedder{
		embedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			results := make([][]float32, len(texts))
			for i := range texts {
				results[i] = []float32{0.1, 0.2, 0.3}
			}
			return results, nil
		},
	}

	mockVectorStore := &mockVectorStore{
		searchFn: func(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
			return nil, nil, assert.AnError
		},
	}

	ret := NewRetriever(mockVectorStore, mockEmbedder)
	ctx := context.Background()

	results, err := ret.Retrieve(ctx, []string{"test query"}, 2)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "vector search failed")
}

func TestRetriever_Retrieve_DefaultTopK(t *testing.T) {
	mockEmbedder := &mockEmbedder{}
	mockVectorStore := &mockVectorStore{
		searchFn: func(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
			// Should use default topK=5
			assert.Equal(t, 5, topK)
			return []*entity.Vector{}, []float32{}, nil
		},
	}

	ret := NewRetriever(mockVectorStore, mockEmbedder) // defaultTopK=5
	ctx := context.Background()

	_, err := ret.Retrieve(ctx, []string{"test query"}, 0) // 0 should trigger default

	assert.NoError(t, err)
}

func TestRetriever_Retrieve_MultipleQueries(t *testing.T) {
	mockEmbedder := &mockEmbedder{}
	mockVectorStore := &mockVectorStore{
		searchFn: func(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
			return []*entity.Vector{
				{ID: "v1", Metadata: map[string]any{"content": "result"}},
			}, []float32{0.9}, nil
		},
	}

	ret := NewRetriever(mockVectorStore, mockEmbedder)
	ctx := context.Background()

	results, err := ret.Retrieve(ctx, []string{"query1", "query2"}, 2)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestRetriever_Retrieve_MetricsRecording(t *testing.T) {
	mockEmbedder := &mockEmbedder{}
	mockVectorStore := &mockVectorStore{
		searchFn: func(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
			return []*entity.Vector{}, []float32{}, nil
		},
	}

	collector := observability.NewNoopCollector()
	ret := NewRetriever(mockVectorStore, mockEmbedder, WithRetrieverCollector(collector))
	ctx := context.Background()

	_, err := ret.Retrieve(ctx, []string{"test"}, 2)

	assert.NoError(t, err)
	// Metrics should be recorded without panic
}
