// Package stepback provides StepBack RAG implementation.
// It abstracts specific queries into general principle questions for broader context retrieval.
package stepback

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	searchercore "github.com/DotNetAge/gorag/infra/searcher/core"
	poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
	prestep "github.com/DotNetAge/gorag/infra/steps/pre_retrieval"
	retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// Searcher implements StepBack RAG pattern.
// It abstracts specific questions into general principles for better background retrieval.
type Searcher struct {
	embedder    embedding.Provider          // dense embedding model
	vectorStore abstraction.VectorStore     // vector index
	generator   retrieval.Generator         // LLM answer generator
	stepBackGen *enhancer.StepBackGenerator // step-back query generator
	logger      logging.Logger              // structured logger
	metrics     abstraction.Metrics         // metrics collector
	topK        int                         // number of results (default: 10)

	pipe *pipeline.Pipeline[*entity.PipelineState]
}

// Option is a functional option for StepBack Searcher.
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

// WithStepBackGenerator sets the StepBack generator for query abstraction.
func WithStepBackGenerator(gen *enhancer.StepBackGenerator) Option {
	return func(s *Searcher) { s.stepBackGen = gen }
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

// New creates a new StepBack Searcher.
//
// Pipeline: [StepBackStep] → VectorSearchStep → GenerationStep
//
// Required: WithGenerator, WithStepBackGenerator.
//
// Example:
//
//	stepBackGen := enhancer.NewStepBackGenerator(llmClient)
//	searcher := stepback.New(
//	    stepback.WithGenerator(gen),
//	    stepback.WithStepBackGenerator(stepBackGen),
//	    stepback.WithTopK(10),
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

// buildPipeline builds the StepBack RAG pipeline.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	p := pipeline.New[*entity.PipelineState]()

	// Step 1: Abstract query to general principle (StepBack)
	if s.stepBackGen != nil {
		p.AddStep(prestep.NewStepBackStep(s.stepBackGen, s.logger))
	}

	// Step 2: Vector Search using abstracted query
	p.AddStep(retrievalstep.NewVectorSearchStep(s.embedder, s.vectorStore, s.topK))

	// Step 3: Generation with original query + retrieved context
	p.AddStep(poststep.NewGenerator(s.generator, s.logger))

	return p
}

// Search executes the StepBack RAG pipeline.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("stepback", err)
		return "", fmt.Errorf("stepback.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("stepback", time.Since(start))
	s.metrics.RecordSearchResult("stepback", chunkCount)
	return state.Answer, nil
}
