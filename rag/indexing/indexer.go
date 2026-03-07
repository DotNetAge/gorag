package indexing

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/vectorstore"
)

// Source represents a document source for indexing
type Source struct {
	Type    string      // Document type/format (e.g., "text", ".pdf", ".docx")
	Path    string      // File path (if indexing a file)
	Content string      // Text content (if indexing a string)
	Reader  interface{} // Reader interface (if indexing from a reader)
}

// Indexer handles document indexing operations
type Indexer struct {
	parsers       map[string]parser.Parser
	defaultParser parser.Parser
	embedder      Embedder
	store         vectorstore.Store
	metrics       Metrics
	logger        Logger
	tracer        Tracer
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
	}
}

// Index adds documents to the RAG engine
func (i *Indexer) Index(ctx context.Context, source Source) error {
	startTime := time.Now()
	status := "success"

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
		err := fmt.Errorf("type is required")
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
		return err
	}

	if source.Content == "" && source.Path == "" {
		err := fmt.Errorf("content or path is required")
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
		return err
	}

	var reader io.Reader
	if source.Content != "" {
		reader = strings.NewReader(source.Content)
	} else if source.Reader != nil {
		if r, ok := source.Reader.(io.Reader); ok {
			reader = r
		} else {
			err := fmt.Errorf("invalid reader type")
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
			return err
		}
	} else if source.Path != "" {
		// Read file from path
		file, err := os.Open(source.Path)
		if err != nil {
			err := fmt.Errorf("failed to open file: %w", err)
			if i.metrics != nil {
				i.metrics.RecordErrorCount(ctx, "invalid_input")
			}
			if i.logger != nil {
				i.logger.Error(ctx, "Failed to open file", err, map[string]interface{}{"path": source.Path})
			}
			if i.tracer != nil {
				if span, ok := i.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"
			return err
		}
		defer file.Close()
		reader = file
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
		return err
	}

	parseStartTime := time.Now()
	// Get the appropriate parser for the source type
	p, ok := i.parsers[source.Type]
	if !ok {
		// Use default parser if no specific parser is found
		p = i.defaultParser
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
		return fmt.Errorf("failed to parse document: %w", err)
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
		return nil
	}

	vsChunks := make([]vectorstore.Chunk, len(chunks))
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		vsChunks[i] = vectorstore.Chunk{
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
		return fmt.Errorf("failed to embed chunks: %w", err)
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
		return fmt.Errorf("failed to store chunks: %w", err)
	}
	if i.logger != nil {
		i.logger.Debug(ctx, "Chunks stored", map[string]interface{}{
			"duration":     time.Since(storeStartTime).Seconds(),
			"chunks_count": len(chunks),
		})
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
			"duration":     time.Since(startTime).Seconds(),
			"chunks_count": len(chunks),
			"source_type":  source.Type,
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
	go func() {
		if err := i.Index(ctx, source); err != nil {
			// Log error (in a real implementation, you would use a logger)
			fmt.Printf("Error indexing document: %v\n", err)
		}
	}()

	return nil
}

// AsyncBatchIndex adds multiple documents to the RAG engine asynchronously
func (i *Indexer) AsyncBatchIndex(ctx context.Context, sources []Source) error {
	go func() {
		if err := i.BatchIndex(ctx, sources); err != nil {
			// Log error (in a real implementation, you would use a logger)
			fmt.Printf("Error batch indexing documents: %v\n", err)
		}
	}()

	return nil
}

// IndexDirectory indexes all files in a directory recursively with concurrent workers
func (i *Indexer) IndexDirectory(ctx context.Context, directoryPath string) error {
	// Create a wait group for concurrency
	var wg sync.WaitGroup
	// Create a channel to receive files (buffered for 100 files)
	fileChan := make(chan string, 100)
	// Create a channel to receive errors (buffered for 10 errors)
	errChan := make(chan error, 10)

	// Start 10 worker goroutines
	workerCount := 10 // Optimal balance for most systems
	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileChan {
				// Get file extension and convert to lowercase
				ext := strings.ToLower(filepath.Ext(filePath))

				// Create source with file type and path
				source := Source{
					Type: ext,      // File extension (e.g., ".pdf", ".docx")
					Path: filePath, // Full path to the file
				}

				// Index the file using the appropriate parser
				if err := i.Index(ctx, source); err != nil {
					errChan <- fmt.Errorf("failed to index file %s: %w", filePath, err)
				}
			}
		}()
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
				if _, ok := i.parsers[ext]; ok {
					select {
					case fileChan <- path:
					case <-ctx.Done():
						return ctx.Err() // Handle cancellation
					}
				}
			}
			return nil
		}); err != nil {
			errChan <- fmt.Errorf("failed to walk directory: %w", err)
		}
	}()

	// Wait for all workers to finish and close error channel
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect all errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// Return aggregated errors if any
	if len(errors) > 0 {
		return fmt.Errorf("failed to index some files: %v", errors)
	}

	return nil
}

// AsyncIndexDirectory indexes all files in a directory recursively asynchronously
func (i *Indexer) AsyncIndexDirectory(ctx context.Context, directoryPath string) error {
	go func() {
		if err := i.IndexDirectory(ctx, directoryPath); err != nil {
			// Log error (in a real implementation, you would use a logger)
			fmt.Printf("Error indexing directory: %v\n", err)
		}
	}()

	return nil
}
