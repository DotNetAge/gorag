package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/embedding"
	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/plugins"
	"github.com/DotNetAge/gorag/rag/indexing"
	"github.com/DotNetAge/gorag/rag/query"
	"github.com/DotNetAge/gorag/rag/retrieval"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

// Engine represents the RAG (Retrieval-Augmented Generation) engine
//
// Engine is the central component of the GoRAG framework that orchestrates
// the entire RAG process: document parsing, embedding generation, vector storage,
// retrieval, and LLM interaction.
//
// Key features:
// - Concurrent directory indexing with 10 workers
// - Automatic parser selection by file extension
// - Streaming large file support (100M+)
// - Hybrid retrieval (vector + keyword search)
// - LLM-based reranking
// - Custom prompt templates
// - Multi-hop RAG for complex questions
// - Agentic RAG with autonomous retrieval
//
// Example:
//
//	engine, err := rag.New(
//	    rag.WithVectorStore(memory.NewStore()),
//	    rag.WithEmbedder(embedderInstance),
//	    rag.WithLLM(llmInstance),
//	)
//
//	// Index entire directory with 10 concurrent workers
//	err = engine.IndexDirectory(ctx, "./documents")
//
//	// Query with custom prompt
//	resp, err := engine.Query(ctx, "What is Go?", rag.QueryOptions{
//	    TopK: 5,
//	    PromptTemplate: "Answer based on context: {context}\nQuestion: {question}",
//	})
type Engine struct {
	parsers             map[string]parser.Parser
	defaultParser       parser.Parser
	embedder            embedding.Provider
	store               vectorstore.Store
	llm                 llm.Client
	retriever           *retrieval.HybridRetriever
	reranker            *retrieval.Reranker
	hydration           *HyDE
	compressor          *ContextCompressor
	conversationManager *ConversationManager
	multiHopRAG         *retrieval.MultiHopRAG
	agenticRAG          *retrieval.AgenticRAG
	cache               Cache
	router              Router
	metrics             observability.Metrics
	logger              observability.Logger
	tracer              observability.Tracer
	pluginOptions       PluginOptions

	// Internal handlers
	indexer *indexing.Indexer
	querier *query.QueryHandler
}

// PluginOptions holds plugin-related configuration
//
// This struct defines options for plugin loading and management.
type PluginOptions struct {
	// PluginDirectory is the directory to load plugins from
	PluginDirectory string
	// PluginConfig is the configuration for plugins
	PluginConfig map[string]interface{}
}

