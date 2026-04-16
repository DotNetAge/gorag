package core

import (
	"context"
)

// GraphStore defines the storage foundation for GraphRAG.
// It tracks Nodes (Entities), Edges (Relationships), and supports semantic property queries.
type GraphStore interface {
	// UpsertNodes inserts or updates entities (e.g., PERSON, ORGANIZATION)
	UpsertNodes(ctx context.Context, nodes []*Node) error

	// UpsertEdges inserts or updates relationships between entities
	UpsertEdges(ctx context.Context, edges []*Edge) error

	// GetNode retrieves a single node/entity by ID
	GetNode(ctx context.Context, id string) (*Node, error)

	// GetNeighbors fetches up to 'limit' connected edges and nodes starting from 'nodeID'
	GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*Node, []*Edge, error)

	// DeleteNode removes a node by ID
	DeleteNode(ctx context.Context, id string) error

	// DeleteEdge removes an edge by ID
	DeleteEdge(ctx context.Context, id string) error

	// Query semantic graph structure. Implementations (Neo4j, Nebula) usually take Cypher/GQL.
	Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)

	// GetNodesByChunkIDs retrieves all nodes associated with the given chunk IDs
	// This is used in hybrid search to find entities related to semantic search results
	GetNodesByChunkIDs(ctx context.Context, chunkIDs []string) ([]*Node, error)

	// GetEdgesByChunkIDs retrieves all edges associated with the given chunk IDs
	// This is used in hybrid search to find relationships related to semantic search results
	GetEdgesByChunkIDs(ctx context.Context, chunkIDs []string) ([]*Edge, error)

	// GetCommunitySummaries fetches hierarchical community abstracts, which are core to Microsoft's GraphRAG paper.
	GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error)

	// Close cleanly tears down Graph Store connections
	Close(ctx context.Context) error
}
