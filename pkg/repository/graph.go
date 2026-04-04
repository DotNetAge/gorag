package repository

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// SyncMode controls which stores are synchronized during graph operations.
type SyncMode int

const (
	// SyncGraphOnly only writes to GraphStore (default for backward compatibility)
	SyncGraphOnly SyncMode = iota
	// SyncGraphWithVector writes to GraphStore and generates vector embedding for nodes
	SyncGraphWithVector
	// SyncFull writes to GraphStore, VectorStore, and DocStore (for complete traceability)
	SyncFull
)

// graphRepository implements GraphRepository with automatic index synchronization.
type graphRepository struct {
	graphStore core.GraphStore
	vecStore   core.VectorStore
	docStore   core.DocStore
	embedder   embedding.Provider
	logger     logging.Logger
	syncMode   SyncMode
}

// NewGraphRepository creates a GraphRepository with index synchronization.
func NewGraphRepository(
	graphStore core.GraphStore,
	vecStore core.VectorStore,
	docStore core.DocStore,
	embedder embedding.Provider,
	logger logging.Logger,
) core.GraphRepository {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}

	return &graphRepository{
		graphStore: graphStore,
		vecStore:   vecStore,
		docStore:   docStore,
		embedder:   embedder,
		logger:     logger,
		syncMode:   SyncGraphOnly, // Default for backward compatibility
	}
}

// WithSyncMode sets the synchronization mode for graph operations.
func (r *graphRepository) WithSyncMode(mode SyncMode) *graphRepository {
	r.syncMode = mode
	return r
}

// Node operations

// CreateNode stores a node and generates vector if the node implements Vectorizable.
// The node is automatically vectorized based on its content.
//
// Example:
//
//	// Standard Node with properties
//	node := &core.Node{
//	    ID:   "person1",
//	    Type: "PERSON",
//	    Properties: map[string]any{
//	        "content": "Alice is a software engineer",  // Will be vectorized
//	    },
//	}
//	repo.CreateNode(ctx, node)
func (r *graphRepository) CreateNode(ctx context.Context, node *core.Node) error {
	// 1. Store in GraphStore
	if err := r.graphStore.UpsertNodes(ctx, []*core.Node{node}); err != nil {
		return fmt.Errorf("failed to store node: %w", err)
	}

	// 2. Optional: Generate vector embedding if syncMode requires it
	if r.syncMode >= SyncGraphWithVector && r.vecStore != nil && r.embedder != nil {
		// Extract content for vectorization
		content := extractNodeContent(node)
		if content != "" {
			embeddings, err := r.embedder.Embed(ctx, []string{content})
			if err != nil {
				r.logger.Warn("failed to embed node content", map[string]interface{}{
					"node_id": node.ID,
					"error":   err.Error(),
				})
			} else {
				vector := &core.Vector{
					ID:       node.ID,
					Values:   embeddings[0],
					Metadata: node.Properties,
				}
				if err := r.vecStore.Upsert(ctx, []*core.Vector{vector}); err != nil {
					r.logger.Warn("failed to store node vector", map[string]interface{}{
						"node_id": node.ID,
						"error":   err.Error(),
					})
				}
			}
		}
	}

	// 3. Optional: Store in DocStore for traceability
	if r.syncMode == SyncFull && r.docStore != nil {
		docID := extractDocumentID(node)
		if docID != "" {
			content := extractNodeContent(node)
			doc := core.NewDocument(node.ID, content, "node", "application/graph-node", node.Properties)
			doc.ID = docID + "-node-" + node.ID
			if err := r.docStore.SetDocument(ctx, doc); err != nil {
				r.logger.Warn("failed to store node document", map[string]interface{}{
					"node_id": node.ID,
					"error":   err.Error(),
				})
			}
		}
	}

	return nil
}

