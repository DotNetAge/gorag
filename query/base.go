package query

import (
	"maps"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/utils"
)

// BaseQuery is a default implementation of the core.Query interface.
type BaseQuery struct {
	raw        string     // raw is the raw query string
	normalized string     // normalized is the normalized query string
	filters    map[string]any
	embedder   core.Embedder
	queryType  string     // 查询类型：semantic/keyword/hybrid/graph
	embedding  []float32  // 查询向量缓存（由 indexer 在 Search 前注入）
}

// Raw returns the raw, unprocessed query string.
//
// Returns:
//   - string: The raw query string
func (q *BaseQuery) Raw() string {
	return q.raw
}

// Keywords returns the extracted keywords from the query.
//
// Returns:
//   - []string: The extracted keywords
func (q *BaseQuery) Keywords() []string {
	if q.raw == "" {
		return []string{}
	}
	return utils.ExtractKeywords(q.normalized)
}

// Filters returns the filters to apply to the search.
//
// Returns:
//   - map[string]any: The filters
func (q *BaseQuery) Filters() map[string]any {
	return q.filters
}

// AddFilter adds a filter to the query.
// The value is copied to prevent external modification.
//
// Parameters:
//   - key: The filter key
//   - value: The filter value
//
// Returns:
//   - core.Query: The query with the added filter
func (q *BaseQuery) AddFilter(key string, value any) core.Query {
	if q.filters == nil {
		q.filters = make(map[string]any)
	}
	// 深拷贝以避免引用类型被外部修改
	if m, ok := value.(map[string]any); ok {
		copyMap := make(map[string]any)
		maps.Copy(copyMap, m)
		q.filters[key] = copyMap
	} else {
		q.filters[key] = value
	}
	return q
}

// Type 返回查询类型（semantic/keyword/hybrid/graph）。
// 默认返回 "semantic"，由查询前优化阶段识别后通过 SetType 设置。
func (q *BaseQuery) Type() string {
	if q.queryType == "" {
		return "semantic"
	}
	return q.queryType
}

// SetType 设置查询类型，返回 Query 自身支持链式调用。
func (q *BaseQuery) SetType(t string) core.Query {
	q.queryType = t
	return q
}

// Embedding 返回查询向量（由 indexer 在 Search 前注入）。
func (q *BaseQuery) Embedding() []float32 {
	return q.embedding
}

// SetEmbedding 设置查询向量，返回 Query 自身支持链式调用。
// 由 indexer 在 Search 之前调用，将计算好的向量缓存到 Query 中。
func (q *BaseQuery) SetEmbedding(vec []float32) core.Query {
	q.embedding = vec
	return q
}
