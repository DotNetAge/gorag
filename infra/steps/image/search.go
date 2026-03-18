// Package image provides image retrieval steps for multimodal RAG pipelines.
package image

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// search performs vector search using image embedding from multimodal pipeline.
type search struct {
	store   abstraction.VectorStore
	topK    int
	logger  logging.Logger
	metrics abstraction.Metrics
}

// Search creates an image-based vector search step with logger and metrics.
//
// Parameters:
//   - store: vector store to search
//   - topK: number of results to retrieve (default: 10)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(image.Search(store, 20, logger, metrics))
func Search(
	store abstraction.VectorStore,
	topK int,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &search{
		store:   store,
		topK:    topK,
		logger:  logger,
		metrics: metrics,
	}
}

// Name returns the step name
func (s *search) Name() string {
	return "ImageSearch"
}

// Execute retrieves chunks using the image query vector.
// It is a no-op when state.Agentic.Custom["image_vector"] is not set.
func (s *search) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Agentic == nil {
		s.logger.Debug("ImageSearch: no AgenticMetadata found, skipping", map[string]interface{}{
			"step": "ImageSearch",
		})
		return nil // no-op: MultimodalEmbeddingStep did not run
	}

	// Get image vector from AgenticMetadata
	imgVec, ok := state.Agentic.Custom["image_vector"].([]float32)
	if !ok || len(imgVec) == 0 {
		s.logger.Debug("ImageSearch: no image_vector found, skipping", map[string]interface{}{
			"step": "ImageSearch",
		})
		return nil // no-op: No image embedding available
	}

	// Perform vector search using image embedding
	vectors, _, err := s.store.Search(ctx, imgVec, s.topK, state.Filters)
	if err != nil {
		s.logger.Error("failed to search vector store", err, map[string]interface{}{
			"step": "ImageSearch",
			"topK": s.topK,
		})
		return fmt.Errorf("ImageSearch: store.Search failed: %w", err)
	}

	// Convert vectors to chunks
	var chunks []*entity.Chunk
	for _, v := range vectors {
		content, _ := v.Metadata["content"].(string)
		chunk := entity.NewChunk(v.ChunkID, "", content, 0, 0, v.Metadata)
		chunks = append(chunks, chunk)
	}

	// Store retrieved chunks in state
	if state.RetrievedChunks == nil {
		state.RetrievedChunks = make([][]*entity.Chunk, 0)
	}
	state.RetrievedChunks = append(state.RetrievedChunks, chunks)

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("image", len(chunks))
	}

	s.logger.Info("ImageSearch completed", map[string]interface{}{
		"step":   "ImageSearch",
		"chunks": len(chunks),
	})

	return nil
}
