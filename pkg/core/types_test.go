package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewQuery(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		text     string
		metadata map[string]any
	}{
		{
			name:     "basic query",
			id:       "query-1",
			text:     "What is RAG?",
			metadata: map[string]any{"user": "test"},
		},
		{
			name:     "nil metadata",
			id:       "query-2",
			text:     "Another query",
			metadata: nil,
		},
		{
			name:     "empty text",
			id:       "query-3",
			text:     "",
			metadata: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := NewQuery(tt.id, tt.text, tt.metadata)

			assert.Equal(t, tt.id, query.ID)
			assert.Equal(t, tt.text, query.Text)
			assert.Equal(t, tt.metadata, query.Metadata)
			assert.False(t, query.CreatedAt.IsZero())
		})
	}
}

func TestIntentType(t *testing.T) {
	tests := []struct {
		name     string
		intent   IntentType
		expected string
	}{
		{"chat intent", IntentChat, "chat"},
		{"domain specific", IntentDomainSpecific, "domain_specific"},
		{"fact check", IntentFactCheck, "fact_check"},
		{"relational", IntentRelational, "relational"},
		{"global", IntentGlobal, "global"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.intent))
		})
	}
}

func TestRetrievalResult(t *testing.T) {
	t.Run("NewRetrievalResult", func(t *testing.T) {
		chunks := []*Chunk{
			NewChunk("chunk-1", "doc-1", "content 1", 0, 10, nil),
			NewChunk("chunk-2", "doc-1", "content 2", 11, 20, nil),
		}
		scores := []float32{0.95, 0.85}
		metadata := map[string]any{"engine": "test"}

		result := NewRetrievalResult("result-1", "query-1", chunks, scores, metadata)

		assert.Equal(t, "result-1", result.ID)
		assert.Equal(t, "query-1", result.QueryID)
		assert.Len(t, result.Chunks, 2)
		assert.Len(t, result.Scores, 2)
		assert.Equal(t, metadata, result.Metadata)
		assert.Empty(t, result.Answer)
	})

	t.Run("with answer", func(t *testing.T) {
		result := NewRetrievalResult("result-1", "query-1", nil, nil, nil)
		result.Answer = "This is the answer"
		assert.Equal(t, "This is the answer", result.Answer)
	})
}

func TestEntityExtractionResult(t *testing.T) {
	result := &EntityExtractionResult{
		Entities: []string{"entity1", "entity2", "entity3"},
	}

	assert.Len(t, result.Entities, 3)
	assert.Equal(t, "entity1", result.Entities[0])
}

func TestDecompositionResult(t *testing.T) {
	result := &DecompositionResult{
		SubQueries: []string{"sub query 1", "sub query 2"},
		Reasoning:  "Complex query needs decomposition",
		IsComplex:  true,
	}

	assert.True(t, result.IsComplex)
	assert.Len(t, result.SubQueries, 2)
	assert.NotEmpty(t, result.Reasoning)
}

func TestCRAGEvaluation(t *testing.T) {
	tests := []struct {
		name  string
		label CRAGLabel
		score float32
	}{
		{"relevant", CRAGRelevant, 0.9},
		{"irrelevant", CRAGIrrelevant, 0.2},
		{"ambiguous", CRAGAmbiguous, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := &CRAGEvaluation{
				Label:     tt.label,
				Reasoning: "Test reasoning",
				Score:     tt.score,
			}

			assert.Equal(t, tt.label, eval.Label)
			assert.NotEmpty(t, eval.Reasoning)
			assert.Equal(t, tt.score, eval.Score)
		})
	}
}

func TestRAGEvaluation(t *testing.T) {
	tests := []struct {
		name          string
		faithfulness  float32
		relevance     float32
		expectedPass  bool
		expectedScore float32
	}{
		{"high quality", 0.9, 0.95, true, 0.925},
		{"low quality", 0.3, 0.4, false, 0.35},
		{"medium quality", 0.6, 0.7, true, 0.65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := &RAGEvaluation{
				Faithfulness: tt.faithfulness,
				Relevance:    tt.relevance,
				OverallScore: tt.expectedScore,
				Passed:       tt.expectedPass,
				Feedback:     "Good evaluation",
			}

			assert.Equal(t, tt.faithfulness, eval.Faithfulness)
			assert.Equal(t, tt.relevance, eval.Relevance)
			assert.Equal(t, tt.expectedScore, eval.OverallScore)
			assert.Equal(t, tt.expectedPass, eval.Passed)
			assert.NotEmpty(t, eval.Feedback)
		})
	}
}

