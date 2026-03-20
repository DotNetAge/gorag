package native

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/retrieval/answer"
	"github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
)

type nativeRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
	tracer   observability.Tracer
}

// Options for native retriever
type Options struct {
	Logger logging.Logger
	Tracer observability.Tracer
}

func NewRetriever(
	vectorStore core.VectorStore,
	embedder embedding.Provider,
	llm chat.Client,
	topK int,
	logger logging.Logger,
) core.Retriever {
	return NewRetrieverWithOptions(vectorStore, embedder, llm, topK, Options{
		Logger: logger,
		Tracer: observability.NewNoopTracer(),
	})
}

func NewRetrieverWithOptions(
	vectorStore core.VectorStore,
	embedder embedding.Provider,
	llm chat.Client,
	topK int,
	opts Options,
) core.Retriever {
	if opts.Logger == nil {
		opts.Logger = logging.NewNoopLogger()
	}
	if opts.Tracer == nil {
		opts.Tracer = observability.NewNoopTracer()
	}

	p := pipeline.New[*core.RetrievalContext]()

	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
		TopK: topK,
	}))

	gen := answer.New(llm, answer.WithLogger(opts.Logger))
	p.AddStep(stepgen.Generate(gen, opts.Logger, nil))

	return &nativeRetriever{
		pipeline: p,
		tracer:   opts.Tracer,
	}
}

func (r *nativeRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, q := range queries {
		retrievalCtx := core.NewRetrievalContext(ctx, q)
		retrievalCtx.Tracer = r.tracer

		// Start root span
		retrievalCtx.Ctx, retrievalCtx.Span = r.tracer.StartSpan(retrievalCtx.Ctx, "NativeRAG.Retrieve")
		retrievalCtx.Span.SetTag("query", q)

		if err := r.pipeline.Execute(retrievalCtx.Ctx, retrievalCtx); err != nil {
			retrievalCtx.Span.LogEvent("error", map[string]any{"error": err.Error()})
			retrievalCtx.Span.End()
			return nil, err
		}

		retrievalCtx.Span.End()

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
