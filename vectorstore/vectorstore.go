package vectorstore

import (
	"context"

	"github.com/DotNetAge/gorag/core"
)

// Store defines the interface for vector storage
//
// This interface is implemented by all vector store backends
// (Memory, Milvus, Qdrant, Pinecone, Weaviate) and allows
// the RAG engine to store and retrieve embeddings.
//
// Example implementation:
//
//	type MemoryStore struct {
//	    vectors []Vector
//	    mutex   sync.RWMutex
//	}
//
//	func (s *MemoryStore) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
//	    // Store chunks and embeddings in memory
//	}
//
//	func (s *MemoryStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]core.Result, error) {
//	    // Search for similar embeddings
//	}
type Store interface {
	// Add adds chunks and their embeddings to the vector store
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - chunks: Slice of document chunks
	// - embeddings: Slice of embeddings, one for each chunk
	//
	// Returns:
	// - error: Error if storage fails
	Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error

	// Search searches for similar vectors to the query embedding
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - query: Query embedding
	// - opts: Search options (TopK, filters, etc.)
	//
	// Returns:
	// - []core.Result: Slice of search results
	// - error: Error if search fails
	Search(ctx context.Context, query []float32, opts SearchOptions) ([]core.Result, error)

	// Delete removes vectors by their IDs
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - ids: Slice of vector IDs to delete
	//
	// Returns:
	// - error: Error if deletion fails
	Delete(ctx context.Context, ids []string) error
}

// SearchOptions configures search behavior
//
// This struct defines options for vector search operations.
//
// Example:
//
//	opts := SearchOptions{
//	    TopK:     5,
//	    MinScore: 0.7,
//	    Filter: map[string]interface{}{
//	        "source": "technical-docs",
//	    },
//	}
type SearchOptions struct {
	TopK     int                    // Number of top results to return
	Filter   map[string]interface{} // Metadata filters
	MinScore float32                // Minimum similarity score
	Metadata map[string]string      // Additional metadata for search
}
