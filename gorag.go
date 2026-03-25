// Package gorag provides a high-level API for building Retrieval-Augmented Generation (RAG) applications.
//
// This package offers pre-configured RAG implementations with support for multiple
// retrieval strategies including Native, Advanced, Agentic, and Graph-based RAG patterns.
// It leverages dependency injection for flexible component customization and supports
// various vector stores, document stores, and LLM providers.
//
// Quick Start:
//
//	rag, err := gorag.DefaultNativeRAG(
//	    gorag.WithWorkDir("./data"),
//	    gorag.WithTopK(5),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer rag.Close()
//
//	// Index documents
//	err = rag.IndexFile(ctx, "path/to/document.pdf")
//
//	// Search
//	result, err := rag.Search(ctx, "your query", 5)
package gorag

import (
	"context"
	"fmt"
	"os"
	"time"

	gochat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/env"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/di"
	"github.com/DotNetAge/gorag/pkg/indexer"
	"github.com/DotNetAge/gorag/pkg/indexing/store/bolt"
	"github.com/DotNetAge/gorag/pkg/indexing/store/sqlite"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/retrieval/cache"
	"github.com/DotNetAge/gorag/pkg/retriever/advanced"
	"github.com/DotNetAge/gorag/pkg/retriever/agentic"
	"github.com/DotNetAge/gorag/pkg/retriever/crag"
	"github.com/DotNetAge/gorag/pkg/retriever/graph"
	"github.com/DotNetAge/gorag/pkg/retriever/native"
	"github.com/DotNetAge/gorag/pkg/retriever/selfrag"
)

type providerToEmbedderAdapter struct {
	provider embedding.Provider
}

func (a *providerToEmbedderAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := a.provider.Embed(ctx, []string{text})
	if err != nil || len(results) == 0 {
		return nil, err
	}
	return results[0], nil
}

func (a *providerToEmbedderAdapter) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return a.provider.Embed(ctx, texts)
}

func (a *providerToEmbedderAdapter) Dimension() int {
	return a.provider.Dimension()
}

// RAGConfig is the single source of truth for all RAG modes.
// It holds configuration parameters for RAG system initialization.
type RAGConfig struct {
	// 0. Identity
	// Name specifies a unique name for the RAG instance, used for resource isolation.
	Name string

	// 1. Persistence
	// WorkDir specifies the working directory for storing persistent data (vector DB, document DB, etc.).
	WorkDir string
	// VectorDBType specifies the type of vector store to use (e.g., "govector", "milvus", "pinecone").
	VectorDBType string // "govector", "milvus"
	// Dimension specifies the dimension of embedding vectors.
	Dimension int

	// 2. Model "Grounding" (Variable Name Driven)
	// EmbedderType specifies the embedding model provider (e.g., "openai", "ollama", "local-onnx").
	EmbedderType string // "openai", "ollama", "local-onnx"
	// LLMType specifies the large language model provider (e.g., "openai", "claude", "ollama").
	LLMType string // "openai", "claude", "ollama"
	// ModelPath specifies the local model file path (typically under .test/models/...).
	ModelPath string // Local model file path (.test/models/...)
	// ModelName specifies the model name identifier (e.g., "qwen-turbo", "bge-small-zh-v1.5").
	ModelName string // e.g., "qwen-turbo", "bge-small-zh-v1.5"
	// APIKeyEnv specifies the environment variable name for the API key (e.g., "DASHSCOPE_API_KEY").
	APIKeyEnv string // Name of the env var, e.g., "DASHSCOPE_API_KEY"
	// BaseURL specifies the base URL for API-compatible model providers.
	BaseURL string // e.g., "https://dashscope.aliyuncs.com/compatible-mode/"

	// 3. Retrieval Tuning
	// TopK specifies the default number of top results to retrieve during search.
	TopK int

	// 4. Semantic Cache
	// EnableSemanticCache enables semantic caching for improved performance.
	EnableSemanticCache bool
	// SemanticCacheType specifies the cache backend type: "memory" or "bolt".
	SemanticCacheType string

	// 5. Advanced Tuning (Self-RAG & CRAG)
	// SelfRAGThreshold specifies the quality threshold for Self-RAG reflection.
	SelfRAGThreshold float32
	// MaxRetries specifies the maximum number of refinement attempts in Self-RAG.
	MaxRetries int
	// WebSearcher is the external search engine for CRAG.
	webSearcher core.WebSearcher

	// 6. Graph RAG Tuning
	// GraphDepth specifies the search depth in the knowledge graph.
	GraphDepth int
	// GraphLimit specifies the maximum number of related nodes to retrieve per entity.
	GraphLimit int

	// 7. Internal Component Injection
	// embedder is the internally injected embedding provider
	embedder embedding.Provider
	// llmClient is the internally injected LLM client
	llmClient gochat.Client
	// metrics is the observability collector
	metrics core.Metrics
	// tracer is the distributed tracer
	tracer observability.Tracer
	// parsers is the list of document parsers
	parsers []core.Parser
	// container is the dependency injection container
	container *di.Container
}

