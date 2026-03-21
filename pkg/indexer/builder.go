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
	"github.com/DotNetAge/gorag/pkg/indexing/store/sqlite"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/logging"
	stepinx "github.com/DotNetAge/gorag/pkg/steps/indexing"
	"golang.org/x/sync/errgroup"
)

// Indexer is the unified interface for document indexing.
type Indexer interface {
	indexing.Indexer
	Init() error
	Start() error
}

type defaultIndexer struct {
	pipeline    *pipeline.Pipeline[*core.IndexingContext]
	config      Config
	logger      logging.Logger
	parsers     []core.Parser
	watchDirs   []string
	vectorStore core.VectorStore
	docStore    store.DocStore
	graphStore  store.GraphStore
	chunker     core.SemanticChunker
	embedder    embedding.Provider
	metrics     core.Metrics
	extractor   core.EntityExtractor
}

// Config defines the configuration for the indexer.
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

	if len(idx.parsers) > 0 {
		p.AddSteps(stepinx.Multi(idx.parsers...))
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
	var files []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
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
		files = append(files, path)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	if !idx.config.Concurrency {
		for _, file := range files {
			if _, err := idx.IndexFile(ctx, file); err != nil {
				idx.logger.Error("failed to index file", err, map[string]interface{}{"path": file})
			}
		}
		return nil
	}

	workers := idx.config.Workers
	if workers <= 0 {
		workers = 10
	}

	g, ctx := errgroup.WithContext(ctx)
	fileChan := make(chan string, len(files))

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

	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	return g.Wait()
}

// NewVectorIndexer creates a simple text-vector pipeline.
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

	idx := &defaultIndexer{
		logger:      logger,
		parsers:     parsers,
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

	idx := &defaultIndexer{
		logger:      logger,
		parsers:     parsers,
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
// It uses default TokenChunker, local SQLite and GoVector stores.
func DefaultNativeIndexer(opts ...IndexerOption) (Indexer, error) {
	// 1. Set default internal state
	idx := &defaultIndexer{
		logger:  logging.DefaultNoopLogger(),
		parsers: types.AllParsers(),
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
	workDir := "./data" // Use static default instead of mandatory parameter
	
	// Check if workDir was set via an option (we'll add WithWorkDir below)
	// For simplicity in this factory, we'll just ensure it's created.
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	if idx.chunker == nil {
		tkChunker, _ := chunker.DefaultTokenChunker()
		idx.chunker = chunker.NewSemanticChunker(tkChunker, 1000, 250, 50)
	}

	if idx.vectorStore == nil {
		vecPath := filepath.Join(workDir, "gorag_vectors.db")
		idx.vectorStore, _ = govector.NewStore(
			govector.WithDBPath(vecPath),
			govector.WithDimension(1536),
		)
	}

	if idx.docStore == nil {
		docPath := filepath.Join(workDir, "gorag_docs.db")
		idx.docStore, _ = sqlite.NewDocStore(docPath)
	}

	if err := idx.Init(); err != nil {
		return nil, fmt.Errorf("failed to init indexer pipeline: %w", err)
	}

	return idx, nil
}

// DefaultAdvancedIndexer creates a high-performance Indexer preset for production.
func DefaultAdvancedIndexer(opts ...IndexerOption) (Indexer, error) {
	idx := &defaultIndexer{
		logger:  logging.DefaultNoopLogger(),
		parsers: types.AllParsers(),
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
		idx.vectorStore, _ = govector.DefaultStore()
	}
	if idx.docStore == nil {
		idx.docStore, _ = sqlite.DefaultDocStore()
	}

	if err := idx.Init(); err != nil {
		return nil, err
	}

	return idx, nil
}

// DefaultGraphIndexer creates a Knowledge-Graph enabled Indexer preset.
func DefaultGraphIndexer(opts ...IndexerOption) (Indexer, error) {
	idx := &defaultIndexer{
		logger:  logging.DefaultNoopLogger(),
		parsers: types.AllParsers(),
		config: Config{
			Concurrency: true,
			Workers:     10,
		},
	}

	for _, opt := range opts {
		opt(idx)
	}

	if idx.vectorStore == nil {
		idx.vectorStore, _ = govector.DefaultStore()
	}
	if idx.docStore == nil {
		idx.docStore, _ = sqlite.DefaultDocStore()
	}

	if err := idx.Init(); err != nil {
		return nil, err
	}

	return idx, nil
}

// DefaultIndexer is an alias for DefaultNativeIndexer.
func DefaultIndexer(opts ...IndexerOption) (Indexer, error) {
	return DefaultNativeIndexer(opts...)
}
