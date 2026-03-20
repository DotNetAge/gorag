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

	// 收集所有需要搜索的文本
	var queriesToSearch []string

	// 1. 优先使用 HyDE 生成的假设性文档
	if context.Agentic != nil && context.Agentic.HydeApplied && context.Agentic.HypotheticalDocument != "" {
		queriesToSearch = append(queriesToSearch, context.Agentic.HypotheticalDocument)
	} else if context.Agentic != nil && len(context.Agentic.SubQueries) > 0 {
		// 2. 如果存在子查询（Fusion RAG），则搜索所有子查询
		queriesToSearch = append(queriesToSearch, context.Agentic.SubQueries...)
	} else {
		// 3. 默认搜索当前查询
		queriesToSearch = append(queriesToSearch, context.Query.Text)
	}

	// 4. 如果存在后退一步查询（Step-back RAG），也将其加入搜索列表
	if context.Agentic != nil && context.Agentic.StepBackQuery != "" {
		queriesToSearch = append(queriesToSearch, context.Agentic.StepBackQuery)
	}

	for _, queryText := range queriesToSearch {
		// 生成查询嵌入向量
		embResults, err := s.embedder.Embed(ctx, []string{queryText})
		if err != nil {
			return fmt.Errorf("failed to embed query [%s]: %w", queryText, err)
		}
		if len(embResults) == 0 {
			continue
		}

		// 执行向量搜索
		vectors, _, err := s.store.Search(ctx, embResults[0], s.opts.TopK, s.opts.Filters)
		if err != nil {
			return fmt.Errorf("vector store search failed for [%s]: %w", queryText, err)
		}

		// 将结果转换为 Chunk 并存入状态
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
	}

	return nil
}
