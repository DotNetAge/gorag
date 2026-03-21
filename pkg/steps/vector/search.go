package vector

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"golang.org/x/sync/errgroup"
)

// SearchOptions 搜索选项
type SearchOptions struct {
	TopK        int
	Filters     map[string]any
	Concurrency int // Industrial Gate: number of concurrent sub-queries
}

type searchStep struct {
	store    core.VectorStore
	embedder embedding.Provider
	opts     SearchOptions
}

func (s *searchStep) Name() string {
	return "VectorSearch"
}

func (s *searchStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if s.store == nil || s.embedder == nil {
		return nil // Defensive: skip if not configured (e.g. in fallback tests)
	}

	// Support both single query and multi-query (RAG-Fusion)
	queries := context.Agentic.SubQueries
	if len(queries) == 0 {
		queries = []string{context.Query.Text}
	}

	concurrency := s.opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1 
	}

	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, concurrency)

	for _, qText := range queries {
		q := qText // capture
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			// 1. Generate Embedding
			vecs, err := s.embedder.Embed(gctx, []string{q})
			if err != nil {
				return fmt.Errorf("embedding failed for query %q: %w", q, err)
			}

			// 2. Search Store (Returns vectors and scores)
			vectors, scores, err := s.store.Search(gctx, vecs[0], s.opts.TopK, s.opts.Filters)
			if err != nil {
				return fmt.Errorf("vector search failed for query %q: %w", q, err)
			}

			// 3. Convert Vectors to Chunks
			chunks := make([]*core.Chunk, 0, len(vectors))
			for i, v := range vectors {
				content := ""
				if c, ok := v.Metadata["content"].(string); ok {
					content = c
				}
				
				chunk := &core.Chunk{
					ID:       v.ID,
					Content:  content,
					Metadata: v.Metadata,
				}
				
				if chunk.Metadata == nil {
					chunk.Metadata = make(map[string]any)
				}
				chunk.Metadata["_score"] = scores[i]
				
				chunks = append(chunks, chunk)
			}

			context.ParallelResults[q] = chunks
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Flatten results into RetrievedChunks
	for _, q := range queries {
		if chunks, ok := context.ParallelResults[q]; ok {
			context.RetrievedChunks = append(context.RetrievedChunks, chunks)
		}
	}

	return nil
}

// Search creates a new vector search step.
func Search(store core.VectorStore, embedder embedding.Provider, opts SearchOptions) pipeline.Step[*core.RetrievalContext] {
	return &searchStep{
		store:    store,
		embedder: embedder,
		opts:     opts,
	}
}
