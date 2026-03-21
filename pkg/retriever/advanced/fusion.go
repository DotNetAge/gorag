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
// It implements RAG-Fusion (Query Decomposition + RRF Fusion) for high-accuracy recall.
func DefaultAdvancedRetriever(opts ...Option) (core.Retriever, error) {
	options := &Options{
		Logger: logging.DefaultNoopLogger(),
		TopK:   10,
	}

	for _, opt := range opts {
		opt(options)
	}

	// Local Fallback for easy start if no enterprise store provided
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

	p := pipeline.New[*core.RetrievalContext]()
	gen := answer.New(llm, answer.WithLogger(logger))
	decomposer := query.NewDecomposer(llm, query.WithDecomposerLogger(logger))
	fusionEngine := fusion.NewRRFFusionEngine()

	// Step 1: Decompose query into sub-queries
	p.AddStep(decompose.Decompose(decomposer, logger))

	// Step 2: Multi-query search (implicitly handled by vector.Search)
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
