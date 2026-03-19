package fusion

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
)

// FusionEngine is responsible for merging multiple retrieval streams.
type FusionEngine interface {
	// ReciprocalRankFusion (RRF) merges results from different modalities
	// (e.g., Sparse + Dense, or Vector + Graph) and re-ranks them based on their reciprocal positions.
	ReciprocalRankFusion(ctx context.Context, resultSets [][]*core.Chunk, topK int) ([]*core.Chunk, error)
}
