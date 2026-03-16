// Package enhancer provides query and document enhancement utilities for RAG systems.
// This file implements parent document expansion for retrieval result enhancement.
package enhancer

import (
	"context"
	"time"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.ResultEnhancer = (*ParentDocExpander)(nil)

// DocumentStore is an interface for retrieving parent documents.
type DocumentStore interface {
	GetByID(ctx context.Context, id string) (*entity.Document, error)
}

// ParentDocExpander expands retrieved chunks to their full parent documents.
// It helps balance fine-grained retrieval with complete context understanding.
type ParentDocExpander struct {
	docStore  DocumentStore
	logger    logging.Logger
	collector observability.Collector
}

// ParentDocExpanderOption configures a ParentDocExpander instance.
type ParentDocExpanderOption func(*ParentDocExpander)

// WithParentDocLogger sets a structured logger.
func WithParentDocLogger(logger logging.Logger) ParentDocExpanderOption {
	return func(e *ParentDocExpander) {
		if logger != nil {
			e.logger = logger
		}
	}
}

// WithParentDocCollector sets an observability collector.
func WithParentDocCollector(collector observability.Collector) ParentDocExpanderOption {
	return func(e *ParentDocExpander) {
		if collector != nil {
			e.collector = collector
		}
	}
}

// NewParentDocExpander creates a new parent document expander.
//
// Required: docStore.
// Optional (via options): WithParentDocLogger, WithParentDocCollector.
func NewParentDocExpander(docStore DocumentStore, opts ...ParentDocExpanderOption) *ParentDocExpander {
	e := &ParentDocExpander{
		docStore:  docStore,
		logger:    logging.NewNoopLogger(),
		collector: observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Enhance expands retrieved chunks to their full parent documents.
//
// Parameters:
// - ctx: The context for cancellation and timeouts
// - results: The retrieval results to expand
//
// Returns:
// - The expanded retrieval results with full parent documents
// - An error if expansion fails
func (e *ParentDocExpander) Enhance(ctx context.Context, results *entity.RetrievalResult) (*entity.RetrievalResult, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("parent_doc_expansion", time.Since(start), nil)
	}()

	if results == nil || len(results.Chunks) == 0 {
		e.logger.Debug("no chunks to expand", map[string]interface{}{
			"operation": "parent_doc_expansion",
		})
		return results, nil
	}

	e.logger.Debug("expanding parent documents", map[string]interface{}{
		"operation":   "parent_doc_expansion",
		"chunk_count": len(results.Chunks),
	})

	// Expand each chunk to its parent document
	expandedChunks := make([]*entity.Chunk, 0, len(results.Chunks))
	seenDocs := make(map[string]bool) // Avoid duplicates

	for i, chunk := range results.Chunks {
		// Skip if no parent ID (already at root level)
		if chunk.ParentID == "" {
			expandedChunks = append(expandedChunks, chunk)
			continue
		}

		// Check if we've already expanded this parent
		if seenDocs[chunk.ParentID] {
			e.logger.Debug("skipping duplicate parent", map[string]interface{}{
				"operation":   "parent_doc_expansion",
				"parent_id":   chunk.ParentID,
				"chunk_index": i,
			})
			continue
		}

		// Retrieve parent document
		parentDoc, err := e.docStore.GetByID(ctx, chunk.ParentID)
		if err != nil {
			e.logger.Warn("failed to get parent document, using original chunk", map[string]interface{}{
				"operation":   "parent_doc_expansion",
				"error":       err,
				"parent_id":   chunk.ParentID,
				"chunk_index": i,
			})
			expandedChunks = append(expandedChunks, chunk)
			continue
		}

		if parentDoc == nil {
			e.logger.Warn("parent document not found, using original chunk", map[string]interface{}{
				"operation":   "parent_doc_expansion",
				"parent_id":   chunk.ParentID,
				"chunk_index": i,
			})
			expandedChunks = append(expandedChunks, chunk)
			continue
		}

		// Create expanded chunk with parent content
		expandedChunk := &entity.Chunk{
			ID:         chunk.ID,
			DocumentID: chunk.DocumentID,
			ParentID:   "", // Now at root level
			Level:      0,
			Content:    parentDoc.Content,
			Metadata:   mergeMetadata(chunk.Metadata, parentDoc.Metadata),
			CreatedAt:  chunk.CreatedAt,
			StartIndex: 0,
			EndIndex:   len(parentDoc.Content),
			VectorID:   chunk.VectorID,
		}

		expandedChunks = append(expandedChunks, expandedChunk)
		seenDocs[chunk.ParentID] = true

		e.logger.Debug("expanded chunk to parent", map[string]interface{}{
			"operation":     "parent_doc_expansion",
			"chunk_id":      chunk.ID,
			"parent_id":     chunk.ParentID,
			"original_size": len(chunk.Content),
			"expanded_size": len(parentDoc.Content),
		})
	}

	e.logger.Info("parent document expansion completed", map[string]interface{}{
		"operation":       "parent_doc_expansion",
		"original_chunks": len(results.Chunks),
		"expanded_chunks": len(expandedChunks),
		"unique_parents":  len(seenDocs),
	})
	e.collector.RecordCount("parent_doc_expansion", "success", nil)

	// Create new retrieval result with expanded chunks
	return entity.NewRetrievalResult(
		results.ID,
		results.QueryID,
		expandedChunks,
		results.Scores[:len(expandedChunks)],
		results.Metadata,
	), nil
}

// mergeMetadata merges child and parent metadata (child takes precedence).
func mergeMetadata(child, parent map[string]any) map[string]any {
	merged := make(map[string]any)

	// Copy parent first
	for k, v := range parent {
		merged[k] = v
	}

	// Override with child
	for k, v := range child {
		merged[k] = v
	}

	return merged
}
