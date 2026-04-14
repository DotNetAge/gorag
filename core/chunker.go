package core

// ChunkStrategy 分块策略类型
type ChunkStrategy string

// Chunker 分块接口，接收解析层输出，生成最终可索引 Chunk
type Chunker interface {
	// Chunk 接收原始文档、结构化文档、实体列表，生成 Chunk 集合
	// 核心逻辑：结合结构边界（StructuredDocument）+ 实体完整性（Entity）做分块
	Chunk(doc *StructuredDocument, entities []*Entity) ([]*Chunk, error)

	// GetStrategy 返回分块策略类型
	GetStrategy() ChunkStrategy
}
