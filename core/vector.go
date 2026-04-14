// Package entity defines the core entities for the goRAG framework.
package core

import "github.com/google/uuid"

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
//   - values: the embedding vector values (float32 slice)
//   - metadata: additional metadata for filtering and tracking
func NewVector(values []float32, metadata map[string]any) *Vector {
	return &Vector{
		ID:       uuid.NewString(),
		Values:   values,
		ChunkID:  uuid.NewString(),
		Metadata: metadata,
	}
}
