package store

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/core"
)

// KVStore defines a generic key-value store interface used as a backend for Document and Index stores.
type KVStore interface {
	Put(ctx context.Context, key string, value map[string]any) error
	Get(ctx context.Context, key string) (map[string]any, error)
	Delete(ctx context.Context, key string) error
	GetAll(ctx context.Context) (map[string]map[string]any, error)
}

// DocStore is responsible for storing the actual Documents and Chunks/Nodes.
// Unlike VectorStore which only stores embeddings and metadata for ANN search,
// DocStore holds the full text and structural relationships (Parent-Child).
// This is essential for Advanced RAG (e.g., Parent Document Retrieval, Multi-Hop).
type DocStore interface {
	// SetDocument stores a full document
	SetDocument(ctx context.Context, doc *core.Document) error
	// GetDocument retrieves a document by ID
	GetDocument(ctx context.Context, docID string) (*core.Document, error)
	// DeleteDocument removes a document and optionally its chunks
	DeleteDocument(ctx context.Context, docID string) error
	
	// SetChunks stores multiple chunks (nodes)
	SetChunks(ctx context.Context, chunks []*core.Chunk) error
	// GetChunk retrieves a specific chunk by ID
	GetChunk(ctx context.Context, chunkID string) (*core.Chunk, error)
	// GetChunksByDocID retrieves all chunks belonging to a document
	GetChunksByDocID(ctx context.Context, docID string) ([]*core.Chunk, error)
}
