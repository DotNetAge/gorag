package stepinx

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/base"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type triples struct {
	extractor *base.TriplesExtractor
	store     core.GraphStore
	logger    logging.Logger
}

// ExtractTriples creates a new step for automated knowledge graph construction.
// It extracts triples (Subject-Predicate-Object) from chunks and upserts them into the GraphStore.
func ExtractTriples(extractor *base.TriplesExtractor, graphStore core.GraphStore, logger logging.Logger) pipeline.Step[*core.IndexingContext] {
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

	if state.Chunks == nil {
		return nil
	}

	// Buffer chunks to allow extraction while maintaining the stream
	var allChunks []*core.Chunk
	for chunk := range state.Chunks {
		allChunks = append(allChunks, chunk)
	}

	// Restore channel for downstream steps
	outChan := make(chan *core.Chunk, len(allChunks))
	for _, c := range allChunks {
		outChan <- c
	}
	close(outChan)
	state.Chunks = outChan

	s.logger.Info("Starting automated triples extraction for graph construction", map[string]any{
		"file":   state.FilePath,
		"chunks": len(allChunks),
	})

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
			// 1. Upsert Subject Node
			subNode := &core.Node{
				ID:   t.Subject,
				Type: t.SubjectType,
				Properties: map[string]any{
					"source_chunk": chunk.ID,
					"source_file":  state.FilePath,
				},
			}
			if err := s.store.UpsertNodes(ctx, []*core.Node{subNode}); err != nil {
				s.logger.Error("Failed to upsert subject node", err)
				continue
			}

			// 2. Upsert Object Node
			objNode := &core.Node{
				ID:   t.Object,
				Type: t.ObjectType,
				Properties: map[string]any{
					"source_chunk": chunk.ID,
					"source_file":  state.FilePath,
				},
			}
			if err := s.store.UpsertNodes(ctx, []*core.Node{objNode}); err != nil {
				s.logger.Error("Failed to upsert object node", err)
				continue
			}

			// 3. Upsert Edge
			edge := &core.Edge{
				ID:     fmt.Sprintf("%s-%s-%s", t.Subject, t.Predicate, t.Object),
				Type:   t.Predicate,
				Source: t.Subject,
				Target: t.Object,
				Properties: map[string]any{
					"source_chunk": chunk.ID,
				},
			}
			if err := s.store.UpsertEdges(ctx, []*core.Edge{edge}); err != nil {
				s.logger.Error("Failed to upsert edge", err)
			}
		}
	}

	s.logger.Info("Graph construction completed for file", map[string]any{"file": state.FilePath})
	return nil
}
