package fusion

import (
	"context"
	"sort"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.FusionEngine = (*RRFFusionEngine)(nil)

// RRFFusionEngine implements Reciprocal Rank Fusion.
// RRF computes a score for each chunk based on its rank in multiple result sets.
// Formula: RRF_Score = 1 / (k + rank)
type RRFFusionEngine struct {
	// k is a smoothing constant, typically set to 60 as per the original RRF paper.
	k float32
}

// NewRRFFusionEngine creates a new fusion engine with the standard k=60.
func NewRRFFusionEngine() *RRFFusionEngine {
	return &RRFFusionEngine{
		k: 60.0,
	}
}

// ReciprocalRankFusion merges results from different modalities.
func (e *RRFFusionEngine) ReciprocalRankFusion(ctx context.Context, resultSets [][]*entity.Chunk, topK int) ([]*entity.Chunk, error) {
	if len(resultSets) == 0 {
		return nil, nil
	}
	if len(resultSets) == 1 {
		return e.limit(resultSets[0], topK), nil
	}

	// map chunk ID to its fused score and its reference
	scoreMap := make(map[string]float32)
	chunkMap := make(map[string]*entity.Chunk)

	for _, resultSet := range resultSets {
		for rank, chunk := range resultSet {
			// rank is 0-indexed, so we add 1 for the formula
			score := 1.0 / (e.k + float32(rank+1))
			
			scoreMap[chunk.ID] += score
			
			if _, exists := chunkMap[chunk.ID]; !exists {
				chunkMap[chunk.ID] = chunk
			}
		}
	}

	// Extract unique chunks and sort them by the fused RRF score
	var fusedChunks []*entity.Chunk
	for _, chunk := range chunkMap {
		// We could optionally inject the RRF score into the chunk metadata
		// chunk.Metadata["rrf_score"] = scoreMap[chunk.ID]
		fusedChunks = append(fusedChunks, chunk)
	}

	// Sort descending
	sort.SliceStable(fusedChunks, func(i, j int) bool {
		return scoreMap[fusedChunks[i].ID] > scoreMap[fusedChunks[j].ID]
	})

	return e.limit(fusedChunks, topK), nil
}

func (e *RRFFusionEngine) limit(chunks []*entity.Chunk, topK int) []*entity.Chunk {
	if len(chunks) <= topK {
		return chunks
	}
	return chunks[:topK]
}