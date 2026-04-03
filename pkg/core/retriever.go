package core

import "context"

// Retriever defines the interface for retrieving relevant chunks based on queries.
// It is the core component responsible for finding and ranking relevant information
type Retriever interface {
	Retrieve(ctx context.Context, queries []string, topK int) ([]*RetrievalResult, error)
}
