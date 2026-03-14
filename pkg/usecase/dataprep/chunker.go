package dataprep

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// Chunker defines the interface for document splitting.
type Chunker interface {
	// Chunk splits a single document into a slice of chunks.
	Chunk(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, error)
}

// SemanticChunker extends Chunker to support Advanced RAG Chunking patterns.
type SemanticChunker interface {
	Chunker
	
	// HierarchicalChunk creates Parent-Child relationships for fine-grained retrieval
	// but broad context augmentation.
	HierarchicalChunk(ctx context.Context, doc *entity.Document) (parents []*entity.Chunk, children []*entity.Chunk, err error)
	
	// ContextualChunk injects a document-level summary into each child chunk's content
	// to preserve global context (Anthropic's Contextual Retrieval pattern).
	ContextualChunk(ctx context.Context, doc *entity.Document, docSummary string) ([]*entity.Chunk, error)
}