// New creates a new RAG engine with the provided options
//
// This function creates a new Engine instance and applies the provided options.
// It automatically loads all built-in parsers for 9 file formats:
// - Text (.txt, .md)
// - PDF (.pdf)
// - DOCX (.docx)
// - HTML (.html)
// - JSON (.json)
// - YAML (.yaml, .yml)
// - Excel (.xlsx)
// - PPT (.pptx)
// - Image (.jpg, .jpeg, .png)
//
// Required options:
// - WithVectorStore: Vector storage backend
// - WithEmbedder: Embedding model provider
// - WithLLM: LLM client for generation
//
// Optional options:
// - WithParser: Default parser for unknown file types
// - WithParsers: Custom parsers for specific file formats
// - WithRetriever: Custom hybrid retriever
// - WithReranker: Custom LLM reranker
// - WithLogger: Custom logger
// - WithMetrics: Custom metrics collector
// - WithTracer: Custom tracer
// - WithPluginDirectory: Directory to load plugins from
//
// Returns:
// - *Engine: Newly created RAG engine
// - error: Error if required components are missing
//
// Example:
//
//	// Create embedding provider
//	embedderInstance, err := embedder.New(embedder.Config{APIKey: apiKey})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Create LLM client
//	llmInstance, err := llm.New(llm.Config{APIKey: apiKey})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Create RAG engine
//	engine, err := rag.New(
//		rag.WithVectorStore(memory.NewStore()),
//		rag.WithEmbedder(embedderInstance),
//		rag.WithLLM(llmInstance),
//		rag.WithPluginDirectory("./plugins"),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Engine is ready with all 9 built-in parsers loaded and plugins from directory
func New(opts ...Option) (*Engine, error) {
	// Create engine with empty parser map
	engine := &Engine{
		parsers: make(map[string]parser.Parser),
	}

	// Apply provided options
	for _, opt := range opts {
		opt(engine)
	}

	// Auto-load all built-in parsers for 9 file formats
	engine.loadDefaultParsers()

	// Set default parser if not provided
	if engine.defaultParser == nil {
		engine.defaultParser = text.NewParser()
	}

	// Set default logger if not provided
	if engine.logger == nil {
		engine.logger = observability.NewJSONLogger()
	}

	// Set default metrics if not provided
	if engine.metrics == nil {
		engine.metrics = observability.NewPrometheusMetrics()
	}

	// Set default tracer if not provided (using no-op tracer for now)
	if engine.tracer == nil {
		engine.tracer = observability.NewNoopTracer()
	}

	// Load plugins from directory if specified
	if engine.pluginOptions.PluginDirectory != "" {
		registry := plugins.NewRegistry()
		successCount, errorCount, err := registry.LoadPluginsFromDirectory(engine.pluginOptions.PluginDirectory)
		if err != nil {
			// Log error but don't fail the engine creation
			engine.logger.Error(context.Background(), "Failed to load plugins from directory", err, map[string]interface{}{
				"directory": engine.pluginOptions.PluginDirectory,
			})
		} else {
			// Log plugin loading results
			engine.logger.Info(context.Background(), "Plugin loading completed", map[string]interface{}{
				"directory":     engine.pluginOptions.PluginDirectory,
				"success_count": successCount,
				"error_count":   errorCount,
			})

			// Register plugins
			for _, plugin := range registry.List() {
				// Initialize plugin with config
				if err := plugin.Init(engine.pluginOptions.PluginConfig); err != nil {
					engine.logger.Error(context.Background(), "Failed to initialize plugin", err, map[string]interface{}{
						"plugin_name": plugin.Name(),
						"plugin_type": plugin.Type(),
					})
					continue
				}

				// Handle different plugin types
				switch p := plugin.(type) {
				case plugins.ParserPlugin:
					// Add parser plugins
					for _, format := range p.Parser().SupportedFormats() {
						engine.parsers[format] = p.Parser()
					}
					engine.logger.Info(context.Background(), "Parser plugin loaded", map[string]interface{}{
						"plugin_name": plugin.Name(),
						"formats":     p.Parser().SupportedFormats(),
					})
				case plugins.VectorStorePlugin:
					// Vector store plugins are handled differently
					engine.logger.Info(context.Background(), "Vector store plugin loaded", map[string]interface{}{
						"plugin_name": plugin.Name(),
					})
				case plugins.EmbedderPlugin:
					// Embedder plugins are handled differently
					engine.logger.Info(context.Background(), "Embedder plugin loaded", map[string]interface{}{
						"plugin_name": plugin.Name(),
					})
				case plugins.LLMPlugin:
					// LLM plugins are handled differently
					engine.logger.Info(context.Background(), "LLM plugin loaded", map[string]interface{}{
						"plugin_name": plugin.Name(),
					})
				default:
					engine.logger.Info(context.Background(), "Unknown plugin type loaded", map[string]interface{}{
						"plugin_name": plugin.Name(),
						"plugin_type": plugin.Type(),
					})
				}
			}
		}
	}

	// Validate required components
	if engine.embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	if engine.store == nil {
		return nil, fmt.Errorf("vector store is required")
	}
	if engine.llm == nil {
		return nil, fmt.Errorf("LLM client is required")
	}

	// Initialize default advanced RAG components
	if engine.multiHopRAG == nil {
		engine.multiHopRAG = retrieval.NewMultiHopRAG(
			engine.llm,
			engine.embedder,
			engine.store,
		)
	}

	if engine.agenticRAG == nil {
		engine.agenticRAG = retrieval.NewAgenticRAG(
			engine.llm,
			engine.embedder,
			engine.store,
		)
	}

	// Initialize internal handlers
	engine.initHandlers()

	return engine, nil
}

