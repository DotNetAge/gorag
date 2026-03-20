package stepinx

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// entities extracts entities from documents for graph indexing.
type entities struct {
	extractor core.EntityExtractor
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
func Entities(extractor core.EntityExtractor, logger logging.Logger) pipeline.Step[*core.IndexingContext] {
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

// Execute extracts entities from all documents.
func (s *entities) Execute(ctx context.Context, state *core.IndexingContext) error {
	if s.extractor == nil {
		return fmt.Errorf("entity extractor not configured")
	}

	// Get chunks from state
	if state.Chunks == nil {
		return fmt.Errorf("no chunks to extract entities from")
	}

	// Collect all chunks first
	var allChunks []*core.Chunk
	for chunk := range state.Chunks {
		if chunk != nil {
			allChunks = append(allChunks, chunk)
		}
	}

	// Re-create channel for downstream steps
	chunkChan := make(chan *core.Chunk, len(allChunks))
	for _, chunk := range allChunks {
		chunkChan <- chunk
	}
	close(chunkChan)
	state.Chunks = chunkChan

	// Extract entities from each chunk
	for _, chunk := range allChunks {
		query := core.NewQuery("", chunk.Content, nil)

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
