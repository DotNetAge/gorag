package core

import "context"

// VectorStore 向量存储接口：负责向量数据的写入、相似度检索与生命周期管理。
//
// 设计要点：
//   - 多维度向量索引：每个 Chunk 在 VectorStore 中有 1~3 条向量记录
//     （主向量 chunkID 对应 Content；辅助向量 chunkID:title / chunkID:summary
//     分别对应 Title/Summary），3 条向量的 embedding 维度相同但数据维度不同
//   - 分块树落地：每条 Vector.Metadata 持有 parent_id 字段，分块树在 VectorStore
//     中可向上追溯（Chunk 不作为 Node 写入 GraphStore）
//   - List 通过 filters 参数控制是否过滤：传 nil 时返回全部，传非 nil 时按条件过滤
//
// 实现可基于 Milvus / Pinecone / Qdrant / Weaviate / govector 等。
type VectorStore interface {
	// Upsert 批量插入或更新向量。
	// 若 vector.ID 已存在则更新，否则插入。
	Upsert(ctx context.Context, vectors []*Vector) error

	// Search 执行相似度检索，返回 topK 条最相似向量及对应得分。
	// filters 用于元数据过滤（如按 doc_id / region_id 过滤），传 nil 表示不过滤。
	Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*Vector, []float32, error)

	// Delete 按 ID 删除单条向量。
	Delete(ctx context.Context, id string) error

	// GetByDocID 按 doc_id 检索该文档的所有向量（按 chunk index 排序），
	// 用于「知识追溯」——从分片重建原文档。
	GetByDocID(ctx context.Context, docID string) ([]*Vector, error)

	// List 分页获取向量，支持可选的元数据过滤条件。
	// filters 为 nil 时返回全部，非 nil 时按条件过滤。
	// 多个 FilterCondition 之间为 AND 语义。
	// 返回分页结果与过滤前总数。
	List(ctx context.Context, offset, limit int, filters []FilterCondition) ([]*Vector, int, error)

	// Count 返回 VectorStore 中的向量总数。
	Count(ctx context.Context) (int, error)

	// Clear 清空所有向量数据，清空后可立即接收新数据。
	Clear(ctx context.Context) error

	// Flush 强制将写入缓冲刷入持久化存储。
	// 底层存储（如 govector）为性能采用攒批缓冲写入：数据先进内存索引（查询
	// 即时可见），只有攒够阈值或显式 Flush/Close 才真正写入磁盘。调用方应在
	// 关键数据写入后立即调用，避免进程异常退出导致缓冲中的数据丢失。
	Flush(ctx context.Context) error

	// Close 优雅关闭连接，释放底层资源。
	Close(ctx context.Context) error
}

// FilterCondition 单个元数据过滤条件，配合 VectorStore.List 使用。
//
// ConditionType 取值：
//   - "exact"：精确匹配 metadata key 的值
//   - "prefix"：前缀匹配（仅适用于 string 类型 value）
type FilterCondition struct {
	Key   string `json:"key"`
	Type  string `json:"type"` // "exact" 或 "prefix"
	Value any    `json:"value"`
}
