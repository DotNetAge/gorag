package core

// Query 查询接口：承载查询前优化、查询类型识别等重要功能。
//
// 设计原则：
//   - Query 必须保持接口形式，不能改为结构体——查询优化（同义词扩展、关键词加权、
//     查询分类等）是行为而非数据，用结构体会丢失这些能力
//   - Type() 返回查询类型，用于查询路由与优化
//   - Embedding()/SetEmbedding() 由 Query 自身承载查询向量
//   - query.New(text) 返回 Query 接口类型，由 query 包提供默认实现
//
// 查询类型（Type() 返回值）：
//   - "semantic"：语义查询，走 VectorStore.Search
//   - "keyword"：关键词查询，走 Metadata 过滤
//   - "hybrid"：混合查询，同时走语义和关键词
//   - "graph"：图查询，走 GraphStore.Query（Cypher）
type Query interface {
	// Raw 返回原始查询字符串
	Raw() string

	// Keywords 返回提取的关键词（用于过滤、BM25 等）
	Keywords() []string

	// Filters 返回元数据过滤条件
	Filters() map[string]any

	// AddFilter 添加过滤条件，返回 Query 自身支持链式调用
	AddFilter(key string, value any) Query

	// Type 返回查询类型（如 "semantic" / "keyword" / "hybrid" / "graph"）
	// 用于查询路由与优化，由查询前优化阶段识别
	Type() string

	// Embedding 返回查询向量（由 indexer 内部计算并缓存，避免重复计算）
	// Embedding 由 Query 自身承载
	Embedding() []float32

	// SetEmbedding 设置查询向量，返回 Query 自身支持链式调用
	// 由 indexer 在 Search 之前调用，将计算好的向量缓存到 Query 中
	SetEmbedding(vec []float32) Query
}
