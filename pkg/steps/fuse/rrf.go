package fuse

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type rrfStep struct {
	fusion  core.FusionEngine
	topK    int
	logger  logging.Logger
}

// RRF 创建一个基于 Reciprocal Rank Fusion 的融合步骤
func RRF(fusion core.FusionEngine, topK int, logger logging.Logger) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &rrfStep{
		fusion:  fusion,
		topK:    topK,
		logger:  logger,
	}
}

func (s *rrfStep) Name() string {
	return "RRF-Fusion"
}

func (s *rrfStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if len(context.RetrievedChunks) == 0 {
		s.logger.Warn("RRF-Fusion: no chunks to fuse", nil)
		return nil
	}

	s.logger.Debug("fusing retrieval results", map[string]interface{}{
		"result_sets": len(context.RetrievedChunks),
		"top_k":       s.topK,
	})

	// 执行 RRF 融合
	fused, err := s.fusion.ReciprocalRankFusion(ctx, context.RetrievedChunks, s.topK)
	if err != nil {
		return fmt.Errorf("RRF-Fusion failed: %w", err)
	}

	// 清空原有的多路结果，替换为融合后的单一结果集
	context.RetrievedChunks = [][]*core.Chunk{fused}

	s.logger.Info("retrieval results fused", map[string]interface{}{
		"fused_count": len(fused),
	})

	return nil
}
