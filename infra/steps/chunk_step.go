package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/infra/indexing"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
)

// ChunkStep 语义分块步骤
type ChunkStep struct {
	chunker dataprep.SemanticChunker
}

// NewChunkStep 创建分块步骤
func NewChunkStep(chunker dataprep.SemanticChunker) *ChunkStep {
	return &ChunkStep{chunker: chunker}
}

// Name 返回步骤名称
func (s *ChunkStep) Name() string {
	return "Chunk"
}

// Execute 执行分块步骤（实现 gochat/pkg/pipeline.Step 接口）
func (s *ChunkStep) Execute(ctx context.Context, state *indexing.State) error {
	if s.chunker == nil {
		return fmt.Errorf("chunker not configured")
	}

	// 从状态中获取 Documents（注意：Documents 是 channel）
	if state.Documents == nil {
		return fmt.Errorf("no documents to chunk")
	}

	// ✅ 遍历所有 documents 并进行分块
	var allChunks []*entity.Chunk

	for doc := range state.Documents {
		if doc == nil {
			continue
		}

		// 使用 chunker 进行分块
		chunks, err := s.chunker.Chunk(ctx, doc)
		if err != nil {
			return fmt.Errorf("failed to chunk document %s: %w", doc.ID, err)
		}

		allChunks = append(allChunks, chunks...)
	}

	// ✅ 创建 channel 并传递 chunks 到下一步
	chunkChan := make(chan *entity.Chunk, len(allChunks))
	for _, chunk := range allChunks {
		chunkChan <- chunk
	}
	close(chunkChan) // 关闭 channel，通知下游步骤

	// 将 chunks channel 存储到状态中，供下一步 EmbedStep 使用
	state.Chunks = chunkChan

	return nil
}
