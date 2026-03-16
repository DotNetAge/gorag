package post_retrieval

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

func TestRAGFusionStep_Execute_EmptyResults(t *testing.T) {
	mockEngine := &MockFusionEngine{}
	step := NewRAGFusionStep(mockEngine, 5)
	ctx := context.Background()
	state := &entity.PipelineState{
		ParallelResults: [][]*entity.Chunk{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestRAGFusionStep_Execute_WithResults(t *testing.T) {
	mockEngine := &MockFusionEngine{
		reciprocalRankFusionFn: func(ctx context.Context, resultSets [][]*entity.Chunk, topK int) ([]*entity.Chunk, error) {
			return []*entity.Chunk{{ID: "fused1", Content: "Fused content"}}, nil
		},
	}
	step := NewRAGFusionStep(mockEngine, 5)
	ctx := context.Background()
	state := &entity.PipelineState{
		ParallelResults: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "Content 1"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.RetrievedChunks)
}
