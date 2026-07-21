// Package result 提供检索结果的融合、去重、压缩与重排能力。
//
// 本包基于 Hit 容器结构（持有 Chunks/Nodes/Edges 三类集合）实现：
//   - RRF：按 Chunks/Nodes/Edges 三类分别 Reciprocal Rank Fusion，返回融合后的 *Hit
//   - Dedup：对 hit.Chunks 做语义去重（MinHash + Jaccard 相似度）
//   - Compress：对 hit.Chunks 调用 LLM 压缩 Content
//   - Rerank：用查询向量与 hit.Chunks 的 Content 向量做余弦相似度重排
package result

import (
	"sort"

	"github.com/DotNetAge/gorag/v2/core"
)

// FusionSource 表示来自单一检索源（如语义/图/关键词索引）的检索结果。
//
// 该源的内部排名由 Hit.Chunks/Nodes/Edges 的切片顺序决定（index 0 = 排名第 1）。
// Hit 为 nil 表示该源无结果，RRF 内部会跳过。
type FusionSource struct {
	Name   string    // 源标识（如 "semantic"、"graph"）
	Hit    *core.Hit // 该源的检索结果容器，nil 表示该源无结果
	Weight float32   // 源权重，0 表示 1.0
}

// NewSource 创建一个新的 FusionSource。
// hit 为 nil 时表示该源无结果，RRF 内部会跳过。
func NewSource(name string, weight float32, hit *core.Hit) *FusionSource {
	return &FusionSource{
		Name:   name,
		Weight: weight,
		Hit:    hit,
	}
}

// RRF 使用默认平滑参数 k=60 执行 Reciprocal Rank Fusion。
//
// 融合规则：
//   - 按 Chunks/Nodes/Edges 三类分别 RRF 融合
//   - 同类内按 ID 去重，相同 ID 的分数累加
//   - 每类内部按融合后分数降序排序
//   - Hit.Score = topChunkScore + topNodeScore + topEdgeScore（缺失类别贡献 0）
//
// 参数：
//   - sources: 多个 FusionSource 输入（Hit 为 nil 的源会被跳过）
//
// 返回：
//   - *core.Hit: 融合后的检索结果容器
//   - error: 仅当 sources 为空或全部为 nil 时返回 (nil, nil)
func RRF(sources ...*FusionSource) (*core.Hit, error) {
	return RRFWithK(60, sources...)
}

// RRFWithK 使用自定义平滑参数 k 执行 RRF 融合。
// 推荐 k 范围：5-100，k 越大对低排名结果越宽容。
//
// RRF 公式：score(doc) = Σ weight_s / (k + rank_s)
func RRFWithK(k int, sources ...*FusionSource) (*core.Hit, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	// 过滤掉 Hit 为 nil 的源
	validSources := make([]*FusionSource, 0, len(sources))
	for _, src := range sources {
		if src != nil && src.Hit != nil {
			validSources = append(validSources, src)
		}
	}
	if len(validSources) == 0 {
		return nil, nil
	}

	// 按 Chunks/Nodes/Edges 三类分别融合
	fusedChunks := fuseChunkHits(k, validSources)
	fusedNodes := fuseNodeHits(k, validSources)
	fusedEdges := fuseEdgeHits(k, validSources)

	// 构建融合后的 Hit
	result := &core.Hit{
		Chunks: fusedChunks,
		Nodes:  fusedNodes,
		Edges:  fusedEdges,
	}

	// 保留 Query（从第一个非 nil 源继承）
	for _, src := range validSources {
		if src.Hit.Query != nil {
			result.Query = src.Hit.Query
			break
		}
	}

	// 计算 Hit.Score = topChunkScore + topNodeScore + topEdgeScore
	// 各类别缺失时贡献 0，保证双线（语义+图）场景分数高于单线场景
	var topChunk, topNode, topEdge float32
	if len(fusedChunks) > 0 {
		topChunk = fusedChunks[0].Score
	}
	if len(fusedNodes) > 0 {
		topNode = fusedNodes[0].Score
	}
	if len(fusedEdges) > 0 {
		topEdge = fusedEdges[0].Score
	}
	result.Score = topChunk + topNode + topEdge

	return result, nil
}

// fuseChunkHits 按 ChunkID 融合多源的 ChunkHit，返回按分数降序排序的结果。
//
// 融合逻辑：
//   - 遍历每个源的 hit.Chunks，rank 由切片下标决定（0 = 排名第 1）
//   - 相同 ChunkID 的 ChunkHit 累加 RRF 分数
//   - 保留首次出现的 ChunkHit 实例（含 *Chunk 指针和 Dim），仅更新 Score
func fuseChunkHits(k int, sources []*FusionSource) []core.ChunkHit {
	type entry struct {
		hit   core.ChunkHit
		score float32
	}
	scoreMap := make(map[string]*entry)

	for _, src := range sources {
		w := src.Weight
		if w == 0 {
			w = 1.0
		}
		for rank, ch := range src.Hit.Chunks {
			if ch.Chunk == nil {
				continue
			}
			e, ok := scoreMap[ch.ID]
			if !ok {
				e = &entry{hit: ch, score: 0}
				scoreMap[ch.ID] = e
			}
			e.score += w / float32(k+rank+1)
		}
	}

	fused := make([]core.ChunkHit, 0, len(scoreMap))
	for _, e := range scoreMap {
		e.hit.Score = e.score
		fused = append(fused, e.hit)
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})

	return fused
}

// fuseNodeHits 按 NodeID 融合多源的 NodeHit，返回按分数降序排序的结果。
//
// 融合逻辑与 fuseChunkHits 对称，详见该函数文档。
func fuseNodeHits(k int, sources []*FusionSource) []core.NodeHit {
	type entry struct {
		hit   core.NodeHit
		score float32
	}
	scoreMap := make(map[string]*entry)

	for _, src := range sources {
		w := src.Weight
		if w == 0 {
			w = 1.0
		}
		for rank, nh := range src.Hit.Nodes {
			if nh.Node == nil {
				continue
			}
			e, ok := scoreMap[nh.ID]
			if !ok {
				e = &entry{hit: nh, score: 0}
				scoreMap[nh.ID] = e
			}
			e.score += w / float32(k+rank+1)
		}
	}

	fused := make([]core.NodeHit, 0, len(scoreMap))
	for _, e := range scoreMap {
		e.hit.Score = e.score
		fused = append(fused, e.hit)
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})

	return fused
}

// fuseEdgeHits 按 EdgeID 融合多源的 EdgeHit，返回按分数降序排序的结果。
//
// 融合逻辑与 fuseChunkHits 对称，详见该函数文档。
func fuseEdgeHits(k int, sources []*FusionSource) []core.EdgeHit {
	type entry struct {
		hit   core.EdgeHit
		score float32
	}
	scoreMap := make(map[string]*entry)

	for _, src := range sources {
		w := src.Weight
		if w == 0 {
			w = 1.0
		}
		for rank, eh := range src.Hit.Edges {
			if eh.Edge == nil {
				continue
			}
			e, ok := scoreMap[eh.ID]
			if !ok {
				e = &entry{hit: eh, score: 0}
				scoreMap[eh.ID] = e
			}
			e.score += w / float32(k+rank+1)
		}
	}

	fused := make([]core.EdgeHit, 0, len(scoreMap))
	for _, e := range scoreMap {
		e.hit.Score = e.score
		fused = append(fused, e.hit)
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})

	return fused
}
