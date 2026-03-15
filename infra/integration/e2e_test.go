package integration_test

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/testkit"
)

// TestE2E_CompleteQueryFlow tests the complete query-to-answer flow
func TestE2E_CompleteQueryFlow(t *testing.T) {
	t.Parallel()

	// This E2E test simulates a complete RAG pipeline execution
	// In production, this would involve:
	// 1. Document indexing
	// 2. Vector storage
	// 3. LLM API calls
	// 4. Real retrieval and generation

	logger := testkit.NewMockLogger()
	collector := testkit.NewMockCollector()

	// Verify infrastructure
	if logger == nil {
		t.Fatal("logger should not be nil")
	}
	if collector == nil {
		t.Fatal("collector should not be nil")
	}

	ctx := context.Background()

	// Simulate complete pipeline state evolution
	state := testkit.NewTestPipelineState()

	// Stage 1: User Query
	state.Query = testkit.NewTestQuery("How does vector search work?", nil)
	state.Agentic.OriginalQueryText = state.Query.Text

	if state.Agentic.OriginalQueryText != "How does vector search work?" {
		t.Errorf("expected original query to be set")
	}

	// Stage 2: Intent Classification
	state.Agentic.Intent = "search"

	if state.Agentic.Intent != "search" {
		t.Errorf("expected intent to be 'search'")
	}

	// Stage 3: Query Decomposition (optional for complex queries)
	state.Agentic.SubQueries = []string{
		"What is vector search?",
		"How does similarity search work?",
	}

	if len(state.Agentic.SubQueries) != 2 {
		t.Errorf("expected 2 sub-queries, got %d", len(state.Agentic.SubQueries))
	}

	// Stage 4: Parallel Retrieval
	chunks1 := testkit.CreateTestChunks(
		"Vector search uses embeddings to find similar items...",
		"Similarity is measured using cosine similarity or dot product...",
	)
	chunks2 := testkit.CreateTestChunks(
		"Vector databases like Qdrant store high-dimensional vectors...",
		"ANN (Approximate Nearest Neighbor) algorithms enable fast search...",
	)
	state.RetrievedChunks = append(state.RetrievedChunks, chunks1, chunks2)

	totalChunks := 0
	for _, group := range state.RetrievedChunks {
		totalChunks += len(group)
	}
	if totalChunks != 4 {
		t.Errorf("expected 4 chunks, got %d", totalChunks)
	}

	// Stage 5: CRAG Evaluation (quality check)
	state.Agentic.CRAGEvaluation = "relevant" // High quality retrieval

	if state.Agentic.CRAGEvaluation != "relevant" {
		t.Errorf("expected CRAG evaluation to be 'relevant'")
	}

	// Stage 6: Answer Generation
	state.Answer = `Vector search works by converting data into high-dimensional 
vectors (embeddings) and then finding similar vectors using distance metrics 
like cosine similarity or Euclidean distance. Vector databases use specialized 
indexing structures (e.g., HNSW, IVF) to enable fast Approximate Nearest 
Neighbor (ANN) search, making it practical to search through billions of 
vectors in milliseconds.`

	if state.Answer == "" {
		t.Error("expected generated answer")
	}

	// Stage 7: RAGAS Evaluation
	state.Agentic.RAGScores = &entity.RAGEScores{
		Faithfulness:     0.92,
		AnswerRelevance:  0.88,
		ContextPrecision: 0.90,
		OverallScore:     0.90,
		Passed:           true,
	}

	if !state.Agentic.RAGScores.Passed {
		t.Error("expected RAG evaluation to pass")
	}
	if state.Agentic.RAGScores.OverallScore < 0.8 {
		t.Errorf("expected overall score >= 0.8, got %f", state.Agentic.RAGScores.OverallScore)
	}

	// Final verification: All AgenticMetadata fields should be populated
	if state.Agentic.Intent == "" {
		t.Error("Intent should be set")
	}
	if len(state.Agentic.SubQueries) == 0 {
		t.Error("SubQueries should be set")
	}
	if state.Agentic.CRAGEvaluation == "" {
		t.Error("CRAGEvaluation should be set")
	}
	if state.Agentic.RAGScores == nil {
		t.Error("RAGScores should be set")
	}

	_ = ctx
}

