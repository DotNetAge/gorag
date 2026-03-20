package advanced

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/retrieval/answer"
	"github.com/DotNetAge/gorag/pkg/retrieval/fusion"
	"github.com/DotNetAge/gorag/pkg/retrieval/query"
	"github.com/DotNetAge/gorag/pkg/steps/decompose"
	"github.com/DotNetAge/gorag/pkg/steps/fuse"
	stepgen "github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
)

type fusionRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
}

// NewFusionRetriever creates a new FusionRetriever for multi-perspective search.
func NewFusionRetriever(
	vectorStore core.VectorStore,
	embedder embedding.Provider,
	llm chat.Client,
	topK int,
	logger logging.Logger,
) core.Retriever {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}

	p := pipeline.New[*core.RetrievalContext]()
	gen := answer.New(llm, answer.WithLogger(logger))
	decomposer := query.NewDecomposer(llm, query.WithDecomposerLogger(logger))
	fusionEngine := fusion.NewRRFFusionEngine()

	// Step 1: Decompose query into sub-queries
	p.AddStep(decompose.Decompose(decomposer, logger))

	// Step 2: Multi-query search (implicitly handled by vector.Search if it supports sub-queries in context)
	// We need to check if vector.Search handles sub-queries.
	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
		TopK: topK,
	}))

	// Step 3: RRF Fusion
	p.AddStep(fuse.RRF(fusionEngine, topK, logger))

	// Step 4: Generate final answer
	p.AddStep(stepgen.Generate(gen, logger, nil))

	return &fusionRetriever{pipeline: p}
}

func (r *fusionRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, q := range queries {
		context := core.NewRetrievalContext(ctx, q)
		
		if err := r.pipeline.Execute(ctx, context); err != nil {
			return nil, err
		}

		var allChunks []*core.Chunk
		for _, group := range context.RetrievedChunks {
			allChunks = append(allChunks, group...)
		}

		res := &core.RetrievalResult{
			Query:  q,
			Chunks: allChunks,
			Answer: context.Answer.Answer,
		}
		results = append(results, res)
	}

	return results, nil
}
