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

	gochat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/di"
	"github.com/DotNetAge/gorag/pkg/indexer"
	"github.com/DotNetAge/gorag/pkg/indexing/store/sqlite"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/retrieval/cache"
	"github.com/DotNetAge/gorag/pkg/retriever/advanced"
	"github.com/DotNetAge/gorag/pkg/retriever/agentic"
	"github.com/DotNetAge/gorag/pkg/retriever/graph"
	"github.com/DotNetAge/gorag/pkg/retriever/native"
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

	// 5. Internal Component Injection
	// embedder is the internally injected embedding provider
	embedder embedding.Provider
	// llmClient is the internally injected LLM client
	llmClient gochat.Client
	// parsers is the list of document parsers
	parsers []core.Parser
	// container is the dependency injection container
	container *di.Container
}

// RAGOption is a function type for configuring RAG instances using the functional options pattern.
type RAGOption func(*RAGConfig)

// WithWorkDir sets the working directory for persistent storage.
func WithWorkDir(path string) RAGOption { return func(c *RAGConfig) { c.WorkDir = path } }

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
	indexer   indexer.Indexer
	retriever core.Retriever
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
	results, err := r.retriever.Retrieve(ctx, []string{query}, topK)
	if err != nil || len(results) == 0 {
		return nil, err
	}
	return results[0], nil
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

// buildRAGWithDefaults applies preset defaults before building the RAG instance.
// It configures mode-specific parameters and then delegates to buildRAG for construction.
func buildRAGWithDefaults(mode string, opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{
		WorkDir:             "./data",
		VectorDBType:        "govector",
		Dimension:           1536,
		TopK:                5,
		EnableSemanticCache: false,
		SemanticCacheType:   "memory",
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

	// Check if directory is actually writable
	testFile := fmt.Sprintf("%s/.write_test", cfg.WorkDir)
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return nil, fmt.Errorf("pre-flight check failed: work directory %q is not writable: %w", cfg.WorkDir, err)
	}
	_ = os.Remove(testFile)

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
	var err error

	// Check if already registered (Pooling)
	if ctr.IsRegistered((*core.VectorStore)(nil)) {
		vStore = ctr.MustResolve((*core.VectorStore)(nil)).(core.VectorStore)
	} else {
		vStore, err = govector.NewStore(govector.WithDBPath(fmt.Sprintf("%s/vectors.db", cfg.WorkDir)), govector.WithDimension(cfg.Dimension))
		if err != nil {
			return nil, fmt.Errorf("failed to init vector store: %w", err)
		}
		ctr.RegisterInstance((*core.VectorStore)(nil), vStore)
	}

	if ctr.IsRegistered((*store.DocStore)(nil)) {
		dStore = ctr.MustResolve((*store.DocStore)(nil)).(store.DocStore)
	} else {
		dStore, err = sqlite.NewDocStore(fmt.Sprintf("%s/docs.db", cfg.WorkDir))
		if err != nil {
			return nil, fmt.Errorf("failed to init doc store: %w", err)
		}
		ctr.RegisterInstance((*store.DocStore)(nil), dStore)
	}

	// 2. Indexer/Retriever Options
	idxOpts := []indexer.IndexerOption{
		indexer.WithVectorStore(vStore),
		indexer.WithDocStore(dStore),
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
		ret, err = native.DefaultNativeRetriever(native.WithVectorStore(vStore), native.WithTopK(cfg.TopK))
	case "advanced":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}
		ret, err = advanced.DefaultAdvancedRetriever(advanced.WithStore(vStore), advanced.WithTopK(cfg.TopK))
	case "agentic":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}

		natRet, nerr := native.DefaultNativeRetriever(native.WithVectorStore(vStore), native.WithTopK(cfg.TopK))
		if nerr != nil {
			return nil, nerr
		}
		advRet, aerr := advanced.DefaultAdvancedRetriever(advanced.WithStore(vStore), advanced.WithTopK(cfg.TopK))
		if aerr != nil {
			return nil, aerr
		}
		grpRet, gerr := graph.DefaultGraphRetriever(graph.WithVectorStore(vStore), graph.WithTopK(cfg.TopK))
		if gerr != nil {
			return nil, gerr
		}

		llm := cfg.llmClient
		if llm == nil && ctr.IsRegistered((*gochat.Client)(nil)) {
			llm = ctr.MustResolve((*gochat.Client)(nil)).(gochat.Client)
		}

		ret = agentic.NewSmartRouter(agentic.NewLLMClassifier(llm), map[core.IntentType]core.Retriever{
			core.IntentChat:           natRet,
			core.IntentFactCheck:      advRet,
			core.IntentRelational:     grpRet,
			core.IntentDomainSpecific: advRet,
		}, natRet, nil)
	case "graph":
		idx, err = indexer.DefaultGraphIndexer(idxOpts...)
		if err != nil {
			return nil, err
		}
		ret, err = graph.DefaultGraphRetriever(graph.WithVectorStore(vStore), graph.WithTopK(cfg.TopK))
	}

	if err != nil {
		return nil, err
	}

	// 3. Wrap retriever with semantic cache if enabled
	if cfg.EnableSemanticCache && semanticEmbedder != nil {
		var semanticCache core.SemanticCache
		cachePath := fmt.Sprintf("%s/cache.db", cfg.WorkDir)

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

	return &defaultRAG{indexer: idx, retriever: ret, container: ctr}, nil
}