// RAGOption is a function type for configuring RAG instances using the functional options pattern.
type RAGOption func(*RAGConfig)

// WithName sets a unique name for the RAG instance for resource isolation.
func WithName(name string) RAGOption { return func(c *RAGConfig) { c.Name = name } }

// WithWorkDir sets the working directory for persistent storage.
func WithWorkDir(path string) RAGOption { return func(c *RAGConfig) { c.WorkDir = path } }

// WithMetrics injects a custom metrics collector.
func WithMetrics(m core.Metrics) RAGOption { return func(c *RAGConfig) { c.metrics = m } }

// WithTracer injects a custom distributed tracer.
func WithTracer(t observability.Tracer) RAGOption { return func(c *RAGConfig) { c.tracer = t } }

// WithModelPath sets the local model file path.
func WithModelPath(path string) RAGOption { return func(c *RAGConfig) { c.ModelPath = path } }

// WithAPIKeyEnv sets the environment variable name for API key.
func WithAPIKeyEnv(name string) RAGOption { return func(c *RAGConfig) { c.APIKeyEnv = name } }

// WithBaseURL sets the base URL for API-compatible providers.
func WithBaseURL(url string) RAGOption { return func(c *RAGConfig) { c.BaseURL = url } }

// WithModelName sets the model name identifier.
func WithModelName(name string) RAGOption { return func(c *RAGConfig) { c.ModelName = name } }

// WithDimension sets the embedding vector dimension.
func WithDimension(dim int) RAGOption { return func(c *RAGConfig) { c.Dimension = dim } }

// WithTopK sets the default number of top results to retrieve.
func WithTopK(k int) RAGOption { return func(c *RAGConfig) { c.TopK = k } }

// WithThreshold sets the quality threshold for Self-RAG reflection.
func WithThreshold(t float32) RAGOption { return func(c *RAGConfig) { c.SelfRAGThreshold = t } }

// WithMaxRetries sets the maximum refinement attempts for Self-RAG.
func WithMaxRetries(r int) RAGOption { return func(c *RAGConfig) { c.MaxRetries = r } }

// WithWebSearcher sets the external search engine for CRAG.
func WithWebSearcher(s core.WebSearcher) RAGOption { return func(c *RAGConfig) { c.webSearcher = s } }

// WithDepth sets the search depth for Graph RAG.
func WithDepth(d int) RAGOption { return func(c *RAGConfig) { c.GraphDepth = d } }

// WithLimit sets the neighbor limit for Graph RAG.
func WithLimit(l int) RAGOption { return func(c *RAGConfig) { c.GraphLimit = l } }

// Expert injection
// WithEmbedder injects a custom embedding provider.
func WithEmbedder(e embedding.Provider) RAGOption { return func(c *RAGConfig) { c.embedder = e } }

// WithLLM injects a custom LLM client.
func WithLLM(l gochat.Client) RAGOption { return func(c *RAGConfig) { c.llmClient = l } }

// WithParsers injects custom document parsers.
func WithParsers(p ...core.Parser) RAGOption { return func(c *RAGConfig) { c.parsers = p } }

// WithContainer injects a custom dependency injection container.
func WithContainer(ctr *di.Container) RAGOption { return func(c *RAGConfig) { c.container = ctr } }

// WithSemanticCache enables semantic caching with specified backend type.
// cacheType can be "memory" (default, in-memory) or "bolt" (persistent).
func WithSemanticCache(enable bool, cacheType ...string) RAGOption {
	return func(c *RAGConfig) {
		c.EnableSemanticCache = enable
		if len(cacheType) > 0 {
			c.SemanticCacheType = cacheType[0]
		} else {
			c.SemanticCacheType = "memory"
		}
	}
}

