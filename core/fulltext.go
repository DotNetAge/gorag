package core

// FullTextStore 全文存储接口（基于 bleve 等搜索引擎）
type FullTextStore interface {
	// Index 将 chunk 写入全文索引
	Index(chunk *Chunk) error

	// Search 执行全文搜索，返回匹配结果列表
	Search(query string, topK int) ([]FullTextSearchResult, error)

	// Delete 从索引中移除指定 chunk
	Delete(chunkID string) error
}

// SearchResult 全文搜索结果
type FullTextSearchResult struct {
	ID      string  // chunk ID
	Score   float64 // 相关性得分（由搜索引擎计算）
	DocID   string  // 所属文档 ID
	Content string  // 匹配的文本内容片段
}
