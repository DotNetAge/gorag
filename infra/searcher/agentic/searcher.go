// Package agentic provides the Agentic RAG searcher:
//
//	AgentLoop [
//	    ReasoningStep → ActionSelectionStep → TerminationCheckStep →
//	    [VectorSearchStep] → ObservationStep
//	] → GenerationStep → [SelfRAGStep]
package agentic

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/searcher/core"
	agenticstep "github.com/DotNetAge/gorag/infra/steps/agentic"
	poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
	retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// Searcher implements the Agentic RAG pipeline using a ReAct-style loop.
//
// Each iteration executes:
//
//	ReasoningStep → ActionSelectionStep → TerminationCheckStep →
//	[VectorSearchStep (retrieve action only)] → ObservationStep
//
// After at most maxIterations the loop exits and a GenerationStep produces
// the final answer. An optional SelfRAGStep validates faithfulness.
type Searcher struct {
	// Loop decision components (all required)
	reasoner       retrieval.AgentReasoner       // analyses state and produces reasoning trace
	actionSelector retrieval.AgentActionSelector // chooses retrieve / reflect / finish

	// Retrieval: prefer retriever; fall back to embedder+vectorStore
	retriever   retrieval.Retriever     // optional parallel retriever (preferred path)
	embedder    embedding.Provider      // optional dense embedding model (fallback path)
	vectorStore abstraction.VectorStore // optional vector index (fallback path)

	generator  retrieval.Generator  // LLM answer generator (required)
	reranker   abstraction.Reranker // optional cross-encoder reranker
	selfJudge  selfJudge            // optional SelfRAG faithfulness evaluator
	strictRAG  bool                 // if true, return error on low faithfulness score
	logger     logging.Logger       // structured logger
	metrics    abstraction.Metrics  // observability metrics collector
	topK       int                  // retrieval candidate size (default: 10)
	rerankTopK int                  // results kept after reranking (default: 5)

	// maxIterations caps the agentic loop to prevent infinite loops (default: 5)
	maxIterations int

	loopBody  *pipeline.Pipeline[*entity.PipelineState] // single-iteration pipeline
	finalPipe *pipeline.Pipeline[*entity.PipelineState] // post-loop generation pipeline
}

// selfJudge is the interface for SelfRAG faithfulness evaluation.
// Implementations score how well an answer is grounded in the retrieved chunks.
type selfJudge interface {
	// EvaluateFaithfulness returns a [0,1] faithfulness score, a human-readable
	// reason string, and any error encountered during evaluation.
	EvaluateFaithfulness(ctx context.Context, query string, chunks []*entity.Chunk, answer string) (float32, string, error)
}

// Option is a functional option for Searcher.
type Option func(*Searcher)

// WithReasoner sets the agent reasoner (required).
func WithReasoner(r retrieval.AgentReasoner) Option {
	return func(s *Searcher) { s.reasoner = r }
}

// WithActionSelector sets the agent action selector (required).
func WithActionSelector(sel retrieval.AgentActionSelector) Option {
	return func(s *Searcher) { s.actionSelector = sel }
}

// WithRetriever sets the parallel retriever (preferred retrieval path).
func WithRetriever(r retrieval.Retriever) Option {
	return func(s *Searcher) { s.retriever = r }
}

// WithEmbedding sets the embedding provider (fallback retrieval path).
func WithEmbedding(provider embedding.Provider) Option {
	return func(s *Searcher) { s.embedder = provider }
}

// WithVectorStore sets the vector store (fallback retrieval path).
func WithVectorStore(store abstraction.VectorStore) Option {
	return func(s *Searcher) { s.vectorStore = store }
}

// WithGenerator sets the LLM answer generator (required).
func WithGenerator(generator retrieval.Generator) Option {
	return func(s *Searcher) { s.generator = generator }
}

// WithReranker adds a reranking pass after retrieval (optional).
func WithReranker(reranker abstraction.Reranker) Option {
	return func(s *Searcher) { s.reranker = reranker }
}