// ReadNode retrieves a node by ID.
func (r *graphRepository) ReadNode(ctx context.Context, nodeID string) (*core.Node, error) {
	return r.graphStore.GetNode(ctx, nodeID)
}

// UpdateNode modifies a node and syncs related indexes.
func (r *graphRepository) UpdateNode(ctx context.Context, node *core.Node) error {
	// Update is same as Create (Upsert semantics)
	return r.CreateNode(ctx, node)
}

// DeleteNode removes a node, its edges, and vector embeddings.
func (r *graphRepository) DeleteNode(ctx context.Context, nodeID string) error {
	// 1. Delete from GraphStore (should cascade delete edges)
	if err := r.graphStore.DeleteNode(ctx, nodeID); err != nil {
		return err
	}

	// 2. Delete vector
	if r.vecStore != nil {
		if err := r.vecStore.Delete(ctx, nodeID); err != nil {
			r.logger.Warn("failed to delete node vector", map[string]interface{}{
				"node_id": nodeID,
				"error":   err.Error(),
			})
		}
	}

	return nil
}

// ListNodes retrieves nodes matching the filter.
func (r *graphRepository) ListNodes(ctx context.Context, filter map[string]any) ([]*core.Node, error) {
	// GraphStore doesn't have a generic ListNodes method yet
	return nil, fmt.Errorf("ListNodes requires GraphStore.ListNodes implementation")
}

// Edge operations

// CreateEdge stores an edge (relationship) between nodes.
func (r *graphRepository) CreateEdge(ctx context.Context, edge *core.Edge) error {
	return r.graphStore.UpsertEdges(ctx, []*core.Edge{edge})
}

// ReadEdge retrieves an edge by ID.
func (r *graphRepository) ReadEdge(ctx context.Context, edgeID string) (*core.Edge, error) {
	// GraphStore doesn't have GetEdge method
	return nil, fmt.Errorf("ReadEdge requires GraphStore.GetEdge implementation")
}

// UpdateEdge modifies an edge.
func (r *graphRepository) UpdateEdge(ctx context.Context, edge *core.Edge) error {
	return r.CreateEdge(ctx, edge)
}

// DeleteEdge removes an edge.
func (r *graphRepository) DeleteEdge(ctx context.Context, edgeID string) error {
	return r.graphStore.DeleteEdge(ctx, edgeID)
}

// ListEdges retrieves edges matching the filter.
func (r *graphRepository) ListEdges(ctx context.Context, filter map[string]any) ([]*core.Edge, error) {
	return nil, fmt.Errorf("ListEdges requires GraphStore.ListEdges implementation")
}

// Graph traversal

// GetNeighbors retrieves neighboring nodes and edges starting from a node.
func (r *graphRepository) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	return r.graphStore.GetNeighbors(ctx, nodeID, depth, limit)
}

// Query executes a graph query (Cypher/GQL).
func (r *graphRepository) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	return r.graphStore.Query(ctx, query, params)
}

// ============================================================================
// Helper functions
// ============================================================================

func extractNodeContent(node *core.Node) string {
	if content, ok := node.Properties["content"].(string); ok {
		return content
	}
	if desc, ok := node.Properties["description"].(string); ok {
		return desc
	}
	// Fallback: concatenate string properties
	return ""
}

func extractDocumentID(node *core.Node) string {
	if docID, ok := node.Properties["document_id"].(string); ok {
		return docID
	}
	if len(node.SourceDocIDs) > 0 {
		return node.SourceDocIDs[0]
	}
	return ""
}

// ============================================================================
// Typed Graph Repository Wrapper
// ============================================================================

// typedGraphRepository is a type-safe wrapper for GraphRepository with custom node and edge types.
type typedGraphRepository[TNode core.Entity, TEdge core.Entity] struct {
	repo core.GraphRepository
}

// NewTypedGraphRepository creates a type-safe graph repository for custom types.
func NewTypedGraphRepository[TNode core.Entity, TEdge core.Entity](repo core.GraphRepository) core.TypedGraphRepository[TNode, TEdge] {
	return &typedGraphRepository[TNode, TEdge]{repo: repo}
}