func TestSearchRequest(t *testing.T) {
	query := NewQuery("q-1", "test query", nil)
	req := &SearchRequest{
		Query:     query,
		TopK:      5,
		UserID:    "user-123",
		SessionID: "session-456",
	}

	assert.NotNil(t, req.Query)
	assert.Equal(t, 5, req.TopK)
	assert.Equal(t, "user-123", req.UserID)
	assert.Equal(t, "session-456", req.SessionID)
}

func TestSearchResponse(t *testing.T) {
	resp := &SearchResponse{
		Answer:          "The answer is 42",
		Chunks:          []string{"chunk 1", "chunk 2"},
		Score:           0.92,
		Intent:          IntentFactCheck,
		SourceDocuments: []string{"doc-1", "doc-2"},
	}

	assert.NotEmpty(t, resp.Answer)
	assert.Len(t, resp.Chunks, 2)
	assert.Greater(t, resp.Score, float32(0.0))
	assert.Equal(t, IntentFactCheck, resp.Intent)
	assert.Len(t, resp.SourceDocuments, 2)
}

func TestAgenticSearchResponse(t *testing.T) {
	baseResp := SearchResponse{
		Answer: "Agentic answer",
		Score:  0.88,
	}

	agenticResp := &AgenticSearchResponse{
		SearchResponse: baseResp,
		SubQueries:     []string{"sub query 1", "sub query 2"},
		CRAGEvaluation: &CRAGEvaluation{Label: CRAGRelevant, Score: 0.9},
		RAGEvaluation:  &RAGEvaluation{Faithfulness: 0.85, Relevance: 0.9, OverallScore: 0.875, Passed: true},
	}

	assert.Equal(t, "Agentic answer", agenticResp.Answer)
	assert.Len(t, agenticResp.SubQueries, 2)
	assert.NotNil(t, agenticResp.CRAGEvaluation)
	assert.NotNil(t, agenticResp.RAGEvaluation)
	assert.True(t, agenticResp.RAGEvaluation.Passed)
}

func TestChatRequest(t *testing.T) {
	req := &ChatRequest{
		Message:   "Hello, how are you?",
		UserID:    "user-123",
		SessionID: "session-456",
		History:   []string{"Previous message 1", "Previous message 2"},
	}

	assert.NotEmpty(t, req.Message)
	assert.Equal(t, "user-123", req.UserID)
	assert.Len(t, req.History, 2)
}

func TestChatResponse(t *testing.T) {
	resp := &ChatResponse{
		Message:   "I'm doing well, thank you!",
		SessionID: "session-456",
	}

	assert.NotEmpty(t, resp.Message)
	assert.Equal(t, "session-456", resp.SessionID)
}

func TestIndexRequest(t *testing.T) {
	docs := []*Document{
		NewDocument("doc-1", "content 1", "/path1", "text/plain", nil),
		NewDocument("doc-2", "content 2", "/path2", "text/plain", nil),
	}

	req := &IndexRequest{
		Documents:  docs,
		Collection: "test-collection",
		BatchSize:  10,
	}

	assert.Len(t, req.Documents, 2)
	assert.Equal(t, "test-collection", req.Collection)
	assert.Equal(t, 10, req.BatchSize)
}

func TestIndexResponse(t *testing.T) {
	resp := &IndexResponse{
		TotalDocuments:  100,
		FailedDocuments: 2,
		Errors:          []string{"Error 1", "Error 2"},
	}

	assert.Equal(t, 100, resp.TotalDocuments)
	assert.Equal(t, 2, resp.FailedDocuments)
	assert.Len(t, resp.Errors, 2)
}
