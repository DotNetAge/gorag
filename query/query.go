package query

import (
	"fmt"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/utils"
)

// 关键约束：Query 必须保持接口形式，禁止改为结构体。
// 查询优化（同义词扩展、关键词加权、查询分类）是行为而非数据，
// 用结构体会丢失这些能力。

// New 创建一个 core.Query 接口的默认实现。
//
// 返回 BaseQuery 实例，预设：
//   - raw = text（原始查询字符串）
//   - normalized = utils.Clean(text)（清洗后的查询字符串，用于关键词提取）
//   - queryType = "semantic"（默认语义查询，由查询前优化阶段识别后通过 SetType 设置）
//   - filters = nil（无过滤条件，由调用方通过 AddFilter 添加）
//   - embedding = nil（由 indexer 在 Search 前通过 SetEmbedding 注入）
func New(text string) core.Query {
	if text == "" {
		text = ""
	}
	normalized := utils.Clean(text)
	return &BaseQuery{
		raw:        text,
		normalized: normalized,
		queryType:  "semantic",
	}
}

// NewWithType 创建一个指定查询类型的 core.Query。
//
// 查询类型取值：
//   - "semantic"：语义查询，走 VectorStore.Search
//   - "keyword"：关键词查询，走 Metadata 过滤
//   - "hybrid"：混合查询，同时走语义和关键词
//   - "graph"：图查询，走 GraphStore.Query（Cypher）
//
// 调用方在查询前优化阶段识别查询类型后通过此函数创建 Query，
// 或通过 New(text).(interface{ SetType(string) core.Query }).SetType(t) 链式调用设置。
func NewWithType(text, queryType string) core.Query {
	q := New(text)
	if bq, ok := q.(*BaseQuery); ok {
		bq.SetType(queryType)
	}
	return q
}

// Validate 校验 Query 的合法性。
// 当前仅校验 raw 非空；后续可扩展（如校验 queryType 取值）。
func Validate(q core.Query) error {
	if q == nil {
		return fmt.Errorf("query: Query is nil")
	}
	if q.Raw() == "" {
		return fmt.Errorf("query: Raw is empty")
	}
	return nil
}
