package rerank

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// sentenceWindowExpand expands chunks with surrounding sentence context.
type sentenceWindowExpand struct {
	expander retrieval.ResultEnhancer
	logger   logging.Logger
	metrics  abstraction.Metrics
}

// SentenceWindowExpand creates a sentence window expansion step with logger and metrics.
//
// Parameters:
//   - expander: result enhancer implementation for expanding chunks with sentence context
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(rerank.SentenceWindowExpand(expander, logger, metrics))
func SentenceWindowExpand(expander retrieval.ResultEnhancer, logger logging.Logger, metrics abstraction.Metrics) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &sentenceWindowExpand{
		expander: expander,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *sentenceWindowExpand) Name() string {
	return "SentenceWindowExpand"
}

// Execute enhances retrieval results by expanding chunks with sentence window context.
func (s *sentenceWindowExpand) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("SentenceWindowExpand: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("SentenceWindowExpand: no chunks to expand", map[string]interface{}{
			"step": "SentenceWindowExpand",
		})
		return nil
	}

	// Flatten chunks
	var allChunks []*entity.Chunk
	for _, chunks := range state.RetrievedChunks {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	s.logger.Info("expanding chunks with sentence window context", map[string]interface{}{
		"step":         "SentenceWindowExpand",
		"chunks_count": len(allChunks),
		"query":        state.Query.Text,
	})

	// Create RetrievalResult for enhancement
	result := entity.NewRetrievalResult(state.Query.ID, "", allChunks, nil, nil)

	// Use ResultEnhancer to expand chunks with sentence window
	enhancedResult, err := s.expander.Enhance(ctx, result)
	if err != nil {
		s.logger.Error("sentence window expansion failed", err, map[string]interface{}{
			"step":  "SentenceWindowExpand",
			"query": state.Query.Text,
		})
		return fmt.Errorf("SentenceWindowExpand: Enhance failed: %w", err)
	}

	// Replace retrieved chunks with expanded chunks
	state.RetrievedChunks = [][]*entity.Chunk{enhancedResult.Chunks}

	s.logger.Info("sentence window expansion completed successfully", map[string]interface{}{
		"step":           "SentenceWindowExpand",
		"original_count": len(allChunks),
		"expanded_count": len(enhancedResult.Chunks),
		"query":          state.Query.Text,
	})

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("sentence_window_expand", len(enhancedResult.Chunks))
	}

	return nil
}
