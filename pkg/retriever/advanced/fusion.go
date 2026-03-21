package advanced

import (
	"context"
	"fmt"
	"path/filepath"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/retrieval/fusion"
	"github.com/DotNetAge/gorag/pkg/retrieval/query"
	"github.com/DotNetAge/gorag/pkg/steps/fuse"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
)

type fusionRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
}

// Options for advanced retriever
type Options struct {
	Logger      logging.Logger
	Embedder    embedding.Provider
	LLM         chat.Client
	TopK        int
	VectorStore core.VectorStore
}

type Option func(*Options)

func WithLogger(l logging.Logger) Option {
	return func(o *Options) { o.Logger = l }
}

func WithEmbedder(e embedding.Provider) Option {
	return func(o *Options) { o.Embedder = e }
}

func WithLLM(l chat.Client) Option {
	return func(o *Options) { o.LLM = l }
}

func WithTopK(k int) Option {
	return func(o *Options) { o.TopK = k }
}

func WithStore(s core.VectorStore) Option {
	return func(o *Options) { o.VectorStore = s }
}

// DefaultAdvancedRetriever creates a best-practice Advanced RAG retriever.
func DefaultAdvancedRetriever(opts ...Option) (core.Retriever, error) {
	options := &Options{
		Logger: logging.DefaultNoopLogger(),
		TopK:   10,
	}

	for _, opt := range opts {
		opt(options)
	}

	vStore := options.VectorStore
	if vStore == nil {
		workDir := "./data"
		vecPath := filepath.Join(workDir, "gorag_vectors.db")
		var err error
		vStore, err = govector.NewStore(
			govector.WithDBPath(vecPath),
			govector.WithDimension(1536),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to init fallback vector store: %w", err)
		}
	}

	return NewFusionRetriever(vStore, options.Embedder, options.LLM, options.TopK, options.Logger), nil
}

type decompositionStep struct {
	decomposer core.QueryDecomposer
	logger     logging.Logger
}

func (s *decompositionStep) Name() string { return "DecomposeWithFallback" }
func (s *decompositionStep) Execute(ctx context.Context, rctx *core.RetrievalContext) error {
	subQueries, err := s.decomposer.Decompose(ctx, rctx.Query)
	if err != nil {
		s.logger.Warn("query decomposition failed, falling back to original query", map[string]any{"error": err, "query": rctx.Query.Text})
		rctx.Agentic.SubQueries = []string{rctx.Query.Text}
		return nil
	}
	
	if subQueries == nil || len(subQueries.SubQueries) == 0 {
		rctx.Agentic.SubQueries = []string{rctx.Query.Text}
	} else {
		qs := subQueries.SubQueries
		const maxSubQueries = 5
		if len(qs) > maxSubQueries {
			qs = qs[:maxSubQueries]
		}
		rctx.Agentic.SubQueries = qs
	}
	return nil
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
		logger = logging.DefaultNoopLogger()
	}

	decomposer := query.NewDecomposer(llm, query.WithDecomposerLogger(logger))
	return NewFusionRetrieverWithComponents(vectorStore, embedder, decomposer, topK, logger)
}

// NewFusionRetrieverWithComponents allows injecting a specific decomposer for audit/testing.
func NewFusionRetrieverWithComponents(
	vectorStore core.VectorStore,
	embedder embedding.Provider,
	decomposer core.QueryDecomposer,
	topK int,
	logger logging.Logger,
) core.Retriever {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	
	p := pipeline.New[*core.RetrievalContext]()

	// Step 1: Decompose query into sub-queries with defensive fallback
	p.AddStep(&decompositionStep{decomposer: decomposer, logger: logger})

	// Step 2: Multi-query search with Concurrency Control (Semaphore)
	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
		TopK:        topK,
		Concurrency: 3,
	}))

	// Step 3: Reciprocal Rank Fusion (RRF)
	p.AddStep(fuse.RRF(fusion.NewRRFFusionEngine(), topK, logger))

	return &fusionRetriever{pipeline: p}
}

func (r *fusionRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
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
