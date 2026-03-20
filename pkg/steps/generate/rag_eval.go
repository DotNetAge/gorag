package stepgen

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ragEvaluator evaluates the quality of generated answers.
type ragEvaluator struct {
	evaluator core.RAGEvaluator
	logger    logging.Logger
	metrics   core.Metrics
}

// RAGEvaluation creates a RAG evaluation step with logger and metrics.
//
// Parameters:
//   - evaluator: RAG evaluator implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(generate.RAGEvaluation(evaluator, logger, metrics))
func RAGEvaluation(evaluator core.RAGEvaluator, logger logging.Logger, metrics core.Metrics) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &ragEvaluator{
		evaluator: evaluator,
		logger:    logger,
		metrics:   metrics,
	}
}

// Name returns the step name
func (s *ragEvaluator) Name() string {
	return "RAGEvaluation"
}

// Execute evaluates the generated answer using the RAG evaluator.
func (s *ragEvaluator) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("ragEvaluator: query required")
	}

	if state.Answer.Answer == "" {
		s.logger.Debug("no answer generated, skipping evaluation", map[string]interface{}{
			"step": "RAGEvaluation",
		})
		return nil
	}

	// Build context from retrieved chunks
	var contextBuilder strings.Builder
	for _, chunkGroup := range state.RetrievedChunks {
		for _, chunk := range chunkGroup {
			if chunk.Content != "" {
				contextBuilder.WriteString(chunk.Content)
				contextBuilder.WriteString("\n\n")
			}
		}
	}
	contextStr := contextBuilder.String()

	if contextStr == "" {
		s.logger.Debug("no context available for evaluation", map[string]interface{}{
			"step": "RAGEvaluation",
		})
		return nil
	}

	s.logger.Info("evaluating generated answer", map[string]interface{}{
		"step":          "RAGEvaluation",
		"query":         state.Query.Text,
		"answer_length": len(state.Answer.Answer),
	})

	// Evaluate the answer
	scores, err := s.evaluator.Evaluate(ctx, state.Query.Text, state.Answer.Answer, contextStr)
	if err != nil {
		s.logger.Error("RAG evaluation failed", err, map[string]interface{}{
			"step":  "RAGEvaluation",
			"query": state.Query.Text,
		})
		return fmt.Errorf("RAGEvaluation: Evaluate failed: %w", err)
	}

	// Store evaluation scores in state for observability
	if state.Agentic == nil {
		state.Agentic = core.NewAgenticState()
	}
	state.Agentic.Custom["rag_scores"] = scores

	s.logger.Info("RAG evaluation completed", map[string]interface{}{
		"step":              "RAGEvaluation",
		"faithfulness":      scores.Faithfulness,
		"answer_relevance":  scores.Faithfulness,
		"context_precision": scores.Relevance,
	})

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("rag_evaluation", 1)
	}

	return nil
}
