package core

// ChunkStrategy 分块策略类型
type ChunkStrategy string

// Chunker 分块接口，接收解析层输出，生成最终可索引 Chunk
type Chunker interface {
	// Chunk 接收结构化文档，结合结构边界生成 Chunk 集合
	// GraphRAG 流程：Document → Chunk → LLM Extractor → Node/Edge → GraphDB
	Chunk(doc *StructuredDocument) ([]*Chunk, error)

	// GetStrategy 返回分块策略类型
	GetStrategy() ChunkStrategy
}
