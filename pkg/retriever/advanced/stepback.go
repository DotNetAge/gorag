package advanced

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/retrieval/answer"
	"github.com/DotNetAge/gorag/pkg/retrieval/query"
	stepgen "github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/stepback"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
)

type stepbackRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
}

// NewStepbackRetriever creates a new StepbackRetriever for abstracting queries.
func NewStepbackRetriever(
	vectorStore core.VectorStore,
	embedder embedding.Provider,
	llm chat.Client,
	topK int,
	logger logging.Logger,
) core.Retriever {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}

	p := pipeline.New[*core.RetrievalContext]()
	gen := answer.New(llm, answer.WithLogger(logger))
	decomposer := query.NewDecomposer(llm, query.WithDecomposerLogger(logger))

	// Step 1: Generate step-back query (abstract principle)
	p.AddStep(stepback.Generate(decomposer))

	// Step 2: Search with original and step-back query
	// Note: We need a search step that can handle multiple queries or we use parallel searches.
	// For simplicity in this standard retriever, we'll use the default search which uses state.Query.
	// In a full implementation, we might want to search both original and step-back.
	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
		TopK: topK,
	}))

	// Step 3: Generate final answer
	p.AddStep(stepgen.Generate(gen, logger, nil))

	return &stepbackRetriever{pipeline: p}
}

func (r *stepbackRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
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
