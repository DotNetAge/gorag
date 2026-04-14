package core

import "context"

// Triple represents a subject-predicate-object relationship extracted from text.
// Triples are the building blocks for knowledge graph construction.
type Triple struct {
	Subject     string `json:"subject"`      // Subject entity (e.g., "张三")
	Predicate   string `json:"predicate"`    // Relationship type (e.g., "WORKS_FOR")
	Object      string `json:"object"`       // Object entity (e.g., "阿里巴巴")
	SubjectType string `json:"subject_type"` // Subject entity type (e.g., "Person")
	ObjectType  string `json:"object_type"`  // Object entity type (e.g., "Organization")

	// Source binding
	SourceChunkID string  `json:"source_chunk_id,omitempty"`
	SourceDocID   string  `json:"source_doc_id,omitempty"`
	Confidence    float32 `json:"confidence,omitempty"`
}

// Node represents a graph node entity in the RAG system.
// In GraphRAG, nodes are derived from text chunks and serve as an index layer.
type Node struct {
	ID         string         `json:"id"`         // Unique identifier for the node
	Type       string         `json:"type"`       // Type of the node (e.g., Person, Organization)
	Properties map[string]any `json:"properties"` // Properties of the node

	// Source binding - following Microsoft GraphRAG design: graph as index layer
	SourceChunkIDs []string `json:"source_chunk_ids,omitempty"` // IDs of source chunks
	SourceDocIDs   []string `json:"source_doc_ids,omitempty"`   // IDs of source documents
}

// Edge represents a graph edge entity in the RAG system.
// Edges represent relationships between entities and are also bound to source text.
type Edge struct {
	ID         string         `json:"id"`         // Unique identifier for the edge
	Type       string         `json:"type"`       // Type of the edge (e.g., WORKS_FOR, LOCATED_IN)
	Source     string         `json:"source"`     // Source node ID
	Target     string         `json:"target"`     // Target node ID
	Properties map[string]any `json:"properties"` // Properties of the edge

	// Source binding - following Microsoft GraphRAG design
	SourceChunkIDs []string `json:"source_chunk_ids,omitempty"` // IDs of source chunks
	SourceDocIDs   []string `json:"source_doc_ids,omitempty"`   // IDs of source documents
}

// Community represents a detected community in the knowledge graph.
// Communities are hierarchical groups of related nodes, enabling global search.
type Community struct {
	ID       string   `json:"id"`                  // Unique identifier for the community
	Level    int      `json:"level"`               // Hierarchy level (0 = finest granularity)
	NodeIDs  []string `json:"node_ids"`            // Node IDs in this community
	EdgeIDs  []string `json:"edge_ids"`            // Edge IDs in this community
	ParentID string   `json:"parent_id,omitempty"` // Parent community ID (for hierarchy)

	// LLM-generated summary
	Summary  string   `json:"summary,omitempty"`  // Community summary
	Keywords []string `json:"keywords,omitempty"` // Key topics/concepts

	// Source binding
	SourceChunkIDs []string `json:"source_chunk_ids,omitempty"`
}

// SearchMode defines the search strategy for GraphRAG retrieval.
type SearchMode string

const (
	// SearchModeLocal uses graph traversal from extracted entities.
	// Best for: specific questions about entities and their relationships.
	SearchModeLocal SearchMode = "local"

	// SearchModeGlobal uses community summaries for macro-level queries.
	// Best for: "What are the main themes?" type questions.
	SearchModeGlobal SearchMode = "global"

	// SearchModeHybrid combines local and global search with vector search.
	// Best for: complex queries needing both specific facts and context.
	SearchModeHybrid SearchMode = "hybrid"
)

// CommunityMatch represents a matched community during global search.
type CommunityMatch struct {
	CommunityID string   `json:"community_id"`
	Score       float32  `json:"score"`
	Summary     string   `json:"summary"`
	Keywords    []string `json:"keywords"`
}

// GraphExtractor defines the interface for extracting graph structures from text chunks.
// It uses LLM-based entity and relationship extraction to build knowledge graphs.
type GraphExtractor interface {
	// Extract parses a chunk and returns a list of Nodes (Entities) and Edges (Relationships).
	Extract(ctx context.Context, chunk *Chunk) ([]Node, []Edge, error)
}

// CommunityDetector defines the interface for community detection algorithms.
type CommunityDetector interface {
	// Detect identifies communities in the graph and returns them hierarchically.
	Detect(ctx context.Context, graphStore GraphStore) ([]*Community, error)
}

// TriplesExtractor extracts knowledge triples from text for graph construction.
// This is the core interface for GraphRAG indexing pipeline.
type TriplesExtractor interface {
	Extract(ctx context.Context, text string) ([]Triple, error)
}
