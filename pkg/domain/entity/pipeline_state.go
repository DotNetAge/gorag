package entity

// RAGEScores holds RAGAS evaluation scores.
type RAGEScores struct {
	Faithfulness     float32 `json:"faithfulness"`
	AnswerRelevance  float32 `json:"answer_relevance"`
	ContextPrecision float32 `json:"context_precision"`
	OverallScore     float32 `json:"overall_score"`
	Passed           bool    `json:"passed"`
}

// AgenticMetadata provides strongly-typed metadata for agentic RAG operations.
// This eliminates the blackboard anti-pattern of using map[string]any.
type AgenticMetadata struct {
	// Intent classification result
	Intent string `json:"intent"`

	// Sub-queries from decomposition
	SubQueries []string `json:"sub_queries"`

	// Extracted entity IDs
	EntityIDs []string `json:"entity_ids"`

	// Whether HyDE was applied
	HydeApplied bool `json:"hyde_applied"`

	// Cache hit status (nil = not checked, false = miss, true = hit)
	CacheHit *bool `json:"cache_hit,omitempty"`

	// Whether tool execution was performed
	ToolExecuted bool `json:"tool_executed"`

	// CRAG evaluation result (relevant/ambiguous/irrelevant)
	CRAGEvaluation string `json:"crag_evaluation"`

	// RAGAS evaluation scores
	RAGScores *RAGEScores `json:"rag_scores,omitempty"`

	// Original query before rewriting (if any)
	OriginalQueryText string `json:"original_query_text"`

	// Rewritten query text
	RewrittenQueryText string `json:"rewritten_query_text"`

	// Hypothetical document from HyDE
	HypotheticalDocument string `json:"hypothetical_document"`

	// Filter constraints extracted from query
	Filters map[string]any `json:"filters,omitempty"`

	// Step-back query for broader context
	StepBackQuery string `json:"step_back_query"`

	// Custom fields for extensibility
	Custom map[string]any `json:"custom,omitempty"`
}

// NewAgenticMetadata creates a new AgenticMetadata instance.
func NewAgenticMetadata() *AgenticMetadata {
	return &AgenticMetadata{
		Custom: make(map[string]any),
	}
}

// SetCacheHit sets the cache hit status.
func (m *AgenticMetadata) SetCacheHit(hit bool) {
	m.CacheHit = &hit
}

// GetCacheHit returns the cache hit status and whether it was set.
func (m *AgenticMetadata) GetCacheHit() (hit bool, ok bool) {
	if m.CacheHit == nil {
		return false, false
	}
	return *m.CacheHit, true
}

// PipelineState defines a strongly-typed state object for pipeline steps
// It contains all the fields that pipeline steps need to exchange data
type PipelineState struct {
	// Query related fields
	Query         *Query `json:"query"`
	OriginalQuery *Query `json:"original_query"`

	// Retrieval related fields
	RetrievedChunks [][]*Chunk     `json:"retrieved_chunks"`
	ParallelResults [][]*Chunk     `json:"parallel_results"`
	RerankScores    []float32      `json:"rerank_scores"`
	Filters         map[string]any `json:"filters"`

	// Generation related fields
	Answer           string `json:"answer"`
	GenerationPrompt string `json:"generation_prompt"`

	// Self-RAG related fields
	SelfRagScore  float32 `json:"self_rag_score"`
	SelfRagReason string  `json:"self_rag_reason"`

	// Agentic RAG metadata (strongly-typed, eliminates blackboard anti-pattern)
	Agentic *AgenticMetadata `json:"agentic"`
}

// NewPipelineState creates a new pipeline state with empty values
func NewPipelineState() *PipelineState {
	return &PipelineState{
		RetrievedChunks: make([][]*Chunk, 0),
		ParallelResults: make([][]*Chunk, 0),
		RerankScores:    make([]float32, 0),
		Filters:         make(map[string]any),
	}
}
