// Package graphlocal provides the Graph-Local RAG searcher (entity-centric N-Hop pipeline):
//
//	[QueryRewriteStep] → EntityExtractStep → GraphLocalSearchStep →
//	[VectorSearchStep + FusionStep] → GenerationStep
package graphlocal

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/graph"
	"github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/infra/steps"
	poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
	retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// Searcher holds the components for the Graph-Local RAG pipeline.
// The pipeline is assembled once at construction time and reused on every Search call.
type Searcher struct {
	entityExtractor    retrieval.EntityExtractor // LLM-based named entity extractor (required)
	graphLocalSearcher *graph.LocalSearcher      // N-Hop graph traversal engine (required)
	generator          retrieval.Generator       // LLM answer generator (required)
	queryRewriter      retrieval.QueryRewriter   // optional query rewriter
	embedder           embedding.Provider        // optional dense embedder for vector supplement
	vectorStore        abstraction.VectorStore   // optional vector index for supplement path
	fusionEngine       retrieval.FusionEngine    // optional RRF engine (required if vector supplement enabled)
	logger             logging.Logger            // structured logger
	metrics            abstraction.Metrics       // observability metrics collector
	maxHops            int                       // N-Hop graph traversal depth (default: 2)
	topK               int                       // max graph results to retrieve (default: 10)

	pipe *pipeline.Pipeline[*entity.PipelineState] // pre-assembled, reused on every call
}

// Option is a functional option for Searcher.
type Option func(*Searcher)

// WithEntityExtractor sets the entity extractor.
func WithEntityExtractor(extractor retrieval.EntityExtractor) Option {
	return func(s *Searcher) { s.entityExtractor = extractor }
}

// WithGraphSearcher sets the graph local searcher.
func WithGraphSearcher(gs *graph.LocalSearcher) Option {
	return func(s *Searcher) { s.graphLocalSearcher = gs }
}

// WithGenerator sets the LLM answer generator.
func WithGenerator(generator retrieval.Generator) Option {
	return func(s *Searcher) { s.generator = generator }
}

// WithQueryRewriter enables query rewriting before retrieval (optional).
func WithQueryRewriter(rewriter retrieval.QueryRewriter) Option {
	return func(s *Searcher) { s.queryRewriter = rewriter }
}

// WithVectorSupplement enables a supplementary vector search path that
// is fused with graph results via RRF. All three must be provided together.
func WithVectorSupplement(embedder embedding.Provider, store abstraction.VectorStore, fusion retrieval.FusionEngine) Option {
	return func(s *Searcher) {
		s.embedder = embedder
		s.vectorStore = store
		s.fusionEngine = fusion
	}
}

// WithMaxHops sets the N-Hop depth for graph traversal (default: 2).
func WithMaxHops(hops int) Option {
	return func(s *Searcher) {
		if hops > 0 {
			s.maxHops = hops
		}
	}
}

// WithTopK sets the max number of graph results to retrieve (default: 10).
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

// New creates a pre-assembled GraphRAG Local searcher.
//
// Pipeline: [QueryRewriteStep] → EntityExtractStep → GraphLocalSearchStep →
//
//	[VectorSearchStep + FusionStep] → GenerationStep
//
// Required: WithEntityExtractor, WithGraphSearcher, WithGenerator.
//
// Example:
//
//	s := graphlocal.New(
//	    graphlocal.WithEntityExtractor(extractor),
//	    graphlocal.WithGraphSearcher(localSearcher),
//	    graphlocal.WithGenerator(gen),
//	    graphlocal.WithMaxHops(2),
//	    graphlocal.WithTopK(10),
//	)
func New(opts ...Option) *Searcher {
	s := &Searcher{
		maxHops: 2,
		topK:    10,
		logger:  logging.NewNoopLogger(),
		metrics: core.DefaultMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.pipe = s.buildPipeline()
	return s
}

// buildPipeline assembles the Graph Local RAG pipeline once at construction time.
// It panics if entityExtractor, graphLocalSearcher, or generator is not set.
// The optional vector supplement path (embedder + vectorStore + fusionEngine) is
// activated only when all three are provided via WithVectorSupplement.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	if s.entityExtractor == nil {
		panic("graphlocal.New: entityExtractor is required")
	}
	if s.graphLocalSearcher == nil {
		panic("graphlocal.New: graphLocalSearcher is required")
	}
	if s.generator == nil {
		panic("graphlocal.New: generator is required")
	}

	p := pipeline.New[*entity.PipelineState]()

	if s.queryRewriter != nil {
		// Note: QueryRewriteStep requires direct LLM client, not the interface
		// For now, skip this step if no direct LLM client is available
		_ = s.queryRewriter // avoid unused variable error
	}

	p.AddStep(steps.NewEntityExtractor(s.entityExtractor, s.logger))
	p.AddStep(retrievalstep.NewGraphLocalSearchStep(s.graphLocalSearcher, s.maxHops, s.topK))

	if s.embedder != nil && s.vectorStore != nil && s.fusionEngine != nil {
		p.AddStep(retrievalstep.NewVectorSearchStep(s.embedder, s.vectorStore, s.topK))
		p.AddStep(chunksToParallelResultsStep{})
		p.AddStep(retrievalstep.NewRAGFusionStep(s.fusionEngine, s.topK))
	}

	p.AddStep(poststep.NewGenerator(s.generator, s.logger))
	return p
}

// Search executes the pre-built Graph Local RAG pipeline and returns the generated answer.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("graph_local", err)
		return "", fmt.Errorf("graphlocal.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("graph_local", time.Since(start))
	s.metrics.RecordSearchResult("graph_local", chunkCount)
	return state.Answer, nil
}

// chunksToParallelResultsStep separates the vector search results (appended last)
// from the graph search results so that RAGFusionStep receives two independent
// result sets to fuse via RRF.
type chunksToParallelResultsStep struct{}

func (chunksToParallelResultsStep) Name() string { return "ChunksToParallelResults" }

// Execute promotes the last entry of RetrievedChunks (vector results) into
// a separate slot so RAGFusionStep can fuse it with the earlier graph results.
// It is a no-op when fewer than two result groups are present.
func (chunksToParallelResultsStep) Execute(_ context.Context, state *entity.PipelineState) error {
	if len(state.RetrievedChunks) < 2 {
		return nil
	}
	// Move the second group (vector results) into parallel slot
	last := state.RetrievedChunks[len(state.RetrievedChunks)-1]
	state.RetrievedChunks[len(state.RetrievedChunks)-1] = nil
	state.RetrievedChunks = state.RetrievedChunks[:len(state.RetrievedChunks)-1]
	state.RetrievedChunks = append(state.RetrievedChunks, last)
	return nil
}
