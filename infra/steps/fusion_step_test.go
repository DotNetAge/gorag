package steps

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockFusionEngine is a mock implementation of retrieval.FusionEngine
type MockFusionEngine struct {
	reciprocalRankFusionFn func(ctx context.Context, resultSets [][]*entity.Chunk, topK int) ([]*entity.Chunk, error)
}

func (m *MockFusionEngine) ReciprocalRankFusion(ctx context.Context, resultSets [][]*entity.Chunk, topK int) ([]*entity.Chunk, error) {
	if m.reciprocalRankFusionFn != nil {
		return m.reciprocalRankFusionFn(ctx, resultSets, topK)
	}
	return nil, nil
}

func TestRAGFusionStep_New(t *testing.T) {
	// Create a mock fusion engine
	mockEngine := &MockFusionEngine{}

	// Test with custom topK
	step := NewRAGFusionStep(mockEngine, 5)
	assert.NotNil(t, step)
	assert.Equal(t, 5, step.topK)

	// Test with negative topK (should default to 10)
	step = NewRAGFusionStep(mockEngine, -1)
	assert.NotNil(t, step)
	assert.Equal(t, 10, step.topK)

	// Test with zero topK (should default to 10)
	step = NewRAGFusionStep(mockEngine, 0)
	assert.NotNil(t, step)
	assert.Equal(t, 10, step.topK)
}

func TestRAGFusionStep_Name(t *testing.T) {
	// Create a mock fusion engine
	mockEngine := &MockFusionEngine{}

	// Create a RAGFusionStep
	step := NewRAGFusionStep(mockEngine, 5)

	// Test Name method
	name := step.Name()
	assert.Equal(t, "RAGFusionStep", name)
}

func TestRAGFusionStep_Execute(t *testing.T) {
	// Create a mock fusion engine
	mockEngine := &MockFusionEngine{
		reciprocalRankFusionFn: func(ctx context.Context, resultSets [][]*entity.Chunk, topK int) ([]*entity.Chunk, error) {
			// Return a fused result
			return []*entity.Chunk{{ID: "fused1", Content: "Fused content"}},
				nil
		},
	}

	// Create a RAGFusionStep
	step := NewRAGFusionStep(mockEngine, 5)

	// Create a pipeline state with empty ParallelResults
	state := &entity.PipelineState{
		ParallelResults: [][]*entity.Chunk{},
	}

	// Test Execute method with empty ParallelResults
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)

	// Create a pipeline state with non-empty ParallelResults
	state = &entity.PipelineState{
		ParallelResults: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "Content 1"}},
			{{ID: "chunk2", Content: "Content 2"}},
		},
	}

	// Test Execute method with non-empty ParallelResults
	err = step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 1)
	assert.Equal(t, "fused1", state.RetrievedChunks[0][0].ID)
	assert.Equal(t, "Fused content", state.RetrievedChunks[0][0].Content)
}
