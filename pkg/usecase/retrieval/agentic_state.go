package retrieval

import (
	"fmt"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// AgenticMetadata provides strongly-typed metadata for agentic RAG operations.
// This eliminates the blackboard anti-pattern of using map[string]any.
type AgenticMetadata struct {
	// Intent classification result
	Intent string

	// Sub-queries from decomposition
	SubQueries []string

	// Extracted entity IDs
	EntityIDs []string

	// Whether HyDE was applied
	HydeApplied bool

	// Cache hit status (nil = not checked, false = miss, true = hit)
	CacheHit *bool

	// Whether tool execution was performed
	ToolExecuted bool

	// CRAG evaluation result (relevant/ambiguous/irrelevant)
	CRAGEvaluation string

	// RAGAS evaluation scores
	RAGScores *RAGEScores

	// Original query before rewriting (if any)
	OriginalQueryText string

	// Rewritten query text
	RewrittenQueryText string

	// Hypothetical document from HyDE
	HypotheticalDocument string

	// Filter constraints extracted from query
	Filters map[string]any

	// Step-back query for broader context
	StepBackQuery string

	// Custom fields for extensibility
	Custom map[string]any
}

// NewAgenticMetadata creates a new AgenticMetadata instance.
func NewAgenticMetadata() *AgenticMetadata {
	return &AgenticMetadata{
		Custom: make(map[string]any),
	}
}

// Validate checks if the metadata is valid.
func (m *AgenticMetadata) Validate() error {
	if m == nil {
		return fmt.Errorf("AgenticMetadata is nil")
	}
	// Add more validation rules as needed
	return nil
}

// MergeToQuery merges the metadata into a Query's Metadata map.
// This is used for backward compatibility with existing code.
func (m *AgenticMetadata) MergeToQuery(query *entity.Query) {
	if m == nil || query == nil {
		return
	}

	if query.Metadata == nil {
		query.Metadata = make(map[string]any)
	}

	// Merge all fields
	if m.Intent != "" {
		query.Metadata["intent"] = m.Intent
	}
	if len(m.SubQueries) > 0 {
		query.Metadata["sub_queries"] = m.SubQueries
	}
	if len(m.EntityIDs) > 0 {
		query.Metadata["entity_ids"] = m.EntityIDs
	}
	if m.HydeApplied {
		query.Metadata["hyde_applied"] = true
	}
	if m.CacheHit != nil {
		query.Metadata["cache_hit"] = *m.CacheHit
	}
	if m.ToolExecuted {
		query.Metadata["tool_executed"] = true
	}
	if m.CRAGEvaluation != "" {
		query.Metadata["crag_evaluation"] = m.CRAGEvaluation
	}
	if m.RAGScores != nil {
		query.Metadata["rag_scores"] = m.RAGScores
	}
	if m.OriginalQueryText != "" {
		query.Metadata["original_query"] = m.OriginalQueryText
	}
	if m.RewrittenQueryText != "" {
		query.Metadata["rewritten_query"] = m.RewrittenQueryText
	}
	if m.HypotheticalDocument != "" {
		query.Metadata["hypothetical_document"] = m.HypotheticalDocument
	}
	if m.Filters != nil {
		query.Metadata["filters"] = m.Filters
	}
	if m.StepBackQuery != "" {
		query.Metadata["step_back_query"] = m.StepBackQuery
	}

	// Merge custom fields
	for k, v := range m.Custom {
		query.Metadata[k] = v
	}
}

// LoadFromQuery loads metadata from a Query's Metadata map.
// This is used for backward compatibility with existing code.
func (m *AgenticMetadata) LoadFromQuery(query *entity.Query) {
	if m == nil || query == nil || query.Metadata == nil {
		return
	}

	// Load all fields
	if v, ok := query.Metadata["intent"].(string); ok {
		m.Intent = v
	}
	if v, ok := query.Metadata["sub_queries"].([]string); ok {
		m.SubQueries = v
	}
	if v, ok := query.Metadata["entity_ids"].([]string); ok {
		m.EntityIDs = v
	}
	if v, ok := query.Metadata["hyde_applied"].(bool); ok {
		m.HydeApplied = v
	}
	if v, ok := query.Metadata["cache_hit"].(bool); ok {
		m.CacheHit = &v
	}
	if v, ok := query.Metadata["tool_executed"].(bool); ok {
		m.ToolExecuted = v
	}
	if v, ok := query.Metadata["crag_evaluation"].(string); ok {
		m.CRAGEvaluation = v
	}
	if v, ok := query.Metadata["original_query"].(string); ok {
		m.OriginalQueryText = v
	}
	if v, ok := query.Metadata["rewritten_query"].(string); ok {
		m.RewrittenQueryText = v
	}
	if v, ok := query.Metadata["hypothetical_document"].(string); ok {
		m.HypotheticalDocument = v
	}
	if v, ok := query.Metadata["step_back_query"].(string); ok {
		m.StepBackQuery = v
	}

	// Load custom fields (exclude known fields)
	knownFields := map[string]bool{
		"intent": true, "sub_queries": true, "entity_ids": true,
		"hyde_applied": true, "cache_hit": true, "tool_executed": true,
		"crag_evaluation": true, "original_query": true, "rewritten_query": true,
		"hypothetical_document": true, "step_back_query": true,
	}

	m.Custom = make(map[string]any)
	for k, v := range query.Metadata {
		if !knownFields[k] {
			m.Custom[k] = v
		}
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
