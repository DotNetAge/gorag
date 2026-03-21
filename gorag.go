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

// --- The Aligned Presets ---

func DefaultNativeRAG(opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{WorkDir: "./data", VectorDBType: "govector", Dimension: 1536, TopK: 5}
	for _, opt := range opts { opt(cfg) }
	return buildRAG(cfg, "native")
}

func DefaultAdvancedRAG(opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{WorkDir: "./data", VectorDBType: "govector", Dimension: 1536, TopK: 10}
	for _, opt := range opts { opt(cfg) }
	return buildRAG(cfg, "advanced")
}

func DefaultAgenticRAG(opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{WorkDir: "./data", VectorDBType: "govector", Dimension: 1536, TopK: 5}
	for _, opt := range opts { opt(cfg) }
	return buildRAG(cfg, "agentic")
}

func DefaultGraphRAG(opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{WorkDir: "./data", VectorDBType: "govector", Dimension: 1536, TopK: 5}
	for _, opt := range opts { opt(cfg) }
	return buildRAG(cfg, "graph")
}

// buildRAG factory leveraging DI for resource pooling
func buildRAG(cfg *RAGConfig, mode string) (RAG, error) {
	if cfg.container == nil {
		cfg.container = di.New()
	}
	ctr := cfg.container

	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return nil, err
	}

	// 1. Resolve or Register Persistence Singleton Resources
	var vStore core.VectorStore
	var dStore store.DocStore

	// Check if already registered (Pooling)
	if ctr.IsRegistered((*core.VectorStore)(nil)) {
		vStore = ctr.MustResolve((*core.VectorStore)(nil)).(core.VectorStore)
	} else {
		vStore, _ = govector.NewStore(govector.WithDBPath(fmt.Sprintf("%s/vectors.db", cfg.WorkDir)), govector.WithDimension(cfg.Dimension))
		ctr.RegisterInstance((*core.VectorStore)(nil), vStore)
	}

	if ctr.IsRegistered((*store.DocStore)(nil)) {
		dStore = ctr.MustResolve((*store.DocStore)(nil)).(store.DocStore)
	} else {
		dStore, _ = sqlite.NewDocStore(fmt.Sprintf("%s/docs.db", cfg.WorkDir))
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
	var err error

	switch mode {
	case "native":
		idx, err = indexer.DefaultNativeIndexer(idxOpts...)
		ret, _ = native.DefaultNativeRetriever(native.WithVectorStore(vStore), native.WithTopK(cfg.TopK))
	case "advanced":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		ret, _ = advanced.DefaultAdvancedRetriever(advanced.WithStore(vStore), advanced.WithTopK(cfg.TopK))
	case "agentic":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		natRet, _ := native.DefaultNativeRetriever(native.WithVectorStore(vStore), native.WithTopK(cfg.TopK))
		advRet, _ := advanced.DefaultAdvancedRetriever(advanced.WithStore(vStore), advanced.WithTopK(cfg.TopK))
		grpRet, _ := graph.DefaultGraphRetriever(graph.WithVectorStore(vStore), graph.WithTopK(cfg.TopK))
		
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
		ret, _ = graph.DefaultGraphRetriever(graph.WithVectorStore(vStore), graph.WithTopK(cfg.TopK))
	}

	if err != nil { return nil, err }
	return &defaultRAG{indexer: idx, retriever: ret, container: ctr}, nil
}
