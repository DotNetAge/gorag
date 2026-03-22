package fuse

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockFusionEngine struct {
	result []*core.Chunk
	err    error
}

func (m *mockFusionEngine) Fuse(ctx context.Context, results [][]*core.Chunk, topK int) ([]*core.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockFusionEngine) ReciprocalRankFusion(ctx context.Context, results [][]*core.Chunk, topK int) ([]*core.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestRRF_Name(t *testing.T) {
	fusion := &mockFusionEngine{result: []*core.Chunk{}}
	step := RRF(fusion, 10, nil)
	assert.Equal(t, "RRF-Fusion", step.Name())
}

func TestRRF_Execute_Success(t *testing.T) {
	fusion := &mockFusionEngine{
		result: []*core.Chunk{
			{ID: "chunk1", Content: "Fused content 1"},
			{ID: "chunk2", Content: "Fused content 2"},
		},
	}
	step := RRF(fusion, 10, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "chunk1", Content: "Content 1"}},
			{{ID: "chunk2", Content: "Content 2"}},
		},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 2)
}

func TestRRF_Execute_EmptyChunks(t *testing.T) {
	fusion := &mockFusionEngine{result: []*core.Chunk{}}
	step := RRF(fusion, 10, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Empty(t, state.RetrievedChunks)
}

func TestRRF_Execute_NilChunks(t *testing.T) {
	fusion := &mockFusionEngine{result: []*core.Chunk{}}
	step := RRF(fusion, 10, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: nil,
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}

func TestRRF_Execute_FusionError(t *testing.T) {
	fusion := &mockFusionEngine{err: errors.New("fusion failed")}
	step := RRF(fusion, 10, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "chunk1", Content: "Content 1"}},
		},
	}

	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fusion failed")
}

func TestRRF_Execute_ReplacesOriginalResults(t *testing.T) {
	fusion := &mockFusionEngine{
		result: []*core.Chunk{
			{ID: "fused1", Content: "Fused 1"},
			{ID: "fused2", Content: "Fused 2"},
		},
	}
	step := RRF(fusion, 10, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "original1", Content: "Original 1"}},
			{{ID: "original2", Content: "Original 2"}},
			{{ID: "original3", Content: "Original 3"}},
		},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Equal(t, "fused1", state.RetrievedChunks[0][0].ID)
	assert.Equal(t, "fused2", state.RetrievedChunks[0][1].ID)
}
