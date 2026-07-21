package core

import "github.com/google/uuid"

// Vector 元数据键名常量。
// 用于 buildVectorMetadata / vectorToChunk 等序列化/反序列化互逆操作，
// 消除键名散落——改一处编译器自动报另一处。
const (
	VecMetaContent  = "content"   // Chunk.Content
	VecMetaTitle    = "title"     // Chunk.Title
	VecMetaSummary  = "summary"   // Chunk.Summary
	VecMetaDocID    = "doc_id"    // Chunk.DocID
	VecMetaParentID = "parent_id" // Chunk.ParentID
	VecMetaSource   = "source"    // Chunk.Source
	VecMetaRegionID = "region_id" // Chunk.RegionID
	VecMetaLanguage = "language"  // Chunk.Language
	VecMetaTags     = "tags"      // Chunk.Tags
	VecMetaStartLine = "start_line" // Chunk.StartLine
	VecMetaEndLine   = "end_line"   // Chunk.EndLine
)

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
	Metadata map[string]any `json:"metadata,omitempty"` // 持有 Chunk 的快照（使用 VecMeta* 常量键名）
}

// NewVector 创建一个新的 Vector 实例。
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
