package fusion

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestRRFFusionEngine_New(t *testing.T) {
	// Test creating a new RRFFusionEngine
	engine := NewRRFFusionEngine()
	assert.NotNil(t, engine)
	assert.Equal(t, float32(60.0), engine.k)
}

func TestRRFFusionEngine_ReciprocalRankFusion_EmptyResultSets(t *testing.T) {
	// Create a RRFFusionEngine
	engine := NewRRFFusionEngine()

	// Test with empty result sets
	ctx := context.Background()
	resultSets := [][]*core.Chunk{}
	topK := 5

	result, err := engine.ReciprocalRankFusion(ctx, resultSets, topK)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestRRFFusionEngine_ReciprocalRankFusion_SingleResultSet(t *testing.T) {
	// Create a RRFFusionEngine
	engine := NewRRFFusionEngine()

	// Create a single result set
	chunk1 := &core.Chunk{ID: "chunk1", Content: "Content 1"}
	chunk2 := &core.Chunk{ID: "chunk2", Content: "Content 2"}
	chunk3 := &core.Chunk{ID: "chunk3", Content: "Content 3"}

	resultSets := [][]*core.Chunk{
		{chunk1, chunk2, chunk3},
	}
	topK := 5

	// Test with single result set
	ctx := context.Background()
	result, err := engine.ReciprocalRankFusion(ctx, resultSets, topK)
	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "chunk1", result[0].ID)
	assert.Equal(t, "chunk2", result[1].ID)
	assert.Equal(t, "chunk3", result[2].ID)
}

func TestRRFFusionEngine_ReciprocalRankFusion_MultipleResultSets(t *testing.T) {
	// Create a RRFFusionEngine
	engine := NewRRFFusionEngine()

	// Create multiple result sets
	chunk1 := &core.Chunk{ID: "chunk1", Content: "Content 1"}
	chunk2 := &core.Chunk{ID: "chunk2", Content: "Content 2"}
	chunk3 := &core.Chunk{ID: "chunk3", Content: "Content 3"}
	chunk4 := &core.Chunk{ID: "chunk4", Content: "Content 4"}

	resultSets := [][]*core.Chunk{
		{chunk1, chunk2, chunk3}, // Set 1: chunk1 (rank 1), chunk2 (rank 2), chunk3 (rank 3)
		{chunk2, chunk4, chunk1}, // Set 2: chunk2 (rank 1), chunk4 (rank 2), chunk1 (rank 3)
	}
	topK := 4

	// Test with multiple result sets
	ctx := context.Background()
	result, err := engine.ReciprocalRankFusion(ctx, resultSets, topK)
	assert.NoError(t, err)
	assert.Len(t, result, 4)

	// Calculate expected scores manually
	// chunk1: 1/(60+1) + 1/(60+3) = 1/61 + 1/63 ≈ 0.01639 + 0.01587 ≈ 0.03226
	// chunk2: 1/(60+2) + 1/(60+1) = 1/62 + 1/61 ≈ 0.01613 + 0.01639 ≈ 0.03252
	// chunk3: 1/(60+3) = 1/63 ≈ 0.01587
	// chunk4: 1/(60+2) = 1/62 ≈ 0.01613
	// Expected order: chunk2, chunk1, chunk4, chunk3

	assert.Equal(t, "chunk2", result[0].ID)
	assert.Equal(t, "chunk1", result[1].ID)
	assert.Equal(t, "chunk4", result[2].ID)
	assert.Equal(t, "chunk3", result[3].ID)
}

func TestRRFFusionEngine_ReciprocalRankFusion_TopK(t *testing.T) {
	// Create a RRFFusionEngine
	engine := NewRRFFusionEngine()

	// Create multiple result sets
	chunk1 := &core.Chunk{ID: "chunk1", Content: "Content 1"}
	chunk2 := &core.Chunk{ID: "chunk2", Content: "Content 2"}
	chunk3 := &core.Chunk{ID: "chunk3", Content: "Content 3"}
	chunk4 := &core.Chunk{ID: "chunk4", Content: "Content 4"}

	resultSets := [][]*core.Chunk{
		{chunk1, chunk2, chunk3, chunk4},
		{chunk2, chunk1, chunk4, chunk3},
	}
	topK := 2

	// Test with topK parameter
	ctx := context.Background()
	result, err := engine.ReciprocalRankFusion(ctx, resultSets, topK)
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	// Expected order: chunk1, chunk2 (or chunk2, chunk1 depending on exact scores)
	// Both should be in the top 2
	assert.Contains(t, []string{result[0].ID, result[1].ID}, "chunk1")
	assert.Contains(t, []string{result[0].ID, result[1].ID}, "chunk2")
}

func TestRRFFusionEngine_Limit(t *testing.T) {
	// Create a RRFFusionEngine
	engine := NewRRFFusionEngine()

	// Create test chunks
	chunk1 := &core.Chunk{ID: "chunk1", Content: "Content 1"}
	chunk2 := &core.Chunk{ID: "chunk2", Content: "Content 2"}
	chunk3 := &core.Chunk{ID: "chunk3", Content: "Content 3"}

	chunks := []*core.Chunk{chunk1, chunk2, chunk3}

	// Test limit with more chunks than topK
	result := engine.limit(chunks, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, "chunk1", result[0].ID)
	assert.Equal(t, "chunk2", result[1].ID)

	// Test limit with fewer chunks than topK
	result = engine.limit(chunks, 5)
	assert.Len(t, result, 3)
	assert.Equal(t, "chunk1", result[0].ID)
	assert.Equal(t, "chunk2", result[1].ID)
	assert.Equal(t, "chunk3", result[2].ID)

	// Test limit with exactly topK chunks
	result = engine.limit(chunks, 3)
	assert.Len(t, result, 3)
	assert.Equal(t, "chunk1", result[0].ID)
	assert.Equal(t, "chunk2", result[1].ID)
	assert.Equal(t, "chunk3", result[2].ID)
}
