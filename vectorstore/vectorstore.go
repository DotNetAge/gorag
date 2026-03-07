package vectorstore

import (
	"context"
)

// Store defines the interface for vector storage
type Store interface {
	Add(ctx context.Context, chunks []Chunk, embeddings [][]float32) error
	Search(ctx context.Context, query []float32, opts SearchOptions) ([]Result, error)
	SearchStructured(ctx context.Context, query *StructuredQuery, embedding []float32) ([]Result, error)
	Delete(ctx context.Context, ids []string) error
	GetByMetadata(ctx context.Context, metadata map[string]string) ([]Result, error)
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
	Metadata  map[string]string
}

// FilterOperator defines filter operators
type FilterOperator string

const (
	FilterOpEq    FilterOperator = "eq"    // Equal
	FilterOpNeq   FilterOperator = "neq"   // Not equal
	FilterOpGt    FilterOperator = "gt"    // Greater than
	FilterOpGte   FilterOperator = "gte"   // Greater than or equal
	FilterOpLt    FilterOperator = "lt"    // Less than
	FilterOpLte   FilterOperator = "lte"   // Less than or equal
	FilterOpIn    FilterOperator = "in"    // In array
	FilterOpNin   FilterOperator = "nin"   // Not in array
	FilterOpContains FilterOperator = "contains" // Contains substring
)

// FilterCondition represents a single filter condition
type FilterCondition struct {
	Field    string
	Operator FilterOperator
	Value    interface{}
}

// StructuredQuery represents a structured search query
type StructuredQuery struct {
	Query     string
	Filters   []FilterCondition
	TopK      int
	MinScore  float32
}
