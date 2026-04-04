package core

import "context"

// Entity is the base interface for any entity that can be stored in a Repository.
// Any type with an ID can implement this interface and be managed by Repository.
//
// Example:
//
//	type User struct {
//	    ID    string
//	    Name  string
//	    Email string
//	}
//	
//	func (u *User) GetID() string { return u.ID }
//	
//	// User can now be stored in Repository
//	repo.Create(ctx, "users", user, user.Email)  // Index by email
type Entity interface {
	GetID() string
}

// Repository is a generic data access interface that ensures consistency
// between data storage and index synchronization.
// It follows the Repository Pattern to decouple business logic from storage implementation.
//
// Key Features:
//  1. Collection-based storage: Each entity type can be stored in its own collection
//  2. Index synchronization: Automatically chunks and indexes the content
//  3. Content flexibility: User decides what content to index
//
// Example:
//
//	repo := rag.Repository()
//	
//	// Store and index different entities
//	user := &User{ID: "u1", Name: "Alice", Bio: "Software engineer..."}
//	repo.Create(ctx, "users", user, user.Bio)  // Index by bio
//	
//	product := &Product{ID: "p1", Name: "Laptop", Description: "..."}
//	repo.Create(ctx, "products", product, product.Description)  // Index by description
type Repository interface {
	// Create stores an entity and indexes the content.
	// The content parameter is what gets chunked and vectorized.
	// - collection: namespace for this entity type
	// - entity: the entity to store (must implement Entity interface)
	// - content: the text content to chunk and index (user decides what to index)
	Create(ctx context.Context, collection string, entity Entity, content string) error

	// Read retrieves an entity by ID from the specified collection.
	Read(ctx context.Context, collection string, id string) (Entity, error)

	// Update modifies an entity and re-indexes the content.
	Update(ctx context.Context, collection string, entity Entity, content string) error

	// Delete removes an entity and all its chunks and vectors.
	Delete(ctx context.Context, collection string, id string) error

	// List retrieves entities matching the filter criteria.
	List(ctx context.Context, collection string, filter map[string]any) ([]Entity, error)
}

// TypedRepository is a type-safe wrapper around Repository for specific entity types.
type TypedRepository[T Entity] interface {
	Create(ctx context.Context, collection string, entity T, content string) error
	Read(ctx context.Context, collection string, id string) (T, error)
	Update(ctx context.Context, collection string, entity T, content string) error
	Delete(ctx context.Context, collection string, id string) error
	List(ctx context.Context, collection string, filter map[string]any) ([]T, error)
}

// GraphRepository provides data access for Nodes and Edges in the knowledge graph.
// It ensures graph storage is synchronized with VectorStore (optional) and DocStore.
//
// Key Features:
//  1. Type-safe node and edge operations
//  2. Automatic vector synchronization for nodes with content
//  3. Support for custom node and edge types
type GraphRepository interface {
	// Node operations

	// CreateNode stores a node and indexes its content property.
	// If node.Properties["content"] exists, it will be chunked and vectorized.
	CreateNode(ctx context.Context, node *Node) error

	// ReadNode retrieves a node by ID.
	ReadNode(ctx context.Context, nodeID string) (*Node, error)

	// UpdateNode modifies a node and re-indexes its content.
	UpdateNode(ctx context.Context, node *Node) error

	// DeleteNode removes a node, its edges, chunks and vectors.
	DeleteNode(ctx context.Context, nodeID string) error

	// ListNodes retrieves nodes matching the filter criteria.
	ListNodes(ctx context.Context, filter map[string]any) ([]*Node, error)

	// Edge operations

	// CreateEdge stores an edge (relationship) between nodes.
	CreateEdge(ctx context.Context, edge *Edge) error

	// ReadEdge retrieves an edge by ID.
	ReadEdge(ctx context.Context, edgeID string) (*Edge, error)

	// UpdateEdge modifies an edge.
	UpdateEdge(ctx context.Context, edge *Edge) error

	// DeleteEdge removes an edge.
	DeleteEdge(ctx context.Context, edgeID string) error

	// ListEdges retrieves edges matching the filter criteria.
	ListEdges(ctx context.Context, filter map[string]any) ([]*Edge, error)

	// Graph traversal

	// GetNeighbors retrieves neighboring nodes and edges starting from a node.
	GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*Node, []*Edge, error)

	// Query executes a graph query (Cypher/GQL).
	Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)
}

// TypedGraphRepository is a type-safe wrapper for GraphRepository with custom node and edge types.
type TypedGraphRepository[TNode Entity, TEdge Entity] interface {
	CreateNode(ctx context.Context, node TNode, content string) error
	ReadNode(ctx context.Context, nodeID string) (TNode, error)
	UpdateNode(ctx context.Context, node TNode, content string) error
	DeleteNode(ctx context.Context, nodeID string) error
	ListNodes(ctx context.Context, filter map[string]any) ([]TNode, error)

	CreateEdge(ctx context.Context, edge TEdge) error
	ReadEdge(ctx context.Context, edgeID string) (TEdge, error)
	UpdateEdge(ctx context.Context, edge TEdge) error
	DeleteEdge(ctx context.Context, edgeID string) error
	ListEdges(ctx context.Context, filter map[string]any) ([]TEdge, error)
}
