package core

// Query defines the interface for queries in the RAG system.
// Queries are used to search the index for relevant content.
type Query interface {

	// Raw returns the raw, unprocessed query string.
	//
	// Returns:
	//   - string: The raw query string
	Raw() string

	// Keywords returns the extracted keywords from the query.
	// 关键词的提取可以用于BM25精确查找。
	// Returns:
	//   - []string: The extracted keywords
	Keywords() []string

	// Filters returns the filters to apply to the search.
	// 提取出查询中的过滤条件，可辅助语义化查询的精确度
	// Returns:
	//   - map[string]any: The filters
	Filters() map[string]any

	// AddFilter adds a filter to the query.
	AddFilter(key string, value any) Query
}
