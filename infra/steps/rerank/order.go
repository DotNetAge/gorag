// Package rerank provides reranking steps for RAG retrieval pipelines.
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

// order re-orders chunks using LLM-based Cross-Encoder result enhancer.
type order struct {
	enhancer retrieval.ResultEnhancer
	logger   logging.Logger
	metrics  abstraction.Metrics
}

// Order creates a new cross-encoder reranking step with logger and metrics.
//
// Parameters:
//   - enhancer: LLM-based result enhancer implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(rerank.Order(enhancer, logger, metrics))
func Order(
	enhancer retrieval.ResultEnhancer,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &order{
		enhancer: enhancer,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *order) Name() string {
	return "CrossEncoderRerank"
}

// Execute enhances retrieval results using cross-encoder reranking.
func (s *order) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("CrossEncoderRerank: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("CrossEncoderRerank: no chunks to rerank", map[string]interface{}{
			"step": "CrossEncoderRerank",
		})
		return nil
	}

	// Flatten chunks from all retrieval sources
	var allChunks []*entity.Chunk
	for _, chunks := range state.RetrievedChunks {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	// Create retrieval result for enhancement
	result := entity.NewRetrievalResult(
		state.Query.ID,
		"", // Empty document ID for pipeline context
		allChunks,
		make([]float32, len(allChunks)), // Initial scores
		nil,
	)

	s.logger.Info("CrossEncoderRerank: starting reranking", map[string]interface{}{
		"step":        "CrossEncoderRerank",
		"chunk_count": len(allChunks),
	})

	// Apply cross-encoder reranking
	enhancedResult, err := s.enhancer.Enhance(ctx, result)
	if err != nil {
		s.logger.Error("cross-encoder enhancement failed", err, map[string]interface{}{
			"step":        "CrossEncoderRerank",
			"chunk_count": len(allChunks),
		})
		return fmt.Errorf("CrossEncoderRerank: enhance failed: %w", err)
	}

	// Update state with reranked chunks
	state.RetrievedChunks = [][]*entity.Chunk{enhancedResult.Chunks}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("cross_encoder_rerank", len(enhancedResult.Chunks))
	}

	s.logger.Info("CrossEncoderRerank: completed reranking", map[string]interface{}{
		"step":           "CrossEncoderRerank",
		"original_count": len(allChunks),
		"reranked_count": len(enhancedResult.Chunks),
	})

	return nil
}
