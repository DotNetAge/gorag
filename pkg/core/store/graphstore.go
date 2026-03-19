package store

import (
	"context"
	
	"github.com/DotNetAge/gorag/pkg/core"
)

// GraphStore defines the storage foundation for GraphRAG.
// It tracks Nodes (Entities), Edges (Relationships), and supports semantic property queries.
type GraphStore interface {
	// AddNodes inserts or updates entities (e.g., PERSON, ORGANIZATION)
	UpsertNodes(ctx context.Context, nodes []*core.Node) error
	
	// AddEdges inserts or updates relationships between entities
	UpsertEdges(ctx context.Context, edges []*core.Edge) error
	
	// GetNode retrieves a single node/entity by ID
	GetNode(ctx context.Context, id string) (*core.Node, error)
	
	// GetNeighbors fetches up to 'limit' connected edges and nodes starting from 'nodeID'
	GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error)
	
	// Query semantic graph structure. Implementations (Neo4j, Nebula) usually take Cypher/GQL.
	Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)
	
	// GetCommunitySummaries fetches hierarchical community abstracts, which are core to Microsoft's GraphRAG paper.
	GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error)
	
	// Close cleanly tears down Graph Store connections
	Close(ctx context.Context) error
}
