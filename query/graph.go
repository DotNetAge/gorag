package query

import "github.com/DotNetAge/gorag/core"

// GraphQuery 图查询
type GraphQuery struct {
	BaseQuery
	Depth int
	Limit int
}

// NewGraphQuery creates a new graph query from the given terms.
func NewGraphQuery(terms string) core.Query {
	return &GraphQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
		Depth: 1,
		Limit: 10,
	}
}
