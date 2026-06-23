package result

import (
	"fmt"
	"math"
	"sort"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/query"
)

// Rerank re-ranks search results using cosine similarity between query vector and content vectors.
//
// In RAG systems, Re-Ranking refers to re-scoring initial retrieval results using
// a more precise method rather than simply ordering by initial retrieval scores.
//
// This implementation uses the pre-computed query vector from SemanticQuery.Vector()
// and computes cosine similarity with each hit's content vector (encoded on-the-fly).
// Results are returned in descending order of cosine similarity score.
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

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if vectors have different lengths or are empty.
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
