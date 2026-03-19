package fuse

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// rrf merges multiple chunk lists using Reciprocal Rank Fusion.
type rrf struct {
	fusionEngine core.FusionEngine
	topK         int
	logger       logging.Logger
	metrics      core.Metrics
}

// RRF creates a new RRF fusion step.
func RRF(
	engine core.FusionEngine,
	topK int,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.State] {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &rrf{
		fusionEngine: engine,
		topK:         topK,
		logger:       logger,
		metrics:      metrics,
	}
}

func (s *rrf) Name() string { return "RRF" }

func (s *rrf) Execute(ctx context.Context, state *core.State) error {
	if len(state.ParallelResults) == 0 {
		return nil
	}

	var results [][]*core.Chunk
	for _, v := range state.ParallelResults {
		results = append(results, v)
	}

	fusedChunks, err := s.fusionEngine.Fuse(ctx, results, s.topK)
	if err != nil {
		return fmt.Errorf("RRF failed: %w", err)
	}

	state.RetrievedChunks = [][]*core.Chunk{fusedChunks}
	return nil
}
