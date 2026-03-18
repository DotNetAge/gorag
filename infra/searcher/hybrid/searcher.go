// Package hybrid provides the Hybrid RAG searcher:
//
//	[QueryToFilterStep] → [StepBackStep] → [HyDEStep] →
//	VectorSearchStep + [SparseSearchStep] → RAGFusionStep → [RerankStep] → GenerationStep
package hybrid

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	searchercore "github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/infra/steps/dedup"
	"github.com/DotNetAge/gorag/infra/steps/filter"
	"github.com/DotNetAge/gorag/infra/steps/fuse"
	"github.com/DotNetAge/gorag/infra/steps/generate"
	"github.com/DotNetAge/gorag/infra/steps/hyde"
	"github.com/DotNetAge/gorag/infra/steps/rerank"
	"github.com/DotNetAge/gorag/infra/steps/sparse"
	"github.com/DotNetAge/gorag/infra/steps/stepback"
	"github.com/DotNetAge/gorag/infra/steps/vector"
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
	sparseStore     *sparse.Searcher            // optional BM25/keyword retrieval path
	reranker        abstraction.Reranker        // optional cross-encoder reranker after fusion
	filterExtractor *enhancer.FilterExtractor   // optional metadata filter extractor
	stepBackGen     *enhancer.StepBackGenerator // optional StepBack abstract query expander (Advanced RAG)
	hydeGenerator   *enhancer.HyDEGenerator     // optional HyDE hypothetical document generator (Advanced RAG)
	queryRewriter   core.Client                 // optional query rewriter LLM client (Advanced RAG)
	llmForCompress  core.Client                 // optional LLM client for context compression (Advanced RAG)
	logger          logging.Logger              // structured logger
	metrics         abstraction.Metrics         // observability metrics collector
	denseTopK       int                         // dense retrieval candidate size (default: 20)
	sparseTopK      int                         // sparse retrieval candidate size (default: 20)
	fusionTopK      int                         // results output from RRF fusion (default: 10)
	rerankTopK      int                         // results kept after reranking (default: 5)
	compressContext bool                        // enable context compression (default: false)

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
func WithSparseStore(ss *sparse.Searcher) Option {
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

// WithQueryRewriter sets the LLM client for query rewriting (optional).
func WithQueryRewriter(llm core.Client) Option {
	return func(s *Searcher) { s.queryRewriter = llm }
}

// WithContextCompression enables context compression using LLM (optional).
// When enabled, an LLM client must be provided via WithLLMForCompression.
func WithContextCompression(enabled bool) Option {
	return func(s *Searcher) { s.compressContext = enabled }
}

// WithLLMForCompression sets the LLM client for context compression (optional).
func WithLLMForCompression(llm core.Client) Option {
	return func(s *Searcher) { s.llmForCompress = llm }
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
		metrics:    searchercore.DefaultMetrics(),
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
	if s.fusionEngine == nil {
		s.fusionEngine = searchercore.DefaultFusionEngine()
	}

	p := pipeline.New[*entity.PipelineState]()

	// === Phase 1: Advanced Query Enhancement (Optional - LLM Calls) ===
	// Note: These are Advanced RAG features, not part of standard Hybrid RAG
	// Reference: LangChain/LlamaIndex Hybrid RAG = Vector + BM25 + Fusion (no LLM)

	// Step 1: Query Rewrite (可选 - Advanced RAG)
	if s.queryRewriter != nil {
		// Note: QueryRewriteStep requires direct LLM client, not the interface
		// For now, skip this step if no direct LLM client is available
		_ = s.queryRewriter // avoid unused variable error
	}

	// Step 2: Filter Extraction (可选 - 元数据硬过滤)
	if s.filterExtractor != nil {
		p.AddStep(filter.FromQuery(s.filterExtractor))
	}

	// Step 3: StepBack (可选 - Advanced RAG: 抽象化查询)
	if s.stepBackGen != nil {
		p.AddStep(stepback.Generate(s.stepBackGen))
	}

	// Step 4: HyDE (可选 - Advanced RAG: 假设性文档增强)
	if s.hydeGenerator != nil {
		p.AddStep(hyde.Generate(s.hydeGenerator, s.logger, s.metrics))
	}

	// === Phase 2: Hybrid Retrieval (Core - No LLM Calls) ===
	// Standard Hybrid RAG: Vector Search + BM25/Sparse Search + RRF Fusion

	// Step 5: Dense Retrieval (向量语义检索)
	p.AddStep(vector.Search(s.embedder, s.vectorStore, s.denseTopK, s.logger, s.metrics))

	// Step 6: Sparse Retrieval (可选 - BM25/关键词检索)
	if s.sparseStore != nil {
		p.AddStep(sparse.Search(*s.sparseStore, s.sparseTopK, s.logger, s.metrics))
	}

	// Step 7: Prepare for Fusion (准备多路结果)
	p.AddStep(chunksToParallelResultsStep{})

	// Step 8: RRF Fusion (Reciprocal Rank Fusion 融合)
	p.AddStep(fuse.RRF(s.fusionEngine, s.fusionTopK, s.logger, s.metrics))

	// === Phase 3: Post-Retrieval Optimization (Optional - Usually No LLM) ===

	// Step 9: Deduplication (可选 - Jaccard 相似度去重)
	p.AddStep(dedup.Unique(0.95, s.logger, s.metrics))

	// Step 10: Cross-Encoder Rerank (可选 - 重排序，非 LLM)
	if s.reranker != nil {
		p.AddStep(rerank.Score(s.reranker, s.rerankTopK, s.logger, s.metrics))
	}

	// === Phase 4: Context & Generation (LLM Call Here) ===

	// Step 11: Context Compression (可选 - Advanced RAG: LLM 压缩上下文)
	// Note: Requires ResultEnhancer interface, not core.Client directly
	// TODO: Wrap core.Client in ResultEnhancer adapter if needed
	if s.compressContext && s.llmForCompress != nil {
		_ = s.llmForCompress // Skip for now - needs adapter
	}

	// Step 12: Generation (唯一必需的 LLM 调用)
	p.AddStep(generate.Generate(s.generator, s.logger, s.metrics))
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
