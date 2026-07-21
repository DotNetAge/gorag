package result

import (
	"fmt"
	"math"
	"sort"

	"github.com/DotNetAge/gorag/v2/core"
)

// Rerank 使用查询向量与每条 ChunkHit 的 Content 向量计算余弦相似度，对结果重排。
//
// 重排规则：
//   - 入参为 *core.Hit（容器），仅对 hit.Chunks 做重排，hit.Nodes 和 hit.Edges 保持不变
//   - 查询向量从 q.Embedding() 读取（由 indexer 在 Search 前注入）
//   - 命中向量编码使用传入的 embedder
//   - 返回新的 *Hit（不修改入参）
func Rerank(q core.Query, embedder core.Embedder, hit *core.Hit) (*core.Hit, error) {
	if hit == nil {
		return nil, nil
	}
	if len(hit.Chunks) == 0 {
		return cloneHit(hit), nil
	}
	if q == nil {
		return nil, fmt.Errorf("重排器：query 不能为空")
	}
	if embedder == nil {
		return nil, fmt.Errorf("重排器：embedder 不能为空")
	}

	queryVec := q.Embedding()
	if len(queryVec) == 0 {
		return nil, fmt.Errorf("重排器：查询向量为空（indexer 应在 Search 前调用 SetEmbedding 注入）")
	}

	type scoredHit struct {
		hit   core.ChunkHit
		score float32
	}
	scored := make([]scoredHit, len(hit.Chunks))

	for i, ch := range hit.Chunks {
		if ch.Chunk == nil {
			scored[i] = scoredHit{hit: ch, score: 0}
			continue
		}
		contentVec, err := embedder.CalcText(ch.Content)
		if err != nil {
			return nil, fmt.Errorf("重排器：编码 Chunk %s 失败: %w", ch.ID, err)
		}
		scored[i] = scoredHit{
			hit:   ch,
			score: cosineSimilarity(queryVec, contentVec.Values),
		}
	}

	// 按余弦相似度降序排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// 构建重排后的 Chunks（覆盖 Score 为重排分数）
	rerankedChunks := make([]core.ChunkHit, len(scored))
	for i, s := range scored {
		s.hit.Score = s.score // 覆盖原始分数为重排分数
		rerankedChunks[i] = s.hit
	}

	// 构建重排后的 Hit（保留原 Query/Score/Nodes/Edges）
	return &core.Hit{
		Query:  hit.Query,
		Score:  hit.Score,
		Chunks: rerankedChunks,
		Nodes:  hit.Nodes,
		Edges:  hit.Edges,
	}, nil
}

// cosineSimilarity 计算两个向量的余弦相似度。
// 向量长度不一致或为空时返回 0。
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProd, normA, normB float32
	for i := range a {
		dotProd += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := float32(math.Sqrt(float64(normA)) * math.Sqrt(float64(normB)))
	if denom == 0 {
		return 0
	}
	return dotProd / denom
}
