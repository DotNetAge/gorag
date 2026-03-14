// Package entity defines the core entities for the goRAG framework.
package entity

// RetrievalResult represents a retrieval result entity in the RAG system.
// It contains the results of a search query, including the retrieved chunks and their scores.
//
// Related RAG concepts:
// - Semantic Search: Represents the results of semantic search
// - Hybrid Retrieval: Can combine results from multiple retrieval strategies
// - Reranking Models: May be used to rerank the retrieved results
// - Context Augmentation: Used to augment the context for response generation
// - Ranking Quality: Scores represent the ranking quality of retrieved results
type RetrievalResult struct {
	ID       string                 `json:"id"`       // Unique identifier for the retrieval result
	QueryID  string                 `json:"query_id"`  // ID of the corresponding query
	Chunks   []*Chunk               `json:"chunks"`   // Retrieved chunks
	Scores   []float32              `json:"scores"`   // Relevance scores for the retrieved chunks
	Metadata map[string]any         `json:"metadata"` // Additional metadata about the retrieval result
}

// NewRetrievalResult creates a new retrieval result entity.
func NewRetrievalResult(id, queryID string, chunks []*Chunk, scores []float32, metadata map[string]any) *RetrievalResult {
	return &RetrievalResult{
		ID:       id,
		QueryID:  queryID,
		Chunks:   chunks,
		Scores:   scores,
		Metadata: metadata,
	}
}
