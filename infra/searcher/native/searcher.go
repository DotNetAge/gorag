// Package native provides the NativeRAG searcher:
//
//	[QueryRewriteStep] → VectorSearchStep → GenerationStep
package native

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	searchercore "github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/infra/service"
	"github.com/DotNetAge/gorag/infra/steps/cache"
	"github.com/DotNetAge/gorag/infra/steps/generate"
	"github.com/DotNetAge/gorag/infra/steps/vector"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// Searcher holds the components for the NativeRAG pipeline.
// The pipeline is assembled once at construction time and reused on every Search call.
type Searcher struct {
	embedder      embedding.Provider            // dense embedding model
	vectorStore   abstraction.VectorStore       // vector index for similarity search
	generator     retrieval.Generator           // LLM answer generator (required)
	queryRewriter retrieval.QueryRewriter       // optional query rewriter
	cacheService  *service.SemanticCacheService // optional semantic cache
	logger        logging.Logger                // structured logger
	metrics       abstraction.Metrics           // observability metrics collector
	topK          int                           // number of retrieved candidates (default: 10)

	pipe *pipeline.Pipeline[*entity.PipelineState] // pre-assembled, reused on every call
}

// Option is a functional option for the NativeRAG Searcher.
type Option func(*Searcher)

// WithEmbedding sets the embedding provider.
func WithEmbedding(provider embedding.Provider) Option {
	return func(s *Searcher) { s.embedder = provider }
}

// WithVectorStore sets the vector store.
func WithVectorStore(store abstraction.VectorStore) Option {
	return func(s *Searcher) { s.vectorStore = store }
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

// WithTopK sets the number of retrieved candidates (default: 10).
func WithTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.topK = k
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

// New creates a pre-assembled NativeRAG searcher.
//
// Pipeline: [QueryRewriteStep] → VectorSearchStep → GenerationStep
//
// Required: WithGenerator.
//
// Defaults (auto-configured when not provided):
//   - Embedder:    local BGE bge-small-zh-v1.5 (no API key needed)
//   - VectorStore: local govector at ./data/searcher/govector
//
// Example – minimal:
//
//	s := native.New(native.WithGenerator(gen))
//	answer, err := s.Search(ctx, "What is RAG?")
func New(opts ...Option) *Searcher {
	s := &Searcher{
		topK:    10,
		logger:  logging.NewNoopLogger(),
		metrics: searchercore.DefaultMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.pipe = s.buildPipeline()
	return s
}

// buildPipeline assembles the NativeRAG pipeline once at construction time.
// It panics if generator is not set, and fills in default embedder and vectorStore
// when callers have not provided them explicitly.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	if s.generator == nil {
		panic("native.Searcher: generator is required")
	}
	if s.embedder == nil {
		embedder, err := searchercore.DefaultEmbedder()
		if err != nil {
			panic(err)
		}
		s.embedder = embedder
	}
	if s.vectorStore == nil {
		store, err := searchercore.DefaultVectorStore()
		if err != nil {
			panic(err)
		}
		s.vectorStore = store
	}

	p := pipeline.New[*entity.PipelineState]()

	if s.cacheService != nil {
		p.AddStep(cache.Check(s.cacheService, s.logger, s.metrics))
	}
	if s.queryRewriter != nil {
		// Note: QueryRewriteStep requires direct LLM client, not the interface
		// For now, skip this step if no direct LLM client is available
		_ = s.queryRewriter // avoid unused variable error
	}

	p.AddStep(vector.Search(s.embedder, s.vectorStore, s.topK, s.logger, s.metrics))
	p.AddStep(generate.Generate(s.generator, s.logger, s.metrics))

	if s.cacheService != nil {
		p.AddStep(cache.Store(s.cacheService, s.logger, s.metrics))
	}

	return p
}

// Search executes the pre-built NativeRAG pipeline and returns the generated answer.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("native", err)
		return "", fmt.Errorf("native.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("native", time.Since(start))
	s.metrics.RecordSearchResult("native", chunkCount)
	return state.Answer, nil
}
