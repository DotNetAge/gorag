package rerank

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// sentenceWindowExpand expands chunks with surrounding sentence context.
type sentenceWindowExpand struct {
	expander core.ResultEnhancer
	logger   logging.Logger
	metrics  core.Metrics
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
func SentenceWindowExpand(expander core.ResultEnhancer, logger logging.Logger, metrics core.Metrics) pipeline.Step[*core.RetrievalContext] {
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

// Execute expands chunks to include surrounding sentences.
func (s *sentenceWindowExpand) Execute(ctx context.Context, state *core.RetrievalContext) error {
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
	var allChunks []*core.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		allChunks = append(allChunks, chunkGroup...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	s.logger.Info("expanding chunks with sentence window context", map[string]interface{}{
		"step":         "SentenceWindowExpand",
		"chunks_count": len(allChunks),
		"query":        state.Query.Text,
	})

	// Use ResultEnhancer to expand chunks with sentence window
	enhancedResult, err := s.expander.Enhance(ctx, state.Query, allChunks)
	if err != nil {
		s.logger.Error("sentence window expansion failed", err, map[string]interface{}{
			"step":  "SentenceWindowExpand",
			"query": state.Query.Text,
		})
		return fmt.Errorf("SentenceWindowExpand: Enhance failed: %w", err)
	}

	// Replace retrieved chunks with expanded chunks
	state.RetrievedChunks = [][]*core.Chunk{enhancedResult}

	s.logger.Info("sentence window expansion completed successfully", map[string]interface{}{
		"step":           "SentenceWindowExpand",
		"original_count": len(allChunks),
		"expanded_count": len(enhancedResult),
		"query":          state.Query.Text,
	})

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("sentence_window_expand", len(enhancedResult))
	}

	return nil
}
