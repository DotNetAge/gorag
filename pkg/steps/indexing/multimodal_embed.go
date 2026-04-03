package stepinx

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
)

// multimodalEmbed generates multi-modal embeddings for chunks.
type multimodalEmbed struct {
	provider embedding.MultimodalProvider
	metrics  core.Metrics
}

// MultimodalEmbed creates a step for multimodal vector generation.
func MultimodalEmbed(provider embedding.MultimodalProvider, metrics core.Metrics) pipeline.Step[*core.IndexingContext] {
	return &multimodalEmbed{
		provider: provider,
		metrics:  metrics,
	}
}

func (s *multimodalEmbed) Name() string {
	return "MultimodalEmbed"
}

func (s *multimodalEmbed) Execute(ctx context.Context, state *core.IndexingContext) error {
	if s.provider == nil {
		return fmt.Errorf("multimodal embedder not configured")
	}

	if state.Chunks == nil {
		return fmt.Errorf("no chunks to embed")
	}

	var vectors []*core.Vector
	var processedChunks []*core.Chunk
	totalChunks := 0

	for chunk := range state.Chunks {
		if chunk == nil {
			continue
		}

		processedChunks = append(processedChunks, chunk)

		// Check modality
		modality, _ := chunk.Metadata["modality"].(string)
		
		var embeddingResults [][]float32
		var err error

		if modality == "image" {
			// Extract image bytes from Metadata
			var imgBytes []byte
			if b, ok := chunk.Metadata["image_bytes"].([]byte); ok {
				imgBytes = b
			} else {
				// Fallback, maybe Content is bytes directly
				imgBytes = []byte(chunk.Content) 
			}

			embeddingResults, err = s.provider.EmbedImages(ctx, [][]byte{imgBytes})
		} else {
			// Default is text
			embeddingResults, err = s.provider.Embed(ctx, []string{chunk.Content})
		}

		if err != nil {
			return fmt.Errorf("failed to embed chunk %s (modality: %s): %w", chunk.ID, modality, err)
		}

		if len(embeddingResults) > 0 && len(embeddingResults[0]) > 0 {
			vector := core.NewVector(
				fmt.Sprintf("vec_%s", chunk.ID),
				embeddingResults[0],
				chunk.ID,
				chunk.Metadata,
			)

			vectors = append(vectors, vector)
			totalChunks++
			chunk.SetVectorID(vector.ID)
		}
	}

	// Store vectors for subsequent MultiStore step
	if state.Vectors == nil {
		state.Vectors = make([]*core.Vector, 0)
	}
	if state.ProcessedChunks == nil {
		state.ProcessedChunks = make([]*core.Chunk, 0)
	}
	state.Vectors = append(state.Vectors, vectors...)
	state.ProcessedChunks = append(state.ProcessedChunks, processedChunks...)
	state.TotalChunks += totalChunks

	if s.metrics != nil && totalChunks > 0 {
		s.metrics.RecordEmbeddingCount(totalChunks)
	}

	return nil
}
