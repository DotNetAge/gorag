package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*MultimodalEmbeddingStep)(nil)

// MultimodalEmbeddingStep encodes the query text (and optionally an image) using a
// MultimodalEmbedder and writes the resulting vectors into the pipeline state so
// that subsequent VectorSearchStep and ImageSearchStep can use them independently.
//
// Vectors are stored in AgenticMetadata.Custom:
//   - "query_vector"  []float32  – text embedding of state.Query.Text
//   - "image_vector"  []float32  – image embedding (only when image_data is present)
//
// To pass an image for cross-modal search, callers must set:
//
//	state.Agentic.Custom["image_data"] = []byte{...}
//
// before executing the pipeline.
type MultimodalEmbeddingStep struct {
	embedder abstraction.MultimodalEmbedder
	logger   logging.Logger
}

// NewMultimodalEmbeddingStep creates a new multimodal embedding step.
func NewMultimodalEmbeddingStep(embedder abstraction.MultimodalEmbedder, logger logging.Logger) *MultimodalEmbeddingStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &MultimodalEmbeddingStep{embedder: embedder, logger: logger}
}

func (s *MultimodalEmbeddingStep) Name() string { return "MultimodalEmbeddingStep" }

// Execute encodes the query text and, if present, the image data.
func (s *MultimodalEmbeddingStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("MultimodalEmbeddingStep: query required")
	}
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}

	// Always embed query text.
	textVec, err := s.embedder.EmbedText(ctx, state.Query.Text)
	if err != nil {
		return fmt.Errorf("MultimodalEmbeddingStep: EmbedText failed: %w", err)
	}
	state.Agentic.Custom["query_vector"] = textVec

	s.logger.Debug("text embedded", map[string]interface{}{
		"step": "MultimodalEmbeddingStep",
		"dims": len(textVec),
	})

	// Optionally embed image data when provided by the caller.
	if imageData, ok := state.Agentic.Custom["image_data"].([]byte); ok && len(imageData) > 0 {
		imgVec, err := s.embedder.EmbedImage(ctx, imageData)
		if err != nil {
			// Non-fatal: log and skip image path.
			s.logger.Error("EmbedImage failed, skipping image retrieval path", err, map[string]interface{}{
				"step": "MultimodalEmbeddingStep",
			})
		} else {
			state.Agentic.Custom["image_vector"] = imgVec
			s.logger.Debug("image embedded", map[string]interface{}{
				"step": "MultimodalEmbeddingStep",
				"dims": len(imgVec),
			})
		}
	}

	return nil
}
