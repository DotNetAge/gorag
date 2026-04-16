package query

import (
	"maps"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/utils"
)

// BaseQuery is a default implementation of the core.Query interface.
type BaseQuery struct {
	raw        string // raw is the raw query string
	normalized string // normalized is the normalized query string
	filters    map[string]any
	embedder   core.Embedder
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
