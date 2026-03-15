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
var _ pipeline.Step[*entity.PipelineState] = (*entityExtractor)(nil)

// entityExtractor is a thin adapter that delegates to infra/service.
type entityExtractor struct {
	extractor retrieval.EntityExtractor
	logger    logging.Logger
}

// NewEntityExtractor creates a new entity extraction step with logger.
func NewEntityExtractor(extractor retrieval.EntityExtractor, logger logging.Logger) *entityExtractor {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &entityExtractor{
		extractor: extractor,
		logger:    logger,
	}
}

// Name returns the step name
func (s *entityExtractor) Name() string {
	return "EntityExtractor"
}

// Execute extracts entities using infra/service.
// This is a thin adapter (<30 lines).
func (s *entityExtractor) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("entityExtractor: query required")
	}

	// Delegate to infra/service (thick business logic)
	result, err := s.extractor.Extract(ctx, state.Query)
	if err != nil {
		s.logger.Error("extract failed", err, map[string]interface{}{
			"step":  "EntityExtractor",
			"query": state.Query.Text,
		})
		return fmt.Errorf("entityExtractor: Extract failed: %w", err)
	}

	// Update state using AgenticMetadata (thin adapter 职责)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	if len(result.Entities) > 0 {
		state.Agentic.EntityIDs = result.Entities
		state.Agentic.ToolExecuted = true
	}

	s.logger.Info("entities extracted", map[string]interface{}{
		"step":           "EntityExtractor",
		"entities_count": len(result.Entities),
		"query":          state.Query.Text,
	})

	return nil
}
