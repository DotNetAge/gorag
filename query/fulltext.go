package query

import "github.com/DotNetAge/gorag/core"

// FulltextQuery 全文查询
type FulltextQuery struct {
	BaseQuery
}

// FulltextQuery creates a new fulltext query from the given terms.
func NewFulltextQuery(terms string) core.Query {
	return &FulltextQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
	}
}
