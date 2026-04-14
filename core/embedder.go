package core

type Embedder interface {
	// Calc 计算 Chunk 的向量表示
	Calc(chunk *Chunk) (*Vector, error)

	// CalcText 直接计算文本的向量表示（用于查询）
	CalcText(text string) (*Vector, error)

	// CalcImage 直接计算图片的向量表示（用于查询）
	CalcImage(data []byte) (*Vector, error)

	// Bulk 批量计算 Chunk 的向量表示
	Bulk(chunks []*Chunk) ([]*Vector, error)
}
