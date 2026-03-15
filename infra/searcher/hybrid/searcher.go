// Package hybrid provides the Hybrid RAG searcher:
//
//	[QueryToFilterStep] → [StepBackStep] → [HyDEStep] →
//	VectorSearchStep + [SparseSearchStep] → RAGFusionStep → [RerankStep] → GenerationStep
package hybrid

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
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

// Searcher holds the components for the Hybrid RAG pipeline.
// The pipeline is assembled once at construction time and reused on every Search call.
type Searcher struct {
	embedder        embedding.Provider          // dense embedding model
	vectorStore     abstraction.VectorStore     // vector index for dense retrieval
	fusionEngine    retrieval.FusionEngine      // RRF fusion engine (default: built-in k=60)
	generator       retrieval.Generator         // LLM answer generator (required)
	sparseStore     steps.SparseSearcher        // optional BM25/keyword retrieval path
	reranker        abstraction.Reranker        // optional cross-encoder reranker after fusion
	filterExtractor *enhancer.FilterExtractor   // optional metadata filter extractor
	stepBackGen     *enhancer.StepBackGenerator // optional StepBack abstract query expander
	hydeGenerator   *enhancer.HyDEGenerator     // optional HyDE hypothetical document generator
	logger          logging.Logger              // structured logger
	metrics         abstraction.Metrics         // observability metrics collector
	denseTopK       int                         // dense retrieval candidate size (default: 20)
	sparseTopK      int                         // sparse retrieval candidate size (default: 20)
	fusionTopK      int                         // results output from RRF fusion (default: 10)
	rerankTopK      int                         // results kept after reranking (default: 5)

	pipe *pipeline.Pipeline[*entity.PipelineState] // pre-assembled, reused on every call
}

// Option is a functional option for the Hybrid Searcher.
type Option func(*Searcher)

// WithEmbedding sets the embedding provider.
func WithEmbedding(provider embedding.Provider) Option {
	return func(s *Searcher) { s.embedder = provider }
}

// WithVectorStore sets the vector store.
func WithVectorStore(store abstraction.VectorStore) Option {
	return func(s *Searcher) { s.vectorStore = store }
}

// WithFusionEngine sets the RRF fusion engine.
func WithFusionEngine(engine retrieval.FusionEngine) Option {
	return func(s *Searcher) { s.fusionEngine = engine }
}

// WithGenerator sets the LLM answer generator.
func WithGenerator(generator retrieval.Generator) Option {
	return func(s *Searcher) { s.generator = generator }
}

// WithSparseStore adds the BM25/keyword retrieval path (optional).
func WithSparseStore(ss steps.SparseSearcher) Option {
	return func(s *Searcher) { s.sparseStore = ss }
}

// WithReranker adds a cross-encoder rerank pass after fusion (optional).
func WithReranker(r abstraction.Reranker) Option {
	return func(s *Searcher) { s.reranker = r }
}

// WithFilterExtractor enables metadata filter extraction from the query (optional).
func WithFilterExtractor(extractor *enhancer.FilterExtractor) Option {
	return func(s *Searcher) { s.filterExtractor = extractor }
}

// WithStepBack enables StepBack prompting for abstract query expansion (optional).
func WithStepBack(gen *enhancer.StepBackGenerator) Option {
	return func(s *Searcher) { s.stepBackGen = gen }
}

// WithHyDE enables Hypothetical Document Embedding augmentation (optional).
func WithHyDE(gen *enhancer.HyDEGenerator) Option {
	return func(s *Searcher) { s.hydeGenerator = gen }
}

// WithDenseTopK sets the dense retrieval candidate size (default: 20).
func WithDenseTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.denseTopK = k
		}
	}
}

// WithSparseTopK sets the sparse retrieval candidate size (default: 20).
func WithSparseTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.sparseTopK = k
		}
	}
}

// WithFusionTopK sets the number of results output from RRF fusion (default: 10).
func WithFusionTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.fusionTopK = k
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

// New creates a pre-assembled Hybrid RAG searcher.
//
// Required: WithGenerator.
//
// Defaults (auto-configured when not provided):
//   - Embedder:     local BGE bge-small-zh-v1.5
//   - VectorStore:  local govector
//   - FusionEngine: built-in RRF (k=60)
//
// Example – minimal:
//
//	s := hybrid.New(hybrid.WithGenerator(gen))
func New(opts ...Option) *Searcher {
	s := &Searcher{
		denseTopK:  20,
		sparseTopK: 20,
		fusionTopK: 10,
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

// buildPipeline assembles the Hybrid RAG pipeline once at construction time.
// It panics if generator is not set, and fills in default embedder, vectorStore,
// and fusionEngine when callers have not provided them explicitly.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	if s.generator == nil {
		panic("hybrid.Searcher: generator is required")
	}
	if s.embedder == nil {
		s.embedder = core.DefaultEmbedder()
	}
	if s.vectorStore == nil {
		s.vectorStore = core.DefaultVectorStore()
	}
	if s.fusionEngine == nil {
		s.fusionEngine = core.DefaultFusionEngine()
	}

	p := pipeline.New[*entity.PipelineState]()

	if s.filterExtractor != nil {
		p.AddStep(steps.NewQueryToFilterStep(s.filterExtractor))
	}
	if s.stepBackGen != nil {
		p.AddStep(steps.NewStepBackStep(s.stepBackGen))
	}
	if s.hydeGenerator != nil {
		p.AddStep(steps.NewHyDEStep(s.hydeGenerator))
	}

	p.AddStep(steps.NewVectorSearchStep(s.embedder, s.vectorStore, s.denseTopK))
	if s.sparseStore != nil {
		p.AddStep(steps.NewSparseSearchStep(s.sparseStore, s.sparseTopK))
	}

	p.AddStep(chunksToParallelResultsStep{})
	p.AddStep(steps.NewRAGFusionStep(s.fusionEngine, s.fusionTopK))

	if s.reranker != nil {
		p.AddStep(steps.NewRerankStep(s.reranker, s.rerankTopK))
	}

	p.AddStep(steps.NewGenerator(s.generator, s.logger))
	return p
}

// Search executes the pre-built Hybrid RAG pipeline and returns the generated answer.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("hybrid", err)
		return "", fmt.Errorf("hybrid.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("hybrid", time.Since(start))
	s.metrics.RecordSearchResult("hybrid", chunkCount)
	return state.Answer, nil
}
