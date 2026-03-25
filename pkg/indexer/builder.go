// Package indexer provides high-level indexers for building RAG pipelines.
//
// This package offers pre-configured indexer implementations:
//   - DefaultNativeIndexer: Lightweight, local-first indexer for prototyping
//   - DefaultAdvancedIndexer: High-performance indexer for production use
//   - DefaultGraphIndexer: Knowledge graph-enhanced indexer
//   - NewVectorIndexer: Custom vector-based indexer
//   - NewMultimodalGraphIndexer: Multimodal and graph-capable indexer
package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/indexing"
	"github.com/DotNetAge/gorag/pkg/indexing/chunker"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/config/types"
	"github.com/DotNetAge/gorag/pkg/indexing/store/bolt"
	"github.com/DotNetAge/gorag/pkg/indexing/store/sqlite"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/logging"
	stepinx "github.com/DotNetAge/gorag/pkg/steps/indexing"
	"golang.org/x/sync/errgroup"
)

// Indexer is the unified interface for document indexing.
// It provides methods for processing files and directories into vector/graph stores.
type Indexer interface {
	indexing.Indexer
	Init() error
	Start() error
}

type defaultIndexer struct {
	name        string
	pipeline    *pipeline.Pipeline[*core.IndexingContext]
	config      Config
	logger      logging.Logger
	registry    *types.ParserRegistry
	watchDirs   []string
	vectorStore core.VectorStore
	docStore    store.DocStore
	graphStore  store.GraphStore
	chunker     core.SemanticChunker
	embedder    embedding.Provider
	extractor   core.EntityExtractor
	metrics     core.Metrics
}

// Config defines the configuration for the indexer.
// It controls concurrency and worker pool settings for parallel document processing.
type Config struct {
	Concurrency bool
	Workers     int
}

func (idx *defaultIndexer) Init() error {
	if idx.pipeline != nil {
		return nil
	}

	p := pipeline.New[*core.IndexingContext]()
	p.AddSteps(stepinx.Discover())

	if idx.registry != nil {
		p.AddSteps(stepinx.MultiFactory(idx.registry))
	}

	if idx.chunker != nil {
		p.AddSteps(stepinx.Chunk(idx.chunker))
	}

	if idx.embedder != nil {
		if multimodal, ok := idx.embedder.(embedding.MultimodalProvider); ok {
			p.AddSteps(stepinx.MultimodalEmbed(multimodal, idx.metrics))
		} else {
			p.AddSteps(stepinx.Batch(idx.embedder, idx.metrics))
		}
	}

	if idx.extractor != nil {
		p.AddSteps(stepinx.Entities(idx.extractor, idx.logger))
	}

	if idx.vectorStore != nil || idx.docStore != nil || idx.graphStore != nil {
		p.AddSteps(stepinx.MultiStore(idx.vectorStore, idx.docStore, idx.graphStore, idx.logger, idx.metrics))
	}

	idx.pipeline = p
	return nil
}

func (idx *defaultIndexer) Start() error {
	if err := idx.Init(); err != nil {
		return err
	}

	if len(idx.watchDirs) == 0 {
		return nil
	}

	watcher, err := indexing.NewFileWatcher(idx, idx.logger)
	if err != nil {
		return err
	}

	for _, dir := range idx.watchDirs {
		watcher.AddConfigs(indexing.WatchConfig{
			Path:      dir,
			Recursive: true,
		})
	}

	return watcher.Start()
}

func (idx *defaultIndexer) IndexFile(ctx context.Context, filePath string) (*core.IndexingContext, error) {
	if idx.pipeline == nil {
		if err := idx.Init(); err != nil {
			return nil, err
		}
	}
	state := core.NewIndexingContext(ctx, filePath)
	err := idx.pipeline.Execute(ctx, state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (idx *defaultIndexer) IndexDirectory(ctx context.Context, dirPath string, recursive bool) error {
	workers := idx.config.Workers
	if workers <= 0 {
		workers = 10
	}

	// Non-concurrent: Simple sequential processing
	if !idx.config.Concurrency {
		return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if path != dirPath && !recursive {
					return filepath.SkipDir
				}
				return nil
			}
			// Skip hidden files
			if strings.HasPrefix(info.Name(), ".") {
				return nil
			}
			if _, err := idx.IndexFile(ctx, path); err != nil {
				idx.logger.Error("failed to index file", err, map[string]interface{}{"path": path})
			}
			return nil
		})
	}

	// Concurrent: Streaming Producer-Consumer
	g, ctx := errgroup.WithContext(ctx)
	// Small buffer to prevent memory pressure while keeping workers busy
	fileChan := make(chan string, workers*2)

	// Start workers (Consumers)
	for i := 0; i < workers; i++ {
		g.Go(func() error {
			for file := range fileChan {
				if _, err := idx.IndexFile(ctx, file); err != nil {
					idx.logger.Error("failed to index file", err, map[string]interface{}{"path": file})
				}
			}
			return nil
		})
	}

	// Producer: Walk directory and stream files to channel
	g.Go(func() error {
		defer close(fileChan)
		return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if path != dirPath && !recursive {
					return filepath.SkipDir
				}
				return nil
			}
			// Skip hidden files
			if strings.HasPrefix(info.Name(), ".") {
				return nil
			}

			select {
			case fileChan <- path:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	})

	return g.Wait()
}

