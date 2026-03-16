package retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*ImageSearchStep)(nil)

// ImageSearchStep performs a vector search using the image embedding produced by
// MultimodalEmbeddingStep. It reads state.Agentic.Custom["image_vector"] and
// queries the VectorStore with that vector.
//
// The step is a no-op when the image vector is absent, making image retrieval
// a fully optional third branch in the multimodal pipeline.
type ImageSearchStep struct {
	store  abstraction.VectorStore
	topK   int
	logger logging.Logger
}

// NewImageSearchStep creates a new image-based vector search step.
func NewImageSearchStep(store abstraction.VectorStore, topK int, logger logging.Logger) *ImageSearchStep {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &ImageSearchStep{store: store, topK: topK, logger: logger}
}

func (s *ImageSearchStep) Name() string { return "ImageSearchStep" }

// Execute retrieves chunks using the image query vector. It is a no-op when
// state.Agentic.Custom["image_vector"] is not set.
func (s *ImageSearchStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Agentic == nil {
		return nil // no-op: MultimodalEmbeddingStep did not run or found no image
	}

	imgVec, ok := state.Agentic.Custom["image_vector"].([]float32)
	if !ok || len(imgVec) == 0 {
		s.logger.Debug("ImageSearchStep: no image_vector found, skipping", map[string]interface{}{
			"step": "ImageSearchStep",
		})
		return nil
	}

	vectors, _, err := s.store.Search(ctx, imgVec, s.topK, state.Filters)
	if err != nil {
		return fmt.Errorf("ImageSearchStep: store.Search failed: %w", err)
	}

	var chunks []*entity.Chunk
	for _, v := range vectors {
		content, _ := v.Metadata["content"].(string)
		chunk := entity.NewChunk(v.ChunkID, "", content, 0, 0, v.Metadata)
		chunks = append(chunks, chunk)
	}

	state.RetrievedChunks = append(state.RetrievedChunks, chunks)

	s.logger.Info("image search completed", map[string]interface{}{
		"step":   "ImageSearchStep",
		"chunks": len(chunks),
	})

	return nil
}
