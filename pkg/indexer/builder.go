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
	"github.com/DotNetAge/gorag/pkg/indexing/parser/config/types"
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

// IndexerOption defines a function to configure the indexer.
type IndexerOption func(*defaultIndexer)

// WithConcurrency enables or disables concurrent indexing.
func WithConcurrency(enabled bool) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.config.Concurrency = enabled
	}
}

// WithWorkers sets the number of workers for concurrent indexing.
func WithWorkers(workers int) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.config.Workers = workers
	}
}

// WithParsers sets the parsers to use.
func WithParsers(parsers ...core.Parser) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.parsers = parsers
	}
}

// WithAllParsers enables all available parsers.
func WithAllParsers() IndexerOption {
	return func(idx *defaultIndexer) {
		all := types.AllParsers()
		parsers := make([]core.Parser, len(all))
		for i, p := range all {
			parsers[i] = p
		}
		idx.parsers = parsers
	}
}

// WithWatchDir adds directories to watch for changes.
func WithWatchDir(dirs ...string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.watchDirs = append(idx.watchDirs, dirs...)
	}
}

// WithStore sets the vector and document stores.
func WithStore(vectorStore core.VectorStore, docStore store.DocStore) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore = vectorStore
		idx.docStore = docStore
	}
}

// WithGraph sets the graph store.
func WithGraph(graphStore store.GraphStore) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.graphStore = graphStore
	}
}

// WithEmbedding sets the embedding provider.
func WithEmbedding(embedder embedding.Provider) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.embedder = embedder
	}
}

// WithMetrics sets the metrics recorder.
func WithMetrics(metrics core.Metrics) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.metrics = metrics
	}
}

// WithLogger sets the logger.
func WithLogger(logger logging.Logger) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.logger = logger
	}
}

// WithChunker sets the semantic chunker.
func WithChunker(chunker core.SemanticChunker) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.chunker = chunker
	}
}

// WithExtractor sets the entity extractor.
func WithExtractor(extractor core.EntityExtractor) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.extractor = extractor
	}
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
		logger = logging.NewNoopLogger()
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
		logger = logging.NewNoopLogger()
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

// DefaultIndexer creates a default indexer with options.
func DefaultIndexer(opts ...IndexerOption) Indexer {
	idx := &defaultIndexer{
		logger: logging.NewNoopLogger(),
		config: Config{
			Concurrency: true,
			Workers:     10,
		},
	}

	for _, opt := range opts {
		opt(idx)
	}

	return idx
}
