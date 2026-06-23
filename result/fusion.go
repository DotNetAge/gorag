// Package result provides result fusion and re-ranking capabilities for multi-source search results.
// It implements Reciprocal Rank Fusion (RRF) for combining results from different indexers
// and cosine similarity-based re-ranking for improving result quality.
package result

import (
	"sort"

	"github.com/DotNetAge/gorag/v2/core"
)

// FusionSource represents search results from a single source (e.g., vector, keyword, graph indexer).
// The Hits field maintains the internal ranking order from that source.
type FusionSource struct {
	Name   string     // 源标识（如 "vector"、"keyword"、"graph"）
	Hits   []core.Hit // 该源的搜索结果，顺序 = 排名
	Weight float32    // 源权重，0 表示 1.0
}

// NewSource creates a new FusionSource with the given name, weight, and hits.
func NewSource(name string, weight float32, hits []core.Hit) *FusionSource {
	return &FusionSource{
		Name:   name,
		Weight: weight,
		Hits:   hits,
	}
}

// RRF performs Reciprocal Rank Fusion on multiple sources using default k=60.
//
// The RRF formula: score(doc) = Σ weight_s / (k + rank_s)
//
// Parameters:
//   - sources: variable number of FusionSource inputs
//
// Returns:
//   - []core.Hit: fused results sorted by score in descending order
//   - error: non-nil only if sources is empty (returns nil, nil)
func RRF(sources ...FusionSource) ([]core.Hit, error) {
	return RRFWithK(60, sources...)
}

// RRFWithK performs RRF fusion with a custom smoothing parameter k.
// Recommended k range: 5-100. Larger values are more tolerant of lower-ranked results.
func RRFWithK(k int, sources ...FusionSource) ([]core.Hit, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	type entry struct {
		hit   core.Hit
		score float32
	}
	scoreMap := make(map[string]*entry)

	for _, src := range sources {
		w := src.Weight
		if w == 0 {
			w = 1.0
		}
		for rank, hit := range src.Hits {
			e, ok := scoreMap[hit.ID]
			if !ok {
				e = &entry{hit: hit, score: 0}
				scoreMap[hit.ID] = e
			}
			e.score += w / float32(k+rank+1)
		}
	}

	fused := make([]core.Hit, 0, len(scoreMap))
	for _, e := range scoreMap {
		e.hit.Score = e.score
		fused = append(fused, e.hit)
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})

	return fused, nil
}
