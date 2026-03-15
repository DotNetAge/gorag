// Package rerank provides the Retrieve-and-Rerank RAG searcher:
//
//	[QueryRewriteStep] → VectorSearchStep → RerankStep → GenerationStep
package rerank

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/infra/service"
	"github.com/DotNetAge/gorag/infra/steps"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// Searcher holds the components for the Retrieve-and-Rerank pipeline.
// The pipeline is assembled once at construction time and reused on every Search call.
type Searcher struct {
	embedder      embedding.Provider            // dense embedding model
	vectorStore   abstraction.VectorStore       // vector index for initial recall
	reranker      abstraction.Reranker          // cross-encoder reranker (required)
	generator     retrieval.Generator           // LLM answer generator (required)
	queryRewriter retrieval.QueryRewriter       // optional query rewriter
	cacheService  *service.SemanticCacheService // optional semantic cache
	logger        logging.Logger                // structured logger
	metrics       abstraction.Metrics           // observability metrics collector
	recallTopK    int                           // initial candidate recall size (default: 20)
	rerankTopK    int                           // results kept after reranking (default: 5)

	pipe *pipeline.Pipeline[*entity.PipelineState] // pre-assembled, reused on every call
}

// Option is a functional option for the Rerank Searcher.
type Option func(*Searcher)

// WithEmbedding sets the embedding provider.
func WithEmbedding(provider embedding.Provider) Option {
	return func(s *Searcher) { s.embedder = provider }
}

// WithVectorStore sets the vector store.
func WithVectorStore(store abstraction.VectorStore) Option {
	return func(s *Searcher) { s.vectorStore = store }
}

// WithReranker sets the cross-encoder reranker.
func WithReranker(r abstraction.Reranker) Option {
	return func(s *Searcher) { s.reranker = r }
}

// WithGenerator sets the LLM answer generator.
func WithGenerator(generator retrieval.Generator) Option {
	return func(s *Searcher) { s.generator = generator }
}

// WithQueryRewriter enables query rewriting before retrieval (optional).
func WithQueryRewriter(rewriter retrieval.QueryRewriter) Option {
	return func(s *Searcher) { s.queryRewriter = rewriter }
}

// WithSemanticCache enables semantic caching (optional).
func WithSemanticCache(svc *service.SemanticCacheService) Option {
	return func(s *Searcher) { s.cacheService = svc }
}

// WithRecallTopK sets the initial candidate recall size before reranking (default: 20).
func WithRecallTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.recallTopK = k
		}
	}
}

// WithRerankTopK sets the number of results kept after reranking (default: 5).
func WithRerankTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.rerankTopK = k
		}
	}
}

// WithLogger sets the logger.
func WithLogger(logger logging.Logger) Option {
	return func(s *Searcher) { s.logger = logger }
}

// WithMetrics sets the metrics collector.
func WithMetrics(m abstraction.Metrics) Option {
	return func(s *Searcher) { s.metrics = m }
}

// New creates a pre-assembled Retrieve-and-Rerank searcher.
//
// Pipeline: [QueryRewriteStep] → VectorSearchStep → RerankStep → GenerationStep
//
// Required: WithReranker, WithGenerator.
//
// Defaults (auto-configured when not provided):
//   - Embedder:    local BGE bge-small-zh-v1.5 (no API key needed)
//   - VectorStore: local govector at ./data/searcher/govector
//
// Example – minimal:
//
//	s := rerank.New(rerank.WithReranker(crossEncoder), rerank.WithGenerator(gen))
func New(opts ...Option) *Searcher {
	s := &Searcher{
		recallTopK: 20,
		rerankTopK: 5,
		logger:     logging.NewNoopLogger(),
		metrics:    core.DefaultMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.pipe = s.buildPipeline()
	return s
}

// buildPipeline assembles the Retrieve-and-Rerank pipeline once at construction time.
// It panics if reranker or generator is not set, and fills in default embedder and
// vectorStore when callers have not provided them explicitly.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	if s.reranker == nil {
		panic("rerank.Searcher: reranker is required")
	}
	if s.generator == nil {
		panic("rerank.Searcher: generator is required")
	}
	if s.embedder == nil {
		s.embedder = core.DefaultEmbedder()
	}
	if s.vectorStore == nil {
		s.vectorStore = core.DefaultVectorStore()
	}

	p := pipeline.New[*entity.PipelineState]()

	if s.cacheService != nil {
		p.AddStep(steps.NewSemanticCacheChecker(s.cacheService, s.logger))
	}
	if s.queryRewriter != nil {
		p.AddStep(steps.NewQueryRewriteStep(s.queryRewriter))
	}

	p.AddStep(steps.NewVectorSearchStep(s.embedder, s.vectorStore, s.recallTopK))
	p.AddStep(steps.NewRerankStep(s.reranker, s.rerankTopK))
	p.AddStep(steps.NewGenerator(s.generator, s.logger))

	if s.cacheService != nil {
		p.AddStep(steps.NewCacheResponseWriter(s.cacheService, s.logger))
	}

	return p
}

// Search executes the pre-built Retrieve-and-Rerank pipeline and returns the generated answer.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("rerank", err)
		return "", fmt.Errorf("rerank.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("rerank", time.Since(start))
	s.metrics.RecordSearchResult("rerank", chunkCount)
	return state.Answer, nil
}
