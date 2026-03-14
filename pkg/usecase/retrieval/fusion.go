package retrieval

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// FusionEngine is responsible for merging multiple retrieval streams.
type FusionEngine interface {
	// ReciprocalRankFusion (RRF) merges results from different modalities
	// (e.g., Sparse + Dense, or Vector + Graph) and re-ranks them based on their reciprocal positions.
	ReciprocalRankFusion(ctx context.Context, resultSets [][]*entity.Chunk, topK int) ([]*entity.Chunk, error)
}