// NewVectorIndexer creates a simple text-vector pipeline for basic RAG setups.
//
// Parameters:
//   - parsers: list of document parsers
//   - chunker: semantic chunker for splitting documents
//   - embedder: embedding provider for vectorization
//   - vectorStore: vector storage backend
//   - docStore: document metadata storage
//   - logger: logging service
//   - metrics: observability metrics service
func NewVectorIndexer(
	parsers []core.Parser,
	chunker core.SemanticChunker,
	embedder embedding.Provider,
	vectorStore core.VectorStore,
	docStore store.DocStore,
	logger logging.Logger,
	metrics core.Metrics,
	opts ...IndexerOption,
) Indexer {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}

	registry := types.NewParserRegistry()
	for _, p := range parsers {
		parser := p // capture
		registry.Register(func() core.Parser { return parser })
	}

	idx := &defaultIndexer{
		logger:      logger,
		registry:    registry,
		chunker:     chunker,
		embedder:    embedder,
		vectorStore: vectorStore,
		docStore:    docStore,
		metrics:     metrics,
		config: Config{
			Concurrency: true,
			Workers:     10,
		},
	}

	for _, opt := range opts {
		opt(idx)
	}

	_ = idx.Init()
	return idx
}

// NewMultimodalGraphIndexer creates an advanced multimodal and graph pipeline.
// It supports both text and image inputs, with knowledge graph extraction capabilities.
//
// Parameters:
//   - parsers: list of document parsers
//   - chunker: semantic chunker for splitting documents
//   - embedder: multimodal embedding provider
//   - entityExtractor: entity extractor for graph construction
//   - vectorStore: vector storage backend
//   - docStore: document metadata storage
//   - graphStore: knowledge graph storage
//   - logger: logging service
//   - metrics: observability metrics service
func NewMultimodalGraphIndexer(
	parsers []core.Parser,
	chunker core.SemanticChunker,
	embedder embedding.MultimodalProvider,
	entityExtractor core.EntityExtractor,
	vectorStore core.VectorStore,
	docStore store.DocStore,
	graphStore store.GraphStore,
	logger logging.Logger,
	metrics core.Metrics,
	opts ...IndexerOption,
) (Indexer, error) {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}

	// Constraint: Multimodal pipeline MUST have GraphStore support
	if graphStore == nil {
		return nil, fmt.Errorf("multimodal pipeline requires GraphStore support to be successfully enabled")
	}
	if entityExtractor == nil {
		return nil, fmt.Errorf("multimodal pipeline requires EntityExtractor to map entities to GraphStore")
	}

	registry := types.NewParserRegistry()
	for _, p := range parsers {
		parser := p // capture
		registry.Register(func() core.Parser { return parser })
	}

	idx := &defaultIndexer{
		logger:      logger,
		registry:    registry,
		chunker:     chunker,
		embedder:    embedder,
		extractor:   entityExtractor,
		vectorStore: vectorStore,
		docStore:    docStore,
		graphStore:  graphStore,
		metrics:     metrics,
		config: Config{
			Concurrency: true,
			Workers:     10,
		},
	}

	for _, opt := range opts {
		opt(idx)
	}

	_ = idx.Init()
	return idx, nil
}

// DefaultNativeIndexer creates a light-weight, local-first Indexer.
// It uses default TokenChunker, local SQLite and GoVector stores, suitable for quick prototyping and testing.
func DefaultNativeIndexer(opts ...IndexerOption) (Indexer, error) {
	// 1. Set default internal state
	idx := &defaultIndexer{
		logger:  logging.DefaultNoopLogger(),
		registry: types.DefaultRegistry,
		config: Config{
			Concurrency: true,
			Workers:     10,
		},
	}

	// 2. Apply options
	for _, opt := range opts {
		opt(idx)
	}

	// 3. Fallback to defaults for missing components
	workDir := "./data"

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create default work directory: %w", err)
	}

	if idx.chunker == nil {
		tkChunker, err := chunker.DefaultTokenChunker()
		if err != nil {
			return nil, fmt.Errorf("failed to create default token chunker: %w", err)
		}
		idx.chunker = chunker.NewSemanticChunker(tkChunker, 1000, 250, 50)
	}

	if idx.vectorStore == nil {
		vecName := "gorag_vectors.db"
		if idx.name != "" {
			vecName = fmt.Sprintf("gorag_vectors_%s.db", idx.name)
		}
		vecPath := filepath.Join(workDir, vecName)
		dimension := 1536
		if idx.embedder != nil {
			dimension = idx.embedder.Dimension()
		}

		colName := "gorag"
		if idx.name != "" {
			colName = idx.name
		}

		vStore, err := govector.NewStore(
			govector.WithDBPath(vecPath),
			govector.WithDimension(dimension),
			govector.WithCollection(colName),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create default vector store: %w", err)
		}
		idx.vectorStore = vStore
	}

	if idx.docStore == nil {
		docFileName := "gorag_docs.db"
		if idx.name != "" {
			docFileName = fmt.Sprintf("gorag_docs_%s.db", idx.name)
		}
		docPath := filepath.Join(workDir, docFileName)
		dStore, err := sqlite.NewDocStore(docPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create default doc store: %w", err)
		}
		idx.docStore = dStore
	}

	if err := idx.Init(); err != nil {
		return nil, fmt.Errorf("failed to init indexer pipeline: %w", err)
	}

	return idx, nil
}

