package core

import "context"

// CRAGEvaluator defines the interface for evaluating retrieval relevance in Corrective RAG.
// It determines whether retrieved context is relevant, irrelevant, or ambiguous to decide on web search fallback.
type CRAGEvaluator interface {
	Evaluate(ctx context.Context, query *Query, chunks []*Chunk) (*CRAGEvaluation, error)
}

// RAGEvaluator defines the interface for evaluating overall RAG quality.
// It assesses faithfulness, relevance, and answer quality using metrics like RAGAS.
type RAGEvaluator interface {
	Evaluate(ctx context.Context, query string, answer string, context string) (*RAGEvaluation, error)
}
