package core

// Hit represents a search result with relevance scoring and content.
// GraphIndexer 的 Hits 同时包含 Chunk + Entities + Relations，客户端可直接使用。
type Hit struct {
	ID        string         `json:"id"`                  // 结果ID
	Title     string         `json:"title"`               // 结果标题（来自原 Chunk.Title）
	Score     float32        `json:"score"`               // 相似度分数
	Content   string         `json:"content"`             // 结果内容
	DocID     string         `json:"doc_id"`              // 文档ID
	Metadata  map[string]any `json:"metadata"`            // 扩展元数据（来自原 Chunk.Metadata）
	ChunkMeta ChunkMeta      `json:"chunk_meta"`          // 分块固定元数据（来自原 Chunk.ChunkMeta）
	Entities  []*Node        `json:"entities,omitempty"`  // 关联的实体节点（GraphIndexer 专用）
	Relations []*Edge        `json:"relations,omitempty"` // 关联的边（GraphIndexer 专用）
}
