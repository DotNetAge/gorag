package pattern

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexer"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/repository"
	graphretriever "github.com/DotNetAge/gorag/pkg/retriever/graph"
)

// ============================================================================
// GraphRAG - Knowledge Graph RAG
// ============================================================================

type graphRAG struct {
	basePattern
	graphRepo core.GraphRepository
	logger    logging.Logger
}

// GraphRAG creates a Knowledge-Graph enabled RAG pattern.
// Zero configuration: just provide a name and embedding model.
//
// Example:
//
//	rag, err := pattern.GraphRAG("myapp", pattern.WithBGE("bge-small-zh-v1.5"))
//
// Automatically configured:
//   - BoltDB document store: ~/.gorag/{name}/docs.bolt
//   - GoVector vector store: ~/.gorag/{name}/vectors.db
//   - In-memory knowledge graph (GoGraph)
//   - Default embedder: bge-small-zh-v1.5 (if not specified)
//
// For Neo4j integration, use WithNeoGraph option.
func GraphRAG(name string, opts ...GraphOption) (GraphRAGPattern, error) {
	o := &graphOptions{topK: 5}
	for _, opt := range opts {
		opt.applyGraph(o)
	}

	// Ensure name is set first
	if name != "" {
		o.indexerOpts = append([]indexer.IndexerOption{indexer.WithName(name)}, o.indexerOpts...)
	}

	idx, err := indexer.DefaultGraphIndexer(o.indexerOpts...)
	if err != nil {
		return nil, fmt.Errorf("GraphRAG: failed to create indexer: %w", err)
	}

	// Auto-configure embedder if not set
	if idx.Embedder() == nil {
		provider, err := embedding.WithBEG("bge-small-zh-v1.5", "")
		if err != nil {
			return nil, fmt.Errorf("GraphRAG: failed to create default embedder: %w (tip: use WithBGE or WithBERT)", err)
		}
		o.indexerOpts = append(o.indexerOpts, indexer.WithEmbedding(provider))
		idx, err = indexer.DefaultGraphIndexer(o.indexerOpts...)
		if err != nil {
			return nil, fmt.Errorf("GraphRAG: failed to create indexer: %w", err)
		}
	}

	// Use graph retriever with extraction strategy
	retrieverOpts := []graphretriever.Option{
		graphretriever.WithTopK(o.topK),
		graphretriever.WithDocStore(idx.DocStore()),
	}
	if o.extractionStrategy != "" {
		retrieverOpts = append(retrieverOpts, graphretriever.WithExtractionStrategy(o.extractionStrategy))
	}
	ret := graphretriever.NewRetriever(
		idx.VectorStore(),
		idx.GraphStore(),
		idx.Embedder(),
		o.llm,
		retrieverOpts...,
	)

	// Create repositories with index synchronization
	repo := repository.NewRepository(
		idx.DocStore(),
		idx.VectorStore(),
		idx.Embedder(),
		idx.Chunker(),
	)

	graphRepo := repository.NewGraphRepository(
		idx.GraphStore(),
		idx.VectorStore(),
		idx.DocStore(),
		idx.Embedder(),
		logging.DefaultNoopLogger(),
	)

	return &graphRAG{
		basePattern: basePattern{
			idx:  idx,
			ret:  ret,
			repo: repo,
		},
		graphRepo: graphRepo,
		logger:    logging.DefaultNoopLogger(),
	}, nil
}

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

// SyncNode wraps a Node with synchronization options.
type SyncNode struct {
	*core.Node
	// Content is the text content to embed for semantic search.
	// If empty, node.Properties["content"] or node.Properties["description"] is used.
	Content string
	// SyncMode controls which stores to sync. Default is SyncGraphOnly.
	SyncMode SyncMode
	// DocumentID links this node to a source document for cascade delete.
	// If empty, node.Properties["document_id"] is used.
	DocumentID string
}

// SyncEdge wraps an Edge with synchronization options.
type SyncEdge struct {
	*core.Edge
	// DocumentID links this edge to a source document for cascade delete.
	// If empty, edge.Properties["document_id"] is used.
	DocumentID string
}

func (g *graphRAG) GraphRepository() core.GraphRepository {
	return g.graphRepo
}

// Graph operations for graphRAG

// AddNode adds a single node to the knowledge graph.
// For backward compatibility, this only writes to GraphStore.
// Use AddSyncNode for full synchronization options.
func (g *graphRAG) AddNode(ctx context.Context, node *core.Node) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}
	return gs.UpsertNodes(ctx, []*core.Node{node})
}

