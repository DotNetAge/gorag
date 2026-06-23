package query

import (
	"github.com/DotNetAge/gorag/core"
)

// GraphQuery 图查询
//
// 支持两种查询模式：
//   - Text：自然语言描述，由 GraphIndexer 内部 LLM 自动生成 Cypher 执行
//   - Raw：直接提供原生 Cypher 语句，GraphIndexer 不做转换直接执行
//
// 如果 Text 和 Raw 都为空，GraphIndexer 走默认的语义向量检索 + 图融合路径。
type GraphQuery struct {
	BaseQuery
	Depth     int
	Limit     int
	EdgeTypes []string // 关系类型过滤（可选），空表示不过滤
	text      string   // 自然语言查询（LLM → Cypher）
	raw       string   // 原生 Cypher 语句（直接执行）
}

// NewGraphQuery creates a new graph query from the given natural language text.
// terms 作为自然语言查询（Text）传递，同时作为原始搜索词保留在 BaseQuery 中。
func NewGraphQuery(terms string) core.Query {
	return &GraphQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
		text:  terms,
		Depth: 1,
		Limit: 10,
	}
}

// SetDepth 设置图遍历深度
func (q *GraphQuery) SetDepth(depth int) {
	q.Depth = depth
}

// SetLimit 设置返回结果数量限制
func (q *GraphQuery) SetLimit(limit int) {
	q.Limit = limit
}

// SetEdgeTypes 设置关系类型过滤（仅遍历指定类型的边）
func (q *GraphQuery) SetEdgeTypes(types []string) {
	q.EdgeTypes = types
}

// TextQuery 返回自然语言查询文本。
// 非空时，GraphIndexer 会使用内部 LLM 将其转换为 Cypher 再执行。
func (q *GraphQuery) TextQuery() string {
	return q.text
}

// SetTextQuery 设置自然语言查询文本。
// GraphIndexer 会在 Search 时使用内部 LLM 将其转换为 Cypher。
func (q *GraphQuery) SetTextQuery(text string) {
	q.text = text
}

// RawCypher 返回原生 Cypher 查询语句。
// 非空时，GraphIndexer 直接将其交给 graphDB 执行。
func (q *GraphQuery) RawCypher() string {
	return q.raw
}

// SetRawCypher 设置原生 Cypher 查询语句。
// GraphIndexer 会在 Search 时直接执行此语句。
func (q *GraphQuery) SetRawCypher(cypher string) {
	q.raw = cypher
}
