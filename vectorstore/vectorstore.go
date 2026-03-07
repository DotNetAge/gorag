package vectorstore

import (
	"context"
)

// Store defines the interface for vector storage
//
// This interface is implemented by all vector store backends
// (Memory, Milvus, Qdrant, Pinecone, Weaviate) and allows
// the RAG engine to store and retrieve embeddings.
//
// Example implementation:
//
//     type MemoryStore struct {
//         vectors []Vector
//         mutex   sync.RWMutex
//     }
//
//     func (s *MemoryStore) Add(ctx context.Context, chunks []Chunk, embeddings [][]float32) error {
//         // Store chunks and embeddings in memory
//     }
//
//     func (s *MemoryStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]Result, error) {
//         // Search for similar embeddings
//     }
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
	Add(ctx context.Context, chunks []Chunk, embeddings [][]float32) error
	
	// Search searches for similar vectors to the query embedding
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - query: Query embedding
	// - opts: Search options (TopK, filters, etc.)
	//
	// Returns:
	// - []Result: Slice of search results
	// - error: Error if search fails
	Search(ctx context.Context, query []float32, opts SearchOptions) ([]Result, error)
	
	// SearchStructured performs a structured search with filters
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - query: Structured query with filters
	// - embedding: Query embedding
	//
	// Returns:
	// - []Result: Slice of search results
	// - error: Error if search fails
	SearchStructured(ctx context.Context, query *StructuredQuery, embedding []float32) ([]Result, error)
	
	// Delete removes vectors by their IDs
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - ids: Slice of vector IDs to delete
	//
	// Returns:
	// - error: Error if deletion fails
	Delete(ctx context.Context, ids []string) error
	
	// GetByMetadata retrieves vectors by metadata
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - metadata: Metadata filter
	//
	// Returns:
	// - []Result: Slice of matching results
	// - error: Error if retrieval fails
	GetByMetadata(ctx context.Context, metadata map[string]string) ([]Result, error)
}

// Chunk represents a document chunk in the vector store
//
// A Chunk contains the content and metadata of a piece of a document
// that has been stored in the vector store.
//
// Example:
//
//     chunk := Chunk{
//         ID:       "chunk-1",
//         Content:  "Go is an open source programming language...",
//         Metadata: map[string]string{
//             "source": "example.txt",
//             "page":   "1",
//         },
//         MediaType: "text/plain",
//     }
type Chunk struct {
	ID         string            // Unique identifier for the chunk
	Content    string            // Text content of the chunk
	Metadata   map[string]string // Metadata about the chunk
	MediaType  string            // Media type (e.g., "text/plain", "image/jpeg")
	MediaData  []byte            // Binary data for non-text content
}

// Result represents a search result
//
// A Result contains a chunk and its similarity score to the query.
//
// Example:
//
//     result := Result{
//         Chunk: Chunk{
//             ID:      "chunk-1",
//             Content: "Go is an open source programming language...",
//         },
//         Score: 0.95, // High similarity score
//     }
type Result struct {
	Chunk          // Embedded chunk
	Score float32  // Similarity score (0.0-1.0)
}

// SearchOptions configures search behavior
//
// This struct defines options for vector search operations.
//
// Example:
//
//     opts := SearchOptions{
//         TopK:     5,
//         MinScore: 0.7,
//         Filter: map[string]interface{}{
//             "source": "technical-docs",
//         },
//     }
type SearchOptions struct {
	TopK     int                    // Number of top results to return
	Filter   map[string]interface{} // Metadata filters
	MinScore float32                // Minimum similarity score
	Metadata map[string]string      // Additional metadata for search
}

// FilterOperator defines filter operators for structured queries
type FilterOperator string

const (
	FilterOpEq      FilterOperator = "eq"      // Equal
	FilterOpNeq     FilterOperator = "neq"     // Not equal
	FilterOpGt      FilterOperator = "gt"      // Greater than
	FilterOpGte     FilterOperator = "gte"     // Greater than or equal
	FilterOpLt      FilterOperator = "lt"      // Less than
	FilterOpLte     FilterOperator = "lte"     // Less than or equal
	FilterOpIn      FilterOperator = "in"      // In array
	FilterOpNin     FilterOperator = "nin"     // Not in array
	FilterOpContains FilterOperator = "contains" // Contains substring
)

// FilterCondition represents a single filter condition
//
// Example:
//
//     condition := FilterCondition{
//         Field:    "source",
//         Operator: FilterOpEq,
//         Value:    "technical-docs",
//     }
type FilterCondition struct {
	Field    string          // Field to filter on
	Operator FilterOperator  // Operator to use
	Value    interface{}     // Value to compare against
}

// StructuredQuery represents a structured search query
//
// Example:
//
//     query := &StructuredQuery{
//         Query: "Go programming language",
//         Filters: []FilterCondition{
//             {
//                 Field:    "category",
//                 Operator: FilterOpEq,
//                 Value:    "programming",
//             },
//         },
//         TopK:     5,
//         MinScore: 0.7,
//     }
type StructuredQuery struct {
	Query    string              // Natural language query
	Filters  []FilterCondition   // Filter conditions
	TopK     int                 // Number of top results
	MinScore float32             // Minimum similarity score
}
