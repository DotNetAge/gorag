package core

import "github.com/google/uuid"

// Vector 向量：Chunk 的向量化表示。
//
// 多维度向量索引设计：
//   - 每个 Chunk 对应 1~3 条向量记录，分别对应 3 个数据维度
//   - ID = chunkID         → Content 维度向量（主向量，必须生成）
//   - ID = chunkID:title   → Title 维度向量（Title 非空时生成）
//   - ID = chunkID:summary → Summary 维度向量（Summary 非空时生成）
//   - 「维度」是数据维度（data dimension），不是向量空间维度（vector space dimension）
//     3 条向量的 embedding 维度（向量空间维度）相同，但对应的数据维度不同
//   - 语义搜索时同时匹配 3 个维度，任一命中即可通过 ChunkID 定位同一 Chunk
//   - 3 条向量的 ChunkID 字段都指向同一个 Chunk，便于反查
type Vector struct {
	ID       string         `json:"id"`                 // 向量唯一标识（chunkID / chunkID:title / chunkID:summary 三种形式）
	ChunkID  string         `json:"chunk_id"`           // 关联的 Chunk ID（3 条向量指向同一 ChunkID）
	Values   []float32      `json:"values"`             // 向量值（embedding model 输出，3 条向量的向量空间维度相同）
	Metadata map[string]any `json:"metadata,omitempty"` // 持有 Chunk 的快照（doc_id/source_file/region_id/parent_id 等）
}

// NewVector 创建一个新的 Vector 实例。
//
// Deprecated: 推荐使用 embedder.CalcChunk 为 Chunk 生成多维度向量，
// 此构造函数仅保留供历史代码继续编译。
//
// 参数：
//   - values: embedding 向量值（float32 切片）
//   - metadata: 附加元数据，用于过滤和追踪
func NewVector(values []float32, metadata map[string]any) *Vector {
	return &Vector{
		ID:       uuid.NewString(),
		Values:   values,
		ChunkID:  uuid.NewString(),
		Metadata: metadata,
	}
}
