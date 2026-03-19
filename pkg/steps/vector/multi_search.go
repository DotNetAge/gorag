package vector

import (
	"context"
	"fmt"
	"sync"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
)

type parallelSearchStep struct {
	store    core.VectorStore
	embedder embedding.Provider
	opts     SearchOptions
}

// MultiSearch 执行并行多路向量搜索
// 它会检查 Agentic.SubQueries，并为每个查询并行发起搜索
func MultiSearch(store core.VectorStore, embedder embedding.Provider, opts SearchOptions) pipeline.Step[*core.RetrievalContext] {
	return &parallelSearchStep{
		store:    store,
		embedder: embedder,
		opts:     opts,
	}
}

func (s *parallelSearchStep) Name() string {
	return "MultiVectorSearch"
}

func (s *parallelSearchStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	queries := []string{context.Query.Text}
	if context.Agentic != nil && len(context.Agentic.SubQueries) > 0 {
		queries = context.Agentic.SubQueries
	}

	resultsChan := make(chan []*core.Chunk, len(queries))
	errChan := make(chan error, len(queries))
	var wg sync.WaitGroup

	for _, q := range queries {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()
			
			// 1. 生成嵌入
			embResults, err := s.embedder.Embed(ctx, []string{query})
			if err != nil {
				errChan <- fmt.Errorf("failed to embed sub-query %s: %w", query, err)
				return
			}
			if len(embResults) == 0 {
				errChan <- fmt.Errorf("no embedding for sub-query %s", query)
				return
			}

			// 2. 搜索
			vectors, _, err := s.store.Search(ctx, embResults[0], s.opts.TopK, s.opts.Filters)
			if err != nil {
				errChan <- fmt.Errorf("search failed for sub-query %s: %w", query, err)
				return
			}

			// 3. 转换
			chunks := make([]*core.Chunk, len(vectors))
			for i, v := range vectors {
				chunks[i] = &core.Chunk{
					ID:       v.ID,
					Metadata: v.Metadata,
				}
				if content, ok := v.Metadata["content"].(string); ok {
					chunks[i].Content = content
				}
			}
			resultsChan <- chunks
		}(q)
	}

	wg.Wait()
	close(resultsChan)
	close(errChan)

	// 检查是否有错误（只要有一个失败，目前采取抛错策略，或者记录警告）
	if len(errChan) > 0 {
		return <-errChan
	}

	// 收集所有结果
	for chunks := range resultsChan {
		context.RetrievedChunks = append(context.RetrievedChunks, chunks)
	}

	return nil
}
