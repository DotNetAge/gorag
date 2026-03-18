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

// parentDocExpand expands child chunks to their parent documents.
type parentDocExpand struct {
	expander retrieval.ResultEnhancer
	logger   logging.Logger
	metrics  abstraction.Metrics
}

// ParentDocExpand creates a parent document expansion step with logger and metrics.
//
// Parameters:
//   - expander: result enhancer implementation for expanding chunks
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(rerank.ParentDocExpand(expander, logger, metrics))
func ParentDocExpand(expander retrieval.ResultEnhancer, logger logging.Logger, metrics abstraction.Metrics) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &parentDocExpand{
		expander: expander,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *parentDocExpand) Name() string {
	return "ParentDocExpand"
}

// Execute enhances retrieval results by expanding child chunks to parent documents.
func (s *parentDocExpand) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("ParentDocExpand: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("ParentDocExpand: no chunks to expand", map[string]interface{}{
			"step": "ParentDocExpand",
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

	s.logger.Info("expanding child chunks to parent documents", map[string]interface{}{
		"step":         "ParentDocExpand",
		"chunks_count": len(allChunks),
		"query":        state.Query.Text,
	})

	// Create RetrievalResult for enhancement
	result := entity.NewRetrievalResult(state.Query.ID, "", allChunks, nil, nil)

	// Use ResultEnhancer to expand chunks to parent documents
	enhancedResult, err := s.expander.Enhance(ctx, result)
	if err != nil {
		s.logger.Error("parent document expansion failed", err, map[string]interface{}{
			"step":  "ParentDocExpand",
			"query": state.Query.Text,
		})
		return fmt.Errorf("ParentDocExpand: Enhance failed: %w", err)
	}

	// Replace retrieved chunks with expanded parent documents
	state.RetrievedChunks = [][]*entity.Chunk{enhancedResult.Chunks}

	s.logger.Info("parent documents expanded successfully", map[string]interface{}{
		"step":           "ParentDocExpand",
		"original_count": len(allChunks),
		"expanded_count": len(enhancedResult.Chunks),
		"query":          state.Query.Text,
	})

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("parent_doc_expand", len(enhancedResult.Chunks))
	}

	return nil
}
