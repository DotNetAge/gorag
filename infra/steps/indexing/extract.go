package indexing

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/indexing"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// entities extracts entities from documents for graph indexing.
type entities struct {
	extractor retrieval.EntityExtractor
	logger    logging.Logger
}

// Entities creates a new entity extraction step.
//
// Parameters:
//   - extractor: entity extractor implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//
// Example:
//
//	p.AddStep(indexing.Entities(extractor, logger))
func Entities(extractor retrieval.EntityExtractor, logger logging.Logger) pipeline.Step[*indexing.State] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &entities{
		extractor: extractor,
		logger:    logger,
	}
}

// Name returns the step name
func (s *entities) Name() string {
	return "ExtractEntities"
}

// Execute extracts entities from all chunks and builds graph nodes/edges.
func (s *entities) Execute(ctx context.Context, state *indexing.State) error {
	if s.extractor == nil {
		return fmt.Errorf("entity extractor not configured")
	}

	// Get chunks from state
	if state.Chunks == nil {
		return fmt.Errorf("no chunks to extract entities from")
	}

	// Collect all chunks first
	var allChunks []*entity.Chunk
	for chunk := range state.Chunks {
		if chunk != nil {
			allChunks = append(allChunks, chunk)
		}
	}

	// Re-create channel for downstream steps
	chunkChan := make(chan *entity.Chunk, len(allChunks))
	for _, chunk := range allChunks {
		chunkChan <- chunk
	}
	close(chunkChan)
	state.Chunks = chunkChan

	// Extract entities from each chunk
	for _, chunk := range allChunks {
		query := entity.NewQuery("", chunk.Content, nil)

		result, err := s.extractor.Extract(ctx, query)
		if err != nil {
			s.logger.Error("failed to extract entities", err, map[string]interface{}{
				"step":      "ExtractEntities",
				"chunk_id":  chunk.ID,
				"file_path": state.FilePath,
			})
			continue // Skip this chunk, continue with others
		}

		// Store entities in chunk metadata for graph construction
		if len(result.Entities) > 0 {
			if chunk.Metadata == nil {
				chunk.Metadata = make(map[string]any)
			}
			chunk.Metadata["entities"] = result.Entities

			s.logger.Debug("entities extracted", map[string]interface{}{
				"step":           "ExtractEntities",
				"chunk_id":       chunk.ID,
				"entities_count": len(result.Entities),
			})
		}
	}

	return nil
}
