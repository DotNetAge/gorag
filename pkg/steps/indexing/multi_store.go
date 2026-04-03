package stepinx

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"golang.org/x/sync/errgroup"
)

// multiStore routes and persists data to multiple storage backends in parallel.
type multiStore struct {
	vectorStore core.VectorStore
	docStore    core.DocStore
	graphStore  core.GraphStore
	logger      logging.Logger
	metrics     core.Metrics
}

// MultiStore creates a step to store chunks, vectors, and entities to multiple backends.
func MultiStore(
	vectorStore core.VectorStore,
	docStore core.DocStore,
	graphStore core.GraphStore,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.IndexingContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &multiStore{
		vectorStore: vectorStore,
		docStore:    docStore,
		graphStore:  graphStore,
		logger:      logger,
		metrics:     metrics,
	}
}

func (s *multiStore) Name() string {
	return "MultiStore"
}

func (s *multiStore) Execute(ctx context.Context, state *core.IndexingContext) error {
	s.logger.Info("Starting MultiStore parallel persistence step", map[string]interface{}{
		"vectors_count": len(state.Vectors),
		"chunks_count":  len(state.ProcessedChunks),
		"nodes_count":   len(state.Nodes),
		"file_path":     state.FilePath,
	})

	g, gCtx := errgroup.WithContext(ctx)

	// 1. VectorStore Persistence
	if s.vectorStore != nil && len(state.Vectors) > 0 {
		g.Go(func() error {
			if err := s.vectorStore.Upsert(gCtx, state.Vectors); err != nil {
				s.logger.Error("Failed to upsert vectors to VectorStore", err, nil)
				return fmt.Errorf("vector store upsert failed: %w", err)
			}
			if s.metrics != nil {
				s.metrics.RecordVectorStoreOperations("upsert_vectors", len(state.Vectors))
			}
			return nil
		})
	}

	// 2. DocStore Persistence (Full Document & Chunks)
	if s.docStore != nil {
		g.Go(func() error {
			// A. Store the main document representation
			doc := core.NewDocument(state.FilePath, "", state.FilePath, "multimodal", map[string]any{
				"source":   state.Metadata.Source,
				"filename": state.Metadata.FileName,
				"size":     state.Metadata.Size,
			})

			if err := s.docStore.SetDocument(gCtx, doc); err != nil {
				s.logger.Error("Failed to store main document in DocStore", err, nil)
				// Don't return error to allow chunk storage to proceed
			}

			// B. Store processed chunks for full-text recovery and hierarchical retrieval
			if len(state.ProcessedChunks) > 0 {
				if err := s.docStore.SetChunks(gCtx, state.ProcessedChunks); err != nil {
					s.logger.Error("Failed to store processed chunks in DocStore", err, nil)
					return fmt.Errorf("doc store chunks upsert failed: %w", err)
				}
				s.logger.Debug("Stored chunks in DocStore", map[string]any{"count": len(state.ProcessedChunks)})
			}
			return nil
		})
	}

	// 3. GraphStore Persistence
	if s.graphStore != nil && (len(state.Nodes) > 0 || len(state.Edges) > 0) {
		g.Go(func() error {
			if len(state.Nodes) > 0 {
				if err := s.graphStore.UpsertNodes(gCtx, state.Nodes); err != nil {
					s.logger.Error("Failed to upsert nodes to GraphStore", err, nil)
				}
			}
			if len(state.Edges) > 0 {
				if err := s.graphStore.UpsertEdges(gCtx, state.Edges); err != nil {
					s.logger.Error("Failed to upsert edges to GraphStore", err, nil)
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	s.logger.Info("MultiStore parallel persistence completed successfully", nil)
	return nil
}
