// Package core defines the fundamental entities, interfaces, and types for the goRAG framework.
//
// This package provides the core abstractions used throughout the RAG system including:
//   - Document and Chunk: Core data structures for representing text units
//   - Vector: Embedding vector representation
//   - Query and RetrievalResult: Query processing and retrieval outputs
//   - Core interfaces: Retriever, Generator, Parser, VectorStore, etc.
//
// The interfaces defined in this package serve as contracts for pluggable components,
// enabling flexible swapping of implementations for different use cases.
package core

import (
	"context"
	"time"
)

// Chunk represents a document chunk entity in the RAG system.
// It is a portion of a document that has been processed for vectorization.
type Chunk struct {
	ID         string         `json:"id"`
	DocumentID string         `json:"document_id"`         // ID of the root document
	ParentID   string         `json:"parent_id,omitempty"` // For Parent-Child Indexing (Hierarchical)
	Level      int            `json:"level"`               // 0: Root, 1: Parent, 2: Child
	Content    string         `json:"content"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
	StartIndex int            `json:"start_index"`
	EndIndex   int            `json:"end_index"`
	VectorID   string         `json:"vector_id,omitempty"`
}

// NewChunk creates a new Chunk instance with the specified parameters.
//
// Parameters:
//   - id: unique identifier for the chunk
//   - documentID: ID of the parent document
//   - content: text content of the chunk
//   - startIndex: starting position in the original document
//   - endIndex: ending position in the original document
//   - metadata: additional metadata (can be nil)
func NewChunk(id, documentID, content string, startIndex, endIndex int, metadata map[string]any) *Chunk {
	return &Chunk{
		ID:         id,
		DocumentID: documentID,
		Content:    content,
		Metadata:   metadata,
		CreatedAt:  time.Now(),
		StartIndex: startIndex,
		EndIndex:   endIndex,
	}
}

// SetVectorID associates a vector store ID with this chunk.
// This ID is used to link the chunk to its embedding in the vector store.
//
// Parameters:
//   - vectorID: The unique identifier from the vector store
func (c *Chunk) SetVectorID(vectorID string) {
	c.VectorID = vectorID
}

// Chunker defines the interface for document splitting implementations.
// Chunkers are responsible for breaking down documents into smaller, manageable pieces
// that can be individually embedded and retrieved.
type Chunker interface {
	// Chunk splits a single document into a slice of chunks.
	// The implementation should preserve document metadata and establish proper relationships.
	Chunk(ctx context.Context, doc *Document) ([]*Chunk, error)
}

// SemanticChunker extends Chunker to support Advanced RAG Chunking patterns.
// It provides additional methods for hierarchical and context-aware chunking strategies.
type SemanticChunker interface {
	Chunker

	// HierarchicalChunk creates Parent-Child relationships for fine-grained retrieval.
	// This is useful for multi-resolution retrieval where coarse chunks provide context
	// and fine chunks provide precise information.
	HierarchicalChunk(ctx context.Context, doc *Document) (parents []*Chunk, children []*Chunk, err error)

	// ContextualChunk injects a document-level summary into each child chunk's content.
	// This enhances retrieval quality by providing global context to local chunks.
	ContextualChunk(ctx context.Context, doc *Document, docSummary string) ([]*Chunk, error)
}
