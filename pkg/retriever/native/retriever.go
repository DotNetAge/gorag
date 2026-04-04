package native

import (
	"context"
	"fmt"
	"path/filepath"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/retrieval/answer"
	"github.com/DotNetAge/gorag/pkg/retrieval/enhancement"
	"github.com/DotNetAge/gorag/pkg/retrieval/expand"
	"github.com/DotNetAge/gorag/pkg/retrieval/fusion"
	"github.com/DotNetAge/gorag/pkg/retrieval/query"
	"github.com/DotNetAge/gorag/pkg/retrieval/rerank"
	stepcache "github.com/DotNetAge/gorag/pkg/steps/cache"
	stepfuse "github.com/DotNetAge/gorag/pkg/steps/fuse"
	stepgen "github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/hyde"
	stepprune "github.com/DotNetAge/gorag/pkg/steps/prune"
	steprerank "github.com/DotNetAge/gorag/pkg/steps/rerank"
	"github.com/DotNetAge/gorag/pkg/steps/rewrite"
	"github.com/DotNetAge/gorag/pkg/steps/stepback"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
	"github.com/DotNetAge/gorag/pkg/store/vector/govector"
)

type nativeRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
	tracer   observability.Tracer
}

// Options for native retriever - users can combine freely
type Options struct {
	Name        string
	Logger      logging.Logger
	Tracer      observability.Tracer
	Embedder    embedding.Provider
	LLM         chat.Client
	TopK        int
	WorkDir     string
	VectorStore core.VectorStore
	DocStore    core.DocStore

	// Pre-Retrieval enhancements (can be combined)
	EnableQueryRewrite bool
	EnableStepBack     bool
	EnableHyDE         bool
	EnableFusion       bool // Fusion: Pre-Retrieval (Decompose) + Post-Retrieval (RRF)
	FusionCount        int

	// Post-Retrieval enhancements (can be combined)
	EnableParentDoc     bool
	ParentDocExpander   core.ResultEnhancer // Required for ParentDoc
	EnableSentenceWindow bool
	SentenceExpander    core.ResultEnhancer // Required for SentenceWindow
	EnablePrune         bool
	ContextPruner       core.ResultEnhancer // Required for Prune
	EnableRerank        bool

	// Cache (works across all phases)
	EnableCache bool
	SemanticCache core.SemanticCache // Required for caching
}

type Option func(*Options)

func WithName(name string) Option               { return func(o *Options) { o.Name = name } }
func WithVectorStore(s core.VectorStore) Option { return func(o *Options) { o.VectorStore = s } }
func WithDocStore(s core.DocStore) Option       { return func(o *Options) { o.DocStore = s } }
func WithWorkDir(dir string) Option             { return func(o *Options) { o.WorkDir = dir } }
func WithLogger(l logging.Logger) Option        { return func(o *Options) { o.Logger = l } }
func WithTracer(t observability.Tracer) Option  { return func(o *Options) { o.Tracer = t } }
func WithEmbedder(e embedding.Provider) Option  { return func(o *Options) { o.Embedder = e } }
func WithLLM(l chat.Client) Option              { return func(o *Options) { o.LLM = l } }
func WithTopK(k int) Option                     { return func(o *Options) { o.TopK = k } }

// Enhancement options - users can combine freely, Retriever handles ordering

// Pre-Retrieval enhancements
func WithQueryRewrite() Option    { return func(o *Options) { o.EnableQueryRewrite = true } }
func WithStepBack() Option        { return func(o *Options) { o.EnableStepBack = true } }
func WithHyDE() Option            { return func(o *Options) { o.EnableHyDE = true } }
func WithFusion(count int) Option { return func(o *Options) { o.EnableFusion = true; o.FusionCount = count } }

// Post-Retrieval enhancements
func WithParentDoc(expander core.ResultEnhancer) Option {
	return func(o *Options) { o.EnableParentDoc = true; o.ParentDocExpander = expander }
}
func WithSentenceWindow(expander core.ResultEnhancer) Option {
	return func(o *Options) { o.EnableSentenceWindow = true; o.SentenceExpander = expander }
}
func WithContextPrune(pruner core.ResultEnhancer) Option {
	return func(o *Options) { o.EnablePrune = true; o.ContextPruner = pruner }
}
func WithRerank() Option { return func(o *Options) { o.EnableRerank = true } }

// Cache (works across all phases)
func WithCache(cache core.SemanticCache) Option {
	return func(o *Options) { o.EnableCache = true; o.SemanticCache = cache }
}

// DefaultNativeRetriever creates an out-of-the-box Native Retriever.
func DefaultNativeRetriever(opts ...Option) (core.Retriever, error) {
	options := &Options{
		Logger:  logging.DefaultNoopLogger(),
		Tracer:  observability.DefaultNoopTracer(),
		TopK:    5,
		WorkDir: "./data",
	}

	for _, opt := range opts {
		opt(options)
	}

	vStore := options.VectorStore
	if vStore == nil {
		vecName := "gorag_vectors.db"
		if options.Name != "" {
			vecName = fmt.Sprintf("gorag_vectors_%s.db", options.Name)
		}
		vecPath := filepath.Join(options.WorkDir, vecName)
		dimension := 1536
		if options.Embedder != nil {
			dimension = options.Embedder.Dimension()
		}

		colName := "gorag"
		if options.Name != "" {
			colName = options.Name
		}

		var err error
		vStore, err = govector.NewStore(
			govector.WithDBPath(vecPath),
			govector.WithDimension(dimension),
			govector.WithCollection(colName),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to init default vector store: %w", err)
		}
	}

	return NewRetriever(vStore, options.Embedder, options.LLM, options.TopK, opts...), nil
}

