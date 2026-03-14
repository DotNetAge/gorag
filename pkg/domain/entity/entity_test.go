package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestChunk tests the Chunk entity
func TestChunk(t *testing.T) {
	// Test NewChunk
	metadata := map[string]any{"lang": "en", "author": "test"}
	chunk := NewChunk("chunk1", "doc1", "Test content", 0, 10, metadata)

	assert.Equal(t, "chunk1", chunk.ID)
	assert.Equal(t, "doc1", chunk.DocumentID)
	assert.Equal(t, "Test content", chunk.Content)
	assert.Equal(t, 0, chunk.StartIndex)
	assert.Equal(t, 10, chunk.EndIndex)
	assert.Equal(t, metadata, chunk.Metadata)
	assert.NotZero(t, chunk.CreatedAt)
	assert.Empty(t, chunk.VectorID)

	// Test SetVectorID
	chunk.SetVectorID("vector1")
	assert.Equal(t, "vector1", chunk.VectorID)
}

// TestDocument tests the Document entity
func TestDocument(t *testing.T) {
	// Test NewDocument
	metadata := map[string]any{"lang": "en", "author": "test"}
	doc := NewDocument("doc1", "Test content", "file.txt", "text/plain", metadata)

	assert.Equal(t, "doc1", doc.ID)
	assert.Equal(t, "Test content", doc.Content)
	assert.Equal(t, "file.txt", doc.Source)
	assert.Equal(t, "text/plain", doc.ContentType)
	assert.Equal(t, metadata, doc.Metadata)
	assert.NotZero(t, doc.CreatedAt)
	assert.NotZero(t, doc.UpdatedAt)

	// Test Update
	newMetadata := map[string]any{"lang": "en", "author": "updated"}
	oldUpdatedAt := doc.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	doc.Update("Updated content", newMetadata)

	assert.Equal(t, "Updated content", doc.Content)
	assert.Equal(t, newMetadata, doc.Metadata)
	assert.True(t, doc.UpdatedAt.After(oldUpdatedAt))
}

// TestPipelineState tests the PipelineState entity
func TestPipelineState(t *testing.T) {
	// Test NewPipelineState
	state := NewPipelineState()

	assert.Nil(t, state.Query)
	assert.Nil(t, state.OriginalQuery)
	assert.NotNil(t, state.RetrievedChunks)
	assert.Len(t, state.RetrievedChunks, 0)
	assert.NotNil(t, state.ParallelResults)
	assert.Len(t, state.ParallelResults, 0)
	assert.NotNil(t, state.RerankScores)
	assert.Len(t, state.RerankScores, 0)
	assert.NotNil(t, state.Filters)
	assert.Len(t, state.Filters, 0)
	assert.Empty(t, state.Answer)
	assert.Empty(t, state.GenerationPrompt)
	assert.Zero(t, state.SelfRagScore)
	assert.Empty(t, state.SelfRagReason)

	// Test setting fields
	query := NewQuery("q1", "What is the capital of France?", nil)
	state.Query = query
	state.OriginalQuery = query

	chunk := NewChunk("chunk1", "doc1", "Paris is the capital of France.", 0, 10, nil)
	state.RetrievedChunks = append(state.RetrievedChunks, []*Chunk{chunk})
	state.ParallelResults = append(state.ParallelResults, []*Chunk{chunk})
	state.RerankScores = append(state.RerankScores, 0.9)
	state.Filters = map[string]any{"lang": "en"}
	state.Answer = "Paris is the capital of France."
	state.GenerationPrompt = "User Question: What is the capital of France?"
	state.SelfRagScore = 0.95
	state.SelfRagReason = "The answer is supported by the context."

	assert.Equal(t, query, state.Query)
	assert.Equal(t, query, state.OriginalQuery)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Equal(t, chunk, state.RetrievedChunks[0][0])
	assert.Len(t, state.ParallelResults, 1)
	assert.Equal(t, chunk, state.ParallelResults[0][0])
	assert.Len(t, state.RerankScores, 1)
	assert.Equal(t, float32(0.9), state.RerankScores[0])
	assert.Equal(t, map[string]any{"lang": "en"}, state.Filters)
	assert.Equal(t, "Paris is the capital of France.", state.Answer)
	assert.Equal(t, "User Question: What is the capital of France?", state.GenerationPrompt)
	assert.Equal(t, float32(0.95), state.SelfRagScore)
	assert.Equal(t, "The answer is supported by the context.", state.SelfRagReason)
}

// TestQuery tests the Query entity
func TestQuery(t *testing.T) {
	// Test NewQuery
	metadata := map[string]any{"user_id": "user1"}
	query := NewQuery("q1", "What is the capital of France?", metadata)

	assert.Equal(t, "q1", query.ID)
	assert.Equal(t, "What is the capital of France?", query.Text)
	assert.Equal(t, metadata, query.Metadata)
	assert.NotZero(t, query.CreatedAt)
}

// TestRetrievalResult tests the RetrievalResult entity
func TestRetrievalResult(t *testing.T) {
	// Test NewRetrievalResult
	metadata := map[string]any{"search_time": 100}
	chunk := NewChunk("chunk1", "doc1", "Paris is the capital of France.", 0, 10, nil)
	chunks := []*Chunk{chunk}
	scores := []float32{0.9}

	result := NewRetrievalResult("result1", "q1", chunks, scores, metadata)

	assert.Equal(t, "result1", result.ID)
	assert.Equal(t, "q1", result.QueryID)
	assert.Len(t, result.Chunks, 1)
	assert.Equal(t, chunk, result.Chunks[0])
	assert.Len(t, result.Scores, 1)
	assert.Equal(t, float32(0.9), result.Scores[0])
	assert.Equal(t, metadata, result.Metadata)
}

// TestVector tests the Vector entity
func TestVector(t *testing.T) {
	// Test NewVector
	metadata := map[string]any{"lang": "en"}
	values := []float32{0.1, 0.2, 0.3}

	vector := NewVector("vec1", values, "chunk1", metadata)

	assert.Equal(t, "vec1", vector.ID)
	assert.Equal(t, values, vector.Values)
	assert.Equal(t, "chunk1", vector.ChunkID)
	assert.Equal(t, metadata, vector.Metadata)
}
