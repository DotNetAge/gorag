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
var _ pipeline.Step[*entity.PipelineState] = (*intentRouter)(nil)

// intentRouter is a thin adapter that delegates to infra/service.
type intentRouter struct {
	classifier retrieval.IntentClassifier
	logger     logging.Logger
}

// NewIntentRouter creates a new intent router step with logger.
func NewIntentRouter(classifier retrieval.IntentClassifier, logger logging.Logger) *intentRouter {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &intentRouter{
		classifier: classifier,
		logger:     logger,
	}
}

// Name returns the step name
func (s *intentRouter) Name() string {
	return "IntentRouter"
}

// Execute classifies query intent using infra/service.
// This is a thin adapter (<30 lines).
func (s *intentRouter) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("intentRouter: query required")
	}

	// Delegate to infra/service (thick business logic)
	result, err := s.classifier.Classify(ctx, state.Query)
	if err != nil {
		s.logger.Error("classify failed", err, map[string]interface{}{
			"step":  "IntentRouter",
			"query": state.Query.Text,
		})
		return fmt.Errorf("intentRouter: Classify failed: %w", err)
	}

	// Update state using AgenticMetadata (thin adapter 职责)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.Intent = string(result.Intent)

	s.logger.Info("intent classified", map[string]interface{}{
		"step":       "IntentRouter",
		"intent":     result.Intent,
		"confidence": result.Confidence,
		"query":      state.Query.Text,
	})

	return nil
}
