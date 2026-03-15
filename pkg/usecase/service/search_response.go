package service

import "github.com/DotNetAge/gorag/pkg/usecase/retrieval"

// SearchResponse is the DTO for search use case output.
type SearchResponse struct {
	// Answer is the generated response
	Answer string

	// Chunks contains the retrieved chunks used in the answer
	Chunks []string

	// Score is the relevance score (0-1)
	Score float32

	// Intent is the classified intent
	Intent retrieval.IntentType

	// SourceDocuments lists the source document IDs
	SourceDocuments []string
}

// ChatResponse is the DTO for chat use case output.
type ChatResponse struct {
	// Message is the chatbot's response
	Message string

	// SessionID tracks the conversation session
	SessionID string
}

// IndexResponse is the DTO for indexing use case output.
type IndexResponse struct {
	// TotalDocuments indexed successfully
	TotalDocuments int

	// FailedDocuments count
	FailedDocuments int

	// Errors encountered during indexing
	Errors []string
}

// AgenticSearchResponse extends SearchResponse with Agentic RAG metadata.
type AgenticSearchResponse struct {
	SearchResponse

	// SubQueries used in parallel retrieval
	SubQueries []string

	// CRAGEvaluation result
	CRAGEvaluation *retrieval.CRAGEvaluation

	// RAGEvaluation result
	RAGEvaluation *retrieval.RAGEScores

	// ToolExecutions tracks any external tool calls
	ToolExecutions []string
}
