// Package dataprep provides data preparation utilities for RAG systems.
// It includes chunkers, parsers, and graph extractors to process and transform
// raw data into structured formats suitable for retrieval and generation.
package dataprep

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// Chunker defines the interface for document splitting.
// Implementations of this interface are responsible for breaking down
// large documents into smaller, more manageable chunks for retrieval.
type Chunker interface {
	// Chunk splits a single document into a slice of chunks.
	// The chunks should be optimized for retrieval and generation tasks.
	//
	// Parameters:
	// - ctx: The context for the operation
	// - doc: The document to split
	//
	// Returns:
	// - A slice of chunks
	// - An error if splitting fails
	Chunk(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, error)
}

// SemanticChunker extends Chunker to support Advanced RAG Chunking patterns.
// It provides additional methods for more sophisticated chunking strategies.
type SemanticChunker interface {
	Chunker
	
	// HierarchicalChunk creates Parent-Child relationships for fine-grained retrieval
	// but broad context augmentation.
	// This pattern allows for retrieving specific chunks while maintaining
	// access to broader context.
	//
	// Parameters:
	// - ctx: The context for the operation
	// - doc: The document to split
	//
	// Returns:
	// - A slice of parent chunks
	// - A slice of child chunks
	// - An error if splitting fails
	HierarchicalChunk(ctx context.Context, doc *entity.Document) (parents []*entity.Chunk, children []*entity.Chunk, err error)
	
	// ContextualChunk injects a document-level summary into each child chunk's content
	// to preserve global context (Anthropic's Contextual Retrieval pattern).
	// This helps maintain context awareness during retrieval and generation.
	//
	// Parameters:
	// - ctx: The context for the operation
	// - doc: The document to split
	// - docSummary: A summary of the document to inject into each chunk
	//
	// Returns:
	// - A slice of chunks with injected context
	// - An error if splitting fails
	ContextualChunk(ctx context.Context, doc *entity.Document, docSummary string) ([]*entity.Chunk, error)
}
