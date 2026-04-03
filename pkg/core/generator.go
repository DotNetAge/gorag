package core

import "context"

// Generator defines the interface for generating answers based on query and retrieved context.
// It combines LLM capabilities with retrieved knowledge to produce accurate responses.
type Generator interface {
	Generate(ctx context.Context, query *Query, chunks []*Chunk) (*Result, error)
	GenerateHypotheticalDocument(ctx context.Context, query *Query) (string, error)
}
