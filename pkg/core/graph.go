package core

import "context"

// Node represents a graph node entity in the RAG system.
type Node struct {
	ID         string         `json:"id"`         // Unique identifier for the node
	Type       string         `json:"type"`       // Type of the node
	Properties map[string]any `json:"properties"` // Properties of the node
}

// Edge represents a graph edge entity in the RAG system.
type Edge struct {
	ID         string         `json:"id"`         // Unique identifier for the edge
	Type       string         `json:"type"`       // Type of the edge
	Source     string         `json:"source"`     // Source node ID
	Target     string         `json:"target"`     // Target node ID
	Properties map[string]any `json:"properties"` // Properties of the edge
}

// GraphExtractor defines the interface for extracting graph structures from text chunks.
// It uses LLM-based entity and relationship extraction to build knowledge graphs.
type GraphExtractor interface {
	// Extract parses a chunk and returns a list of Nodes (Entities) and Edges (Relationships).
	Extract(ctx context.Context, chunk *Chunk) ([]Node, []Edge, error)
}