// routerAdapter adapts a rag.Router to a query.Router
type routerAdapter struct {
	router Router
}

// Route adapts the rag.Router.Route method to query.Router.Route
func (ra *routerAdapter) Route(ctx context.Context, question string) (query.RouteResult, error) {
	if ra.router == nil {
		return query.RouteResult{
			Type: "hybrid",
			Params: map[string]interface{}{
				"topK": 5,
			},
		}, nil
	}

	result, err := ra.router.Route(ctx, question)
	if err != nil {
		return query.RouteResult{
			Type: "hybrid",
			Params: map[string]interface{}{
				"topK": 5,
			},
		}, err
	}

	return query.RouteResult{
		Type:   result.Type,
		Params: result.Params,
	}, nil
}

// initHandlers initializes the internal indexing and query handlers
func (e *Engine) initHandlers() {
	// Initialize indexer
	e.indexer = indexing.NewIndexer(
		e.parsers,
		e.defaultParser,
		e.embedder,
		e.store,
		e,
		e,
		e,
	)

	// Initialize query handler
	e.querier = query.NewQueryHandler(
		e.embedder,
		e.store,
		e.llm,
		e.retriever,
		e.reranker,
		e.hydration,
		e.compressor,
		e.multiHopRAG,
		e.agenticRAG,
		e,
		&routerAdapter{router: e.router},
		e,
		e,
		e,
	)
}



// AddParser adds a parser for a specific format
func (e *Engine) AddParser(format string, p parser.Parser) {
	e.parsers[format] = p
}

// SetDefaultParser sets the default parser for unknown formats
func (e *Engine) SetDefaultParser(p parser.Parser) {
	e.defaultParser = p
}

// Index adds documents to the RAG engine
func (e *Engine) Index(ctx context.Context, source Source) error {
	idxSource := indexing.Source{
		Type:    source.Type,
		Path:    source.Path,
		Content: source.Content,
		Reader:  source.Reader,
	}
	return e.indexer.Index(ctx, idxSource)
}

// BatchIndex adds multiple documents to the RAG engine in batch
func (e *Engine) BatchIndex(ctx context.Context, sources []Source) error {
	idxSources := make([]indexing.Source, len(sources))
	for i, s := range sources {
		idxSources[i] = indexing.Source{
			Type:    s.Type,
			Path:    s.Path,
			Content: s.Content,
			Reader:  s.Reader,
		}
	}
	return e.indexer.BatchIndex(ctx, idxSources)
}

// AsyncIndex adds documents to the RAG engine asynchronously
func (e *Engine) AsyncIndex(ctx context.Context, source Source) error {
	idxSource := indexing.Source{
		Type:    source.Type,
		Path:    source.Path,
		Content: source.Content,
		Reader:  source.Reader,
	}
	return e.indexer.AsyncIndex(ctx, idxSource)
}

// AsyncBatchIndex adds multiple documents to the RAG engine asynchronously
func (e *Engine) AsyncBatchIndex(ctx context.Context, sources []Source) error {
	idxSources := make([]indexing.Source, len(sources))
	for i, s := range sources {
		idxSources[i] = indexing.Source{
			Type:    s.Type,
			Path:    s.Path,
			Content: s.Content,
			Reader:  s.Reader,
		}
	}
	return e.indexer.AsyncBatchIndex(ctx, idxSources)
}

// IndexDirectory indexes all files in a directory recursively with concurrent workers
func (e *Engine) IndexDirectory(ctx context.Context, directoryPath string) error {
	return e.indexer.IndexDirectory(ctx, directoryPath)
}

// AsyncIndexDirectory indexes all files in a directory recursively asynchronously
func (e *Engine) AsyncIndexDirectory(ctx context.Context, directoryPath string) error {
	return e.indexer.AsyncIndexDirectory(ctx, directoryPath)
}

