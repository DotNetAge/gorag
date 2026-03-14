// Package abstraction defines the storage abstraction interfaces for the goRAG framework.
package abstraction

import (
	"context"
)

// Node represents a graph node entity in the RAG system.
// It contains the node ID, type, and properties.
//
// Related RAG concepts:
// - GraphRAG: Used to represent entities and their relationships in a knowledge graph
// - LightRAG: Can be used in lightweight graph structures for efficient retrieval
type Node struct {
	ID         string                 `json:"id"`         // Unique identifier for the node
	Type       string                 `json:"type"`       // Type of the node
	Properties map[string]any         `json:"properties"` // Properties of the node
}

// Edge represents a graph edge entity in the RAG system.
// It contains the edge ID, type, source, target, and properties.
//
// Related RAG concepts:
// - GraphRAG: Used to represent relationships between entities in a knowledge graph
// - LightRAG: Can be used in lightweight graph structures for efficient retrieval
type Edge struct {
	ID         string                 `json:"id"`         // Unique identifier for the edge
	Type       string                 `json:"type"`       // Type of the edge
	Source     string                 `json:"source"`     // Source node ID
	Target     string                 `json:"target"`     // Target node ID
	Properties map[string]any         `json:"properties"` // Properties of the edge
}

// GraphStore defines the interface for graph storage in the RAG system.
// It provides methods for creating, retrieving, and managing nodes and edges.
//
// Related RAG concepts:
// - GraphRAG: Implements graph storage for knowledge graph-based RAG
// - LightRAG: Can be used for lightweight graph structures
// - Multi-Hop Reasoning: Enables multi-hop reasoning over graph data
type GraphStore interface {
	// CreateNode creates a new node in the graph.
	CreateNode(ctx context.Context, node *Node) error
	
	// CreateEdge creates a new edge in the graph.
	CreateEdge(ctx context.Context, edge *Edge) error
	
	// GetNode retrieves a node by its ID.
	GetNode(ctx context.Context, id string) (*Node, error)
	
	// GetEdge retrieves an edge by its ID.
	GetEdge(ctx context.Context, id string) (*Edge, error)
	
	// DeleteNode deletes a node from the graph.
	DeleteNode(ctx context.Context, id string) error
	
	// DeleteEdge deletes an edge from the graph.
	DeleteEdge(ctx context.Context, id string) error
	
	// Query executes a graph query and returns the results.
	Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)
	
	// GetNeighbors retrieves the neighbors of a node.
	GetNeighbors(ctx context.Context, nodeID string, limit int) ([]*Node, error)
	
	// GetCommunitySummaries retrieves community summaries from the graph.
	GetCommunitySummaries(ctx context.Context, limit int) ([]map[string]any, error)
	
	// UpsertNodes batch updates or inserts nodes.
	UpsertNodes(ctx context.Context, nodes []*Node) error
	
	// UpsertEdges batch updates or inserts edges.
	UpsertEdges(ctx context.Context, edges []*Edge) error
	
	// Close closes the graph store.
	Close(ctx context.Context) error
}
