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

func (c *Chunk) SetVectorID(vectorID string) {
	c.VectorID = vectorID
}

// Chunker defines the interface for document splitting.
type Chunker interface {
	// Chunk splits a single document into a slice of chunks.
	Chunk(ctx context.Context, doc *Document) ([]*Chunk, error)
}

// SemanticChunker extends Chunker to support Advanced RAG Chunking patterns.
type SemanticChunker interface {
	Chunker

	// HierarchicalChunk creates Parent-Child relationships for fine-grained retrieval
	HierarchicalChunk(ctx context.Context, doc *Document) (parents []*Chunk, children []*Chunk, err error)

	// ContextualChunk injects a document-level summary into each child chunk's content
	ContextualChunk(ctx context.Context, doc *Document, docSummary string) ([]*Chunk, error)
}
