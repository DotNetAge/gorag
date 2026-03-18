// Package crag provides CRAG (Corrective RAG) implementation.
// It evaluates retrieval quality and corrects by searching external sources if needed.
package crag

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	searchercore "github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/infra/steps/crag"
	"github.com/DotNetAge/gorag/infra/steps/generate"
	"github.com/DotNetAge/gorag/infra/steps/vector"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// Searcher implements CRAG (Corrective RAG) pattern.
// It evaluates retrieval quality and triggers corrective actions.
type Searcher struct {
	embedder      embedding.Provider      // dense embedding model
	vectorStore   abstraction.VectorStore // vector index
	generator     retrieval.Generator     // LLM answer generator
	cragEvaluator *enhancer.CRAGEvaluator // retrieval quality evaluator
	logger        logging.Logger          // structured logger
	metrics       abstraction.Metrics     // metrics collector
	topK          int                     // number of results (default: 10)

	pipe *pipeline.Pipeline[*entity.PipelineState]
}

// Option is a functional option for CRAG Searcher.
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

// WithCRAGEvaluator sets the CRAG evaluator for retrieval quality assessment.
func WithCRAGEvaluator(evaluator *enhancer.CRAGEvaluator) Option {
	return func(s *Searcher) { s.cragEvaluator = evaluator }
}

// WithTopK sets the number of results to retrieve (default: 10).
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

// New creates a new CRAG Searcher.
//
// Pipeline: [CRAGEvaluatorStep] → VectorSearchStep → GenerationStep
//
// Required: WithGenerator, WithCRAGEvaluator.
//
// Example:
//
//	cragEval := enhancer.NewCRAGEvaluator(llmClient)
//	searcher := crag.New(
//	    crag.WithGenerator(gen),
//	    crag.WithCRAGEvaluator(cragEval),
//	    crag.WithTopK(10),
//	)
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

// buildPipeline builds the CRAG RAG pipeline.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	p := pipeline.New[*entity.PipelineState]()

	// Step 1: Evaluate retrieval quality (CRAG)
	if s.cragEvaluator != nil {
		p.AddStep(crag.Evaluate(s.cragEvaluator, s.logger, s.metrics))
	}

	// Step 2: Vector Search (may be corrected by CRAG)
	p.AddStep(vector.Search(s.embedder, s.vectorStore, s.topK, s.logger, s.metrics))

	// Step 3: Generation
	p.AddStep(generate.Generate(s.generator, s.logger, s.metrics))

	return p
}

// Search executes the CRAG RAG pipeline.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("crag", err)
		return "", fmt.Errorf("crag.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("crag", time.Since(start))
	s.metrics.RecordSearchResult("crag", chunkCount)
	return state.Answer, nil
}
