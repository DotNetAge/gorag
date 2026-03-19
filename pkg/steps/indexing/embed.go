package stepinx

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
)

// batch embeds chunks into vectors in batches.
type batch struct {
	embedder embedding.Provider
	metrics  core.Metrics
}

// Batch creates a new batch embedding step with metrics collection.
//
// Parameters:
//   - embedder: embedding provider implementation
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(indexing.Batch(embedder, metrics))
func Batch(embedder embedding.Provider, metrics core.Metrics) pipeline.Step[*core.State] {
	return &batch{
		embedder: embedder,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *batch) Name() string {
	return "Embed"
}

// Execute embeds all chunks from the channel into vectors.
func (s *batch) Execute(ctx context.Context, state *core.State) error {
	if s.embedder == nil {
		return fmt.Errorf("embedder not configured")
	}

	// Get chunks channel from state
	if state.Chunks == nil {
		return fmt.Errorf("no chunks to embed")
	}

	// Collect and embed all chunks
	var vectors []*core.Vector
	totalChunks := 0

	for chunk := range state.Chunks {
		if chunk == nil {
			continue
		}

		// Generate embedding for chunk content
		embeddingResults, err := s.embedder.Embed(ctx, []string{chunk.Content})
		if err != nil {
			return fmt.Errorf("failed to embed chunk %s: %w", chunk.ID, err)
		}

		// Create vector entity
		if len(embeddingResults) > 0 && len(embeddingResults[0]) > 0 {
			vector := core.NewVector(
				fmt.Sprintf("vec_%s", chunk.ID),
				embeddingResults[0],
				chunk.ID,
				chunk.Metadata,
			)

			vectors = append(vectors, vector)
			totalChunks++

			// Update chunk's vector ID reference
			chunk.SetVectorID(vector.ID)
		}
	}

	// Store vectors in state for StoreStep
	state.Vectors = vectors
	state.TotalChunks = totalChunks

	// Record embedding metrics
	if s.metrics != nil && totalChunks > 0 {
		s.metrics.RecordEmbeddingCount(totalChunks)
	}

	return nil
}
