package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*ragEvaluator)(nil)

// ragEvaluator is a thin adapter that delegates to infra/service.
type ragEvaluator struct {
	evaluator retrieval.RAGEvaluator
	logger    logging.Logger
}

// NewRAGEvaluator creates a new RAG evaluator step with logger.
func NewRAGEvaluator(evaluator retrieval.RAGEvaluator, logger logging.Logger) *ragEvaluator {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &ragEvaluator{
		evaluator: evaluator,
		logger:    logger,
	}
}

// Name returns the step name
func (s *ragEvaluator) Name() string {
	return "RAGEvaluator"
}

// Execute evaluates the generated answer using infra/service.
// This is a thin adapter (<30 lines).
func (s *ragEvaluator) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("ragEvaluator: query required")
	}

	if state.Answer == "" {
		s.logger.Debug("no answer generated, skipping evaluation", map[string]interface{}{
			"step": "RAGEvaluator",
		})
		return nil
	}

	s.logger.Debug("evaluating generated answer", map[string]interface{}{
		"step":          "RAGEvaluator",
		"query":         state.Query.Text,
		"answer_length": len(state.Answer),
	})

	// Flatten RetrievedChunks to context string
	var contextBuilder strings.Builder
	for i, chunkGroup := range state.RetrievedChunks {
		for j, chunk := range chunkGroup {
			contextBuilder.WriteString(fmt.Sprintf("[Chunk %d-%d]\n%s\n\n", i+1, j+1, chunk.Content))
		}
	}
	contextStr := contextBuilder.String()

	// Delegate to infra/service (thick business logic)
	result, err := s.evaluator.Evaluate(ctx, state.Query.Text, state.Answer, contextStr)
	if err != nil {
		s.logger.Error("evaluate failed", err, map[string]interface{}{
			"step":  "RAGEvaluator",
			"query": state.Query.Text,
		})
		return fmt.Errorf("ragEvaluator: Evaluate failed: %w", err)
	}

	// Update state using AgenticMetadata (thin adapter 职责)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.RAGScores = result

	s.logger.Info("RAG evaluation completed", map[string]interface{}{
		"step":    "RAGEvaluator",
		"overall": result.OverallScore,
		"passed":  result.Passed,
		"query":   state.Query.Text,
	})

	return nil
}
