package enrich

import (
	"context"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type docstoreEnrichStep struct {
	store  core.DocStore
	logger logging.Logger
}

// EnrichWithDocStore creates a pipeline step to enrich retrieved chunks with full document context.
func EnrichWithDocStore(s core.DocStore, logger logging.Logger) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &docstoreEnrichStep{
		store:  s,
		logger: logger,
	}
}

func (s *docstoreEnrichStep) Name() string {
	return "DocStoreEnrichment"
}

func (s *docstoreEnrichStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if s.store == nil {
		return nil
	}

	for _, group := range context.RetrievedChunks {
		for _, chunk := range group {
			// If chunk already has content but we want the full parent document
			if chunk.DocumentID != "" {
				doc, err := s.store.GetDocument(ctx, chunk.DocumentID)
				if err != nil {
					s.logger.Warn("failed to enrich chunk from docstore", map[string]any{
						"doc_id": chunk.DocumentID,
						"err":    err,
					})
					continue
				}

				// Attach parent document content to chunk metadata or replace content
				// depending on strategy. Here we add it to metadata for the generator to decide.
				if chunk.Metadata == nil {
					chunk.Metadata = make(map[string]any)
				}
				chunk.Metadata["parent_content"] = doc.Content
				s.logger.Debug("enriched chunk with parent document", map[string]any{"doc_id": doc.ID})
			}
		}
	}

	return nil
}
