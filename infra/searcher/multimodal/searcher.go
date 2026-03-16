// Package multimodal provides the Multimodal RAG searcher:
//
//	[QueryRewriteStep] → MultimodalEmbeddingStep →
//	VectorSearchStep + [SparseSearchStep] + [ImageSearchStep] →
//	chunksToParallelResultsStep → RAGFusionStep → [RerankStep] → GenerationStep
//
// The three retrieval branches are:
//   - Branch A (text vector): VectorSearchStep — always active
//   - Branch B (keyword):     SparseSearchStep — optional, enabled via WithSparseStore
//   - Branch C (image vector): ImageSearchStep — optional, active only when image_data
//     is present in state.Agentic.Custom at query time
//
// At least one of WithSparseStore or image_data at query time must complement the
// always-active text vector branch.
package multimodal

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/infra/steps"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// chunksToParallelResultsStep moves all current RetrievedChunks into the
// ParallelResults accumulator and clears RetrievedChunks, so that subsequent
// search steps produce a second independent result set that RAGFusionStep can fuse.
type chunksToParallelResultsStep struct{}

func (chunksToParallelResultsStep) Name() string { return "ChunksToParallelResults" }
func (chunksToParallelResultsStep) Execute(_ context.Context, state *entity.PipelineState) error {
	if len(state.RetrievedChunks) > 0 {
		state.ParallelResults = append(state.ParallelResults, state.RetrievedChunks...)
		state.RetrievedChunks = nil
	}
	return nil
}

// Searcher holds the components for the Multimodal RAG pipeline.
// It supports up to three retrieval branches (text vector, keyword/BM25, image vector),
// fuses results via RRF, and optionally re-ranks before answer generation.
// The pipeline is assembled once at construction time and reused on every Search call.
type Searcher struct {
	multimodalEmbedder abstraction.MultimodalEmbedder // CLIP/VLM embedder for text + image (required)
	vectorStore        abstraction.VectorStore        // vector index for text and image retrieval (required)
	sparseStore        steps.SparseSearcher           // optional BM25/keyword searcher (Branch B)
	fusionEngine       retrieval.FusionEngine         // RRF fusion engine (default: built-in k=60)
	generator          retrieval.Generator            // LLM answer generator (required)
	queryRewriter      retrieval.QueryRewriter        // optional query rewriter
	reranker           abstraction.Reranker           // optional cross-encoder reranker after fusion
	logger             logging.Logger                 // structured logger
	metrics            abstraction.Metrics            // observability metrics collector
	denseTopK          int                            // text vector retrieval candidate size (default: 15)
	sparseTopK         int                            // sparse retrieval candidate size (default: 15)
	imageTopK          int                            // image vector retrieval candidate size (default: 10)
	rerankTopK         int                            // results kept after reranking (default: 5)

	pipe *pipeline.Pipeline[*entity.PipelineState] // pre-assembled, reused on every call
}

// Option is a functional option for the Multimodal Searcher.
type Option func(*Searcher)

// WithMultimodalEmbedder sets the multimodal embedder (required).
// The embedder must project both text and image inputs into a shared vector space.
func WithMultimodalEmbedder(embedder abstraction.MultimodalEmbedder) Option {
	return func(s *Searcher) { s.multimodalEmbedder = embedder }
}

// WithVectorStore sets the vector store used for both text and image retrieval.
func WithVectorStore(store abstraction.VectorStore) Option {
	return func(s *Searcher) { s.vectorStore = store }
}

// WithSparseStore adds the BM25/keyword retrieval branch (optional).
func WithSparseStore(ss steps.SparseSearcher) Option {
	return func(s *Searcher) { s.sparseStore = ss }
}

// WithFusionEngine sets the RRF fusion engine.
func WithFusionEngine(engine retrieval.FusionEngine) Option {
	return func(s *Searcher) { s.fusionEngine = engine }
}

// WithGenerator sets the LLM answer generator.
func WithGenerator(generator retrieval.Generator) Option {
	return func(s *Searcher) { s.generator = generator }
}