// DefaultAdvancedIndexer creates a high-performance Indexer preset for production use.
// It features increased worker concurrency and optimized defaults for enterprise workloads.
func DefaultAdvancedIndexer(opts ...IndexerOption) (Indexer, error) {
	idx := &defaultIndexer{
		logger:  logging.DefaultNoopLogger(),
		registry: types.DefaultRegistry,
		config: Config{
			Concurrency: true,
			Workers:     20, // Enterprise default
		},
	}

	for _, opt := range opts {
		opt(idx)
	}

	// Fallback logic
	if idx.vectorStore == nil {
		vecName := "gorag_vectors.db"
		if idx.name != "" {
			vecName = fmt.Sprintf("gorag_vectors_%s.db", idx.name)
		}
		dimension := 1536
		if idx.embedder != nil {
			dimension = idx.embedder.Dimension()
		}
		colName := "gorag"
		if idx.name != "" {
			colName = idx.name
		}

		vStore, err := govector.NewStore(
			govector.WithDBPath(vecName),
			govector.WithDimension(dimension),
			govector.WithCollection(colName),
		)
		if err != nil {
			return nil, err
		}
		idx.vectorStore = vStore
	}
	if idx.docStore == nil {
		docFileName := "gorag_docs.db"
		if idx.name != "" {
			docFileName = fmt.Sprintf("gorag_docs_%s.db", idx.name)
		}
		dStore, err := sqlite.NewDocStore(docFileName)
		if err != nil {
			return nil, err
		}
		idx.docStore = dStore
	}

	if err := idx.Init(); err != nil {
		return nil, err
	}

	return idx, nil
}

// DefaultGraphIndexer creates a Knowledge-Graph enabled Indexer preset.
// It integrates graph-based entity relationship extraction for complex query understanding.
func DefaultGraphIndexer(opts ...IndexerOption) (Indexer, error) {
	idx := &defaultIndexer{
		logger:  logging.DefaultNoopLogger(),
		registry: types.DefaultRegistry,
		config: Config{
			Concurrency: true,
			Workers:     10,
		},
	}

	for _, opt := range opts {
		opt(idx)
	}

	if idx.vectorStore == nil {
		vecName := "gorag_vectors.db"
		if idx.name != "" {
			vecName = fmt.Sprintf("gorag_vectors_%s.db", idx.name)
		}
		dimension := 1536
		if idx.embedder != nil {
			dimension = idx.embedder.Dimension()
		}
		colName := "gorag"
		if idx.name != "" {
			colName = idx.name
		}

		vStore, err := govector.NewStore(
			govector.WithDBPath(vecName),
			govector.WithDimension(dimension),
			govector.WithCollection(colName),
		)
		if err != nil {
			return nil, err
		}
		idx.vectorStore = vStore
	}
	if idx.docStore == nil {
		docFileName := "gorag_docs.db"
		if idx.name != "" {
			docFileName = fmt.Sprintf("gorag_docs_%s.db", idx.name)
		}
		dStore, err := sqlite.NewDocStore(docFileName)
		if err != nil {
			return nil, err
		}
		idx.docStore = dStore
	}

	if idx.graphStore == nil {
		graphName := "gorag_graph.bolt"
		if idx.name != "" {
			graphName = fmt.Sprintf("gorag_graph_%s.bolt", idx.name)
		}
		gStore, err := bolt.NewGraphStore(graphName)
		if err != nil {
			return nil, err
		}
		idx.graphStore = gStore
	}

	if err := idx.Init(); err != nil {
		return nil, err
	}

	return idx, nil
}

// DefaultIndexer is an alias for DefaultNativeIndexer, provided for backward compatibility.
func DefaultIndexer(opts ...IndexerOption) (Indexer, error) {
	return DefaultNativeIndexer(opts...)
}
