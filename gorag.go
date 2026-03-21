package gorag

import (
	"context"
	"fmt"
	"os"

	"github.com/DotNetAge/gochat/pkg/embedding"
	gochat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/di"
	"github.com/DotNetAge/gorag/pkg/indexer"
	"github.com/DotNetAge/gorag/pkg/indexing/store/sqlite"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/retriever/advanced"
	"github.com/DotNetAge/gorag/pkg/retriever/agentic"
	"github.com/DotNetAge/gorag/pkg/retriever/graph"
	"github.com/DotNetAge/gorag/pkg/retriever/native"
)

// RAGConfig is the single source of truth for all RAG modes.
type RAGConfig struct {
	// 1. Persistence
	WorkDir      string
	VectorDBType string // "govector", "milvus"
	Dimension    int

	// 2. Model "Grounding" (Variable Name Driven)
	EmbedderType string // "openai", "ollama", "local-onnx"
	LLMType      string // "openai", "claude", "ollama"
	ModelPath    string // Local model file path (.test/models/...)
	ModelName    string // e.g., "qwen-turbo", "bge-small-zh-v1.5"
	APIKeyEnv    string // Name of the env var, e.g., "DASHSCOPE_API_KEY"
	BaseURL      string // e.g., "https://dashscope.aliyuncs.com/compatible-mode/"

	// 3. Retrieval Tuning
	TopK int

	// 4. Internal Component Injection
	embedder  embedding.Provider
	llmClient gochat.Client
	parsers   []core.Parser
	container *di.Container
}

type RAGOption func(*RAGConfig)

func WithWorkDir(path string) RAGOption   { return func(c *RAGConfig) { c.WorkDir = path } }
func WithModelPath(path string) RAGOption { return func(c *RAGConfig) { c.ModelPath = path } }
func WithAPIKeyEnv(name string) RAGOption { return func(c *RAGConfig) { c.APIKeyEnv = name } }
func WithBaseURL(url string) RAGOption    { return func(c *RAGConfig) { c.BaseURL = url } }
func WithModelName(name string) RAGOption { return func(c *RAGConfig) { c.ModelName = name } }
func WithDimension(dim int) RAGOption     { return func(c *RAGConfig) { c.Dimension = dim } }
func WithTopK(k int) RAGOption            { return func(c *RAGConfig) { c.TopK = k } }

// Expert injection
func WithEmbedder(e embedding.Provider) RAGOption { return func(c *RAGConfig) { c.embedder = e } }
func WithLLM(l gochat.Client) RAGOption           { return func(c *RAGConfig) { c.llmClient = l } }
func WithParsers(p ...core.Parser) RAGOption      { return func(c *RAGConfig) { c.parsers = p } }
func WithContainer(ctr *di.Container) RAGOption   { return func(c *RAGConfig) { c.container = ctr } }

// RAG application interface
type RAG interface {
	IndexFile(ctx context.Context, filePath string) error
	IndexDirectory(ctx context.Context, dirPath string, recursive bool) error
	Search(ctx context.Context, query string, topK int) (*core.RetrievalResult, error)
	Container() *di.Container
	Close() error
}

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

func DefaultNativeRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("native", opts...)
}

func DefaultAdvancedRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("advanced", opts...)
}

func DefaultAgenticRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("agentic", opts...)
}

func DefaultGraphRAG(opts ...RAGOption) (RAG, error) {
	return buildRAGWithDefaults("graph", opts...)
}

// buildRAGWithDefaults applies preset defaults before building the RAG instance
func buildRAGWithDefaults(mode string, opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{
		WorkDir:      "./data",
		VectorDBType: "govector",
		Dimension:    1536,
		TopK:         5,
	}

	// Mode-specific defaults
	switch mode {
	case "advanced":
		cfg.TopK = 10
	case "agentic":
		cfg.TopK = 5
	}

	for _, opt := range opts {
		opt(cfg)
	}
	return buildRAG(cfg, mode)
}

// buildRAG factory leveraging DI for resource pooling
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
	
	if cfg.embedder != nil {
		idxOpts = append(idxOpts, indexer.WithEmbedding(cfg.embedder))
	} else if ctr.IsRegistered((*embedding.Provider)(nil)) {
		idxOpts = append(idxOpts, indexer.WithEmbedding(ctr.MustResolve((*embedding.Provider)(nil)).(embedding.Provider)))
	}

	if len(cfg.parsers) > 0 {
		idxOpts = append(idxOpts, indexer.WithParsers(cfg.parsers...))
	}

	var idx indexer.Indexer
	var ret core.Retriever

	switch mode {
	case "native":
		idx, err = indexer.DefaultNativeIndexer(idxOpts...)
		if err != nil { return nil, err }
		ret, err = native.DefaultNativeRetriever(native.WithVectorStore(vStore), native.WithTopK(cfg.TopK))
	case "advanced":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		if err != nil { return nil, err }
		ret, err = advanced.DefaultAdvancedRetriever(advanced.WithStore(vStore), advanced.WithTopK(cfg.TopK))
	case "agentic":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		if err != nil { return nil, err }
		
		natRet, nerr := native.DefaultNativeRetriever(native.WithVectorStore(vStore), native.WithTopK(cfg.TopK))
		if nerr != nil { return nil, nerr }
		advRet, aerr := advanced.DefaultAdvancedRetriever(advanced.WithStore(vStore), advanced.WithTopK(cfg.TopK))
		if aerr != nil { return nil, aerr }
		grpRet, gerr := graph.DefaultGraphRetriever(graph.WithVectorStore(vStore), graph.WithTopK(cfg.TopK))
		if gerr != nil { return nil, gerr }
		
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
		if err != nil { return nil, err }
		ret, err = graph.DefaultGraphRetriever(graph.WithVectorStore(vStore), graph.WithTopK(cfg.TopK))
	}

	if err != nil { return nil, err }
	return &defaultRAG{indexer: idx, retriever: ret, container: ctr}, nil
}
