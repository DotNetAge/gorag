package core

import "context"

// QueryDecomposer defines the interface for decomposing complex queries into simpler sub-queries.
// It breaks down multi-hop or compound questions into atomic queries for better retrieval coverage.
type QueryDecomposer interface {
	Decompose(ctx context.Context, query *Query) (*DecompositionResult, error)
}