// DefaultRetriever is an alias for DefaultNativeRetriever.
func DefaultRetriever(opts ...Option) (core.Retriever, error) {
	return DefaultNativeRetriever(opts...)
}

// NewRetriever creates a Native Retriever with options.
// Pipeline is automatically assembled in three phases: Pre-Retrieval → Retrieval → Post-Retrieval
// Users don't need to know the order - Retriever handles it internally.
func NewRetriever(
	vectorStore core.VectorStore,
	embedder embedding.Provider,
	llm chat.Client,
	topK int,
	opts ...Option,
) core.Retriever {
	options := &Options{
		Logger: logging.DefaultNoopLogger(),
		Tracer: observability.DefaultNoopTracer(),
		TopK:   topK,
	}

	for _, opt := range opts {
		opt(options)
	}

	if options.Logger == nil {
		options.Logger = logging.DefaultNoopLogger()
	}
	if options.Tracer == nil {
		options.Tracer = observability.DefaultNoopTracer()
	}

	p := pipeline.New[*core.RetrievalContext]()

	// ========================================================================
	// PHASE 0: Cache Check (if enabled)
	// ========================================================================
	if options.EnableCache && options.SemanticCache != nil {
		p.AddStep(stepcache.Check(options.SemanticCache, options.Logger, nil))
	}

	// ========================================================================
	// PHASE 1: Pre-Retrieval (Query Enhancement)
	// Order: QueryRewrite → HyDE → StepBack → Decompose (for Fusion)
	// ========================================================================
	if options.EnableQueryRewrite {
		p.AddStep(rewrite.Rewrite(llm, options.Logger, nil))
	}

	if options.EnableHyDE {
		gen := answer.New(llm, answer.WithLogger(options.Logger))
		p.AddStep(hyde.Generate(gen, options.Logger))
	}

	if options.EnableStepBack {
		decomposer := query.NewDecomposer(llm, query.WithDecomposerLogger(options.Logger))
		p.AddStep(stepback.Generate(decomposer))
	}

	if options.EnableFusion {
		// Fusion requires query decomposition
		decomposer := query.NewDecomposer(llm, query.WithDecomposerLogger(options.Logger))
		p.AddStep(&decompositionStep{decomposer: decomposer, logger: options.Logger})
	}

	// ========================================================================
	// PHASE 2: Retrieval (Vector Search)
	// ========================================================================
	searchOpts := vector.SearchOptions{TopK: options.TopK}
	if options.EnableFusion {
		// Fusion needs multi-query search with concurrency
		searchOpts.Concurrency = 3
	}
	p.AddStep(vector.Search(vectorStore, embedder, searchOpts))

	// ========================================================================
	// PHASE 3: Post-Retrieval (Result Enhancement)
	// Order: RRF → ParentDoc → SentenceWindow → Prune → Rerank → Generate
	// ========================================================================
	if options.EnableFusion {
		// Reciprocal Rank Fusion for multi-query results
		p.AddStep(stepfuse.RRF(fusion.NewRRFFusionEngine(), options.TopK, options.Logger))
	}

	// ParentDoc expansion - requires DocStore
	if options.EnableParentDoc {
		expander := options.ParentDocExpander
		if expander == nil && options.DocStore != nil {
			expander = expand.NewParentDoc(options.DocStore)
		}
		if expander != nil {
			p.AddStep(steprerank.ParentDocExpand(expander, options.Logger, nil))
		}
	}

	// SentenceWindow expansion - no dependencies
	if options.EnableSentenceWindow {
		expander := options.SentenceExpander
		if expander == nil {
			expander = enhancement.NewSentenceWindowExpander()
		}
		p.AddStep(steprerank.SentenceWindowExpand(expander, options.Logger, nil))
	}

	// Context pruning - requires LLM
	if options.EnablePrune && llm != nil {
		pruner := options.ContextPruner
		if pruner == nil {
			pruner = enhancement.NewContextPruner(llm)
		}
		p.AddStep(stepprune.Prune(pruner, options.Logger, nil))
	}

	if options.EnableRerank && llm != nil {
		// Create cross-encoder reranker using LLM
		reranker := rerank.NewCrossEncoder(llm, rerank.WithRerankTopK(options.TopK))
		p.AddStep(steprerank.Score(reranker, options.TopK, options.Logger, nil))
	}

	// Generation (always last if LLM is provided)
	if llm != nil {
		gen := answer.New(llm, answer.WithLogger(options.Logger))
		p.AddStep(stepgen.Generate(gen, options.Logger, nil))
	}

	// ========================================================================
	// PHASE 4: Cache Store (if enabled)
	// ========================================================================
	if options.EnableCache && options.SemanticCache != nil {
		p.AddStep(stepcache.Store(options.SemanticCache, options.Logger, nil))
	}

	return &nativeRetriever{pipeline: p, tracer: options.Tracer}
}

// decompositionStep handles query decomposition with fallback
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

func (r *nativeRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))
	for _, q := range queries {
		retrievalCtx := core.NewRetrievalContext(ctx, q)
		retrievalCtx.Tracer = r.tracer
		retrievalCtx.Ctx, retrievalCtx.Span = r.tracer.StartSpan(retrievalCtx.Ctx, "NativeRAG.Retrieve")
		if err := r.pipeline.Execute(retrievalCtx.Ctx, retrievalCtx); err != nil {
			retrievalCtx.Span.End()
			return nil, err
		}
		retrievalCtx.Span.End()
		var allChunks []*core.Chunk
		for _, group := range retrievalCtx.RetrievedChunks {
			allChunks = append(allChunks, group...)
		}
		results = append(results, &core.RetrievalResult{Query: q, Chunks: allChunks, Answer: retrievalCtx.Answer.Answer})
	}
	return results, nil
}
