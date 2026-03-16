package post_retrieval

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockReranker is a mock implementation of abstraction.Reranker
type MockReranker struct {
	rerankFn func(ctx context.Context, query string, chunks []*entity.Chunk, topK int) ([]*entity.Chunk, []float32, error)
}

func (m *MockReranker) Rerank(ctx context.Context, query string, chunks []*entity.Chunk, topK int) ([]*entity.Chunk, []float32, error) {
	if m.rerankFn != nil {
		return m.rerankFn(ctx, query, chunks, topK)
	}
	return chunks, nil, nil
}

func TestRerankStep_New(t *testing.T) {
	// Create a mock reranker
	mockReranker := &MockReranker{}

	// Test with custom topK
	step := NewRerankStep(mockReranker, 5)
	assert.NotNil(t, step)
	assert.Equal(t, 5, step.topK)

	// Test with negative topK (should default to 5)
	step = NewRerankStep(mockReranker, -1)
	assert.NotNil(t, step)
	assert.Equal(t, 5, step.topK)

	// Test with zero topK (should default to 5)
	step = NewRerankStep(mockReranker, 0)
	assert.NotNil(t, step)
	assert.Equal(t, 5, step.topK)
}

func TestRerankStep_Name(t *testing.T) {
	// Create a mock reranker
	mockReranker := &MockReranker{}

	// Create a RerankStep
	step := NewRerankStep(mockReranker, 5)

	// Test Name method
	name := step.Name()
	assert.Equal(t, "RerankStep", name)
}

func TestRerankStep_Execute_NoQuery(t *testing.T) {
	// Create a mock reranker
	mockReranker := &MockReranker{}

	// Create a RerankStep
	step := NewRerankStep(mockReranker, 5)

	// Create a pipeline state without query
	state := &entity.PipelineState{}

	// Test Execute method without query
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RerankStep: 'query' not found in state")
}

func TestRerankStep_Execute_EmptyRetrievedChunks(t *testing.T) {
	// Create a mock reranker
	mockReranker := &MockReranker{}

	// Create a RerankStep
	step := NewRerankStep(mockReranker, 5)

	// Create a pipeline state with empty RetrievedChunks
	state := &entity.PipelineState{
		Query: &entity.Query{
			Text: "What is the capital of France?",
		},
		RetrievedChunks: [][]*entity.Chunk{},
	}

	// Test Execute method with empty RetrievedChunks
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}

func TestRerankStep_Execute_EmptyFlattenedChunks(t *testing.T) {
	// Create a mock reranker
	mockReranker := &MockReranker{}

	// Create a RerankStep
	step := NewRerankStep(mockReranker, 5)

	// Create a pipeline state with empty chunks in RetrievedChunks
	state := &entity.PipelineState{
		Query: &entity.Query{
			Text: "What is the capital of France?",
		},
		RetrievedChunks: [][]*entity.Chunk{{}},
	}

	// Test Execute method with empty flattened chunks
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}

func TestRerankStep_Execute_WithRetrievedChunks(t *testing.T) {
	// Create a mock reranker that returns specific reranked chunks and scores
	mockReranker := &MockReranker{
		rerankFn: func(ctx context.Context, query string, chunks []*entity.Chunk, topK int) ([]*entity.Chunk, []float32, error) {
			assert.Equal(t, "What is the capital of France?", query)
			assert.Len(t, chunks, 2)
			assert.Equal(t, 5, topK)
			
			// Return reranked chunks and scores
			reranked := []*entity.Chunk{
				{ID: "chunk2", Content: "Paris is the capital of France."},
				{ID: "chunk1", Content: "France is a country in Europe."},
			}
			scores := []float32{0.9, 0.7}
			return reranked, scores, nil
		},
	}

	// Create a RerankStep
	step := NewRerankStep(mockReranker, 5)

	// Create a pipeline state with retrieved chunks
	state := &entity.PipelineState{
		Query: &entity.Query{
			Text: "What is the capital of France?",
		},
		RetrievedChunks: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "France is a country in Europe."}},
			{{ID: "chunk2", Content: "Paris is the capital of France."}},
		},
	}

	// Test Execute method with retrieved chunks
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)

	// Check that the chunks were reranked
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 2)
	assert.Equal(t, "chunk2", state.RetrievedChunks[0][0].ID)
	assert.Equal(t, "chunk1", state.RetrievedChunks[0][1].ID)

	// Check that the scores were set
	assert.Len(t, state.RerankScores, 2)
	assert.Equal(t, float32(0.9), state.RerankScores[0])
	assert.Equal(t, float32(0.7), state.RerankScores[1])
}
