package vectorstore

import (
	"context"
)

// Store defines the interface for vector storage
type Store interface {
	Add(ctx context.Context, chunks []Chunk, embeddings [][]float32) error
	Search(ctx context.Context, query []float32, opts SearchOptions) ([]Result, error)
	Delete(ctx context.Context, ids []string) error
}

// Chunk represents a document chunk
type Chunk struct {
	ID       string
	Content  string
	Metadata map[string]string
	MediaType string // e.g., "text/plain", "image/jpeg", "image/png"
	MediaData []byte // Binary data for non-text content
}

// Result represents a search result
type Result struct {
	Chunk
	Score float32
}

// SearchOptions configures search behavior
type SearchOptions struct {
	TopK      int
	Filter    map[string]interface{}
	MinScore  float32
}