// Query performs a RAG query
func (e *Engine) Query(ctx context.Context, question string, opts QueryOptions) (*Response, error) {
	qOpts := query.QueryOptions{
		TopK:              opts.TopK,
		PromptTemplate:    opts.PromptTemplate,
		Stream:            opts.Stream,
		UseMultiHopRAG:    opts.UseMultiHopRAG,
		UseAgenticRAG:     opts.UseAgenticRAG,
		MaxHops:           opts.MaxHops,
		AgentInstructions: opts.AgentInstructions,
	}
	resp, err := e.querier.Query(ctx, question, qOpts)
	if err != nil {
		return nil, err
	}
	return &Response{
		Answer:  resp.Answer,
		Sources: resp.Sources,
	}, nil
}

// QueryStream performs a streaming RAG query
func (e *Engine) QueryStream(ctx context.Context, question string, opts QueryOptions) (<-chan StreamResponse, error) {
	qOpts := query.QueryOptions{
		TopK:              opts.TopK,
		PromptTemplate:    opts.PromptTemplate,
		Stream:            opts.Stream,
		UseMultiHopRAG:    opts.UseMultiHopRAG,
		UseAgenticRAG:     opts.UseAgenticRAG,
		MaxHops:           opts.MaxHops,
		AgentInstructions: opts.AgentInstructions,
	}

	ch, err := e.querier.QueryStream(ctx, question, qOpts)
	if err != nil {
		return nil, err
	}

	// Convert channel type
	resultCh := make(chan StreamResponse, 10)
	go func() {
		defer close(resultCh)
		for resp := range ch {
			resultCh <- StreamResponse{
				Chunk:   resp.Chunk,
				Sources: resp.Sources,
				Done:    resp.Done,
				Error:   resp.Error,
			}
		}
	}()

	return resultCh, nil
}

// BatchQuery performs multiple RAG queries in batch
func (e *Engine) BatchQuery(ctx context.Context, questions []string, opts QueryOptions) ([]*Response, error) {
	qOpts := query.QueryOptions{
		TopK:              opts.TopK,
		PromptTemplate:    opts.PromptTemplate,
		Stream:            opts.Stream,
		UseMultiHopRAG:    opts.UseMultiHopRAG,
		UseAgenticRAG:     opts.UseAgenticRAG,
		MaxHops:           opts.MaxHops,
		AgentInstructions: opts.AgentInstructions,
	}
	responses, err := e.querier.BatchQuery(ctx, questions, qOpts)
	if err != nil {
		return nil, err
	}

	result := make([]*Response, len(responses))
	for i, resp := range responses {
		result[i] = &Response{
			Answer:  resp.Answer,
			Sources: resp.Sources,
		}
	}
	return result, nil
}

// generateChunkID generates a unique chunk ID
func generateChunkID() string {
	return uuid.New().String()
}

// RecordIndexedDocuments records the number of indexed documents
func (e *Engine) RecordIndexedDocuments(ctx context.Context, count int) {
	if e.metrics != nil {
		e.metrics.RecordIndexedDocuments(ctx, count)
	}
}

// RecordIndexingDocuments records the number of documents being indexed
func (e *Engine) RecordIndexingDocuments(ctx context.Context, count int) {
	if e.metrics != nil {
		e.metrics.RecordIndexingDocuments(ctx, count)
	}
}

// RecordMonitoredDocuments records the number of monitored documents
func (e *Engine) RecordMonitoredDocuments(ctx context.Context, count int) {
	if e.metrics != nil {
		e.metrics.RecordMonitoredDocuments(ctx, count)
	}
}

// RecordSystemMetrics records system metrics (CPU, memory)
func (e *Engine) RecordSystemMetrics(ctx context.Context, cpuUsage float64, memoryUsage float64) {
	if e.metrics != nil {
		e.metrics.RecordSystemMetrics(ctx, cpuUsage, memoryUsage)
	}
}

// ReindexFile reindexes a file by first deleting existing chunks and then reindexing
func (e *Engine) ReindexFile(ctx context.Context, filePath string, sourceType string) error {
	// Search for chunks with the file path in metadata
	metadata := map[string]string{
		"file_path": filePath,
	}
	chunks, err := e.store.SearchByMetadata(ctx, metadata)
	if err != nil {
		return fmt.Errorf("failed to search chunks by metadata: %w", err)
	}

	// Delete existing chunks
	if len(chunks) > 0 {
		ids := make([]string, len(chunks))
		for i, chunk := range chunks {
			ids[i] = chunk.ID
		}
		if err := e.store.Delete(ctx, ids); err != nil {
			return fmt.Errorf("failed to delete existing chunks: %w", err)
		}
	}

	// Reindex the file
	source := Source{
		Type: sourceType,
		Path: filePath,
	}
	return e.Index(ctx, source)
}

