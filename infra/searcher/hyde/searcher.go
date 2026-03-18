// Package hyde provides HyDE (Hypothetical Document Embeddings) RAG implementation.
// It generates a hypothetical answer, then searches using that answer's embedding.
package hyde

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	searchercore "github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/infra/steps/generate"
	"github.com/DotNetAge/gorag/infra/steps/hyde"
	"github.com/DotNetAge/gorag/infra/steps/vector"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// Searcher implements HyDE (Hypothetical Document Embeddings) RAG pattern.
// It generates a hypothetical answer to bridge the query-document semantic gap.
type Searcher struct {
	embedder      embedding.Provider      // dense embedding model
	vectorStore   abstraction.VectorStore // vector index
	generator     retrieval.Generator     // LLM answer generator
	hydeGenerator *enhancer.HyDEGenerator // hypothetical document generator
	logger        logging.Logger          // structured logger
	metrics       abstraction.Metrics     // metrics collector
	topK          int                     // number of results (default: 10)

	pipe *pipeline.Pipeline[*entity.PipelineState]
}

// Option is a functional option for HyDE Searcher.
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

// WithHyDEGenerator sets the HyDE generator for hypothetical document creation.
func WithHyDEGenerator(gen *enhancer.HyDEGenerator) Option {
	return func(s *Searcher) { s.hydeGenerator = gen }
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

// New creates a new HyDE Searcher.
//
// Pipeline: [HyDEStep] → VectorSearchStep → GenerationStep
//
// Required: WithGenerator, WithHyDEGenerator.
//
// Example:
//
//	hydeGen := enhancer.NewHyDEGenerator(llmClient)
//	searcher := hyde.New(
//	    hyde.WithGenerator(gen),
//	    hyde.WithHyDEGenerator(hydeGen),
//	    hyde.WithTopK(10),
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

// buildPipeline builds the HyDE RAG pipeline.
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	p := pipeline.New[*entity.PipelineState]()

	// Step 1: Generate hypothetical document (HyDE)
	if s.hydeGenerator != nil {
		p.AddStep(hyde.Generate(s.hydeGenerator, s.logger, s.metrics))
	}

	// Step 2: Vector Search using hypothetical document's embedding
	p.AddStep(vector.Search(s.embedder, s.vectorStore, s.topK, s.logger, s.metrics))

	// Step 3: Generation
	p.AddStep(generate.Generate(s.generator, s.logger, s.metrics))

	return p
}

// Search executes the HyDE RAG pipeline.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)

	if err := s.pipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("hyde", err)
		return "", fmt.Errorf("hyde.Searcher.Search: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("hyde", time.Since(start))
	s.metrics.RecordSearchResult("hyde", chunkCount)
	return state.Answer, nil
}
