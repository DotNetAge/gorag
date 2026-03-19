package native

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/retrieval/answer"
	"github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
)

type nativeRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
}

func NewRetriever(
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

	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
		TopK: topK,
	}))

	gen := answer.New(llm, answer.WithLogger(logger))
	p.AddStep(stepgen.Generate(gen, logger, nil))

	return &nativeRetriever{pipeline: p}
}

func (r *nativeRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
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
