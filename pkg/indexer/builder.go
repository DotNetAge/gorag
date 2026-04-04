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
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexing"
	"github.com/DotNetAge/gorag/pkg/indexing/chunker"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/config/types"
	"github.com/DotNetAge/gorag/pkg/logging"
	stepinx "github.com/DotNetAge/gorag/pkg/steps/indexing"
	"github.com/DotNetAge/gorag/pkg/store/doc/bolt"
	"github.com/DotNetAge/gorag/pkg/store/graph/gograph"
	"github.com/DotNetAge/gorag/pkg/store/vector/govector"
	"golang.org/x/sync/errgroup"
)

// Indexer is the unified interface for document indexing.
// It provides methods for processing files and directories into vector/graph stores.
type Indexer interface {
	indexing.Indexer
	Init() error
	Start() error
	VectorStore() core.VectorStore
	DocStore() core.DocStore
	GraphStore() core.GraphStore
	Embedder() embedding.Provider
	Chunker() core.SemanticChunker
}

type defaultIndexer struct {
	name             string
	pipeline         *pipeline.Pipeline[*core.IndexingContext]
	config           Config
	logger           logging.Logger
	registry         *types.ParserRegistry
	watchDirs        []string
	vectorStore      core.VectorStore
	docStore         core.DocStore
	graphStore       core.GraphStore
	chunker          core.SemanticChunker
	embedder         embedding.Provider
	extractor        core.EntityExtractor
	triplesExtractor core.TriplesExtractor // LLM-based triple extraction for GraphRAG
	metrics          core.Metrics
}

func (idx *defaultIndexer) VectorStore() core.VectorStore {
	return idx.vectorStore
}

func (idx *defaultIndexer) DocStore() core.DocStore {
	return idx.docStore
}

func (idx *defaultIndexer) GraphStore() core.GraphStore {
	return idx.graphStore
}

func (idx *defaultIndexer) Embedder() embedding.Provider {
	return idx.embedder
}

func (idx *defaultIndexer) Chunker() core.SemanticChunker {
	return idx.chunker
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

	// GraphRAG: Extract triples and build knowledge graph
	if idx.triplesExtractor != nil && idx.graphStore != nil {
		p.AddStep(stepinx.ExtractTriples(idx.triplesExtractor, idx.graphStore, idx.logger))
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

// IndexText indexes plain text content directly without file parsing.
func (idx *defaultIndexer) IndexText(ctx context.Context, text string, metadata ...map[string]any) error {
	doc := core.NewDocument("", text, "text", "text/plain", nil)
	if len(metadata) > 0 {
		doc.Metadata = metadata[0]
	}
	return idx.IndexDocuments(ctx, doc)
}

// IndexTexts indexes multiple plain text contents in batch.
func (idx *defaultIndexer) IndexTexts(ctx context.Context, texts []string, metadata ...map[string]any) error {
	docs := make([]*core.Document, len(texts))
	for i, text := range texts {
		doc := core.NewDocument("", text, "text", "text/plain", nil)
		if len(metadata) > 0 && i < len(metadata) {
			doc.Metadata = metadata[i]
		} else if len(metadata) > 0 {
			doc.Metadata = metadata[0]
		}
		docs[i] = doc
	}
	return idx.IndexDocuments(ctx, docs...)
}

// IndexDocuments indexes documents directly into Vector/Doc/Graph stores.
// This bypasses file parsing and is useful for programmatic document management.
func (idx *defaultIndexer) IndexDocuments(ctx context.Context, docs ...*core.Document) error {
	for _, doc := range docs {
		if err := idx.indexDocument(ctx, doc); err != nil {
			return fmt.Errorf("failed to index document %s: %w", doc.ID, err)
		}
	}
	return nil
}

func (idx *defaultIndexer) indexDocument(ctx context.Context, doc *core.Document) error {
	// Generate ID if not provided
	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc-%d", time.Now().UnixNano())
	}

	// 1. Chunk the document content
	var chunks []*core.Chunk
	if idx.chunker != nil {
		var err error
		chunks, err = idx.chunker.Chunk(ctx, doc)
		if err != nil {
			return fmt.Errorf("chunking failed: %w", err)
		}
	} else {
		// No chunker: treat entire content as single chunk
		chunks = []*core.Chunk{{
			ID:         fmt.Sprintf("%s-chunk-0", doc.ID),
			DocumentID: doc.ID,
			Content:    doc.Content,
			Metadata:   doc.Metadata,
		}}
	}

	// Set DocumentID and merge metadata for each chunk
	for i, chunk := range chunks {
		if chunk.DocumentID == "" {
			chunk.DocumentID = doc.ID
		}
		if chunk.ID == "" {
			chunk.ID = fmt.Sprintf("%s-chunk-%d", doc.ID, i)
		}
		// Merge document metadata into chunk metadata
		if chunk.Metadata == nil {
			chunk.Metadata = make(map[string]any)
		}
		for k, v := range doc.Metadata {
			if _, exists := chunk.Metadata[k]; !exists {
				chunk.Metadata[k] = v
			}
		}
		chunk.Metadata["source"] = doc.Source
	}

	// 2. Generate embeddings
	if idx.embedder != nil && idx.vectorStore != nil {
		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Content
		}

		embeddings, err := idx.embedder.Embed(ctx, texts)
		if err != nil {
			return fmt.Errorf("embedding failed: %w", err)
		}

		// Store vectors using Upsert (batch)
		vectors := make([]*core.Vector, len(chunks))
		for i, chunk := range chunks {
			vectors[i] = &core.Vector{
				ID:       chunk.ID,
				Values:   embeddings[i],
				Metadata: chunk.Metadata,
			}
		}
		if err := idx.vectorStore.Upsert(ctx, vectors); err != nil {
			return fmt.Errorf("failed to store vectors: %w", err)
		}
	}

	// 3. Store document and chunks
	if idx.docStore != nil {
		if err := idx.docStore.SetDocument(ctx, doc); err != nil {
			return fmt.Errorf("failed to store document: %w", err)
		}
		if err := idx.docStore.SetChunks(ctx, chunks); err != nil {
			return fmt.Errorf("failed to store chunks: %w", err)
		}
	}

	// 4. Extract and store graph entities (GraphRAG sync)
	if idx.graphStore != nil && idx.triplesExtractor != nil {
		nodes, edges := idx.extractGraphEntities(ctx, doc.ID, chunks)
		if len(nodes) > 0 {
			if err := idx.graphStore.UpsertNodes(ctx, nodes); err != nil {
				idx.logger.Warn("failed to store graph nodes", map[string]interface{}{
					"doc_id": doc.ID,
					"error":  err.Error(),
				})
			} else {
				idx.logger.Debug("stored graph nodes", map[string]interface{}{
					"doc_id": doc.ID,
					"count":  len(nodes),
				})
			}
		}
		if len(edges) > 0 {
			if err := idx.graphStore.UpsertEdges(ctx, edges); err != nil {
				idx.logger.Warn("failed to store graph edges", map[string]interface{}{
					"doc_id": doc.ID,
					"error":  err.Error(),
				})
			} else {
				idx.logger.Debug("stored graph edges", map[string]interface{}{
					"doc_id": doc.ID,
					"count":  len(edges),
				})
			}
		}
	}

	return nil
}