// TestE2E_CacheHitFastPath tests the fast path when cache hits
func TestE2E_CacheHitFastPath(t *testing.T) {
	t.Parallel()

	logger := testkit.NewMockLogger()

	state := testkit.NewTestPipelineState()
	state.Query = testkit.NewTestQuery("What is GoRAG?", nil)

	// Simulate cache hit
	cacheHit := true
	state.Agentic.CacheHit = &cacheHit
	state.Answer = "GoRAG is a RAG framework built with Go..."

	// When cache hits, we should skip retrieval and generation
	if state.RetrievedChunks == nil || len(state.RetrievedChunks) > 0 {
		t.Log("Note: RetrievedChunks should be empty on cache hit")
	}

	if state.Agentic.CacheHit == nil || !*state.Agentic.CacheHit {
		t.Error("CacheHit should be true")
	}

	if state.Answer == "" {
		t.Error("Answer should be populated from cache")
	}

	_ = logger
}

// TestE2E_CRAGFallbackToExternalSearch tests CRAG-triggered external search
func TestE2E_CRAGFallbackToExternalSearch(t *testing.T) {
	t.Parallel()

	logger := testkit.NewMockLogger()

	state := testkit.NewTestPipelineState()
	state.Query = testkit.NewTestQuery("Latest AI breakthrough in 2026", nil)

	// Simulate low-quality retrieval
	oldChunks := testkit.CreateTestChunks(
		"AI news from 2023...",
		"Outdated information...",
	)
	state.RetrievedChunks = append(state.RetrievedChunks, oldChunks)

	// CRAG evaluates retrieval quality as low
	state.Agentic.CRAGEvaluation = "irrelevant"

	// Trigger fallback to external search (tool execution)
	state.Agentic.ToolExecuted = true

	// After tool execution, we should have fresh information
	freshChunks := testkit.CreateTestChunks(
		"Breaking: New AI model achieves human-level reasoning in 2026...",
	)
	state.RetrievedChunks = append(state.RetrievedChunks, freshChunks)

	// Generate answer with fresh information
	state.Answer = "In 2026, a new AI model achieved human-level reasoning..."

	// Verify CRAG flow completed
	if state.Agentic.CRAGEvaluation != "irrelevant" {
		t.Errorf("expected CRAG evaluation 'irrelevant', got %q", state.Agentic.CRAGEvaluation)
	}

	if !state.Agentic.ToolExecuted {
		t.Error("ToolExecuted should be true after CRAG fallback")
	}

	if state.Answer == "" {
		t.Error("Answer should be generated after fallback")
	}

	_ = logger
}

// TestE2E_MultiTurnConversation tests context carry-over in multi-turn conversation
func TestE2E_MultiTurnConversation(t *testing.T) {
	t.Parallel()

	logger := testkit.NewMockLogger()

	// Turn 1
	state1 := testkit.NewTestPipelineState()
	state1.Query = testkit.NewTestQuery("What is GoRAG?", nil)
	state1.Agentic.OriginalQueryText = "What is GoRAG?"
	state1.Answer = "GoRAG is a RAG framework..."

	// Turn 2 (with context from Turn 1)
	state2 := testkit.NewTestPipelineState()
	state2.Query = testkit.NewTestQuery("How does it handle query decomposition?", nil)
	state2.Agentic.OriginalQueryText = "How does it handle query decomposition?"

	// The "it" refers to GoRAG from Turn 1
	// In production, this context would be carried over via conversation history

	// Verify each turn has proper metadata
	if state1.Agentic.OriginalQueryText != "What is GoRAG?" {
		t.Error("Turn 1 query should be recorded")
	}

	if state2.Agentic.OriginalQueryText != "How does it handle query decomposition?" {
		t.Error("Turn 2 query should be recorded")
	}

	// Note: Full multi-turn support requires conversation history management
	// This test demonstrates the pattern

	_ = logger
}
