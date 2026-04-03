package core

// Metrics defines the interface for observability and performance monitoring.
// It tracks key metrics like search duration, indexing time, and error rates for system monitoring.
type Metrics interface {
	// Infrastructure Metrics
	RecordSearchDuration(engine string, duration any)
	RecordSearchResult(engine string, count int)
	RecordSearchError(engine string, err error)
	RecordIndexingDuration(parser string, duration any)
	RecordIndexingResult(parser string, count int)
	RecordEmbeddingCount(count int)
	RecordVectorStoreOperations(op string, count int)

	// RAG Business Metrics
	RecordQueryCount(engine string)                               // For QPS
	RecordLLMTokenUsage(model string, prompt int, completion int) // For Cost
	RecordRAGEvaluation(metric string, score float32)             // For Quality (Faithfulness, Relevance, etc.)
}