// extractGraphEntities extracts nodes and edges from chunks for graph storage.
// Each node/edge is tagged with document_id for cascade delete support.
func (idx *defaultIndexer) extractGraphEntities(ctx context.Context, docID string, chunks []*core.Chunk) ([]*core.Node, []*core.Edge) {
	nodeMap := make(map[string]*core.Node)
	var edges []*core.Edge

	for _, chunk := range chunks {
		triples, err := idx.triplesExtractor.Extract(ctx, chunk.Content)
		if err != nil {
			idx.logger.Warn("failed to extract triples from chunk", map[string]interface{}{
				"chunk_id": chunk.ID,
				"error":    err.Error(),
			})
			continue
		}

		for _, t := range triples {
			// Create subject node (deduplicated)
			if _, exists := nodeMap[t.Subject]; !exists {
				nodeMap[t.Subject] = &core.Node{
					ID:   t.Subject,
					Type: t.SubjectType,
					Properties: map[string]any{
						"document_id":  docID,
						"source_chunk": chunk.ID,
					},
				}
			}

			// Create object node (deduplicated)
			if _, exists := nodeMap[t.Object]; !exists {
				nodeMap[t.Object] = &core.Node{
					ID:   t.Object,
					Type: t.ObjectType,
					Properties: map[string]any{
						"document_id":  docID,
						"source_chunk": chunk.ID,
					},
				}
			}

			// Create edge
			edges = append(edges, &core.Edge{
				ID:     fmt.Sprintf("%s-%s-%s", t.Subject, t.Predicate, t.Object),
				Type:   t.Predicate,
				Source: t.Subject,
				Target: t.Object,
				Properties: map[string]any{
					"document_id":  docID,
					"source_chunk": chunk.ID,
				},
			})
		}
	}

	nodes := make([]*core.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	return nodes, edges
}

// DeleteDocument removes a document and all its associated data across all stores.
// It performs cascade delete: DocStore → VectorStore → GraphStore
func (idx *defaultIndexer) DeleteDocument(ctx context.Context, docID string) error {
	// 1. Get all chunks for this document
	chunks, err := idx.docStore.GetChunksByDocID(ctx, docID)
	if err != nil {
		return fmt.Errorf("failed to get chunks for document %s: %w", docID, err)
	}

	// 2. Delete vectors for each chunk
	for _, chunk := range chunks {
		if idx.vectorStore != nil {
			if err := idx.vectorStore.Delete(ctx, chunk.ID); err != nil {
				idx.logger.Warn("failed to delete vector", map[string]interface{}{"chunkID": chunk.ID, "error": err.Error()})
			}
		}
	}

	// 3. Cascade delete graph nodes and edges associated with this document
	if idx.graphStore != nil {
		if err := idx.deleteGraphByDocumentID(ctx, docID); err != nil {
			idx.logger.Warn("failed to delete graph entities", map[string]interface{}{
				"doc_id": docID,
				"error":  err.Error(),
			})
			// Continue with document deletion even if graph cleanup fails
		}
	}

	// 4. Delete document (which also deletes chunks in doc store)
	if idx.docStore != nil {
		if err := idx.docStore.DeleteDocument(ctx, docID); err != nil {
			return fmt.Errorf("failed to delete document %s: %w", docID, err)
		}
	}

	return nil
}

// deleteGraphByDocumentID removes all nodes and edges associated with a document.
// This implements cascade delete for graph entities.
func (idx *defaultIndexer) deleteGraphByDocumentID(ctx context.Context, docID string) error {
	// Query for all edges with this document_id
	edgesResult, err := idx.graphStore.Query(ctx,
		"MATCH ()-[r {document_id: $docID}]-() RETURN r.id as id",
		map[string]any{"docID": docID},
	)
	if err != nil {
		return fmt.Errorf("failed to query edges by document_id: %w", err)
	}

	// Delete all edges
	for _, row := range edgesResult {
		if edgeID, ok := row["id"].(string); ok {
			if err := idx.graphStore.DeleteEdge(ctx, edgeID); err != nil {
				idx.logger.Warn("failed to delete edge", map[string]interface{}{
					"edge_id": edgeID,
					"error":   err.Error(),
				})
			}
		}
	}

	// Query for all nodes with this document_id
	nodesResult, err := idx.graphStore.Query(ctx,
		"MATCH (n {document_id: $docID}) RETURN n.id as id",
		map[string]any{"docID": docID},
	)
	if err != nil {
		return fmt.Errorf("failed to query nodes by document_id: %w", err)
	}

	// Delete all nodes
	for _, row := range nodesResult {
		if nodeID, ok := row["id"].(string); ok {
			if err := idx.graphStore.DeleteNode(ctx, nodeID); err != nil {
				idx.logger.Warn("failed to delete node", map[string]interface{}{
					"node_id": nodeID,
					"error":   err.Error(),
				})
			}
		}
	}

	idx.logger.Info("cascade deleted graph entities", map[string]interface{}{
		"doc_id":      docID,
		"edges_count": len(edgesResult),
		"nodes_count": len(nodesResult),
	})

	return nil
}

// GetDocument retrieves a document by its ID.
func (idx *defaultIndexer) GetDocument(ctx context.Context, docID string) (*core.Document, error) {
	if idx.docStore == nil {
		return nil, fmt.Errorf("doc store not configured")
	}
	return idx.docStore.GetDocument(ctx, docID)
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
	docStore core.DocStore,
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
	docStore core.DocStore,
	graphStore core.GraphStore,
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

// createDefaultStores creates default vector and doc stores for indexers.
// This helper function reduces code duplication across DefaultNativeIndexer, DefaultAdvancedIndexer, and DefaultGraphIndexer.
func createDefaultStores(idx *defaultIndexer, workDir string) error {
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
			return fmt.Errorf("failed to create default vector store: %w", err)
		}
		idx.vectorStore = vStore
	}

	if idx.docStore == nil {
		docFileName := "gorag_docs.bolt"
		if idx.name != "" {
			docFileName = fmt.Sprintf("gorag_docs_%s.bolt", idx.name)
		}
		docPath := filepath.Join(workDir, docFileName)
		dStore, err := bolt.NewDocStore(docPath)
		if err != nil {
			return fmt.Errorf("failed to create default doc store: %w", err)
		}
		idx.docStore = dStore
	}

	return nil
}

