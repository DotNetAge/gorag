package query

import (
	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
)

// GraphQuery 图查询
type GraphQuery struct {
	BaseQuery
	Depth  int
	Limit  int
	cypher string
}

// TODO: 一是对实体的精确查询（通过关键词）二是对实体内存的分块ID的相关查询以获得实体，然后通过实现的边再查询下一跳的其它相关实体

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

func (q *GraphQuery) SetDepth(depth int) {
	q.Depth = depth
}

func (q *GraphQuery) Cypher() string {
	return q.cypher
}

func (q *GraphQuery) Text2Cypher(client chat.Client) error {
	// TODO: Implement text to Cypher query conversion
	return nil
}
