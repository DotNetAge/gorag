package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*generationStep)(nil)

// generator is a thin adapter that delegates to infra/service.
type generationStep struct {
	generator retrieval.Generator
	logger    logging.Logger
}

// NewGenerator creates a new generation step with logger.
func NewGenerator(generator retrieval.Generator, logger logging.Logger) *generationStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &generationStep{
		generator: generator,
		logger:    logger,
	}
}

// Name returns the step name
func (s *generationStep) Name() string {
	return "Generator"
}

// Execute generates answer using infra/service.
// This is a thin adapter (<30 lines).
func (s *generationStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("generator: query required")
	}

	s.logger.Debug("generating response", map[string]interface{}{
		"step":  "Generator",
		"query": state.Query.Text,
	})

	// Flatten RetrievedChunks to []*entity.Chunk
	var chunks []*entity.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		chunks = append(chunks, chunkGroup...)
	}

	// Delegate to infra/service (thick business logic)
	result, err := s.generator.Generate(ctx, state.Query, chunks)
	if err != nil {
		s.logger.Error("generate failed", err, map[string]interface{}{
			"step":  "Generator",
			"query": state.Query.Text,
		})
		return fmt.Errorf("generator: Generate failed: %w", err)
	}

	// Update state (thin adapter 职责)
	state.Answer = result.Answer

	s.logger.Info("response generated", map[string]interface{}{
		"step":          "Generator",
		"answer_length": len(result.Answer),
		"query":         state.Query.Text,
	})

	return nil
}
