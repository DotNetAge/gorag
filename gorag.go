package gorag

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/indexer"
	"github.com/DotNetAge/gorag/pkg/indexing/store/sqlite"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/retriever/native"
	"github.com/DotNetAge/gorag/pkg/retriever/advanced"
	"github.com/DotNetAge/gorag/pkg/retriever/graph"
)

// RAGConfig is the flat, primitive config for the entire RAG app.
type RAGConfig struct {
	WorkDir      string
	VectorDBType string // "govector", "milvus"
	GraphDBType  string // "neo4j", "sqlite"
	EmbedderType string
	LLMType      string
	APIKey       string
	Collection   string
	Dimension    int
	TopK         int
}

type RAGOption func(*RAGConfig)

func WithWorkDir(path string) RAGOption { return func(c *RAGConfig) { c.WorkDir = path } }
func WithAPIKey(key string) RAGOption   { return func(c *RAGConfig) { c.APIKey = key } }

// RAG application interface
type RAG interface {
	IndexFile(ctx context.Context, filePath string) error
	IndexDirectory(ctx context.Context, dirPath string, recursive bool) error
	Search(ctx context.Context, query string, topK int) (*core.RetrievalResult, error)
}

type defaultRAG struct {
	indexer   indexer.Indexer
	retriever core.Retriever
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

// DefaultNativeRAG preset
func DefaultNativeRAG(opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{
		WorkDir:      "./data",
		VectorDBType: "govector",
		Dimension:    1536,
		TopK:         5,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return buildRAG(cfg, "native")
}

// DefaultAdvancedRAG preset
func DefaultAdvancedRAG(opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{
		WorkDir:      "./data",
		VectorDBType: "govector", // Default to local for easy start
		Dimension:    1536,
		TopK:         10,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return buildRAG(cfg, "advanced")
}

// DefaultGraphRAG preset
func DefaultGraphRAG(opts ...RAGOption) (RAG, error) {
	cfg := &RAGConfig{
		WorkDir:      "./data",
		VectorDBType: "govector",
		Dimension:    1536,
		TopK:         5,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return buildRAG(cfg, "graph")
}

// buildRAG is the "Grand Factory" with full resource management.
func buildRAG(cfg *RAGConfig, mode string) (RAG, error) {
	// 1. Initialize core resources as Singletons for this RAG app
	var vStore core.VectorStore
	var dStore store.DocStore
	var err error

	// 1.1 Store Initialization
	switch cfg.VectorDBType {
	case "govector":
		vecPath := fmt.Sprintf("%s/vectors.db", cfg.WorkDir)
		vStore, err = govector.NewStore(govector.WithDBPath(vecPath), govector.WithDimension(cfg.Dimension))
		if err != nil { return nil, err }
		
		dStore, err = sqlite.NewDocStore(fmt.Sprintf("%s/docs.db", cfg.WorkDir))
		if err != nil { return nil, err }
	case "milvus":
		// milvus.NewStore(...)
	}

	// 2. Indexer Options
	idxOpts := []indexer.IndexerOption{
		indexer.WithVectorStore(vStore),
		indexer.WithDocStore(dStore),
		indexer.WithZapLogger(fmt.Sprintf("%s/gorag.log", cfg.WorkDir), 100, 30, 7, false),
	}

	// 3. Retriever Options (Cross-package normalization)
	var idx indexer.Indexer
	var ret core.Retriever

	switch mode {
	case "native":
		idx, err = indexer.DefaultNativeIndexer(idxOpts...)
		ret, err = native.DefaultNativeRetriever(
			native.WithVectorStore(vStore), // FORCE SHARED STORE
			native.WithTopK(cfg.TopK),
		)
	case "advanced":
		idx, err = indexer.DefaultAdvancedIndexer(idxOpts...)
		ret, err = advanced.DefaultAdvancedRetriever(
			advanced.WithStore(vStore), // FORCE SHARED STORE
			advanced.WithTopK(cfg.TopK),
		)
	case "graph":
		idx, err = indexer.DefaultGraphIndexer(idxOpts...)
		// Graph needs both vector and graph stores
		ret, err = graph.DefaultGraphRetriever(
			graph.WithVectorStore(vStore),
			graph.WithTopK(cfg.TopK),
		)
	}

	if err != nil {
		return nil, err
	}

	return &defaultRAG{indexer: idx, retriever: ret}, nil
}