// RAG application interface defines the main entry point for RAG operations.
type RAG interface {
	// IndexFile processes a single file and adds it to the knowledge base.
	IndexFile(ctx context.Context, filePath string) error
	// IndexDirectory processes all files in a directory and adds them to the knowledge base.
	// If recursive is true, it will also process subdirectories.
	IndexDirectory(ctx context.Context, dirPath string, recursive bool) error
	// Search performs a retrieval query and returns the top K results.
	Search(ctx context.Context, query string, topK int) (*core.RetrievalResult, error)
	// Container returns the underlying dependency injection container for advanced usage.
	Container() *di.Container
	// Close releases all resources held by the RAG instance.
	Close() error
}

// defaultRAG is the default implementation of the RAG interface.
type defaultRAG struct {
	mode      string
	indexer   indexer.Indexer
	retriever core.Retriever
	metrics   core.Metrics
	container *di.Container
}

func (r *defaultRAG) IndexFile(ctx context.Context, filePath string) error {
	_, err := r.indexer.IndexFile(ctx, filePath)
	return err
}
func (r *defaultRAG) IndexDirectory(ctx context.Context, dirPath string, recursive bool) error {
	return r.indexer.IndexDirectory(ctx, dirPath, recursive)
}
func (r *defaultRAG) Search(ctx context.Context, query string, topK int) (*core.RetrievalResult, error) {
	start := time.Now()
	r.metrics.RecordQueryCount(r.mode)

	results, err := r.retriever.Retrieve(ctx, []string{query}, topK)

	r.metrics.RecordSearchDuration(r.mode, time.Since(start))
	if err != nil {
		r.metrics.RecordSearchError(r.mode, err)
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	res := results[0]
	r.metrics.RecordSearchResult(r.mode, len(res.Chunks))

	// Async Evaluation and Metrics Recording
	if eval, err := r.container.Resolve((*core.RAGEvaluator)(nil)); err == nil {
		go func(q string, a string, chks []*core.Chunk) {
			// We use a background context to avoid being cancelled by the request's context
			evalCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			contextText := ""
			for _, c := range chks {
				contextText += c.Content + "\n"
			}

			report, err := eval.(core.RAGEvaluator).Evaluate(evalCtx, q, a, contextText)
			if err == nil {
				r.metrics.RecordRAGEvaluation("faithfulness", report.Faithfulness)
				r.metrics.RecordRAGEvaluation("relevance", report.Relevance)
				r.metrics.RecordRAGEvaluation("recall", report.ContextRecall)
				r.metrics.RecordRAGEvaluation("overall", report.OverallScore)
			}

		}(query, res.Answer, res.Chunks)
	}

	return res, nil
}

func (r *defaultRAG) Container() *di.Container { return r.container }
func (r *defaultRAG) Close() error             { return r.container.Close() }

// --- The Aligned Presets ---

// DefaultNativeRAG creates a lightweight, local-first RAG instance.
// It uses default TokenChunker, local SQLite and GoVector stores, suitable for quick prototyping.
//
// Example:
//
//	rag, err := gorag.DefaultNativeRAG(
//	    gorag.WithWorkDir("./data"),
//	    gorag.WithTopK(5),
//	)
func DefaultNativeRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("native", opts...)
}

// DefaultAdvancedRAG creates a high-performance RAG instance optimized for production use.
// It supports advanced features like query rewriting, fusion, and reranking.
func DefaultAdvancedRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("advanced", opts...)
}

// DefaultAgenticRAG creates an agentic RAG instance with smart routing capabilities.
// It automatically selects the best retrieval strategy based on query intent classification.
func DefaultAgenticRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("agentic", opts...)
}

// DefaultGraphRAG creates a knowledge graph-enhanced RAG instance.
// It leverages entity relationships and structured knowledge for complex queries.
func DefaultGraphRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("graph", opts...)
}

// DefaultSelfRAG creates a self-correcting RAG instance with reflection capabilities.
// It evaluates retrieval relevance and generation quality for high-precision answers.
func DefaultSelfRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("selfrag", opts...)
}

