package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/infra/indexing"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// EmbedStep 向量化步骤
type EmbedStep struct {
	embedder embedding.Provider
	metrics  abstraction.Metrics
}

// NewEmbedStep 创建向量化步骤（支持指标收集）
func NewEmbedStep(embedder embedding.Provider, metrics abstraction.Metrics) *EmbedStep {
	return &EmbedStep{
		embedder: embedder,
		metrics:  metrics,
	}
}

// Name 返回步骤名称
func (s *EmbedStep) Name() string {
	return "Embed"
}

// Execute 执行向量化步骤（实现 gochat/pkg/pipeline.Step 接口）
func (s *EmbedStep) Execute(ctx context.Context, state *indexing.State) error {
	if s.embedder == nil {
		return fmt.Errorf("embedder not configured")
	}

	// 从状态中获取 Chunks（注意：Chunks 是 channel）
	if state.Chunks == nil {
		return fmt.Errorf("no chunks to embed")
	}

	// ✅ 遍历所有 chunks 并生成向量
	var vectors []*entity.Vector
	totalChunks := 0

	for chunk := range state.Chunks {
		if chunk == nil {
			continue
		}

		// 使用 embedder 生成向量（批量处理）
		embeddingResults, err := s.embedder.Embed(ctx, []string{chunk.Content})
		if err != nil {
			return fmt.Errorf("failed to embed chunk %s: %w", chunk.ID, err)
		}

		// 获取第一个结果（因为是批量处理，但这里只有一个）
		if len(embeddingResults) > 0 && len(embeddingResults[0]) > 0 {
			// 创建向量实体
			vector := entity.NewVector(
				fmt.Sprintf("vec_%s", chunk.ID),
				embeddingResults[0], // 取第一个向量的值
				chunk.ID,
				chunk.Metadata,
			)

			vectors = append(vectors, vector)
			totalChunks++

			// 更新 chunk 的 VectorID
			chunk.SetVectorID(vector.ID)
		}
	}

	// 将向量存储到状态中，供下一步 StoreStep 使用
	state.Vectors = vectors
	state.TotalChunks = totalChunks

	// ✅ 记录 Embedding 指标
	if s.metrics != nil && totalChunks > 0 {
		s.metrics.RecordEmbeddingCount(totalChunks)
	}

	return nil
}
