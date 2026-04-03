package core

import "context"

// EntityExtractor defines the interface for extracting entities from queries.
// It identifies key terms, names, and concepts to improve retrieval precision.
type EntityExtractor interface {
	Extract(ctx context.Context, query *Query) (*EntityExtractionResult, error)
}

// FilterExtractor defines the interface for extracting search filters from queries.
// It identifies metadata constraints (date ranges, categories, etc.) to refine vector searches.
type FilterExtractor interface {
	Extract(ctx context.Context, query *Query) (map[string]any, error)
}
