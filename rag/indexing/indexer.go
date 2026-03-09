package indexing

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/errors"
	"github.com/DotNetAge/gorag/lazyloader"
	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/vectorstore"
)

// Source represents a document source for indexing
type Source = core.Source

// IndexerConfig holds configurable parameters for the Indexer
type IndexerConfig struct {
	// WorkerCount is the number of concurrent workers for directory indexing (default: 10)
	WorkerCount int
	// FileChannelBuffer is the buffer size for the file channel in directory indexing (default: 100)
	FileChannelBuffer int
	// ErrorChannelBuffer is the buffer size for the error channel in directory indexing (default: 10)
	ErrorChannelBuffer int
}

// DefaultIndexerConfig returns the default indexer configuration
func DefaultIndexerConfig() IndexerConfig {
	return IndexerConfig{
		WorkerCount:        10,
		FileChannelBuffer:  100,
		ErrorChannelBuffer: 10,
	}
}

// Indexer handles document indexing operations
type Indexer struct {
	parsers            map[string]parser.Parser
	defaultParser      parser.Parser
	embedder           Embedder
	store              vectorstore.Store
	metrics            Metrics
	logger             Logger
	tracer             Tracer
	config             IndexerConfig
	indexedDocuments   int
	indexingDocuments  int
	monitoredDocuments int
	mu                 sync.RWMutex
	lazyLoader         *lazyloader.LazyDocumentManager
}

// Embedder defines the interface for embedding providers
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Metrics defines the interface for metrics collection
type Metrics interface {
	RecordErrorCount(ctx context.Context, errorType string)
	RecordIndexLatency(ctx context.Context, duration time.Duration)
	RecordIndexCount(ctx context.Context, status string)
	RecordIndexedDocuments(ctx context.Context, count int)
	RecordIndexingDocuments(ctx context.Context, count int)
	RecordMonitoredDocuments(ctx context.Context, count int)
	RecordSystemMetrics(ctx context.Context, cpuUsage float64, memoryUsage float64)
}

// Logger defines the interface for logging
type Logger interface {
	Info(ctx context.Context, message string, fields map[string]interface{})
	Debug(ctx context.Context, message string, fields map[string]interface{})
	Error(ctx context.Context, message string, err error, fields map[string]interface{})
}

// Tracer defines the interface for tracing
type Tracer interface {
	StartSpan(ctx context.Context, name string) (context.Context, observability.Span)
	Extract(ctx context.Context) (observability.Span, bool)
}

// NewIndexer creates a new indexer
func NewIndexer(
	parsers map[string]parser.Parser,
	defaultParser parser.Parser,
	embedder Embedder,
	store vectorstore.Store,
	metrics Metrics,
	logger Logger,
	tracer Tracer,
) *Indexer {
	return &Indexer{
		parsers:       parsers,
		defaultParser: defaultParser,
		embedder:      embedder,
		store:         store,
		metrics:       metrics,
		logger:        logger,
		tracer:        tracer,
		config:        DefaultIndexerConfig(),
		lazyLoader:    lazyloader.NewLazyDocumentManager(100 * 1024 * 1024), // 100MB limit
	}
}

// WithConfig sets the indexer configuration
func (i *Indexer) WithConfig(config IndexerConfig) *Indexer {
	if config.WorkerCount > 0 {
		i.config.WorkerCount = config.WorkerCount
	}
	if config.FileChannelBuffer > 0 {
		i.config.FileChannelBuffer = config.FileChannelBuffer
	}
	if config.ErrorChannelBuffer > 0 {
		i.config.ErrorChannelBuffer = config.ErrorChannelBuffer
	}
	return i
}

