package core

import "context"

// WebSearcher defines the interface for external web search capabilities (used in CRAG).
// It provides fallback to web search when internal knowledge base is insufficient.
type WebSearcher interface {
	Search(ctx context.Context, query string, topK int) ([]*Chunk, error)
}
