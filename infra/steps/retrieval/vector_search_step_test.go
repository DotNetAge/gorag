package retrieval

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockEmbedder is a mock implementation of embedding.Provider
type MockEmbedder struct {
	embedFn func(ctx context.Context, texts []string) ([][]float32, error)
}

func (m *MockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedFn != nil {
		return m.embedFn(ctx, texts)
	}
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}

func (m *MockEmbedder) Dimension() int {
	return 3
}

// MockVectorStore is a mock implementation of abstraction.VectorStore
type MockVectorStore struct {
	searchFn func(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*entity.Vector, []float32, error)
}

func (m *MockVectorStore) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*entity.Vector, []float32, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, topK, filters)
	}
	return nil, nil, nil
}

func (m *MockVectorStore) Upsert(ctx context.Context, vectors []*entity.Vector) error {
	return nil
}

func (m *MockVectorStore) Add(ctx context.Context, vector *entity.Vector) error {
	return nil
}

func (m *MockVectorStore) AddBatch(ctx context.Context, vectors []*entity.Vector) error {
	return nil
}

func (m *MockVectorStore) Delete(ctx context.Context, chunkID string) error {
	return nil
}

func (m *MockVectorStore) DeleteBatch(ctx context.Context, chunkIDs []string) error {
	return nil
}

func (m *MockVectorStore) GetByChunkID(ctx context.Context, chunkID string) (*entity.Vector, error) {
	return nil, nil
}

func (m *MockVectorStore) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *MockVectorStore) Close(ctx context.Context) error {
	return nil
}

func TestVectorSearchStep_New(t *testing.T) {
	// Create mock dependencies
	mockEmbedder := &MockEmbedder{}
	mockStore := &MockVectorStore{}

	// Test with custom topK
	step := NewVectorSearchStep(mockEmbedder, mockStore, 5)
	assert.NotNil(t, step)
	assert.Equal(t, 5, step.topK)

	// Test with negative topK (should default to 10)
	step = NewVectorSearchStep(mockEmbedder, mockStore, -1)
	assert.NotNil(t, step)
	assert.Equal(t, 10, step.topK)

	// Test with zero topK (should default to 10)
	step = NewVectorSearchStep(mockEmbedder, mockStore, 0)
	assert.NotNil(t, step)
	assert.Equal(t, 10, step.topK)
}

func TestVectorSearchStep_Name(t *testing.T) {
	// Create mock dependencies
	mockEmbedder := &MockEmbedder{}
	mockStore := &MockVectorStore{}

	// Create a VectorSearchStep
	step := NewVectorSearchStep(mockEmbedder, mockStore, 5)

	// Test Name method
	name := step.Name()
	assert.Equal(t, "VectorSearchStep", name)
}

func TestVectorSearchStep_Execute_NoQuery(t *testing.T) {
	// Create mock dependencies
	mockEmbedder := &MockEmbedder{}
	mockStore := &MockVectorStore{}

	// Create a VectorSearchStep
	step := NewVectorSearchStep(mockEmbedder, mockStore, 5)

	// Create a pipeline state without query
	state := &entity.PipelineState{}

	// Test Execute method without query
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VectorSearchStep: 'query' not found in state")
}

func TestVectorSearchStep_Execute_EmptyEmbeddings(t *testing.T) {
	// Create a mock embedder that returns empty embeddings
	mockEmbedder := &MockEmbedder{
		embedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			return [][]float32{}, nil
		},
	}
	mockStore := &MockVectorStore{}

	// Create a VectorSearchStep
	step := NewVectorSearchStep(mockEmbedder, mockStore, 5)

	// Create a pipeline state with query
	state := &entity.PipelineState{
		Query: &entity.Query{Text: "What is the capital of France?"},
	}

	// Test Execute method with empty embeddings
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VectorSearchStep failed to get query embedding")
}

func TestVectorSearchStep_Execute_WithQuery(t *testing.T) {
	// Create a mock embedder that returns a specific embedding
	mockEmbedder := &MockEmbedder{
		embedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			assert.Len(t, texts, 1)
			assert.Equal(t, "What is the capital of France?", texts[0])
			return [][]float32{{0.1, 0.2, 0.3}}, nil
		},
	}

	// Create a mock vector store that returns specific vectors
	mockStore := &MockVectorStore{
		searchFn: func(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*entity.Vector, []float32, error) {
			assert.Equal(t, []float32{0.1, 0.2, 0.3}, query)
			assert.Equal(t, 5, topK)
			assert.Nil(t, filters)

			// Return mock vectors
			vectors := []*entity.Vector{
				{
					ChunkID:  "chunk1",
					Metadata: map[string]any{"content": "Paris is the capital of France."},
				},
				{
					ChunkID:  "chunk2",
					Metadata: map[string]any{"content": "France is a country in Europe."},
				},
			}
			scores := []float32{0.9, 0.7}
			return vectors, scores, nil
		},
	}

	// Create a VectorSearchStep
	step := NewVectorSearchStep(mockEmbedder, mockStore, 5)

	// Create a pipeline state with query
	state := &entity.PipelineState{
		Query: &entity.Query{Text: "What is the capital of France?"},
	}

	// Test Execute method with query
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)

	// Check that the retrieved chunks were added to the state
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 2)
	assert.Equal(t, "chunk1", state.RetrievedChunks[0][0].ID)
	assert.Equal(t, "Paris is the capital of France.", state.RetrievedChunks[0][0].Content)
	assert.Equal(t, "chunk2", state.RetrievedChunks[0][1].ID)
	assert.Equal(t, "France is a country in Europe.", state.RetrievedChunks[0][1].Content)
}