// Index adds documents to the RAG engine
func (i *Indexer) Index(ctx context.Context, source Source) error {
	startTime := time.Now()
	status := "success"

	// Increment indexing documents count
	i.mu.Lock()
	i.indexingDocuments++
	if i.metrics != nil {
		i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
	}
	i.mu.Unlock()

	// Start span
	if i.tracer != nil {
		var span observability.Span
		ctx, span = i.tracer.StartSpan(ctx, "RAGIndex")
		if span != nil {
			defer span.End()
			span.SetAttribute("source_type", source.Type)
			span.SetAttribute("has_content", source.Content != "")
			span.SetAttribute("has_path", source.Path != "")
		}
	}

	if source.Type == "" {
		err := errors.ErrInvalidInput("source type is required")
		if i.metrics != nil {
			i.metrics.RecordErrorCount(ctx, "invalid_input")
		}
		if i.logger != nil {
			i.logger.Error(ctx, "Invalid index input", err, nil)
		}
		if i.tracer != nil {
			if span, ok := i.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"

		// Decrement indexing documents count
		i.mu.Lock()
		i.indexingDocuments--
		if i.metrics != nil {
			i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
		}
		i.mu.Unlock()

		return err
	}

	if source.Content == "" && source.Path == "" {
		err := errors.ErrInvalidInput("content or path is required")
		if i.metrics != nil {
			i.metrics.RecordErrorCount(ctx, "invalid_input")
		}
		if i.logger != nil {
			i.logger.Error(ctx, "Invalid index input", err, nil)
		}
		if i.tracer != nil {
			if span, ok := i.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"

		// Decrement indexing documents count
		i.mu.Lock()
		i.indexingDocuments--
		if i.metrics != nil {
			i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
		}
		i.mu.Unlock()

		return err
	}

	var reader io.Reader
	if source.Content != "" {
		reader = strings.NewReader(source.Content)
	} else if source.Reader != nil {
		if r, ok := source.Reader.(io.Reader); ok {
			reader = r
		} else {
			err := errors.ErrInvalidInput("reader must implement io.Reader interface")
			if i.metrics != nil {
				i.metrics.RecordErrorCount(ctx, "invalid_input")
			}
			if i.logger != nil {
				i.logger.Error(ctx, "Invalid reader type", err, nil)
			}
			if i.tracer != nil {
				if span, ok := i.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"

			// Decrement indexing documents count
			i.mu.Lock()
			i.indexingDocuments--
			if i.metrics != nil {
				i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
			}
			i.mu.Unlock()

			return err
		}
	} else if source.Path != "" {
		// Use lazy loading for large files
		doc := i.lazyLoader.AddDocument(source.Path)
		content, err := doc.LoadWithContext(ctx)
		if err != nil {
			err = fmt.Errorf("failed to load file at path %s: %w", source.Path, err)
			if i.metrics != nil {
				i.metrics.RecordErrorCount(ctx, "invalid_input")
			}
			if i.logger != nil {
				i.logger.Error(ctx, "Failed to load file", err, map[string]interface{}{"path": source.Path})
			}
			if i.tracer != nil {
				if span, ok := i.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"

			// Decrement indexing documents count
			i.mu.Lock()
			i.indexingDocuments--
			if i.metrics != nil {
				i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
			}
			i.mu.Unlock()

			return err
		}
		reader = strings.NewReader(string(content))
		// Unload the document to free memory
		defer doc.Unload()
	}

	if reader == nil {
		err := fmt.Errorf("no content to index")
		if i.metrics != nil {
			i.metrics.RecordErrorCount(ctx, "invalid_input")
		}
		if i.logger != nil {
			i.logger.Error(ctx, "No content to index", err, nil)
		}
		if i.tracer != nil {
			if span, ok := i.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"

		// Decrement indexing documents count
		i.mu.Lock()
		i.indexingDocuments--
		if i.metrics != nil {
			i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
		}
		i.mu.Unlock()

		return err
	}

	parseStartTime := time.Now()
	// Get the appropriate parser for the source type
	i.mu.RLock()
	p, ok := i.parsers[source.Type]
	if !ok {
		// Use default parser if no specific parser is found
		p = i.defaultParser
	}
	i.mu.RUnlock()

	// 传递文件路径到解析器
	if source.Path != "" {
		ctx = context.WithValue(ctx, "file_path", source.Path)
	}

	chunks, err := p.Parse(ctx, reader)
	if err != nil {
		if i.metrics != nil {
			i.metrics.RecordErrorCount(ctx, "parsing")
		}
		if i.logger != nil {
			i.logger.Error(ctx, "Failed to parse document", err, map[string]interface{}{"source_type": source.Type})
		}
		if i.tracer != nil {
			if span, ok := i.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"

		// Decrement indexing documents count
		i.mu.Lock()
		i.indexingDocuments--
		if i.metrics != nil {
			i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
		}
		i.mu.Unlock()

		return errors.ErrParsing(source.Type, err)
	}
	if i.logger != nil {
		i.logger.Debug(ctx, "Document parsed", map[string]interface{}{
			"duration":     time.Since(parseStartTime).Seconds(),
			"chunks_count": len(chunks),
			"source_type":  source.Type,
			"parser":       p.SupportedFormats()[0],
		})
	}

	if len(chunks) == 0 {
		if i.logger != nil {
			i.logger.Info(ctx, "No chunks to index", map[string]interface{}{"source_type": source.Type})
		}

		// Decrement indexing documents count
		i.mu.Lock()
		i.indexingDocuments--
		if i.metrics != nil {
			i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
		}
		i.mu.Unlock()

		return nil
	}

	vsChunks := make([]core.Chunk, len(chunks))
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		vsChunks[i] = core.Chunk{
			ID:        chunk.ID,
			Content:   chunk.Content,
			Metadata:  chunk.Metadata,
			MediaType: chunk.MediaType,
			MediaData: chunk.MediaData,
		}
		texts[i] = chunk.Content
	}

	embedStartTime := time.Now()
	embeddings, err := i.embedder.Embed(ctx, texts)
	if err != nil {
		if i.metrics != nil {
			i.metrics.RecordErrorCount(ctx, "embedding")
		}
		if i.logger != nil {
			i.logger.Error(ctx, "Failed to embed chunks", err, map[string]interface{}{"chunks_count": len(chunks)})
		}
		if i.tracer != nil {
			if span, ok := i.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"

		// Decrement indexing documents count
		i.mu.Lock()
		i.indexingDocuments--
		if i.metrics != nil {
			i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
		}
		i.mu.Unlock()

		return errors.ErrEmbedding("embedder", err).
			WithContext("chunks_count", fmt.Sprintf("%d", len(chunks)))
	}
	if i.logger != nil {
		i.logger.Debug(ctx, "Chunks embedded", map[string]interface{}{
			"duration":         time.Since(embedStartTime).Seconds(),
			"chunks_count":     len(chunks),
			"embeddings_count": len(embeddings),
		})
	}

	storeStartTime := time.Now()
	err = i.store.Add(ctx, vsChunks, embeddings)
	if err != nil {
		if i.metrics != nil {
			i.metrics.RecordErrorCount(ctx, "storage")
		}
		if i.logger != nil {
			i.logger.Error(ctx, "Failed to store chunks", err, map[string]interface{}{"chunks_count": len(chunks)})
		}
		if i.tracer != nil {
			if span, ok := i.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"

		// Decrement indexing documents count
		i.mu.Lock()
		i.indexingDocuments--
		if i.metrics != nil {
			i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
		}
		i.mu.Unlock()

		return errors.ErrStorage("add chunks", err).
			WithContext("chunks_count", fmt.Sprintf("%d", len(chunks)))
	}
	if i.logger != nil {
		i.logger.Debug(ctx, "Chunks stored", map[string]interface{}{
			"duration":     time.Since(storeStartTime).Seconds(),
			"chunks_count": len(chunks),
		})
	}

	// Increment indexed documents count
	i.mu.Lock()
	i.indexedDocuments++
	i.indexingDocuments--
	if i.metrics != nil {
		i.metrics.RecordIndexedDocuments(ctx, i.indexedDocuments)
		i.metrics.RecordIndexingDocuments(ctx, i.indexingDocuments)
	}
	i.mu.Unlock()

	// Record system metrics
	if i.metrics != nil {
		// In a real implementation, you would collect actual system metrics
		// For now, we'll use dummy values
		cpuUsage := 0.0
		memoryUsage := 0.0
		i.metrics.RecordSystemMetrics(ctx, cpuUsage, memoryUsage)
	}

	// Record metrics
	if i.metrics != nil {
		duration := time.Since(startTime)
		i.metrics.RecordIndexLatency(ctx, duration)
		i.metrics.RecordIndexCount(ctx, status)
	}

	// Log index
	if i.logger != nil {
		i.logger.Info(ctx, "Index completed", map[string]interface{}{
			"duration":           time.Since(startTime).Seconds(),
			"chunks_count":       len(chunks),
			"source_type":        source.Type,
			"indexed_documents":  i.indexedDocuments,
			"indexing_documents": i.indexingDocuments,
		})
	}

	return nil
}

// BatchIndex adds multiple documents to the RAG engine in batch
func (i *Indexer) BatchIndex(ctx context.Context, sources []Source) error {
	if len(sources) == 0 {
		return nil
	}

	// Process each source in batch
	for _, source := range sources {
		if err := i.Index(ctx, source); err != nil {
			return fmt.Errorf("failed to index source: %w", err)
		}
	}

	return nil
}

// AsyncIndex adds documents to the RAG engine asynchronously
func (i *Indexer) AsyncIndex(ctx context.Context, source Source) error {
	// Use a detached context so background work survives request cancellation
	bgCtx := context.WithoutCancel(ctx)
	go func() {
		if err := i.Index(bgCtx, source); err != nil {
			if i.logger != nil {
				i.logger.Error(bgCtx, "Async indexing failed", err, map[string]interface{}{
					"source_path": source.Path,
				})
			}
		}
	}()

	return nil
}

// AsyncBatchIndex adds multiple documents to the RAG engine asynchronously
func (i *Indexer) AsyncBatchIndex(ctx context.Context, sources []Source) error {
	bgCtx := context.WithoutCancel(ctx)
	go func() {
		if err := i.BatchIndex(bgCtx, sources); err != nil {
			if i.logger != nil {
				i.logger.Error(bgCtx, "Async batch indexing failed", err, map[string]interface{}{
					"source_count": len(sources),
				})
			}
		}
	}()

	return nil
}

// IndexDirectory indexes all files in a directory recursively with concurrent workers
func (i *Indexer) IndexDirectory(ctx context.Context, directoryPath string) error {
	startTime := time.Now()
	var fileCount, successCount, errorCount int64 // Use int64 for atomic operations

	// Create a wait group for concurrency
	var wg sync.WaitGroup
	// Create a channel to receive files
	fileChan := make(chan string, i.config.FileChannelBuffer)
	// Create a channel to receive errors (unbuffered to avoid deadlock)
	errChan := make(chan error)
	// Create a channel to receive indexing results
	resultChan := make(chan bool, i.config.FileChannelBuffer)

	// Start worker goroutines
	workerCount := i.config.WorkerCount
	if i.logger != nil {
		i.logger.Info(ctx, "Starting directory indexing", map[string]interface{}{
			"directory":    directoryPath,
			"worker_count": workerCount,
			"buffer_size":  i.config.FileChannelBuffer,
		})
	}

	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			if i.logger != nil {
				i.logger.Debug(ctx, "Worker started", map[string]interface{}{"worker_id": workerID})
			}
			for filePath := range fileChan {
				// Get file extension and convert to lowercase
				ext := strings.ToLower(filepath.Ext(filePath))

				// Create source with file type and path
				source := Source{
					Type: ext,      // File extension (e.g., ".pdf", ".docx")
					Path: filePath, // Full path to the file
				}

				// Index the file using the appropriate parser
				fileStartTime := time.Now()
				if err := i.Index(ctx, source); err != nil {
					atomic.AddInt64(&errorCount, 1)
					select {
					case errChan <- fmt.Errorf("failed to index file %s: %w", filePath, err):
					case <-ctx.Done():
						return
					}
					resultChan <- false
					if i.logger != nil {
						i.logger.Error(ctx, "File indexing failed", err, map[string]interface{}{
							"file_path": filePath,
							"worker_id": workerID,
							"duration":  time.Since(fileStartTime).Seconds(),
						})
					}
				} else {
					atomic.AddInt64(&successCount, 1)
					resultChan <- true
					if i.logger != nil {
						i.logger.Debug(ctx, "File indexing succeeded", map[string]interface{}{
							"file_path": filePath,
							"worker_id": workerID,
							"duration":  time.Since(fileStartTime).Seconds(),
						})
					}
				}
			}
		}(workerID)
	}

	// Walk directory and send files to workers
	go func() {
		defer close(fileChan) // Close channel when done walking
		if err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				// Check if we have a parser for this file extension
				ext := strings.ToLower(filepath.Ext(path))
				i.mu.RLock()
				_, ok := i.parsers[ext]
				i.mu.RUnlock()

				if ok {
					atomic.AddInt64(&fileCount, 1)
					select {
					case fileChan <- path:
					case <-ctx.Done():
						return ctx.Err() // Handle cancellation
					}
				}
			}
			return nil
		}); err != nil {
			select {
			case errChan <- fmt.Errorf("failed to walk directory: %w", err):
			case <-ctx.Done():
			}
		}
	}()

	// Collect errors and results concurrently
	var indexErrors []error
	errorCollectorDone := make(chan struct{})
	go func() {
		defer close(errorCollectorDone)
		for err := range errChan {
			indexErrors = append(indexErrors, err)
		}
	}()

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(errChan)
		close(resultChan)
	}()

	// Consume results
	for range resultChan {
		// Just consume the channel
	}

	// Wait for error collector to finish
	<-errorCollectorDone

	// Record metrics
	if i.metrics != nil {
		duration := time.Since(startTime)
		i.metrics.RecordIndexLatency(ctx, duration)
		i.metrics.RecordIndexedDocuments(ctx, int(atomic.LoadInt64(&successCount)))
		errCount := int(atomic.LoadInt64(&errorCount))
		for j := 0; j < errCount; j++ {
			i.metrics.RecordErrorCount(ctx, "indexing")
		}
	}

	// Load final counts
	finalFileCount := atomic.LoadInt64(&fileCount)
	finalSuccessCount := atomic.LoadInt64(&successCount)
	finalErrorCount := atomic.LoadInt64(&errorCount)

	// Log summary
	if i.logger != nil {
		successRate := 0.0
		if finalFileCount > 0 {
			successRate = float64(finalSuccessCount) / float64(finalFileCount) * 100
		}
		i.logger.Info(ctx, "Directory indexing completed", map[string]interface{}{
			"directory":    directoryPath,
			"total_files":  finalFileCount,
			"success":      finalSuccessCount,
			"errors":       finalErrorCount,
			"duration":     time.Since(startTime).Seconds(),
			"success_rate": successRate,
		})
	}

	// Return aggregated errors if any
	if len(indexErrors) > 0 {
		return errors.NewError(errors.ErrorTypeStorage, fmt.Sprintf("failed to index %d files in directory %s", len(indexErrors), directoryPath)).
			WithContext("failed_count", fmt.Sprintf("%d", len(indexErrors))).
			WithContext("total_count", fmt.Sprintf("%d", finalFileCount)).
			WithContext("success_count", fmt.Sprintf("%d", finalSuccessCount)).
			WithContext("directory", directoryPath).
			WithSuggestion("Check the individual file errors for details").
			WithSuggestion("Ensure all files are in supported formats")
	}

	return nil
}

// AsyncIndexDirectory indexes all files in a directory recursively asynchronously
func (i *Indexer) AsyncIndexDirectory(ctx context.Context, directoryPath string) error {
	bgCtx := context.WithoutCancel(ctx)
	go func() {
		if err := i.IndexDirectory(bgCtx, directoryPath); err != nil {
			if i.logger != nil {
				i.logger.Error(bgCtx, "Async directory indexing failed", err, map[string]interface{}{
					"directory": directoryPath,
				})
			}
		}
	}()

	return nil
}