// DefaultCRAG creates a corrective RAG instance with external search fallback.
// It automatically triggers web search when internal knowledge is insufficient or incorrect.
func DefaultCRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("crag", opts...)
}

// CheckEnvironment verifies if the GoRAG environment is ready for operation.
func CheckEnvironment(ctx context.Context) (bool, []string) {
	return env.DefaultEnvironment().Check(ctx)
}

// PrepareEnvironment downloads all required models for GoRAG.
func PrepareEnvironment(ctx context.Context, progress func(modelName, fileName string, downloaded, total int64)) error {
	return env.DefaultEnvironment().Prepare(ctx, progress)
}

// buildRAGWithDefaults applies preset defaults before building the RAG instance.
// It configures mode-specific parameters and then delegates to buildRAG for construction.
func buildRAGWithDefaults(mode string, opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{
		WorkDir:             "./data",
		ModelPath:           ".test/models",
		VectorDBType:        "govector",
		Dimension:           1536,
		TopK:                5,
		EnableSemanticCache: false,
		SemanticCacheType:   "memory",
		SelfRAGThreshold:    0.7,
		MaxRetries:          3,
		GraphDepth:          1,
		GraphLimit:          10,
	}

	// Mode-specific defaults
	switch mode {
	case "advanced":
		cfg.TopK = 10
		cfg.EnableSemanticCache = true
	case "agentic":
		cfg.TopK = 5
		cfg.EnableSemanticCache = true
	case "graph":
		cfg.EnableSemanticCache = false
	case "selfrag":
		cfg.TopK = 5
		cfg.EnableSemanticCache = true
	case "crag":
		cfg.TopK = 5
		cfg.EnableSemanticCache = true
	}

	for _, opt := range opts {
		opt(cfg)
	}
	return buildRAG(cfg, mode)
}

