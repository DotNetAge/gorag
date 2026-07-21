package core

// Embedder 向量计算接口：只负责纯向量计算，不与 Chunk/Chunker 耦合。
//
// 设计要点：
//   - 只接收文本或图片字节，返回向量
//   - 不感知 Chunk、Chunker 或维度策略
//   - 多维度向量（Content/Title/Summary）的组装由 SemanticIndexer 负责
//   - 调用方负责从 Chunk 提取文本后调用本接口
type Embedder interface {
	// CalcText 计算文本的向量表示（Content/Title/Summary/查询均通过此方法）
	CalcText(text string) (*Vector, error)

	// CalcImage 计算图片的向量表示（用于图片查询或图片分块向量化）
	CalcImage(data []byte) (*Vector, error)

	// Dim 返回向量维度
	Dim() int

	// Multimoding 是否支持多模态（文本+图片）
	Multimoding() bool
}
