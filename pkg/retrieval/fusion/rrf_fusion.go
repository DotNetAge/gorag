// Package fusion provides result fusion strategies for combining multiple retrieval results.
// It implements algorithms to merge and rank results from different retrieval methods
// or sources into a single, optimized result set.
//
// The package includes Reciprocal Rank Fusion (RRF), a robust algorithm for combining
// ranked lists that doesn't require score normalization.
package fusion

import (
	"context"
	"sort"

	"github.com/DotNetAge/gorag/pkg/core"
)

var _ core.FusionEngine = (*RRFFusionEngine)(nil)

// RRFFusionEngine implements Reciprocal Rank Fusion for combining multiple result sets.
// RRF is a simple yet effective method for merging ranked lists that computes scores
// based on the rank positions rather than the raw similarity scores.
//
// The RRF formula is: RRF_Score(d) = Σ 1/(k + rank_i(d))
// where:
//   - d is a document/chunk
//   - rank_i(d) is the rank of d in the i-th result list
//   - k is a smoothing constant (default: 60)
//
// Advantages of RRF:
//   - Score normalization not required
//   - Handles different retrieval methods gracefully
//   - Robust to outliers and score scale differences
//   - Simple and computationally efficient
//
// Example:
//
//	engine := fusion.NewRRFFusionEngine()
//	results, err := engine.ReciprocalRankFusion(ctx, [][]*core.Chunk{
//	    vectorResults,    // Results from vector search
//	    keywordResults,   // Results from keyword search
//	    graphResults,     // Results from graph traversal
//	}, 10)
type RRFFusionEngine struct {
	k float32
}

// NewRRFFusionEngine creates a new RRF fusion engine with the standard k=60.
// The value k=60 is recommended by the original RRF paper and works well
// across different domains and retrieval systems.
//
// Returns:
//   - *RRFFusionEngine: Configured fusion engine
//
// Example:
//
//	engine := fusion.NewRRFFusionEngine()
//	fused, err := engine.ReciprocalRankFusion(ctx, resultSets, 10)
func NewRRFFusionEngine() *RRFFusionEngine {
	return &RRFFusionEngine{
		k: 60.0,
	}
}

// Fuse performs a simple merge of multiple result sets without ranking.
// It deduplicates chunks by ID and returns up to topK results.
// This is a basic fusion method; use ReciprocalRankFusion for better results.
//
// Parameters:
//   - ctx: Context for cancellation (currently unused)
//   - resultSets: Multiple result sets to merge
//   - topK: Maximum number of results to return
//
// Returns:
//   - []*core.Chunk: Merged and deduplicated results (up to topK)
//   - error: Always nil (included for interface compatibility)
func (e *RRFFusionEngine) Fuse(ctx context.Context, resultSets [][]*core.Chunk, topK int) ([]*core.Chunk, error) {
	if len(resultSets) == 0 {
		return nil, nil
	}
	if len(resultSets) == 1 {
		return e.limit(resultSets[0], topK), nil
	}

	seen := make(map[string]bool)
	var merged []*core.Chunk
	for _, resultSet := range resultSets {
		for _, chunk := range resultSet {
			if !seen[chunk.ID] {
				merged = append(merged, chunk)
				seen[chunk.ID] = true
			}
		}
	}

	return e.limit(merged, topK), nil
}

// ReciprocalRankFusion merges results from different retrieval methods using RRF algorithm.
// It computes a fused score for each chunk based on its rank position in each result set,
// then returns the chunks sorted by their fused scores.
//
// Parameters:
//   - ctx: Context for cancellation (currently unused)
//   - resultSets: Multiple ranked result sets to fuse
//   - topK: Maximum number of results to return
//
// Returns:
//   - []*core.Chunk: Fused and ranked results (up to topK)
//   - error: Always nil (included for interface compatibility)
//
// The algorithm:
//  1. For each chunk in each result set, compute RRF score: 1/(k + rank)
//  2. Sum RRF scores across all result sets for each unique chunk
//  3. Sort chunks by their total RRF score (descending)
//  4. Return topK chunks
//
// Example:
//
//	vectorResults := []*core.Chunk{chunk1, chunk2, chunk3}
//	keywordResults := []*core.Chunk{chunk2, chunk4, chunk1}
//	fused, _ := engine.ReciprocalRankFusion(ctx, [][]*core.Chunk{
//	    vectorResults, keywordResults,
//	}, 5)
func (e *RRFFusionEngine) ReciprocalRankFusion(ctx context.Context, resultSets [][]*core.Chunk, topK int) ([]*core.Chunk, error) {
	if len(resultSets) == 0 {
		return nil, nil
	}
	if len(resultSets) == 1 {
		return e.limit(resultSets[0], topK), nil
	}

	scoreMap := make(map[string]float32)
	chunkMap := make(map[string]*core.Chunk)

	for _, resultSet := range resultSets {
		for rank, chunk := range resultSet {
			score := 1.0 / (e.k + float32(rank+1))

			scoreMap[chunk.ID] += score

			if _, exists := chunkMap[chunk.ID]; !exists {
				chunkMap[chunk.ID] = chunk
			}
		}
	}

	var fusedChunks []*core.Chunk
	for _, chunk := range chunkMap {
		fusedChunks = append(fusedChunks, chunk)
	}

	sort.SliceStable(fusedChunks, func(i, j int) bool {
		return scoreMap[fusedChunks[i].ID] > scoreMap[fusedChunks[j].ID]
	})

	return e.limit(fusedChunks, topK), nil
}

// limit returns at most topK chunks from the input slice.
func (e *RRFFusionEngine) limit(chunks []*core.Chunk, topK int) []*core.Chunk {
	if len(chunks) <= topK {
		return chunks
	}
	return chunks[:topK]
}
