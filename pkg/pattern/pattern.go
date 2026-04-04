package pattern

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexer"
)

// RAGPattern defines a high-level application interface that combines indexing and retrieval.
// It simplifies the usage of complex RAG pipelines for common application scenarios.
//
// RAGPattern exposes three independent interfaces, each with distinct responsibilities:
//
//	Indexer()    - Index construction and management
//	Retriever()  - Semantic search and retrieval
//	Repository() - Entity data access with index synchronization (CRUD + index sync)
//
// This separation ensures:
//   - Clear separation of concerns
//   - Data consistency between storage and indexes
//   - Flexibility for developers to choose the appropriate interface
type RAGPattern interface {
	// Indexer returns the underlying indexer for document processing.
	Indexer() indexer.Indexer
	// Retriever returns the underlying retriever for search operations.
	Retriever() core.Retriever
	// Repository returns the entity data access interface with automatic index synchronization.
	// Use Repository to store any entity type in different collections with auto-vectorization.
	Repository() core.Repository

	// IndexFile is a convenience method for indexing a single file.
	IndexFile(ctx context.Context, filePath string) error
	// IndexDirectory is a convenience method for indexing an entire directory.
	IndexDirectory(ctx context.Context, dirPath string, recursive bool) error
	// Retrieve performs parallel retrieval for multiple queries.
	Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error)

	// IndexText indexes plain text content directly (no file parsing required).
	IndexText(ctx context.Context, text string, metadata ...map[string]any) error
	// IndexTexts indexes multiple plain text contents in batch.
	IndexTexts(ctx context.Context, texts []string, metadata ...map[string]any) error
	// Delete removes indexed content by ID (cascades to chunks and vectors).
	Delete(ctx context.Context, id string) error
}

// GraphRAGPattern extends RAGPattern with graph-specific operations.
type GraphRAGPattern interface {
	RAGPattern

	// GraphRepository returns the graph data access interface with automatic index synchronization.
	// Use GraphRepository for Node/Edge CRUD operations that require storage-index consistency.
	GraphRepository() core.GraphRepository

	// AddNode adds a single node to the knowledge graph.
	AddNode(ctx context.Context, node *core.Node) error
	// AddNodes adds multiple nodes to the knowledge graph.
	AddNodes(ctx context.Context, nodes []*core.Node) error
	// GetNode retrieves a node by its ID.
	GetNode(ctx context.Context, nodeID string) (*core.Node, error)
	// DeleteNode removes a node and its associated edges from the graph.
	DeleteNode(ctx context.Context, nodeID string) error

	// AddEdge adds a single edge (relationship) to the knowledge graph.
	AddEdge(ctx context.Context, edge *core.Edge) error
	// AddEdges adds multiple edges to the knowledge graph.
	AddEdges(ctx context.Context, edges []*core.Edge) error
	// DeleteEdge removes an edge from the graph.
	DeleteEdge(ctx context.Context, edgeID string) error

	// QueryGraph executes a Cypher/GQL query against the knowledge graph.
	QueryGraph(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)
	// GetNeighbors retrieves neighboring nodes and edges starting from a node.
	GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error)
}
