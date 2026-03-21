// Package entity defines the core entities for the goRAG framework.
package core

// Vector represents a vector entity in the RAG system.
// It contains the vector representation of a document chunk.
type Vector struct {
	ID       string         `json:"id"`       // Unique identifier for the vector
	Values   []float32      `json:"values"`   // The vector values
	ChunkID  string         `json:"chunk_id"` // ID of the corresponding chunk
	Metadata map[string]any `json:"metadata"` // Additional metadata about the vector
}

// NewVector creates a new Vector instance with the specified parameters.
//
// Parameters:
//   - id: unique identifier for the vector
//   - values: the embedding vector values (float32 slice)
//   - chunkID: ID of the corresponding chunk this vector represents
//   - metadata: additional metadata for filtering and tracking
func NewVector(id string, values []float32, chunkID string, metadata map[string]any) *Vector {
	return &Vector{
		ID:       id,
		Values:   values,
		ChunkID:  chunkID,
		Metadata: metadata,
	}
}
