package core

import "context"

// VectorStore defines the interface for vector storage and similarity search.
// It provides methods for storing embedding vectors and performing efficient nearest neighbor searches.
type VectorStore interface {
	Upsert(ctx context.Context, vectors []*Vector) error
	Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*Vector, []float32, error)
	Delete(ctx context.Context, id string) error // 统一为单 ID 删除以匹配实现
	Close(ctx context.Context) error
}