// DefaultNativeIndexer creates a light-weight, local-first Indexer.
// It uses default TokenChunker, local SQLite and GoVector stores, suitable for quick prototyping and testing.
func DefaultNativeIndexer(opts ...IndexerOption) (Indexer, error) {
	// 1. Set default internal state
	idx := &defaultIndexer{
		logger:   logging.DefaultNoopLogger(),
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

	// Create default stores using helper function
	if err := createDefaultStores(idx, workDir); err != nil {
		return nil, err
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
		logger:   logging.DefaultNoopLogger(),
		registry: types.DefaultRegistry,
		config: Config{
			Concurrency: true,
			Workers:     20, // Enterprise default
		},
	}

	for _, opt := range opts {
		opt(idx)
	}

	// Create default stores using helper function
	if err := createDefaultStores(idx, "./data"); err != nil {
		return nil, err
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
		logger:   logging.DefaultNoopLogger(),
		registry: types.DefaultRegistry,
		config: Config{
			Concurrency: true,
			Workers:     10,
		},
	}

	for _, opt := range opts {
		opt(idx)
	}

	// Create default stores using helper function
	if err := createDefaultStores(idx, "./data"); err != nil {
		return nil, err
	}

	if idx.graphStore == nil {
		graphName := "gorag_graph.db"
		if idx.name != "" {
			graphName = fmt.Sprintf("gorag_graph_%s.db", idx.name)
		}
		gStore, err := gograph.NewGraphStore(graphName)
		if err != nil {
			return nil, fmt.Errorf("failed to create default graph store: %w", err)
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
