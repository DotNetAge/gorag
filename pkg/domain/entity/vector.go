// Package entity defines the core entities for the goRAG framework.
package entity

// Vector represents a vector entity in the RAG system.
// It contains the vector representation of a document chunk.
//
// Related RAG concepts:
// - Vector Embedding: Represents the vector embedding of a chunk
// - Semantic Search: Used for semantic search based on vector similarity
// - Cosine Similarity & Distance Metrics: Used to calculate similarity between vectors
// - Vector Database: Stored in vector databases for efficient retrieval
type Vector struct {
	ID        string                 `json:"id"`        // Unique identifier for the vector
	Values    []float32              `json:"values"`    // The vector values
	ChunkID   string                 `json:"chunk_id"`   // ID of the corresponding chunk
	Metadata  map[string]any         `json:"metadata"`  // Additional metadata about the vector
}

// NewVector creates a new vector entity.
func NewVector(id string, values []float32, chunkID string, metadata map[string]any) *Vector {
	return &Vector{
		ID:       id,
		Values:   values,
		ChunkID:  chunkID,
		Metadata: metadata,
	}
}
