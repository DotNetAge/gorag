package cache

import "context"

// SemanticCache provides caching for queries based on their vector semantic similarity.
type SemanticCache interface {
	// Get retrieves a cached response if the semantic distance of the query vector
	// is within the given threshold (e.g., Cosine Similarity > 0.98).
	Get(ctx context.Context, queryEmbedding []float32, threshold float32) (string, bool, error)

	// Set stores the query vector and its corresponding response in the cache.
	Set(ctx context.Context, queryEmbedding []float32, response string) error
}
