package result

import (
	"fmt"
	"math"
	"sort"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/query"
)

// Reranker 实现基于向量相似度的结果重排（Re-Ranking）。
//
// RAG 中的 Re-Ranking 指的是对初步检索结果用更精确的方式重新打分排序，
// 而不是简单地按初始检索分数排列。
//
// 本实现使用 SemanticQuery 中已预计算的查询向量与每条检索结果的
// 内容向量进行余弦相似度比较，以此作为重排依据。
// 查询向量由 SemanticQuery.Vector() 直接获取，不重复编码。
//
// Rerank 用查询向量与每条 Hit 的内容向量计算余弦相似度作为新分数，按此分数降序排列。
func Rerank(query *query.SemanticQuery, hits []core.Hit) ([]core.Hit, error) {
	if len(hits) == 0 {
		return hits, nil
	}
	if query == nil {
		return nil, fmt.Errorf("reranker: semantic query is required")
	}

	queryVec := query.Vector()
	if queryVec == nil {
		return nil, fmt.Errorf("reranker: query vector is nil")
	}

	type scoredHit struct {
		hit   core.Hit
		score float32
	}
	scored := make([]scoredHit, len(hits))

	for i, h := range hits {
		contentVec, err := query.Embedder.CalcText(h.Content)
		if err != nil {
			return nil, fmt.Errorf("reranker: encode hit %s: %w", h.ID, err)
		}
		scored[i] = scoredHit{
			hit:   h,
			score: cosineSimilarity(queryVec.Values, contentVec.Values),
		}
	}

	// 按余弦相似度降序排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]core.Hit, len(scored))
	for i, s := range scored {
		s.hit.Score = s.score // 覆盖原始分数为重排分数
		result[i] = s.hit
	}

	return result, nil
}

// cosineSimilarity 计算两个向量的余弦相似度。
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