// buildRAG factory leverages dependency injection for resource pooling and component management.
// It initializes vector stores, document stores, and configures indexers/retrievers based on mode.
func buildRAG(cfg *RAGConfig, mode string) (RAG, error) {
	if cfg.container == nil {
		cfg.container = di.New()
	}
	ctr := cfg.container

	// Pre-flight check: Ensure persistence path is stable and writable
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("pre-flight check failed: work directory %q is not accessible: %w", cfg.WorkDir, err)
	}

	// 0. Observability Setup (Metrics & Tracing)
	var metrics core.Metrics
	if cfg.metrics != nil {
		metrics = cfg.metrics
	} else if ctr.IsRegistered((*core.Metrics)(nil)) {
		metrics = ctr.MustResolve((*core.Metrics)(nil)).(core.Metrics)
	} else {
		metrics = &observability.NoopMetrics{}
	}
	ctr.RegisterInstance((*core.Metrics)(nil), metrics)

	var tracer observability.Tracer
	if cfg.tracer != nil {
		tracer = cfg.tracer
	} else if ctr.IsRegistered((*observability.Tracer)(nil)) {
		tracer = ctr.MustResolve((*observability.Tracer)(nil)).(observability.Tracer)
	} else {
		tracer = observability.DefaultNoopTracer()
	}
	ctr.RegisterInstance((*observability.Tracer)(nil), tracer)

	// Runtime check for local models if ModelPath is set
	if cfg.ModelPath != "" {

		e := &env.Environment{ModelDir: cfg.ModelPath, WorkDir: cfg.WorkDir}
		if ok, missing := e.Check(context.Background()); !ok {
			return nil, fmt.Errorf("environment pre-flight check failed: %v. Please run 'make models' or call gorag.PrepareEnvironment()", missing)
		}
	}

	// 0. Resolve or Register Embedding Provider first to align Dimension
	var embedder embedding.Provider
	if cfg.embedder != nil {
		embedder = cfg.embedder
	} else if ctr.IsRegistered((*embedding.Provider)(nil)) {
		embedder = ctr.MustResolve((*embedding.Provider)(nil)).(embedding.Provider)
	}

	// Align Dimension with Embedder if possible
	if embedder != nil && cfg.Dimension == 1536 {
		cfg.Dimension = embedder.Dimension()
	}

	var semanticEmbedder core.Embedder
	if embedder != nil {
		semanticEmbedder = &providerToEmbedderAdapter{provider: embedder}
	}

	// 1. Resolve or Register Persistence Singleton Resources
	var vStore core.VectorStore
	var dStore store.DocStore
	var gStore store.GraphStore
	var err error

	// Check if already registered (Pooling)
	if ctr.IsRegistered((*core.VectorStore)(nil)) {
		vStore = ctr.MustResolve((*core.VectorStore)(nil)).(core.VectorStore)
	} else {
		vecName := "vectors.db"
		colName := "gorag"
		if cfg.Name != "" {
			vecName = fmt.Sprintf("vectors_%s.db", cfg.Name)
			colName = cfg.Name
		}
		vStore, err = govector.NewStore(
			govector.WithDBPath(fmt.Sprintf("%s/%s", cfg.WorkDir, vecName)),
			govector.WithDimension(cfg.Dimension),
			govector.WithCollection(colName),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to init vector store: %w", err)
		}
		ctr.RegisterInstance((*core.VectorStore)(nil), vStore)
	}

	if ctr.IsRegistered((*store.DocStore)(nil)) {
		dStore = ctr.MustResolve((*store.DocStore)(nil)).(store.DocStore)
	} else {
		docName := "docs.db"
		if cfg.Name != "" {
			docName = fmt.Sprintf("docs_%s.db", cfg.Name)
		}
		dStore, err = sqlite.NewDocStore(fmt.Sprintf("%s/%s", cfg.WorkDir, docName))
		if err != nil {
			return nil, fmt.Errorf("failed to init doc store: %w", err)
		}
		ctr.RegisterInstance((*store.DocStore)(nil), dStore)
	}

	if mode == "graph" || mode == "agentic" {
		if ctr.IsRegistered((*store.GraphStore)(nil)) {
			gStore = ctr.MustResolve((*store.GraphStore)(nil)).(store.GraphStore)
		} else {
			graphName := "graph.bolt"
			if cfg.Name != "" {
				graphName = fmt.Sprintf("graph_%s.bolt", cfg.Name)
			}
			gStore, err = bolt.NewGraphStore(fmt.Sprintf("%s/%s", cfg.WorkDir, graphName))
			if err != nil {
				return nil, fmt.Errorf("failed to init graph store: %w", err)
			}
			ctr.RegisterInstance((*store.GraphStore)(nil), gStore)
		}
	}

	// 2. Indexer/Retriever Options
	idxOpts := []indexer.IndexerOption{
		indexer.WithName(cfg.Name),
		indexer.WithVectorStore(vStore),
		indexer.WithDocStore(dStore),
		indexer.WithMetrics(metrics),
	}

	if gStore != nil {
		idxOpts = append(idxOpts, indexer.WithGraph(gStore))
	}

	if embedder != nil {
		idxOpts = append(idxOpts, indexer.WithEmbedding(embedder))
	}

	if len(cfg.parsers) > 0 {
		idxOpts = append(idxOpts, indexer.WithParsers(cfg.parsers...))
	}

	var idx indexer.Indexer
	var ret core.Retriever

	switch mode {
	case "native":
		idx, err = indexer.DefaultNativeIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}
		ret, err = native.DefaultNativeRetriever(native.WithName(cfg.Name), native.WithVectorStore(vStore), native.WithTopK(cfg.TopK))
	case "advanced":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}
		ret, err = advanced.DefaultAdvancedRetriever(advanced.WithName(cfg.Name), advanced.WithStore(vStore), advanced.WithTopK(cfg.TopK))
	case "agentic":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}

		llm := cfg.llmClient
		if llm == nil && ctr.IsRegistered((*gochat.Client)(nil)) {
			llm = ctr.MustResolve((*gochat.Client)(nil)).(gochat.Client)
		}

		natRet, nerr := native.DefaultNativeRetriever(native.WithName(cfg.Name), native.WithVectorStore(vStore), native.WithTopK(cfg.TopK))
		if nerr != nil {
			return nil, nerr
		}
		advRet, aerr := advanced.DefaultAdvancedRetriever(advanced.WithName(cfg.Name), advanced.WithStore(vStore), advanced.WithTopK(cfg.TopK))
		if aerr != nil {
			return nil, aerr
		}
		grpRet, gerr := graph.DefaultGraphRetriever(graph.WithName(cfg.Name), graph.WithVectorStore(vStore), graph.WithGraphStore(gStore), graph.WithTopK(cfg.TopK))
		if gerr != nil {
			return nil, gerr
		}

		// Corrective RAG (CRAG) with WebSearcher fallback
		var webSearcher core.WebSearcher
		if cfg.webSearcher != nil {
			webSearcher = cfg.webSearcher
		} else if ctr.IsRegistered((*core.WebSearcher)(nil)) {
			webSearcher = ctr.MustResolve((*core.WebSearcher)(nil)).(core.WebSearcher)
		}
		cragRet := crag.NewRetriever(vStore, embedder, nil, llm, crag.WithWebSearcher(webSearcher), crag.WithTopK(cfg.TopK))

		// Self-RAG with Reflection Evaluator
		selfRet := selfrag.NewRetriever(vStore, embedder, nil, llm, selfrag.WithTopK(cfg.TopK), selfrag.WithThreshold(cfg.SelfRAGThreshold), selfrag.WithMaxRetries(cfg.MaxRetries))

		ret = agentic.NewSmartRouter(agentic.NewLLMClassifier(llm), map[core.IntentType]core.Retriever{
			core.IntentChat:           natRet,
			core.IntentFactCheck:      selfRet, // Self-RAG for high precision
			core.IntentRelational:     grpRet,  // GraphRAG for entities
			core.IntentDomainSpecific: cragRet, // CRAG for domain specific (likely needs update)
			core.IntentGlobal:         advRet,  // AdvancedRAG for global knowledge
		}, natRet, nil)
	case "graph":
		idx, err = indexer.DefaultGraphIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}
		ret, err = graph.DefaultGraphRetriever(graph.WithName(cfg.Name), graph.WithVectorStore(vStore), graph.WithGraphStore(gStore), graph.WithTopK(cfg.TopK), graph.WithDepth(cfg.GraphDepth), graph.WithLimit(cfg.GraphLimit))
	case "selfrag":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}
		llm := cfg.llmClient
		if llm == nil && ctr.IsRegistered((*gochat.Client)(nil)) {
			llm = ctr.MustResolve((*gochat.Client)(nil)).(gochat.Client)
		}
		ret = selfrag.NewRetriever(vStore, embedder, nil, llm, selfrag.WithTopK(cfg.TopK), selfrag.WithThreshold(cfg.SelfRAGThreshold), selfrag.WithMaxRetries(cfg.MaxRetries))
	case "crag":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}
		llm := cfg.llmClient
		if llm == nil && ctr.IsRegistered((*gochat.Client)(nil)) {
			llm = ctr.MustResolve((*gochat.Client)(nil)).(gochat.Client)
		}
		var webSearcher core.WebSearcher
		if cfg.webSearcher != nil {
			webSearcher = cfg.webSearcher
		} else if ctr.IsRegistered((*core.WebSearcher)(nil)) {
			webSearcher = ctr.MustResolve((*core.WebSearcher)(nil)).(core.WebSearcher)
		}
		ret = crag.NewRetriever(vStore, embedder, nil, llm, crag.WithWebSearcher(webSearcher), crag.WithTopK(cfg.TopK))
	}

	if err != nil {
		return nil, err
	}

	// 3. Wrap retriever with semantic cache if enabled
	if cfg.EnableSemanticCache && semanticEmbedder != nil {
		var semanticCache core.SemanticCache
		cacheFileName := "cache.db"
		if cfg.Name != "" {
			cacheFileName = fmt.Sprintf("cache_%s.db", cfg.Name)
		}
		cachePath := fmt.Sprintf("%s/%s", cfg.WorkDir, cacheFileName)

		switch cfg.SemanticCacheType {
		case "bolt":
			semanticCache, err = cache.NewBoltSemanticCache(
				semanticEmbedder,
				cache.WithBoltDBPath(cachePath),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create bolt semantic cache: %w", err)
			}
		default: // "memory"
			semanticCache = cache.NewInMemorySemanticCache(
				semanticEmbedder,
				cache.WithDBPath(cachePath),
			)
		}

		ret = cache.NewRetrieverWithCache(ret, semanticCache, logging.DefaultNoopLogger())
	}

	return &defaultRAG{
		mode:      mode,
		indexer:   idx,
		retriever: ret,
		metrics:   metrics,
		container: ctr,
	}, nil
}
