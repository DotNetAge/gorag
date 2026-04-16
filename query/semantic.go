package query

import "github.com/DotNetAge/gorag/core"

// SemanticQuery 语义查询
type SemanticQuery struct {
	BaseQuery
	Embedder core.Embedder // 向量编码器，用于语义相似度计算
}

// Vector returns the vector representations of the query.
func (q *SemanticQuery) Vector() *core.Vector {
	vector, err := q.Embedder.CalcText(q.normalized)
	if err != nil {
		return nil
	}
	return vector
}

// NewSemanticQuery creates a new semantic query from the given terms.
func NewSemanticQuery(terms string, embedder core.Embedder) core.Query {
	return &SemanticQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			embedder:   embedder,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
		Embedder: embedder,
	}
}