func (r *typedGraphRepository[TNode, TEdge]) CreateNode(ctx context.Context, node TNode, content string) error {
	// Convert custom node to core.Node if needed
	if n, ok := any(node).(*core.Node); ok {
		// Set content in properties if provided
		if content != "" && n.Properties == nil {
			n.Properties = make(map[string]any)
		}
		if content != "" {
			n.Properties["content"] = content
		}
		return r.repo.CreateNode(ctx, n)
	}
	// For custom types, they need to implement conversion logic
	return fmt.Errorf("custom node types must be convertible to *core.Node")
}

func (r *typedGraphRepository[TNode, TEdge]) ReadNode(ctx context.Context, nodeID string) (TNode, error) {
	node, err := r.repo.ReadNode(ctx, nodeID)
	if err != nil {
		var zero TNode
		return zero, err
	}
	
	// Convert core.Node to custom type
	var result TNode
	if any(result) == any(node) {
		return any(node).(TNode), nil
	}
	
	var zero TNode
	return zero, fmt.Errorf("type conversion not implemented for custom node types")
}

func (r *typedGraphRepository[TNode, TEdge]) UpdateNode(ctx context.Context, node TNode, content string) error {
	if n, ok := any(node).(*core.Node); ok {
		// Set content in properties if provided
		if content != "" && n.Properties == nil {
			n.Properties = make(map[string]any)
		}
		if content != "" {
			n.Properties["content"] = content
		}
		return r.repo.UpdateNode(ctx, n)
	}
	return fmt.Errorf("custom node types must be convertible to *core.Node")
}

func (r *typedGraphRepository[TNode, TEdge]) DeleteNode(ctx context.Context, nodeID string) error {
	return r.repo.DeleteNode(ctx, nodeID)
}

func (r *typedGraphRepository[TNode, TEdge]) ListNodes(ctx context.Context, filter map[string]any) ([]TNode, error) {
	nodes, err := r.repo.ListNodes(ctx, filter)
	if err != nil {
		return nil, err
	}
	
	result := make([]TNode, len(nodes))
	for i, n := range nodes {
		result[i] = any(n).(TNode)
	}
	return result, nil
}

func (r *typedGraphRepository[TNode, TEdge]) CreateEdge(ctx context.Context, edge TEdge) error {
	if e, ok := any(edge).(*core.Edge); ok {
		return r.repo.CreateEdge(ctx, e)
	}
	return fmt.Errorf("custom edge types must be convertible to *core.Edge")
}

func (r *typedGraphRepository[TNode, TEdge]) ReadEdge(ctx context.Context, edgeID string) (TEdge, error) {
	edge, err := r.repo.ReadEdge(ctx, edgeID)
	if err != nil {
		var zero TEdge
		return zero, err
	}
	return any(edge).(TEdge), nil
}

func (r *typedGraphRepository[TNode, TEdge]) UpdateEdge(ctx context.Context, edge TEdge) error {
	if e, ok := any(edge).(*core.Edge); ok {
		return r.repo.UpdateEdge(ctx, e)
	}
	return fmt.Errorf("custom edge types must be convertible to *core.Edge")
}

func (r *typedGraphRepository[TNode, TEdge]) DeleteEdge(ctx context.Context, edgeID string) error {
	return r.repo.DeleteEdge(ctx, edgeID)
}

func (r *typedGraphRepository[TNode, TEdge]) ListEdges(ctx context.Context, filter map[string]any) ([]TEdge, error) {
	edges, err := r.repo.ListEdges(ctx, filter)
	if err != nil {
		return nil, err
	}
	
	result := make([]TEdge, len(edges))
	for i, e := range edges {
		result[i] = any(e).(TEdge)
	}
	return result, nil
}
