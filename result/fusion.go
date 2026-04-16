package result

import (
	"sort"

	"github.com/DotNetAge/gorag/core"
)

// Fusion 使用 Reciprocal Rank Fusion (RRF) 算法将多个搜索源的
// 结果融合为统一排序。
//
// RRF 公式：
//
//	score(doc) = Σ weight_s / (k + rank_s)
//
// rank 从 1 开始，k 为平滑参数（默认 60）。

// FusionSource 代表单个搜索源返回的原始结果。
// 字段 Hits 的顺序即该源内部的排名顺序。
type FusionSource struct {
	Name   string     // 源标识（如 "vector"、"keyword"、"graph"）
	Hits   []core.Hit // 该源的搜索结果，顺序 = 排名
	Weight float32    // 源权重，0 表示 1.0
}

func NewSource(name string, weight float32, hits []core.Hit) *FusionSource {
	return &FusionSource{
		Name:   name,
		Weight: weight,
		Hits:   hits,
	}
}

// Merge 对多个源执行 RRF 融合，返回按融合分数降序排列的结果。
// k 为 RRF 平滑参数，默认值 60（推荐范围 5~100，越大对低排名结果越宽容）。
func RRF(sources ...FusionSource) ([]core.Hit, error) {
	return RRFWithK(60, sources...)
}

// RRFWithK 使用自定义 k 参数的 RRF 融合。
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
