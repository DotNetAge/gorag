package core

import "context"

// FusionEngine defines the interface for fusing multiple retrieval results.
// It combines results from different retrievers or strategies using techniques like Reciprocal Rank Fusion.
type FusionEngine interface {
	Fuse(ctx context.Context, results [][]*Chunk, topK int) ([]*Chunk, error)
	ReciprocalRankFusion(ctx context.Context, results [][]*Chunk, topK int) ([]*Chunk, error)
}
