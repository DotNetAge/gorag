package rerank

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
)

// Reranker defines the interface for Cross-Encoder re-ranking models (e.g., bge-reranker).
// It performs a deep semantic interaction between the query and the retrieved chunks.
type Reranker interface {
	// Rerank re-scores a list of chunks against the query and returns the topK most relevant chunks.
	Rerank(ctx context.Context, query string, chunks []*core.Chunk, topK int) ([]*core.Chunk, []float32, error)
}
