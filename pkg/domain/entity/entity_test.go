package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewQuery(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		metadata := map[string]any{"key": "value"}
		query := NewQuery("id123", "test query", metadata)

		assert.Equal(t, "id123", query.ID)
		assert.Equal(t, "test query", query.Text)
		assert.Equal(t, metadata, query.Metadata)
		assert.False(t, query.CreatedAt.IsZero())
	})

	t.Run("with nil metadata", func(t *testing.T) {
		query := NewQuery("", "test query", nil)

		assert.Equal(t, "", query.ID)
		assert.Equal(t, "test query", query.Text)
		assert.Nil(t, query.Metadata)
	})
}

func TestNewChunk(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		metadata := map[string]any{"source": "test.pdf"}
		chunk := NewChunk("chunk1", "doc1", "content here", 0, 12, metadata)

		assert.Equal(t, "chunk1", chunk.ID)
		assert.Equal(t, "doc1", chunk.DocumentID)
		assert.Equal(t, "content here", chunk.Content)
		assert.Equal(t, metadata, chunk.Metadata)
		assert.Equal(t, 0, chunk.StartIndex)
		assert.Equal(t, 12, chunk.EndIndex)
	})

	t.Run("with empty content", func(t *testing.T) {
		chunk := NewChunk("chunk2", "doc2", "", 0, 0, nil)

		assert.Equal(t, "chunk2", chunk.ID)
		assert.Equal(t, "doc2", chunk.DocumentID)
		assert.Equal(t, "", chunk.Content)
		assert.Nil(t, chunk.Metadata)
	})
}

func TestNewVector(t *testing.T) {
	t.Run("with embedding", func(t *testing.T) {
		embedding := []float32{0.1, 0.2, 0.3}
		metadata := map[string]any{"doc_id": "123"}

		vector := NewVector("vec1", embedding, "chunk1", metadata)

		assert.Equal(t, "vec1", vector.ID)
		assert.Equal(t, embedding, vector.Values)
		assert.Equal(t, "chunk1", vector.ChunkID)
		assert.Equal(t, metadata, vector.Metadata)
	})

	t.Run("with nil embedding", func(t *testing.T) {
		vector := NewVector("vec2", nil, "chunk2", nil)

		assert.Equal(t, "vec2", vector.ID)
		assert.Nil(t, vector.Values)
		assert.Equal(t, "chunk2", vector.ChunkID)
		assert.Nil(t, vector.Metadata)
	})
}

func TestNewDocument(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		content := "This is the document content."
		metadata := map[string]any{
			"filename": "test.pdf",
			"size":     1024,
		}

		doc := NewDocument("doc1", content, "file:///test.pdf", "pdf", metadata)

		assert.Equal(t, "doc1", doc.ID)
		assert.Equal(t, content, doc.Content)
		assert.Equal(t, metadata, doc.Metadata)
		assert.Equal(t, "file:///test.pdf", doc.Source)
		assert.Equal(t, "pdf", doc.ContentType)
	})

	t.Run("with empty content", func(t *testing.T) {
		doc := NewDocument("doc2", "", "", "", nil)

		assert.Equal(t, "doc2", doc.ID)
		assert.Equal(t, "", doc.Content)
		assert.Nil(t, doc.Metadata)
	})
}