// AddSyncNode adds a node with full synchronization options.
// It can optionally generate vector embeddings and link to a source document.
func (g *graphRAG) AddSyncNode(ctx context.Context, syncNode *SyncNode) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}

	// 1. Ensure properties map exists
	if syncNode.Properties == nil {
		syncNode.Properties = make(map[string]any)
	}

	// 2. Set document_id for cascade delete support
	docID := syncNode.DocumentID
	if docID == "" {
		if v, ok := syncNode.Properties["document_id"].(string); ok {
			docID = v
		}
	}
	if docID != "" {
		syncNode.Properties["document_id"] = docID
	}

	// 3. Write to GraphStore
	if err := gs.UpsertNodes(ctx, []*core.Node{syncNode.Node}); err != nil {
		return fmt.Errorf("failed to upsert node: %w", err)
	}

	// 4. Optional: Generate vector embedding for semantic search
	if syncNode.SyncMode >= SyncGraphWithVector && g.idx.Embedder() != nil && g.idx.VectorStore() != nil {
		content := syncNode.Content
		if content == "" {
			if v, ok := syncNode.Properties["content"].(string); ok {
				content = v
			} else if v, ok := syncNode.Properties["description"].(string); ok {
				content = v
			}
		}

		if content != "" {
			embeddings, err := g.idx.Embedder().Embed(ctx, []string{content})
			if err != nil {
				g.logger.Warn("failed to embed node content", map[string]interface{}{
					"node_id": syncNode.ID,
					"error":   err.Error(),
				})
				// Continue without vector - node is still in graph
			} else {
				vector := &core.Vector{
					ID:       syncNode.ID,
					Values:   embeddings[0],
					Metadata: syncNode.Properties,
				}
				if err := g.idx.VectorStore().Upsert(ctx, []*core.Vector{vector}); err != nil {
					g.logger.Warn("failed to store node vector", map[string]interface{}{
						"node_id": syncNode.ID,
						"error":   err.Error(),
					})
				}
			}
		}
	}

	// 5. Optional: Store in DocStore for traceability
	if syncNode.SyncMode == SyncFull && g.idx.DocStore() != nil && docID != "" {
		doc := core.NewDocument(
			syncNode.ID,
			syncNode.Content,
			"node",
			"application/graph-node",
			syncNode.Properties,
		)
		doc.ID = docID + "-node-" + syncNode.ID
		if err := g.idx.DocStore().SetDocument(ctx, doc); err != nil {
			g.logger.Warn("failed to store node document", map[string]interface{}{
				"node_id": syncNode.ID,
				"error":   err.Error(),
			})
		}
	}

	return nil
}

func (g *graphRAG) AddNodes(ctx context.Context, nodes []*core.Node) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}
	return gs.UpsertNodes(ctx, nodes)
}

// AddSyncNodes adds multiple nodes with synchronization options.
func (g *graphRAG) AddSyncNodes(ctx context.Context, syncNodes []*SyncNode) error {
	for _, sn := range syncNodes {
		if err := g.AddSyncNode(ctx, sn); err != nil {
			return err
		}
	}
	return nil
}

func (g *graphRAG) GetNode(ctx context.Context, nodeID string) (*core.Node, error) {
	gs := g.idx.GraphStore()
	if gs == nil {
		return nil, fmt.Errorf("GraphRAG: graph store not configured")
	}
	return gs.GetNode(ctx, nodeID)
}

// DeleteNode removes a node from the graph and optionally its vector.
func (g *graphRAG) DeleteNode(ctx context.Context, nodeID string) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}

	// 1. Delete from GraphStore
	if err := gs.DeleteNode(ctx, nodeID); err != nil {
		return err
	}

	// 2. Cascade delete from VectorStore (if exists)
	if g.idx.VectorStore() != nil {
		if err := g.idx.VectorStore().Delete(ctx, nodeID); err != nil {
			// Log but don't fail - node might not have a vector
			g.logger.Warn("failed to delete node vector", map[string]interface{}{
				"node_id": nodeID,
				"error":   err.Error(),
			})
		}
	}

	return nil
}

func (g *graphRAG) AddEdge(ctx context.Context, edge *core.Edge) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}
	return gs.UpsertEdges(ctx, []*core.Edge{edge})
}

// AddSyncEdge adds an edge with synchronization options.
func (g *graphRAG) AddSyncEdge(ctx context.Context, syncEdge *SyncEdge) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}

	// Ensure properties map exists
	if syncEdge.Properties == nil {
		syncEdge.Properties = make(map[string]any)
	}

	// Set document_id for cascade delete support
	docID := syncEdge.DocumentID
	if docID == "" {
		if v, ok := syncEdge.Properties["document_id"].(string); ok {
			docID = v
		}
	}
	if docID != "" {
		syncEdge.Properties["document_id"] = docID
	}

	return gs.UpsertEdges(ctx, []*core.Edge{syncEdge.Edge})
}

func (g *graphRAG) AddEdges(ctx context.Context, edges []*core.Edge) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}
	return gs.UpsertEdges(ctx, edges)
}

// AddSyncEdges adds multiple edges with synchronization options.
func (g *graphRAG) AddSyncEdges(ctx context.Context, syncEdges []*SyncEdge) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}

	edges := make([]*core.Edge, len(syncEdges))
	for i, se := range syncEdges {
		if se.Properties == nil {
			se.Properties = make(map[string]any)
		}
		docID := se.DocumentID
		if docID == "" {
			if v, ok := se.Properties["document_id"].(string); ok {
				docID = v
			}
		}
		if docID != "" {
			se.Properties["document_id"] = docID
		}
		edges[i] = se.Edge
	}

	return gs.UpsertEdges(ctx, edges)
}

func (g *graphRAG) DeleteEdge(ctx context.Context, edgeID string) error {
	gs := g.idx.GraphStore()
	if gs == nil {
		return fmt.Errorf("GraphRAG: graph store not configured")
	}
	return gs.DeleteEdge(ctx, edgeID)
}

func (g *graphRAG) QueryGraph(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	gs := g.idx.GraphStore()
	if gs == nil {
		return nil, fmt.Errorf("GraphRAG: graph store not configured")
	}
	return gs.Query(ctx, query, params)
}

func (g *graphRAG) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	gs := g.idx.GraphStore()
	if gs == nil {
		return nil, nil, fmt.Errorf("GraphRAG: graph store not configured")
	}
	return gs.GetNeighbors(ctx, nodeID, depth, limit)
}