// WithSelfRAGJudge attaches a faithfulness evaluator (Self-RAG reflection, optional).
// Set strict=true to return an error when the score is below threshold.
func WithSelfRAGJudge(judge selfJudge, strict bool) Option {
	return func(s *Searcher) {
		s.selfJudge = judge
		s.strictRAG = strict
	}
}

// WithMaxIterations sets the maximum number of agentic loop iterations (default: 5).
func WithMaxIterations(n int) Option {
	return func(s *Searcher) {
		if n > 0 {
			s.maxIterations = n
		}
	}
}

// WithTopK sets the retrieval candidate size (default: 10).
func WithTopK(k int) Option {
	return func(s *Searcher) {
		if k > 0 {
			s.topK = k
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

// New creates a pre-assembled Agentic RAG searcher with a ReAct-style loop.
//
// Pipeline per iteration:
//
//	ReasoningStep → ActionSelectionStep → TerminationCheckStep →
//	[VectorSearchStep (on retrieve action)] → ObservationStep
//
// Post-loop:
//
//	[RerankStep] → GenerationStep → [SelfRAGStep]
//
// Required: WithReasoner, WithActionSelector, WithGenerator.
//
// Retrieval path (in order of preference):
//   - WithRetriever               (parallel multi-query retrieval)
//   - WithEmbedding + WithVectorStore  (explicit vector search)
//   - neither provided: built-in defaults are used automatically
//
// Example:
//
//	s := agentic.New(
//	    agentic.WithReasoner(reasoner),
//	    agentic.WithActionSelector(selector),
//	    agentic.WithGenerator(gen),
//	    agentic.WithMaxIterations(5),
//	    agentic.WithLogger(logger),
//	)
func New(opts ...Option) *Searcher {
	s := &Searcher{
		topK:          10,
		rerankTopK:    5,
		maxIterations: 5,
		logger:        logging.NewNoopLogger(),
		metrics:       core.DefaultMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.loopBody, s.finalPipe = s.buildPipelines()
	return s
}

// buildPipelines assembles the loop body and the post-loop final pipeline.
// Panics if reasoner, actionSelector, or generator is not set.
func (s *Searcher) buildPipelines() (loopBody, finalPipe *pipeline.Pipeline[*entity.PipelineState]) {
	if s.reasoner == nil {
		panic("agentic.New: reasoner is required")
	}
	if s.actionSelector == nil {
		panic("agentic.New: actionSelector is required")
	}
	if s.generator == nil {
		panic("agentic.New: generator is required")
	}

	// Ensure defaults for vector path
	if s.retriever == nil {
		if s.embedder == nil {
			embedder, err := core.DefaultEmbedder()
			if err != nil {
				panic(err)
			}
			s.embedder = embedder
		}
		if s.vectorStore == nil {
			store, err := core.DefaultVectorStore()
			if err != nil {
				panic(err)
			}
			s.vectorStore = store
		}
	}

	// --- Loop body ---
	loop := pipeline.New[*entity.PipelineState]()
	loop.AddStep(agenticstep.NewReasoningStep(s.reasoner, s.logger))
	loop.AddStep(agenticstep.NewActionSelectionStep(s.actionSelector, s.maxIterations, s.logger))
	loop.AddStep(agenticstep.NewTerminationCheckStep(s.logger))

	// Retrieval step (only executes when action == retrieve; guarded inside the loop by Search)
	if s.retriever != nil {
		loop.AddStep(agenticstep.NewParallelRetriever(s.retriever, s.topK, s.logger))
	} else {
		loop.AddStep(retrievalstep.NewVectorSearchStep(s.embedder, s.vectorStore, s.topK))
	}

	loop.AddStep(agenticstep.NewObservationStep(s.logger))

	// --- Final pipeline (post-loop) ---
	final := pipeline.New[*entity.PipelineState]()
	if s.reranker != nil {
		final.AddStep(poststep.NewRerankStep(s.reranker, s.rerankTopK))
	}
	final.AddStep(poststep.NewGenerator(s.generator, s.logger))
	if s.selfJudge != nil {
		final.AddStep(&selfRAGStep{judge: s.selfJudge, strict: s.strictRAG, threshold: 0.8})
	}

	return loop, final
}

// Search executes the Agentic RAG loop and returns the generated answer.
//
// Each iteration runs the loop body. The loop exits when:
//   - TerminationCheckStep sets the finished flag (ActionFinish), OR
//   - maxIterations is reached.
//
// After the loop, the final pipeline generates the answer from all accumulated chunks.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	start := time.Now()
	state := entity.NewPipelineState()
	state.Query = entity.NewQuery("", query, nil)
	state.Agentic = entity.NewAgenticMetadata()
	// Store original query so ReasoningStep always has access to it.
	state.Agentic.OriginalQueryText = query

	s.logger.Info("agentic search started", map[string]interface{}{
		"query":          query,
		"max_iterations": s.maxIterations,
	})

	for i := 0; i < s.maxIterations; i++ {
		agenticstep.AgentSetIteration(state, i)

		if err := s.loopBody.Execute(ctx, state); err != nil {
			s.metrics.RecordSearchError("agentic", err)
			return "", fmt.Errorf("agentic.Searcher.Search: loop iteration %d: %w", i, err)
		}

		if agenticstep.AgentFinished(state) {
			s.logger.Info("agent loop finished early", map[string]interface{}{
				"iteration": i,
			})
			break
		}
	}

	// Run the final generation pipeline on all accumulated chunks.
	if err := s.finalPipe.Execute(ctx, state); err != nil {
		s.metrics.RecordSearchError("agentic", err)
		return "", fmt.Errorf("agentic.Searcher.Search: finalPipe: %w", err)
	}

	chunkCount := 0
	for _, g := range state.RetrievedChunks {
		chunkCount += len(g)
	}
	s.metrics.RecordSearchDuration("agentic", time.Since(start))
	s.metrics.RecordSearchResult("agentic", chunkCount)

	s.logger.Info("agentic search completed", map[string]interface{}{
		"query":         query,
		"answer_length": len(state.Answer),
		"total_chunks":  chunkCount,
	})

	return state.Answer, nil
}

// selfRAGStep adapts selfJudge to the pipeline.Step interface.
// It evaluates the generated answer's faithfulness against the retrieved chunks
// and either appends a warning or returns an error depending on the strict setting.
type selfRAGStep struct {
	judge     selfJudge // faithfulness evaluator
	strict    bool      // if true, return error when score < threshold
	threshold float32   // minimum acceptable faithfulness score (default: 0.8)
}

func (a *selfRAGStep) Name() string { return "AgenticSelfRAG" }

// Execute evaluates the faithfulness of the generated answer. It is a no-op when
// the query, answer, or retrieved chunks are missing. On low scores it either
// appends a hallucination warning (non-strict) or returns an error (strict).
func (a *selfRAGStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Answer == "" || len(state.RetrievedChunks) == 0 {
		return nil
	}

	var allChunks []*entity.Chunk
	for _, group := range state.RetrievedChunks {
		allChunks = append(allChunks, group...)
	}

	score, reason, err := a.judge.EvaluateFaithfulness(ctx, state.Query.Text, allChunks, state.Answer)
	if err != nil {
		return fmt.Errorf("selfRAGStep: EvaluateFaithfulness failed: %w", err)
	}

	state.SelfRagScore = score
	state.SelfRagReason = reason

	if score < a.threshold {
		if a.strict {
			return fmt.Errorf("SelfRAG validation failed (score %.2f < %.2f): %s", score, a.threshold, reason)
		}
		state.Answer += "\n\n[Warning: System detected potential hallucinations in this answer.]"
	}

	return nil
}
