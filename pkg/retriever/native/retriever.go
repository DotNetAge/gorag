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
	stepgen "github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
	"github.com/DotNetAge/gorag/pkg/store/vector/govector"
)

type nativeRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
	tracer   observability.Tracer
}

// Options for native retriever
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

	return NewRetrieverWithOptions(vStore, options.Embedder, options.LLM, options.TopK, *options), nil
}

// DefaultRetriever is an alias for DefaultNativeRetriever.
func DefaultRetriever(opts ...Option) (core.Retriever, error) {
	return DefaultNativeRetriever(opts...)
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
		Tracer: observability.DefaultNoopTracer(),
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
		opts.Logger = logging.DefaultNoopLogger()
	}
	if opts.Tracer == nil {
		opts.Tracer = observability.DefaultNoopTracer()
	}

	p := pipeline.New[*core.RetrievalContext]()
	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{TopK: topK}))
	gen := answer.New(llm, answer.WithLogger(opts.Logger))
	p.AddStep(stepgen.Generate(gen, opts.Logger, nil))

	return &nativeRetriever{pipeline: p, tracer: opts.Tracer}
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
