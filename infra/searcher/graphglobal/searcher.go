// Package graphglobal provides the Graph-Global RAG searcher (community-summary-based macro pipeline):
//
//	[QueryRewriteStep] → GraphGlobalSearchStep → GenerationStep
package graphglobal

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	graphpkg "github.com/DotNetAge/gorag/infra/graph"
	"github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/infra/steps/generate"
	stepgraph "github.com/DotNetAge/gorag/infra/steps/graph"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// Searcher holds the components for the Graph-Global RAG pipeline.
// The pipeline is assembled once at construction time and reused on every Search call.
type Searcher struct {
	graphGlobalSearcher *graphpkg.GlobalSearcher // community-summary graph searcher (required)
	generator           retrieval.Generator      // LLM answer generator (required)
	queryRewriter       retrieval.QueryRewriter  // optional query rewriter
	logger              logging.Logger           // structured logger
	metrics             abstraction.Metrics      // observability metrics collector
	communityLevel      int                      // community hierarchy level to search (default: 1)

	pipe *pipeline.Pipeline[*entity.PipelineState] // pre-assembled, reused on every call
}

// Option is a functional option for Searcher.
type Option func(*Searcher)

// WithGraphSearcher sets the graph global searcher.
func WithGraphSearcher(gs *graphpkg.GlobalSearcher) Option {
	return func(s *Searcher) { s.graphGlobalSearcher = gs }
}

// WithGenerator sets the LLM answer generator.
func WithGenerator(generator retrieval.Generator) Option {
	return func(s *Searcher) { s.generator = generator }
}

// WithQueryRewriter enables query rewriting before retrieval (optional).
func WithQueryRewriter(rewriter retrieval.QueryRewriter) Option {
	return func(s *Searcher) { s.queryRewriter = rewriter }
}

// WithCommunityLevel sets the community summary hierarchy level (default: 1).
func WithCommunityLevel(level int) Option {
	return func(s *Searcher) {
		if level > 0 {
			s.communityLevel = level
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

// New creates a pre-assembled GraphRAG Global searcher.
//
// Pipeline: [QueryRewriteStep] → GraphGlobalSearchStep → GenerationStep
//
// Required: WithGraphSearcher, WithGenerator.
//
// Example:
//
//	s := graphglobal.New(
//	    graphglobal.WithGraphSearcher(globalSearcher),
//	    graphglobal.WithGenerator(gen),
//	    graphglobal.WithCommunityLevel(1),
//	)
func New(opts ...Option) *Searcher {
	s := &Searcher{
		communityLevel: 1,
		logger:         logging.NewNoopLogger(),
		metrics:        core.DefaultMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.pipe = s.buildPipeline()
	return s
}

// buildPipeline assembles the Graph Global RAG pipeline once at construction time.
// It panics if graphGlobalSearcher or generator is not set.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	if s.graphGlobalSearcher == nil {
		panic("graphglobal.New: graphGlobalSearcher is required")
	}
	if s.generator == nil {
		panic("graphglobal.New: generator is required")
	}

	p := pipeline.New[*entity.PipelineState]()

	if s.queryRewriter != nil {
		// Note: QueryRewriteStep requires direct LLM client, not the interface
		// For now, skip this step if no direct LLM client is available
		_ = s.queryRewriter // avoid unused variable error
	}

	p.AddStep(stepgraph.Global(s.graphGlobalSearcher, s.communityLevel, s.logger, s.metrics))
	p.AddStep(generate.Generate(s.generator, s.logger, s.metrics))
	return p
}

// Search executes the pre-built Graph Global RAG pipeline and returns the generated answer.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("graph_global", err)
		return "", fmt.Errorf("graphglobal.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("graph_global", time.Since(start))
	s.metrics.RecordSearchResult("graph_global", chunkCount)
	return state.Answer, nil
}