// StartWatch starts watching a directory for changes and automatically indexes files
//
// Parameters:
// - targetIndexDir: Directory to watch for changes
//
// Returns:
// - error: Error if watching fails
//
// This method will:
// 1. First perform an initial indexing of the directory
// 2. Then start watching for file changes
// 3. Automatically reindex files when they change
// 4. Use asynchronous indexing to avoid blocking
func (e *Engine) StartWatch(targetIndexDir string) error {
	ctx := context.Background()
	
	// First perform initial indexing of the directory
	e.logger.Info(ctx, "Starting initial indexing of directory", map[string]interface{}{
		"directory": targetIndexDir,
	})
	
	if err := e.IndexDirectory(ctx, targetIndexDir); err != nil {
		e.logger.Error(ctx, "Failed to perform initial indexing", err, map[string]interface{}{
			"directory": targetIndexDir,
		})
		return fmt.Errorf("failed to perform initial indexing: %w", err)
	}
	
	e.logger.Info(ctx, "Initial indexing completed", map[string]interface{}{
		"directory": targetIndexDir,
	})
	
	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		e.logger.Error(ctx, "Failed to create watcher", err, nil)
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()
	
	// Add the directory and all subdirectories to the watcher
	if err := e.addDirToWatcher(watcher, targetIndexDir); err != nil {
		e.logger.Error(ctx, "Failed to add directory to watcher", err, map[string]interface{}{
			"directory": targetIndexDir,
		})
		return fmt.Errorf("failed to add directory to watcher: %w", err)
	}
	
	e.logger.Info(ctx, "Starting to watch directory for changes", map[string]interface{}{
		"directory": targetIndexDir,
	})
	
	// Start watching for events
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			
			// Only process regular files
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
				// Check if it's a file (not a directory)
				if !strings.HasSuffix(event.Name, "/") {
					// Get file extension
					ext := strings.ToLower(filepath.Ext(event.Name))
					
					// Check if we have a parser for this file type
					if _, ok := e.parsers[ext]; ok {
						// Use asynchronous indexing to avoid blocking
						source := Source{
							Type: ext,
							Path: event.Name,
						}
						
						if err := e.AsyncIndex(ctx, source); err != nil {
							e.logger.Error(ctx, "Failed to asynchronously index file", err, map[string]interface{}{
							"file": event.Name,
						})
						}
						e.logger.Info(ctx, "File change detected, indexing asynchronously", map[string]interface{}{
							"file": event.Name,
							"operation": event.Op,
						})
					}
				}
				
				// If a directory was created, add it to the watcher
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := e.addDirToWatcher(watcher, event.Name); err != nil {
						e.logger.Error(ctx, "Failed to add new directory to watcher", err, map[string]interface{}{
							"directory": event.Name,
						})
					}
				}
			}
			}
			
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			e.logger.Error(ctx, "Watcher error", err, nil)
		}
	}
}

// addDirToWatcher adds a directory and all its subdirectories to the watcher
func (e *Engine) addDirToWatcher(watcher *fsnotify.Watcher, dir string) error {
	// Add the directory itself
	if err := watcher.Add(dir); err != nil {
		return err
	}
	
	// Add all subdirectories
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}

// StartWatchAsync starts watching a directory for changes asynchronously
//
// Parameters:
// - targetIndexDir: Directory to watch for changes
//
// Returns:
// - error: Error if starting the watch fails
func (e *Engine) StartWatchAsync(targetIndexDir string) error {
	go func() {
		if err := e.StartWatch(targetIndexDir); err != nil {
			e.logger.Error(context.Background(), "Watch failed", err, map[string]interface{}{
				"directory": targetIndexDir,
			})
		}
	}()
	return nil
}

