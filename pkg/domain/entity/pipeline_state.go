package entity

// PipelineState defines a strongly-typed state object for pipeline steps
// It contains all the fields that pipeline steps need to exchange data
type PipelineState struct {
	// Query related fields
	Query         *Query           `json:"query"`
	OriginalQuery *Query           `json:"original_query"`
	
	// Retrieval related fields
	RetrievedChunks [][]*Chunk      `json:"retrieved_chunks"`
	ParallelResults [][]*Chunk      `json:"parallel_results"`
	RerankScores    []float32      `json:"rerank_scores"`
	Filters         map[string]any `json:"filters"`
	
	// Generation related fields
	Answer           string `json:"answer"`
	GenerationPrompt string `json:"generation_prompt"`
	
	// Self-RAG related fields
	SelfRagScore  float32 `json:"self_rag_score"`
	SelfRagReason string  `json:"self_rag_reason"`
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
