package core

// Structurizer 结构化接口，负责数据清洗与文档结构解析
type Structurizer interface {
	// Parse 接收原始文档，先清洗再结构化，输出结构化文档
	// 内部流程：RawDocument（脏）→ 数据清洗 → 结构化解析 → StructuredDocument（干净）
	Parse(raw Document) (*StructuredDocument, error)
}
