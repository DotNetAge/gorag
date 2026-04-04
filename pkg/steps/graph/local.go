// Package graph provides GraphRAG retrieval steps following Microsoft GraphRAG architecture.
// Steps include: LocalSearch, GlobalSearch, and HybridSearch for different query scenarios.
package graph

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// LocalSearch performs graph traversal from extracted entities.
// Best for: specific questions about entities and their relationships.
// Following Microsoft GraphRAG: retrieves source chunks bound to nodes.
type LocalSearch struct {
	graphStore core.GraphStore
	depth      int
	limit      int
	logger     logging.Logger
}

type LocalSearchOption func(*LocalSearch)

func WithDepth(depth int) LocalSearchOption {
	return func(s *LocalSearch) {
		s.depth = depth
	}
}

func WithLimit(limit int) LocalSearchOption {
	return func(s *LocalSearch) {
		s.limit = limit
	}
}

// NewLocalSearch creates a local search step for entity-centric retrieval.
func NewLocalSearch(graphStore core.GraphStore, opts ...LocalSearchOption) pipeline.Step[*core.RetrievalContext] {
	s := &LocalSearch{
		graphStore: graphStore,
		depth:      2,
		limit:      10,
		logger:     logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *LocalSearch) Name() string {
	return "LocalSearch"
}

func (s *LocalSearch) Execute(ctx context.Context, rctx *core.RetrievalContext) error {
	if s.graphStore == nil {
		s.logger.Warn("GraphStore is nil, skipping local search", nil)
		return nil
	}

	// Get extracted entities from context
	entities, ok := rctx.Custom["extracted_entities"].([]string)
	if !ok || len(entities) == 0 {
		s.logger.Debug("No entities extracted, skipping local search", nil)
		return nil
	}

	s.logger.Info("Starting local graph search", map[string]any{
		"entities": entities,
		"depth":    s.depth,
	})

	var (
		allNodes []*core.Node
		allEdges []*core.Edge
		chunkSet = make(map[string]bool)
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	// Traverse graph from each entity concurrently
	for _, entity := range entities {
		wg.Add(1)
		go func(ent string) {
			defer wg.Done()

			nodes, edges, err := s.graphStore.GetNeighbors(ctx, ent, s.depth, s.limit)
			if err != nil {
				s.logger.Warn("Failed to get neighbors", map[string]any{
					"entity": ent,
					"error":  err.Error(),
				})
				return
			}

			mu.Lock()
			allNodes = append(allNodes, nodes...)
			allEdges = append(allEdges, edges...)

			// Collect source chunks from nodes
			for _, node := range nodes {
				for _, chunkID := range node.SourceChunkIDs {
					chunkSet[chunkID] = true
				}
			}
			mu.Unlock()
		}(entity)
	}

	wg.Wait()

	// Store results in context
	rctx.GraphNodes = allNodes
	rctx.GraphEdges = allEdges

	// Build graph context string for LLM
	rctx.GraphContext = s.buildGraphContext(entities, allNodes, allEdges)

	// Store chunk IDs for later retrieval
	chunkIDs := make([]string, 0, len(chunkSet))
	for id := range chunkSet {
		chunkIDs = append(chunkIDs, id)
	}
	rctx.Custom["graph_chunk_ids"] = chunkIDs

	s.logger.Info("Local search completed", map[string]any{
		"nodes":      len(allNodes),
		"edges":      len(allEdges),
		"chunks":     len(chunkIDs),
	})

	return nil
}

func (s *LocalSearch) buildGraphContext(entities []string, nodes []*core.Node, edges []*core.Edge) string {
	var sb strings.Builder

	sb.WriteString("[Knowledge Graph Context]\n")
	sb.WriteString(fmt.Sprintf("Query Entities: %s\n\n", strings.Join(entities, ", ")))

	if len(nodes) > 0 {
		sb.WriteString("Related Entities:\n")
		for _, node := range nodes {
			sb.WriteString(fmt.Sprintf("- %s (Type: %s)\n", node.ID, node.Type))
		}
	}

	if len(edges) > 0 {
		sb.WriteString("\nRelationships:\n")
		for _, edge := range edges {
			sb.WriteString(fmt.Sprintf("- %s --[%s]--> %s\n", edge.Source, edge.Type, edge.Target))
		}
	}

	return sb.String()
}
