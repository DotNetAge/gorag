// Package abstraction defines the storage abstraction interfaces for the goRAG framework.
package abstraction

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// VectorStore defines the interface for vector storage in the RAG system.
// It provides methods for adding, searching, and managing vectors.
//
// Related RAG concepts:
// - Vector Database: Implements vector storage functionality
// - ANN Algorithm: Uses approximate nearest neighbor algorithms for efficient search
// - Vector DB Selection & Optimization: Allows for different vector database implementations
// - Metadata & Filtering: Supports filtering based on metadata
type VectorStore interface {
	// Add adds a single vector to the store.
	Add(ctx context.Context, vector *entity.Vector) error
	
	// AddBatch adds multiple vectors to the store in batch.
	AddBatch(ctx context.Context, vectors []*entity.Vector) error
	
	// Search searches for similar vectors based on the query vector.
	Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error)
	
	// Delete deletes a vector from the store.
	Delete(ctx context.Context, id string) error
	
	// DeleteBatch deletes multiple vectors from the store in batch.
	DeleteBatch(ctx context.Context, ids []string) error
	
	// Close closes the vector store.
	Close(ctx context.Context) error
}
