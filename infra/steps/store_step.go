package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/infra/indexing"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
)

// StoreStep 存储步骤
type StoreStep struct {
	vectorStore abstraction.VectorStore
	metrics     abstraction.Metrics
}

// NewStoreStep 创建存储步骤（支持指标收集）
func NewStoreStep(vectorStore abstraction.VectorStore, metrics abstraction.Metrics) *StoreStep {
	return &StoreStep{
		vectorStore: vectorStore,
		metrics:     metrics,
	}
}

// Name 返回步骤名称
func (s *StoreStep) Name() string {
	return "Store"
}

// Execute 执行存储步骤（实现 gochat/pkg/pipeline.Step 接口）
func (s *StoreStep) Execute(ctx context.Context, state *indexing.State) error {
	if s.vectorStore == nil {
		return fmt.Errorf("vector store not configured")
	}

	// 从状态中获取 Chunks
	if state.Chunks == nil {
		return fmt.Errorf("no chunks to store")
	}

	// 将 chunks 存储到向量数据库
	// 注意：这里需要根据实际的 Chunk 类型和 VectorStore 接口进行适配
	// 目前只是一个占位实现，等待接口统一后完善

	// ✅ 记录 VectorStore 操作指标
	if s.metrics != nil && state.TotalChunks > 0 {
		s.metrics.RecordVectorStoreOperations("store", state.TotalChunks)
	}

	return nil
}