func TestAgenticMetadata(t *testing.T) {
	t.Run("initialization", func(t *testing.T) {
		meta := &AgenticMetadata{}

		assert.Empty(t, meta.Intent)
		assert.Empty(t, meta.EntityIDs)
		assert.Empty(t, meta.SubQueries)
		assert.Empty(t, meta.RewrittenQueryText)
		assert.Empty(t, meta.OriginalQueryText)
		assert.Nil(t, meta.CacheHit)
		assert.Empty(t, meta.CRAGEvaluation)
		assert.False(t, meta.HydeApplied)
		assert.Empty(t, meta.StepBackQuery)
		assert.Nil(t, meta.Filters)
		assert.False(t, meta.ToolExecuted)
	})

	t.Run("set and get values", func(t *testing.T) {
		cacheHit := true
		meta := &AgenticMetadata{
			Intent:               "search",
			EntityIDs:            []string{"entity1", "entity2"},
			SubQueries:           []string{"sub1", "sub2"},
			RewrittenQueryText:   "rewritten query",
			OriginalQueryText:    "original query",
			CacheHit:             &cacheHit,
			CRAGEvaluation:       "relevant",
			HydeApplied:          true,
			StepBackQuery:        "step back query",
			Filters:              map[string]any{"field": "value"},
			ToolExecuted:         true,
			HypotheticalDocument: "hypothetical doc",
		}

		assert.Equal(t, "search", meta.Intent)
		assert.Equal(t, []string{"entity1", "entity2"}, meta.EntityIDs)
		assert.Equal(t, []string{"sub1", "sub2"}, meta.SubQueries)
		assert.Equal(t, "rewritten query", meta.RewrittenQueryText)
		assert.Equal(t, "original query", meta.OriginalQueryText)
		assert.NotNil(t, meta.CacheHit)
		assert.True(t, *meta.CacheHit)
		assert.Equal(t, "relevant", meta.CRAGEvaluation)
		assert.True(t, meta.HydeApplied)
		assert.Equal(t, "step back query", meta.StepBackQuery)
		assert.NotNil(t, meta.Filters)
		assert.True(t, meta.ToolExecuted)
		assert.Equal(t, "hypothetical doc", meta.HypotheticalDocument)
	})
}

func TestPipelineState(t *testing.T) {
	t.Run("initialization", func(t *testing.T) {
		state := &PipelineState{}

		assert.Nil(t, state.Query)
		assert.Nil(t, state.RetrievedChunks)
		assert.Nil(t, state.ParallelResults)
		assert.Nil(t, state.Agentic)
		assert.Empty(t, state.Answer)
		assert.Empty(t, state.GenerationPrompt)
	})

	t.Run("with query and chunks", func(t *testing.T) {
		state := &PipelineState{
			Query: NewQuery("q1", "test", nil),
			RetrievedChunks: [][]*Chunk{
				{{ID: "c1", Content: "content 1"}},
			},
			Answer: "answer",
		}

		assert.NotNil(t, state.Query)
		assert.Equal(t, "test", state.Query.Text)
		assert.Len(t, state.RetrievedChunks, 1)
		assert.Len(t, state.RetrievedChunks[0], 1)
		assert.Equal(t, "answer", state.Answer)
	})
}

func TestRAGEScores(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		scores := &RAGEScores{}

		assert.Zero(t, scores.Faithfulness)
		assert.Zero(t, scores.AnswerRelevance)
		assert.Zero(t, scores.ContextPrecision)
		assert.Zero(t, scores.OverallScore)
		assert.False(t, scores.Passed)
	})

	t.Run("with values", func(t *testing.T) {
		scores := &RAGEScores{
			Faithfulness:     0.9,
			AnswerRelevance:  0.8,
			ContextPrecision: 0.7,
			OverallScore:     0.82,
			Passed:           true,
		}

		assert.Equal(t, float32(0.9), scores.Faithfulness)
		assert.Equal(t, float32(0.8), scores.AnswerRelevance)
		assert.Equal(t, float32(0.7), scores.ContextPrecision)
		assert.Equal(t, float32(0.82), scores.OverallScore)
		assert.True(t, scores.Passed)
	})
}

func TestRetrievalResult(t *testing.T) {
	t.Run("creation", func(t *testing.T) {
		result := &RetrievalResult{
			ID:       "result1",
			QueryID:  "query1",
			Chunks:   []*Chunk{{ID: "c1", Content: "content 1"}},
			Scores:   []float32{0.95},
			Metadata: map[string]any{"key": "value"},
		}

		assert.Equal(t, "result1", result.ID)
		assert.Equal(t, "query1", result.QueryID)
		assert.Len(t, result.Chunks, 1)
		assert.Equal(t, float32(0.95), result.Scores[0])
		assert.Equal(t, "value", result.Metadata["key"])
	})
}
