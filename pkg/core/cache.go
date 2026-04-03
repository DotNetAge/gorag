package core

import "context"

// SemanticCache provides semantic-based caching for queries.
// It stores and retrieves cached responses based on query similarity.
type SemanticCache interface {
	// CheckCache checks if a cached response exists for the given query.
	// Returns CacheResult with Hit=true and Answer if similarity exceeds threshold.
	// Returns CacheResult with Hit=false if no match found or similarity below threshold.
	CheckCache(ctx context.Context, query *Query) (*CacheResult, error)

	// CacheResponse stores the query and its response in the cache.
	// The cache may use query text or embedding for similarity matching.
	CacheResponse(ctx context.Context, query *Query, answer *Result) error
}

// CacheResult holds the result of a cache check operation.
type CacheResult struct {
	Hit    bool
	Answer string
}
