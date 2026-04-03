package expand

import (
	"context"
	"time"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// ensure interface implementation
var _ core.ResultEnhancer = (*ParentDoc)(nil)

// ParentDoc expands retrieved chunks to their full parent documents.
type ParentDoc struct {
	docStore  core.DocStore
	logger    logging.Logger
	collector observability.Collector
}

// ParentDocOption configures a ParentDoc instance.
type ParentDocOption func(*ParentDoc)

// WithParentDocLogger sets a structured logger.
func WithParentDocLogger(logger logging.Logger) ParentDocOption {
	return func(e *ParentDoc) {
		if logger != nil {
			e.logger = logger
		}
	}
}

// WithParentDocCollector sets an observability collector.
func WithParentDocCollector(collector observability.Collector) ParentDocOption {
	return func(e *ParentDoc) {
		if collector != nil {
			e.collector = collector
		}
	}
}

// NewParentDoc creates a new parent document expander.
func NewParentDoc(docStore core.DocStore, opts ...ParentDocOption) *ParentDoc {
	e := &ParentDoc{
		docStore:  docStore,
		logger:    logging.DefaultNoopLogger(),
		collector: observability.DefaultNoopCollector(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Enhance implements core.ResultEnhancer.
func (e *ParentDoc) Enhance(ctx context.Context, query *core.Query, chunks []*core.Chunk) ([]*core.Chunk, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("parent_doc_expansion", time.Since(start), nil)
	}()

	if len(chunks) == 0 {
		return chunks, nil
	}

	expandedChunks := make([]*core.Chunk, 0, len(chunks))
	seenDocs := make(map[string]bool)

	for _, chunk := range chunks {
		if chunk.ParentID == "" {
			expandedChunks = append(expandedChunks, chunk)
			continue
		}

		if seenDocs[chunk.ParentID] {
			continue
		}

		parentDoc, err := e.docStore.GetDocument(ctx, chunk.ParentID)
		if err != nil || parentDoc == nil {
			expandedChunks = append(expandedChunks, chunk)
			continue
		}

		expandedChunk := &core.Chunk{
			ID:         chunk.ID,
			DocumentID: chunk.DocumentID,
			ParentID:   "",
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
	}

	return expandedChunks, nil
}

func mergeMetadata(child, parent map[string]any) map[string]any {
	merged := make(map[string]any)
	for k, v := range parent {
		merged[k] = v
	}
	for k, v := range child {
		merged[k] = v
	}
	return merged
}
