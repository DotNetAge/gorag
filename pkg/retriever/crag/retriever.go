package crag

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/retrieval/answer"
	"github.com/DotNetAge/gorag/pkg/steps/crag"
	"github.com/DotNetAge/gorag/pkg/steps/enrich"
	"github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
)

type cragRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
}

// NewRetriever creates a new Corrective RAG (CRAG) retriever.
func NewRetriever(
	vectorStore core.VectorStore,
	embedder embedding.Provider,
	evaluator core.CRAGEvaluator,
	llm chat.Client,
	opts ...Option,
) core.Retriever {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	if options.logger == nil {
		options.logger = logging.NewNoopLogger()
	}

	p := pipeline.New[*core.RetrievalContext]()

	// 1. Initial Vector Search
	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
		TopK: options.topK,
	}))

	// 2. CRAG Evaluation
	p.AddStep(crag.Evaluate(evaluator, options.logger, nil))

	// 2.5 DocStore Enrichment (PDR)
	if options.docStore != nil {
		p.AddStep(enrich.EnrichWithDocStore(options.docStore, options.logger))
	}

	// 3. Fallback to Web Search if needed
	p.AddStep(&cragFallbackStep{
		searcher: options.webSearcher,
		logger:   options.logger,
		topK:     options.topK,
	})

	// 4. Final Answer Generation
	gen := answer.New(llm, answer.WithLogger(options.logger))
	p.AddStep(stepgen.Generate(gen, options.logger, nil))

	return &cragRetriever{pipeline: p}
}

func (r *cragRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, q := range queries {
		retrievalCtx := core.NewRetrievalContext(ctx, q)

		if err := r.pipeline.Execute(ctx, retrievalCtx); err != nil {
			return nil, err
		}

		var allChunks []*core.Chunk
		for _, group := range retrievalCtx.RetrievedChunks {
			allChunks = append(allChunks, group...)
		}

		res := &core.RetrievalResult{
			Query:  q,
			Chunks: allChunks,
			Answer: retrievalCtx.Answer.Answer,
		}
		results = append(results, res)
	}

	return results, nil
}

// cragFallbackStep handles web search fallback based on CRAG evaluation.
type cragFallbackStep struct {
	searcher core.WebSearcher
	logger   logging.Logger
	topK     int
}

func (s *cragFallbackStep) Name() string {
	return "CRAGFallback"
}

func (s *cragFallbackStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if context.Agentic == nil || context.Agentic.Custom["crag_evaluation"] == nil {
		return nil
	}

	evaluation, ok := context.Agentic.Custom["crag_evaluation"].(*core.CRAGEvaluation)
	if !ok {
		return nil
	}

	// Logic:
	// - Correct: use retrieved chunks (do nothing here)
	// - Incorrect: replace with web search results
	// - Ambiguous: supplement with web search results

	if evaluation.Label == core.CRAGRelevant {
		s.logger.Debug("CRAG: context relevant, skipping fallback", map[string]any{"query": context.Query.Text})
		return nil
	}

	if s.searcher == nil {
		s.logger.Warn("CRAG: fallback needed but no WebSearcher provided", map[string]any{"label": evaluation.Label})
		return nil
	}

	s.logger.Info("CRAG: trigger fallback web search", map[string]any{
		"label": evaluation.Label,
		"query": context.Query.Text,
	})

	webChunks, err := s.searcher.Search(ctx, context.Query.Text, s.topK)
	if err != nil {
		s.logger.Error("CRAG: web search fallback failed", err)
		return nil // Non-fatal
	}

	if evaluation.Label == core.CRAGIrrelevant {
		// Replace all retrieved chunks
		context.RetrievedChunks = [][]*core.Chunk{webChunks}
	} else if evaluation.Label == core.CRAGAmbiguous {
		// Append to existing chunks
		context.RetrievedChunks = append(context.RetrievedChunks, webChunks)
	}

	return nil
}
// Options for CRAG retriever
type Options struct {
	topK        int
	webSearcher core.WebSearcher
	docStore    store.DocStore
	logger      logging.Logger
}

func defaultOptions() *Options {
	return &Options{
		topK: 5,
	}
}

type Option func(*Options)

func WithTopK(k int) Option {
	return func(o *Options) {
		o.topK = k
	}
}

func WithWebSearcher(s core.WebSearcher) Option {
	return func(o *Options) {
		o.webSearcher = s
	}
}

func WithDocStore(s store.DocStore) Option {
	return func(o *Options) {
		o.docStore = s
	}
}

func WithLogger(l logging.Logger) Option {
...
	return func(o *Options) {
		o.logger = l
	}
}
