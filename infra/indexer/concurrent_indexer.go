package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"golang.org/x/sync/errgroup"
)

// ensure interface implementation
var _ dataprep.Indexer = (*ConcurrentIndexer)(nil)

// ConcurrentIndexer implements the Next-Gen Indexing Pipeline using Goroutines and Channels.
// It relies strictly on the interfaces defined in pkg/usecase/dataprep and pkg/domain/abstraction.
type ConcurrentIndexer struct {
	parser      dataprep.Parser
	chunker     dataprep.SemanticChunker // we use the advanced chunker for parent-child features
	embedder    embedding.Provider       // directly depends on gochat
	vectorStore abstraction.VectorStore
	graphStore  abstraction.GraphStore  // Optional: can be nil if GraphRAG is disabled
	extractor   dataprep.GraphExtractor // Optional: can be nil if GraphRAG is disabled
	metrics     abstraction.Metrics     // Optional: metrics collector

	parseWorkers  int
	embedWorkers  int
	upsertWorkers int
}

// ConcurrentIndexerOption configures a ConcurrentIndexer instance.
type ConcurrentIndexerOption func(*ConcurrentIndexer)

// WithGraphSupport enables optional GraphRAG by setting a graph store and extractor.
func WithGraphSupport(graphStore abstraction.GraphStore, extractor dataprep.GraphExtractor) ConcurrentIndexerOption {
	return func(idx *ConcurrentIndexer) {
		idx.graphStore = graphStore
		idx.extractor = extractor
	}
}

// WithParseWorkers overrides the number of file-parsing goroutines.
func WithParseWorkers(n int) ConcurrentIndexerOption {
	return func(idx *ConcurrentIndexer) {
		if n > 0 {
			idx.parseWorkers = n
		}
	}
}

// WithEmbedWorkers overrides the number of embedding goroutines.
func WithEmbedWorkers(n int) ConcurrentIndexerOption {
	return func(idx *ConcurrentIndexer) {
		if n > 0 {
			idx.embedWorkers = n
		}
	}
}

// WithUpsertWorkers overrides the number of vector-store upsert goroutines.
func WithUpsertWorkers(n int) ConcurrentIndexerOption {
	return func(idx *ConcurrentIndexer) {
		if n > 0 {
			idx.upsertWorkers = n
		}
	}
}

// WithIndexerMetrics sets the metrics collector for the indexer.
func WithIndexerMetrics(metrics abstraction.Metrics) ConcurrentIndexerOption {
	return func(idx *ConcurrentIndexer) {
		if metrics != nil {
			idx.metrics = metrics
		}
	}
}

// NewConcurrentIndexer creates a highly concurrent, channel-based indexing pipeline.
//
// Required: parser, chunker, embedder, vectorStore.
// Optional (via options): WithGraphSupport, WithParseWorkers, WithEmbedWorkers, WithUpsertWorkers.
func NewConcurrentIndexer(
	parser dataprep.Parser,
	chunker dataprep.SemanticChunker,
	embedder embedding.Provider,
	vectorStore abstraction.VectorStore,
	opts ...ConcurrentIndexerOption,
) *ConcurrentIndexer {
	idx := &ConcurrentIndexer{
		parser:        parser,
		chunker:       chunker,
		embedder:      embedder,
		vectorStore:   vectorStore,
		parseWorkers:  5,
		embedWorkers:  4,
		upsertWorkers: 10,
	}
	for _, opt := range opts {
		opt(idx)
	}
	return idx
}

// IndexFile processes a single file into the stores.
func (idx *ConcurrentIndexer) IndexFile(ctx context.Context, filePath string) error {
	// A single file is just a specific case of a channel with one item
	fileChan := make(chan string, 1)
	fileChan <- filePath
	close(fileChan)

	return idx.runPipeline(ctx, fileChan)
}

// IndexDirectory concurrently processes an entire directory.
func (idx *ConcurrentIndexer) IndexDirectory(ctx context.Context, dirPath string, recursive bool) error {
	fileChan := make(chan string, 100)

	// File Discovery Phase
	go func() {
		defer close(fileChan)
		_ = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				if !recursive && path != dirPath {
					return filepath.SkipDir
				}
				return nil
			}

			// Note: We could add Checkpoint/Hash deduplication here before sending to fileChan.
			// This matches the "File Discovery (去重缓存)" spec.

			select {
			case <-ctx.Done():
				return ctx.Err()
			case fileChan <- path:
			}
			return nil
		})
	}()

	return idx.runPipeline(ctx, fileChan)
}

