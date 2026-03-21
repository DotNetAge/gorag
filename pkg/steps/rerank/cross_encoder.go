package rerank

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// crossEncoderRerank performs cross-encoder based reranking.
type crossEncoderRerank struct {
	reranker core.Reranker
	topK     int
	logger   logging.Logger
	metrics  core.Metrics
}

// CrossEncoderRerank creates a cross-encoder reranking step.
func CrossEncoderRerank(reranker core.Reranker, topK int, logger logging.Logger, metrics core.Metrics) pipeline.Step[*core.RetrievalContext] {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.DefaultNoopLogger()
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

// Execute reranks retrieved chunks.
func (s *crossEncoderRerank) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("crossEncoderRerank: query required")
	}

	if len(state.RetrievedChunks) == 0 {
		return nil
	}

	// Flatten chunks from RetrievedChunks
	var allChunks []*core.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		allChunks = append(allChunks, chunkGroup...)
	}

	rerankedChunks, err := s.reranker.Rerank(ctx, state.Query, allChunks)
	if err != nil {
		return fmt.Errorf("CrossEncoderRerank: Rerank failed: %w", err)
	}

	if len(rerankedChunks) > s.topK {
		rerankedChunks = rerankedChunks[:s.topK]
	}

	state.RetrievedChunks = [][]*core.Chunk{rerankedChunks}

	if s.metrics != nil {
		s.metrics.RecordSearchResult("cross_encoder_rerank", len(rerankedChunks))
	}

	return nil
}
