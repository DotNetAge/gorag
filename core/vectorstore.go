package core

import "context"

// VectorStore defines the interface for vector storage and similarity search.
// It provides methods for storing embedding vectors and performing efficient nearest neighbor searches.
// Implementations can use various vector databases like Milvus, Pinecone, Qdrant, Weaviate, or in-memory stores.
//
// Key responsibilities:
//   - Store and update vector embeddings with associated metadata
//   - Perform similarity searches using cosine distance or other metrics
//   - Support metadata filtering during searches
//   - Manage the lifecycle of stored vectors
//
// Example usage:
//
//	store := NewMilvusVectorStore()
//	err := store.Upsert(ctx, vectors)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	results, scores, err := store.Search(ctx, queryVector, 10, filters)
type VectorStore interface {
	// Upsert inserts or updates vectors in the store.
	// If a vector with the same ID exists, it will be updated; otherwise, it will be inserted.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - vectors: Slice of vectors to insert or update
	//
	// Returns:
	//   - error: Any error that occurred during the operation
	Upsert(ctx context.Context, vectors []*Vector) error

	// Search performs a similarity search to find the most similar vectors.
	// It returns the topK most similar vectors along with their similarity scores.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - query: The query vector to search for
	//   - topK: Maximum number of results to return
	//   - filters: Optional metadata filters to apply
	//
	// Returns:
	//   - []*Vector: The most similar vectors found
	//   - []float32: Similarity scores for each result
	//   - error: Any error that occurred during search
	Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*Vector, []float32, error)

	// Delete removes a vector from the store by its ID.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - id: The unique identifier of the vector to delete
	//
	// Returns:
	//   - error: Any error that occurred during deletion
	Delete(ctx context.Context, id string) error

	// Close gracefully shuts down the vector store connection.
	// It should release all resources and close any open connections.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//
	// Returns:
	//   - error: Any error that occurred during shutdown
	Close(ctx context.Context) error
}
