package advanced

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/retrieval/answer"
	stepgen "github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/rewrite"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
)

type rewriteRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
}

// NewRewriteRetriever creates a new RewriteRetriever that clarifies ambiguous queries.
func NewRewriteRetriever(
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

	// Step 1: Clarify/Rewrite the query
	p.AddStep(rewrite.Rewrite(llm, logger, nil))

	// Step 2: Search with the clarified query
	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
		TopK: topK,
	}))

	// Step 3: Generate final answer
	p.AddStep(stepgen.Generate(gen, logger, nil))

	return &rewriteRetriever{pipeline: p}
}

func (r *rewriteRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
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
