package vector

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
)

// SearchOptions 搜索选项
type SearchOptions struct {
	TopK    int
	Filters map[string]any
}

type searchStep struct {
	store    core.VectorStore
	embedder embedding.Provider
	opts     SearchOptions
}

// Search 执行向量搜索的原子步骤
func Search(store core.VectorStore, embedder embedding.Provider, opts SearchOptions) pipeline.Step[*core.RetrievalContext] {
	return &searchStep{
		store:    store,
		embedder: embedder,
		opts:     opts,
	}
}

func (s *searchStep) Name() string {
	return "VectorSearch"
}

func (s *searchStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if context.Query == nil || context.Query.Text == "" {
		return fmt.Errorf("search query is empty in context")
	}

	// 优先使用 HyDE 生成的假设性文档进行向量化检索
	queryText := context.Query.Text
	if context.Agentic != nil && context.Agentic.HydeApplied && context.Agentic.HypotheticalDocument != "" {
		queryText = context.Agentic.HypotheticalDocument
	}

	// 1. 生成查询嵌入向量
	embResults, err := s.embedder.Embed(ctx, []string{queryText})
	if err != nil {
		return fmt.Errorf("failed to embed query: %w", err)
	}
	if len(embResults) == 0 {
		return fmt.Errorf("no embedding returned for query")
	}

	// 2. 执行向量搜索
	vectors, _, err := s.store.Search(ctx, embResults[0], s.opts.TopK, s.opts.Filters)
	if err != nil {
		return fmt.Errorf("vector store search failed: %w", err)
	}

	// 3. 将结果转换为 Chunk 并存入状态
	chunks := make([]*core.Chunk, len(vectors))
	for i, v := range vectors {
		chunks[i] = &core.Chunk{
			ID:       v.ID,
			Content:  "",
			Metadata: v.Metadata,
		}
		if content, ok := v.Metadata["content"].(string); ok {
			chunks[i].Content = content
		}
	}

	// 将结果添加到 RetrievedChunks 列表中
	context.RetrievedChunks = append(context.RetrievedChunks, chunks)
	return nil
}