// WithQueryRewriter enables query rewriting before retrieval (optional).
func WithQueryRewriter(rewriter retrieval.QueryRewriter) Option {
	return func(s *Searcher) { s.queryRewriter = rewriter }
}

// WithReranker adds a cross-encoder rerank pass after fusion (optional).
func WithReranker(r abstraction.Reranker) Option {
	return func(s *Searcher) { s.reranker = r }
}

// WithDenseTopK sets the text vector retrieval candidate size (default: 15).
func WithDenseTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.denseTopK = k
		}
	}
}

// WithSparseTopK sets the sparse retrieval candidate size (default: 15).
func WithSparseTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.sparseTopK = k
		}
	}
}

// WithImageTopK sets the image vector retrieval candidate size (default: 10).
func WithImageTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.imageTopK = k
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

// New creates a pre-assembled Multimodal RAG searcher.
//
// Pipeline:
//
//	[QueryRewriteStep] → MultimodalEmbeddingStep →
//	VectorSearchStep + [SparseSearchStep] + [ImageSearchStep] →
//	chunksToParallelResultsStep → RAGFusionStep → [RerankStep] → GenerationStep
//
// Required: WithMultimodalEmbedder, WithGenerator.
//
// Defaults (auto-configured when not provided):
//   - VectorStore:  local govector
//   - FusionEngine: built-in RRF (k=60)
//
// Image retrieval (Branch C) is automatically activated when the caller sets
// state.Agentic.Custom["image_data"] = []byte{...} before calling Search.
func New(opts ...Option) *Searcher {
	s := &Searcher{
		denseTopK:  15,
		sparseTopK: 15,
		imageTopK:  10,
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

// buildPipeline assembles the Multimodal RAG pipeline once at construction time.
// Panics if multimodalEmbedder or generator is not set.
// Fills in default vectorStore and fusionEngine when not explicitly provided.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	if s.multimodalEmbedder == nil {
		panic("multimodal.Searcher: multimodalEmbedder is required")
	}
	if s.generator == nil {
		panic("multimodal.Searcher: generator is required")
	}
	if s.vectorStore == nil {
		store, err := core.DefaultVectorStore()
		if err != nil {
			panic(err)
		}
		s.vectorStore = store
	}
	if s.fusionEngine == nil {
		s.fusionEngine = core.DefaultFusionEngine()
	}

	p := pipeline.New[*entity.PipelineState]()

	if s.queryRewriter != nil {
		p.AddStep(steps.NewQueryRewriteStep(s.queryRewriter))
	}

	// MultimodalEmbeddingStep replaces the plain VectorSearchStep's embed call.
	// It writes query_vector (and optionally image_vector) to state.Agentic.Custom.
	p.AddStep(steps.NewMultimodalEmbeddingStep(s.multimodalEmbedder, s.logger))

	// Branch A: text vector search (uses query_vector via VectorSearchStep)
	p.AddStep(steps.NewVectorSearchStep(nil, s.vectorStore, s.denseTopK))

	// Branch B: sparse/keyword search (optional)
	if s.sparseStore != nil {
		p.AddStep(steps.NewSparseSearchStep(s.sparseStore, s.sparseTopK, s.logger))
	}

	// Branch C: image vector search (no-op when image_vector absent)
	p.AddStep(steps.NewImageSearchStep(s.vectorStore, s.imageTopK, s.logger))

	p.AddStep(chunksToParallelResultsStep{})
	p.AddStep(steps.NewRAGFusionStep(s.fusionEngine, s.denseTopK))

	if s.reranker != nil {
		p.AddStep(steps.NewRerankStep(s.reranker, s.rerankTopK))
	}

	p.AddStep(steps.NewGenerator(s.generator, s.logger))
	return p
}

// Search executes the pre-built Multimodal RAG pipeline and returns the generated answer.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)
	state.Agentic = entity.NewAgenticMetadata()

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("multimodal", err)
		return "", fmt.Errorf("multimodal.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("multimodal", time.Since(start))
	s.metrics.RecordSearchResult("multimodal", chunkCount)
	return state.Answer, nil
}
