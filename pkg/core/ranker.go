package core

import "context"

// Reranker is a specialized ResultEnhancer for reranking retrieved chunks.
// It applies cross-encoder models or other sophisticated scoring methods to reorder results by relevance.
type Reranker interface {
	Rerank(ctx context.Context, query *Query, chunks []*Chunk) ([]*Chunk, error)
}
