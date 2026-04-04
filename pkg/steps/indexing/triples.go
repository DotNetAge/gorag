package stepinx

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type triples struct {
	extractor core.TriplesExtractor
	store     core.GraphStore
	logger    logging.Logger
}

// ExtractTriples creates a new step for automated knowledge graph construction.
// It extracts triples (Subject-Predicate-Object) from chunks and upserts them into the GraphStore.
// Following Microsoft GraphRAG design: nodes and edges are bound to source chunks.
func ExtractTriples(extractor core.TriplesExtractor, graphStore core.GraphStore, logger logging.Logger) pipeline.Step[*core.IndexingContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &triples{
		extractor: extractor,
		store:     graphStore,
		logger:    logger,
	}
}

func (s *triples) Name() string {
	return "ExtractTriples"
}

func (s *triples) Execute(ctx context.Context, state *core.IndexingContext) error {
	if s.extractor == nil || s.store == nil {
		return fmt.Errorf("triples extractor or graph store not configured")
	}

	// 1. Get chunks from state (prefer ProcessedChunks slice if available, fallback to Chunks channel)
	var allChunks []*core.Chunk
	if len(state.ProcessedChunks) > 0 {
		allChunks = state.ProcessedChunks
	} else if state.Chunks != nil {
		// Collect from channel and re-create it for downstream
		for chunk := range state.Chunks {
			if chunk != nil {
				allChunks = append(allChunks, chunk)
			}
		}
		// Restore channel
		chunkChan := make(chan *core.Chunk, len(allChunks))
		for _, chunk := range allChunks {
			chunkChan <- chunk
		}
		close(chunkChan)
		state.Chunks = chunkChan
	}

	if len(allChunks) == 0 {
		return nil // Nothing to extract from
	}

	s.logger.Info("Starting automated triples extraction for graph construction", map[string]any{
		"file":   state.FilePath,
		"chunks": len(allChunks),
	})

	var allTriples []*core.Triple
	for _, chunk := range allChunks {
		extracted, err := s.extractor.Extract(ctx, chunk.Content)
		if err != nil {
			s.logger.Warn("Failed to extract triples from chunk", map[string]any{
				"chunk_id": chunk.ID,
				"error":    err,
			})
			continue
		}

		for _, t := range extracted {
			// Bind source information to triple
			t.SourceChunkID = chunk.ID
			t.SourceDocID = chunk.DocumentID
			allTriples = append(allTriples, &t)

			// 1. Upsert Subject Node with source binding
			subNode := &core.Node{
				ID:   t.Subject,
				Type: t.SubjectType,
				SourceChunkIDs: []string{chunk.ID},
				SourceDocIDs:   []string{chunk.DocumentID},
			}
			if err := s.store.UpsertNodes(ctx, []*core.Node{subNode}); err != nil {
				s.logger.Error("Failed to upsert subject node", err)
				continue
			}

			// 2. Upsert Object Node with source binding
			objNode := &core.Node{
				ID:   t.Object,
				Type: t.ObjectType,
				SourceChunkIDs: []string{chunk.ID},
				SourceDocIDs:   []string{chunk.DocumentID},
			}
			if err := s.store.UpsertNodes(ctx, []*core.Node{objNode}); err != nil {
				s.logger.Error("Failed to upsert object node", err)
				continue
			}

			// 3. Upsert Edge with source binding
			edge := &core.Edge{
				ID:     fmt.Sprintf("%s-%s-%s", t.Subject, t.Predicate, t.Object),
				Type:   t.Predicate,
				Source: t.Subject,
				Target: t.Object,
				SourceChunkIDs: []string{chunk.ID},
				SourceDocIDs:   []string{chunk.DocumentID},
			}
			if err := s.store.UpsertEdges(ctx, []*core.Edge{edge}); err != nil {
				s.logger.Error("Failed to upsert edge", err)
			}
		}
	}

	// Store triples in state for downstream steps
	state.Triples = allTriples

	s.logger.Info("Graph construction completed for file", map[string]any{
		"file":    state.FilePath,
		"triples": len(allTriples),
	})
	return nil
}
