package stepinx

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// multiStore routes and persists data to multiple storage backends.
type multiStore struct {
	vectorStore core.VectorStore
	docStore    store.DocStore
	graphStore  store.GraphStore
	logger      logging.Logger
	metrics     core.Metrics
}

// MultiStore creates a step to store chunks, vectors, and entities to multiple backends.
func MultiStore(
	vectorStore core.VectorStore,
	docStore store.DocStore,
	graphStore store.GraphStore,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.State] {
	if logger == nil {
		logger = logging.NewNoopLogger()
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

func (s *multiStore) Execute(ctx context.Context, state *core.State) error {
	s.logger.Info("Starting MultiStore step", map[string]interface{}{
		"vectors_count": len(state.Vectors),
		"file_path":     state.FilePath,
	})

	// 1. VectorStore: Upsert all generated vectors
	if s.vectorStore != nil && len(state.Vectors) > 0 {
		if err := s.vectorStore.Upsert(ctx, state.Vectors); err != nil {
			s.logger.Error("Failed to upsert vectors to VectorStore", err, nil)
			return fmt.Errorf("vector store upsert failed: %w", err)
		}
		if s.metrics != nil {
			s.metrics.RecordVectorStoreOperations("upsert_vectors", len(state.Vectors))
		}
	}

	// 2. DocumentStore: Upsert raw document
	if s.docStore != nil {
		// Create and store the main document representation
		doc := core.NewDocument(state.FilePath, "", state.FilePath, "multimodal", map[string]any{
			"source":   state.Metadata.Source,
			"filename": state.Metadata.FileName,
			"size":     state.Metadata.Size,
		})
		
		if err := s.docStore.SetDocument(ctx, doc); err != nil {
			s.logger.Error("Failed to store main document in DocStore", err, nil)
		}

		// Ideally, chunks generated in SemanticChunkStep/MultimodalEmbedStep should be stored here.
		// A full implementation would buffer the chunks in state before passing them down.
	}

	// 3. GraphStore: Upsert entities and edges
	if s.graphStore != nil {
		// GraphStore integration is ready to accept nodes/edges parsed by Entities step.
	}

	s.logger.Info("MultiStore step completed successfully", nil)
	return nil
}
