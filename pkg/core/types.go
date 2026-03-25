package core

import (
	"time"
)

// Query represents a query entity in the RAG system.
type Query struct {
	ID        string         `json:"id"`         // Unique identifier for the query
	Text      string         `json:"text"`       // The actual query text
	Metadata  map[string]any `json:"metadata"`   // Additional metadata about the query
	CreatedAt time.Time      `json:"created_at"` // Creation timestamp
}

// NewQuery creates a new query entity.
func NewQuery(id, text string, metadata map[string]any) *Query {
	return &Query{
		ID:        id,
		Text:      text,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}
}

// IntentType represents the classified intent of a query.
type IntentType string

const (
	IntentChat           IntentType = "chat"
	IntentDomainSpecific IntentType = "domain_specific"
	IntentFactCheck      IntentType = "fact_check"
	IntentRelational     IntentType = "relational" // New: Queries about entity relationships (GraphRAG)
	IntentGlobal         IntentType = "global"     // New: Summary or global knowledge queries
)

// IntentResult holds the result of intent classification.
type IntentResult struct {
	Intent     IntentType `json:"intent"`
	Confidence float32    `json:"confidence"`
	Reason     string     `json:"reason"`
}

// RetrievalResult represents a search query's retrieved chunks and scores.
type RetrievalResult struct {
	ID       string         `json:"id"`
	QueryID  string         `json:"query_id"`
	Query    string         `json:"query"`  // Original query text
	Chunks   []*Chunk       `json:"chunks"`
	Scores   []float32      `json:"scores"`
	Answer   string         `json:"answer"` // Generated answer (if any)
	Metadata map[string]any `json:"metadata"`
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

// EntityExtractionResult holds the result of entity extraction.
type EntityExtractionResult struct {
	Entities []string `json:"entities"`
}

// DecompositionResult holds the result of query decomposition.
type DecompositionResult struct {
	SubQueries []string `json:"sub_queries"`
	Reasoning  string   `json:"reasoning"`
	IsComplex  bool     `json:"is_complex"`
}

// CRAGLabel represents the relevance label for CRAG.
type CRAGLabel string

const (
	CRAGRelevant   CRAGLabel = "relevant"
	CRAGIrrelevant CRAGLabel = "irrelevant"
	CRAGAmbiguous  CRAGLabel = "ambiguous"
)

// CRAGEvaluation holds the result of CRAG evaluation.
type CRAGEvaluation struct {
	Label     CRAGLabel
	Reasoning string
	Score     float32
}

// RAGEvaluation holds the result of RAG quality evaluation.
type RAGEvaluation struct {
	Faithfulness  float32
	Relevance     float32
	ContextRecall float32 // How much of the necessary information was retrieved
	OverallScore  float32
	Passed        bool
	Feedback      string
}

// SearchRequest is the DTO for search use case input.
type SearchRequest struct {
	Query     *Query
	TopK      int
	UserID    string
	SessionID string
}

// SearchResponse is the DTO for search use case output.
type SearchResponse struct {
	Answer          string
	Chunks          []string
	Score           float32
	Intent          IntentType
	SourceDocuments []string
}

// AgenticSearchResponse extends SearchResponse with agentic metadata.
type AgenticSearchResponse struct {
	SearchResponse
	SubQueries     []string
	CRAGEvaluation *CRAGEvaluation
	RAGEvaluation  *RAGEvaluation
}

// ChatRequest is the DTO for chat use case input.
type ChatRequest struct {
	Message   string
	UserID    string
	SessionID string
	History   []string
}

// ChatResponse is the DTO for chat use case output.
type ChatResponse struct {
	Message   string
	SessionID string
}

// IndexRequest is the DTO for indexing use case input.
type IndexRequest struct {
	Documents  []*Document
	Collection string
	BatchSize  int
}

// IndexResponse is the DTO for indexing use case output.
type IndexResponse struct {
	TotalDocuments  int
	FailedDocuments int
	Errors          []string
}