// runPipeline defines the 4-stage Concurrent Worker Pool (File -> Parse -> Chunk/Embed -> Store).
func (idx *ConcurrentIndexer) runPipeline(ctx context.Context, fileChan <-chan string) error {
	// Use separate errgroups for different stages to avoid deadlock
	parseGroup, parseCtx := errgroup.WithContext(ctx)
	processGroup, processCtx := errgroup.WithContext(ctx)

	// Stage 1: Parsing
	docChan := make(chan *entity.Document, 50)
	totalFiles := 0
	for i := 0; i < idx.parseWorkers; i++ {
		parseGroup.Go(func() error {
			for path := range fileChan {
				totalFiles++
				f, err := os.Open(path)
				if err != nil {
					// Log error and continue or return error to fail the whole pipeline.
					// For robustness, usually we log and continue. We'll return for strictness here.
					return fmt.Errorf("failed to open file %s: %w", path, err)
				}

				meta := map[string]any{"source": path, "filename": filepath.Base(path)}

				// Using the Next-Gen ParseStream interface for O(1) memory
				docStream, err := idx.parser.ParseStream(parseCtx, f, meta)
				if err != nil {
					f.Close()
					return fmt.Errorf("failed to start parsing %s: %w", path, err)
				}

				for doc := range docStream {
					select {
					case <-parseCtx.Done():
						f.Close()
						return parseCtx.Err()
					case docChan <- doc:
					}
				}
				f.Close()
			}
			return nil
		})
	}

	// Close docChan when all parse workers finish
	go func() {
		_ = parseGroup.Wait() // Wait only for parse stage to complete
		close(docChan)
	}()

	// Stage 2 & 3: Chunking & Embedding (Merged for simplicity, can be separated)
	// We use gochat's embedding BatchProcessor conceptually here.
	chunkChan := make(chan *entity.Vector, 200)
	totalChunks := 0

	for i := 0; i < idx.embedWorkers; i++ {
		processGroup.Go(func() error {
			for doc := range docChan {
				// We use HierarchicalChunk as designed in Specs 02
				parents, children, err := idx.chunker.HierarchicalChunk(processCtx, doc)
				if err != nil {
					return err
				}

				// If GraphExtraction is enabled, process parents (which are larger)
				if idx.graphStore != nil && idx.extractor != nil {
					// We could send parents to a separate GraphWorker pool here.
					for _, p := range parents {
						nodes, edges, extErr := idx.extractor.Extract(processCtx, p)
						if extErr == nil {
							// Convert []abstraction.Node to []*abstraction.Node
							nodePtrs := make([]*abstraction.Node, len(nodes))
							for i, node := range nodes {
								nodePtrs[i] = &node
							}
							_ = idx.graphStore.UpsertNodes(processCtx, nodePtrs)

							// Convert []abstraction.Edge to []*abstraction.Edge
							edgePtrs := make([]*abstraction.Edge, len(edges))
							for i, edge := range edges {
								edgePtrs[i] = &edge
							}
							_ = idx.graphStore.UpsertEdges(processCtx, edgePtrs)
						}
					}
				}

				// Process children for vector embeddings
				// Depending on gochat's embedding signature, this is a placeholder
				// for batch processing.
				// Use batch embedding for better performance
				texts := make([]string, len(children))
				for i, child := range children {
					texts[i] = child.Content
				}

				embeddings, err := idx.embedder.Embed(processCtx, texts)
				if err != nil {
					continue // skip failed embeddings
				}

				for i, child := range children {
					if i < len(embeddings) {
						vec := entity.NewVector(child.ID, embeddings[i], child.ID, child.Metadata)
						totalChunks++
						select {
						case <-processCtx.Done():
							return processCtx.Err()
						case chunkChan <- vec:
						}
					}
				}
			}
			return nil
		})
	}

	// Close chunkChan when all embed workers finish
	go func() {
		_ = processGroup.Wait() // Wait for embed stage to complete
		close(chunkChan)
	}()

	// Stage 4: Storing to VectorStore
	storeGroup, storeCtx := errgroup.WithContext(ctx)

	// Start Upsert Workers
	for i := 0; i < idx.upsertWorkers; i++ {
		storeGroup.Go(func() error {
			// A simple batch collector could be implemented here (e.g., collect 100 before AddBatch)
			// Using Add() for simplicity in this demo class.
			for vec := range chunkChan {
				err := idx.vectorStore.Add(storeCtx, vec)
				if err != nil {
					// Dead Letter Queue could be implemented here
					_ = err
				}
			}
			return nil
		})
	}

	// Wait for all stages to complete
	if err := parseGroup.Wait(); err != nil {
		return err
	}
	if err := processGroup.Wait(); err != nil {
		return err
	}
	if err := storeGroup.Wait(); err != nil {
		return err
	}

	// Record metrics after successful completion
	if idx.metrics != nil {
		idx.metrics.RecordParsingErrors("", nil) // Reset parsing errors
		idx.metrics.RecordEmbeddingCount(totalChunks)
		idx.metrics.RecordVectorStoreOperations("concurrent_index", totalChunks)
	}

	return nil
}
