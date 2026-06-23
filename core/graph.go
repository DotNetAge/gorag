// Package core provides fundamental types and interfaces for the goRAG framework.
package core

// Node represents a graph node entity in the RAG knowledge graph.
// In GraphRAG, nodes are derived from text chunks and serve as an index layer
// for enhanced retrieval capabilities. Nodes represent entities extracted from documents.
type Node struct {
	ID     string   `json:"id"`     // Unique identifier for the node
	Labels []string `json:"labels"` // Types/categories (e.g., ["Person", "Organization"]), aligns with gograph.Node.Labels
	Name   string   `json:"name"`   // Entity name (cleaned text, e.g., "张三", "阿里巴巴")

	// Properties stores extended features with standardized keys:
	// - "confidence": float32 - extraction confidence (0~1 from LLM/rules)
	// - "frequency": int - occurrence count across documents
	// - "vectors": []float32 - semantic embedding vectors
	// - "aliases": []string - alternative names
	// - custom fields as needed
	Properties map[string]any `json:"properties,omitempty"`

	// Source binding - following Microsoft GraphRAG design: graph as index layer
	SourceChunkIDs []string `json:"source_chunk_ids,omitempty"` // IDs of source chunks
	SourceDocIDs   []string `json:"source_doc_ids,omitempty"`   // IDs of source documents
}

// Edge represents a graph edge (relationship) between two nodes in the knowledge graph.
// Edges capture relationships between entities and are bound to source text chunks
// for traceability and evidence retrieval.
type Edge struct {
	ID        string `json:"id"`                  // Unique identifier for the edge
	Type      string `json:"type"`                // Type of the edge (e.g., WORKS_FOR, LOCATED_IN, BELONGS_TO)
	Source    string `json:"source"`              // Source node ID (subject entity)
	Target    string `json:"target"`              // Target node ID (object entity)
	Predicate string `json:"predicate,omitempty"` // Relationship type alias (e.g., "就职于", "属于")

	// Properties stores extended features with standardized keys:
	// - "confidence": float32 - extraction confidence (0~1 from LLM/rules)
	// - "score": float32 - relationship strength score
	// - "evidence": string - text evidence for the relationship
	// - custom fields as needed
	Properties map[string]any `json:"properties,omitempty"`

	// Source binding - following Microsoft GraphRAG design
	SourceChunkIDs []string `json:"source_chunk_ids,omitempty"` // IDs of source chunks
	SourceDocIDs   []string `json:"source_doc_ids,omitempty"`   // IDs of source documents
}

