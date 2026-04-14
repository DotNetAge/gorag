package indexer

import (
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
	return map[string]any{}
}

// AddFilter adds a filter to the query.
//
// Parameters:
//   - key: The filter key
//   - value: The filter value
//
// Returns:
//   - core.Query: The query with the added filter
func (q *BaseQuery) AddFilter(key string, value any) core.Query {
	q.filters[key] = value
	return q
}

// semanticQuery 语义查询
type semanticQuery struct {
	BaseQuery
	embedder core.Embedder
}

// Vector returns the vector representations of the query.
func (q *semanticQuery) Vector() *core.Vector {
	vector, err := q.embedder.CalcText(q.normalized)
	if err != nil {
		return nil
	}
	return vector
}

// SemanticQuery creates a new semantic query from the given terms.
func SemanticQuery(terms string, embedder core.Embedder) core.Query {
	return &semanticQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			embedder:   embedder,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
		embedder: embedder,
	}
}

// fulltextQuery 全文查询
type fulltextQuery struct {
	BaseQuery
}

// FulltextQuery creates a new fulltext query from the given terms.
func FulltextQuery(terms string) core.Query {
	return &fulltextQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
	}
}

// graphQuery 图查询
type graphQuery struct {
	BaseQuery
	depth int
	limit int
}

// Depth sets the traversal depth for graph query.
func (q *graphQuery) Depth(depth int) *graphQuery {
	q.depth = depth
	return q
}

// Limit sets the max results limit for graph query.
func (q *graphQuery) Limit(limit int) *graphQuery {
	q.limit = limit
	return q
}

// GraphQuery creates a new graph query from the given terms.
func GraphQuery(terms string) core.Query {
	return &graphQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
		depth: 1,
		limit: 10,
	}
}
