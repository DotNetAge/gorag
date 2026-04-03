package core

import "context"

// ResultEnhancer defines the interface for enhancing retrieval results.
// It applies post-processing techniques like reranking, expansion, or filtering to improve result quality.
type ResultEnhancer interface {
	Enhance(ctx context.Context, query *Query, chunks []*Chunk) ([]*Chunk, error)
}
