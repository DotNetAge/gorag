package core

// ChunkMeta 分块元数据结构体。
//
// Deprecated: Index/StartPos/EndPos 已提升为 Chunk 顶层字段，
// HeadingLevel/HeadingPath 已删除（由 Chunker 填充到 Chunk.Metadata["heading_path"]）。
// 此结构体仅保留为别名，供 hit.go/fulltext.go 等待迁移的代码继续编译。
type ChunkMeta struct {
	Index        int      `json:"index"`         // 分块在文档中的序号（0,1,2...）
	StartPos     int      `json:"start_pos"`     // 分块在原始清洗后文本中的起始位置
	EndPos       int      `json:"end_pos"`       // 分块在原始清洗后文本中的结束位置
	HeadingLevel int      `json:"heading_level"` // 分块对应的标题层级（已删除，移至 Metadata）
	HeadingPath  []string `json:"heading_path"`  // 分块对应的标题路径（已删除，移至 Metadata）
}

// Chunk 分片：可索引的最小语义单元，承载语义线。
//
// 设计要点：
//  1. 双线结构——通过 ParentID 自连结成可追溯的分块树；只在 VectorStore 中存在，
//     不作为 Node 写入 GraphStore；与实体 Node 的关联通过 Node.SourceChunkIDs 反向引用实现。
//  2. 多维度向量索引——Title/Summary/Content 三大属性分别向量化：
//     - Content 向量 ID = chunkID（主向量）
//     - Title 向量 ID = chunkID:title
//     - Summary 向量 ID = chunkID:summary
//     任一维度命中都能定位同一 Chunk，覆盖用户不同查询方式以提高召回率。
//     这里的「维度」是数据维度（data dimension），不是向量空间维度（vector space dimension）。
type Chunk struct {
	ID       string         `json:"id"`                  // Chunk 唯一标识（docID + 序号 + 内容哈希）
	ParentID string         `json:"parent_id,omitempty"` // 父 Chunk ID（空表示文档级 Chunk）；建立分块树，支持向上追溯
	DocID    string         `json:"doc_id"`              // 所属文档 ID（RawDoc.ID()）
	Title    string         `json:"title"`               // 分片标题（由 Chunker 从 Markdown heading/代码符号名等提取；向量化为 chunkID:title）
	Summary  string         `json:"summary"`             // 分片摘要（由 Chunker/Extractor 生成；向量化为 chunkID:summary）
	Content  string         `json:"content"`             // 分片内容（清洗后纯文本；向量化为主向量 chunkID）
	Index    int            `json:"index"`               // 分片在文档中的序号
	StartPos int            `json:"start_pos"`           // 在原文中的起始字节位置
	EndPos   int            `json:"end_pos"`             // 在原文中的结束字节位置
	Metadata map[string]any `json:"metadata,omitempty"`  // 扩展元数据（tags/source_file/region_id/heading_path 等；不再含 title/summary，已提升为字段）
}
