package dedup

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

func TestUnique_Name(t *testing.T) {
	step := Unique(0.95, nil, nil)
	assert.Equal(t, "Unique", step.Name())
}

func TestUnique_Execute_EmptyChunks(t *testing.T) {
	step := Unique(0.95, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{},
		ParallelResults: make(map[string][]*core.Chunk),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Empty(t, state.RetrievedChunks)
}

func TestUnique_Execute_NilParallelResults(t *testing.T) {
	step := Unique(0.95, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{
				{ID: "chunk1", Content: "Hello world"},
				{ID: "chunk2", Content: "Hello world"},
			},
		},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 1)
	assert.Equal(t, "chunk1", state.RetrievedChunks[0][0].ID)
}

func TestUnique_Execute_WithParallelResults(t *testing.T) {
	step := Unique(0.95, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{
				{ID: "chunk1", Content: "Content A"},
				{ID: "chunk2", Content: "Content B"},
			},
		},
		ParallelResults: map[string][]*core.Chunk{
			"search1": {
				{ID: "chunk3", Content: "Content C"},
				{ID: "chunk4", Content: "Content A"},
			},
		},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 3)

	contentMap := make(map[string]bool)
	for _, chunk := range state.RetrievedChunks[0] {
		contentMap[chunk.Content] = true
	}
	assert.True(t, contentMap["Content A"])
	assert.True(t, contentMap["Content B"])
	assert.True(t, contentMap["Content C"])
}

func TestUnique_Execute_AllUnique(t *testing.T) {
	step := Unique(0.95, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{
				{ID: "chunk1", Content: "Content A"},
				{ID: "chunk2", Content: "Content B"},
				{ID: "chunk3", Content: "Content C"},
			},
		},
		ParallelResults: make(map[string][]*core.Chunk),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 3)
}

func TestUnique_Execute_AllDuplicates(t *testing.T) {
	step := Unique(0.95, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{
				{ID: "chunk1", Content: "Same content"},
				{ID: "chunk2", Content: "Same content"},
				{ID: "chunk3", Content: "Same content"},
			},
		},
		ParallelResults: make(map[string][]*core.Chunk),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 1)
	assert.Equal(t, "chunk1", state.RetrievedChunks[0][0].ID)
}

func TestUnique_Execute_PreservesFirstOccurrence(t *testing.T) {
	step := Unique(0.95, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{
				{ID: "first", Content: "Original"},
				{ID: "second", Content: "Duplicate"},
				{ID: "third", Content: "Original"},
			},
		},
		ParallelResults: make(map[string][]*core.Chunk),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks[0], 2)
	assert.Equal(t, "first", state.RetrievedChunks[0][0].ID)
	assert.Equal(t, "second", state.RetrievedChunks[0][1].ID)
}

func TestUnique_Execute_ClearsParallelResults(t *testing.T) {
	step := Unique(0.95, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{
			{
				{ID: "chunk1", Content: "Content A"},
			},
		},
		ParallelResults: map[string][]*core.Chunk{
			"search1": {{ID: "chunk2", Content: "Content B"}},
			"search2": {{ID: "chunk3", Content: "Content C"}},
		},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, state.ParallelResults)
	assert.Empty(t, state.ParallelResults)
}

func TestUnique_DefaultThreshold(t *testing.T) {
	step := Unique(0, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: [][]*core.Chunk{},
		ParallelResults: make(map[string][]*core.Chunk),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}

func TestUnique_Execute_NilRetrievedChunks(t *testing.T) {
	step := Unique(0.95, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		RetrievedChunks: nil,
		ParallelResults: make(map[string][]*core.Chunk),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}
