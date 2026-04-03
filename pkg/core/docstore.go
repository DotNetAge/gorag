package core

import (
	"context"
)

// DocStore is responsible for storing the actual Documents and Chunks/Nodes.
// Unlike VectorStore which only stores embeddings and metadata for ANN search,
// DocStore holds the full text and structural relationships (Parent-Child).
// This is essential for Advanced RAG (e.g., Parent Document Retrieval, Multi-Hop).
type DocStore interface {
	// SetDocument stores a full document
	SetDocument(ctx context.Context, doc *Document) error
	// GetDocument retrieves a document by ID
	GetDocument(ctx context.Context, docID string) (*Document, error)
	// DeleteDocument removes a document and optionally its chunks
	DeleteDocument(ctx context.Context, docID string) error

	// SetChunks stores multiple chunks (nodes)
	SetChunks(ctx context.Context, chunks []*Chunk) error
	// GetChunk retrieves a specific chunk by ID
	GetChunk(ctx context.Context, chunkID string) (*Chunk, error)
	// GetChunksByDocID retrieves all chunks belonging to a document
	GetChunksByDocID(ctx context.Context, docID string) ([]*Chunk, error)
}
