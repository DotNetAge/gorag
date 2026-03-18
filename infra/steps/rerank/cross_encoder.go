package rerank

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// crossEncoderRerank performs cross-encoder based reranking.
type crossEncoderRerank struct {
	reranker abstraction.Reranker
	topK     int
	logger   logging.Logger
	metrics  abstraction.Metrics
}

// CrossEncoderRerank creates a cross-encoder reranking step with logger and metrics.
//
// Parameters:
//   - reranker: cross-encoder reranker implementation
//   - topK: number of results to keep after reranking (default: 10)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(rerank.CrossEncoderRerank(reranker, 5, logger, metrics))
func CrossEncoderRerank(reranker abstraction.Reranker, topK int, logger logging.Logger, metrics abstraction.Metrics) pipeline.Step[*entity.PipelineState] {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &crossEncoderRerank{
		reranker: reranker,
		topK:     topK,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *crossEncoderRerank) Name() string {
	return "CrossEncoderRerank"
}

// Execute reranks retrieved chunks using cross-encoder model.
func (s *crossEncoderRerank) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("crossEncoderRerank: query required")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("CrossEncoderRerank: no chunks to rerank", map[string]interface{}{
			"step": "CrossEncoderRerank",
		})
		return nil
	}

	// Flatten all chunks
	var allChunks []*entity.Chunk
	for _, chunks := range state.RetrievedChunks {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	s.logger.Info("reranking chunks using cross-encoder", map[string]interface{}{
		"step":         "CrossEncoderRerank",
		"chunks_count": len(allChunks),
		"query":        state.Query.Text,
	})

	// Use reranker to rerank chunks
	rerankedChunks, _, err := s.reranker.Rerank(ctx, state.Query.Text, allChunks, s.topK)
	if err != nil {
		s.logger.Error("cross-encoder reranking failed", err, map[string]interface{}{
			"step":  "CrossEncoderRerank",
			"query": state.Query.Text,
		})
		return fmt.Errorf("CrossEncoderRerank: Rerank failed: %w", err)
	}

	// Replace retrieved chunks with reranked chunks
	state.RetrievedChunks = [][]*entity.Chunk{rerankedChunks}

	s.logger.Info("cross-encoder reranking completed", map[string]interface{}{
		"step":           "CrossEncoderRerank",
		"original_count": len(allChunks),
		"reranked_count": len(rerankedChunks),
		"query":          state.Query.Text,
	})

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("cross_encoder_rerank", len(rerankedChunks))
	}

	return nil
}
