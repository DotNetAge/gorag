// Package core 提供 goRAG 框架的基础类型与接口。
package core

// Node 表示 RAG 知识图谱中的图节点实体。
// 在 GraphRAG 中，节点从文本分片派生，作为增强检索能力的索引层，
// 表示从文档中抽取的实体。
type Node struct {
	ID     string   `json:"id"`     // 节点唯一标识
	Labels []string `json:"labels"` // 类型/类别（如 ["Person", "Organization"]），与 gograph.Node.Labels 对齐
	Name   string   `json:"name"`   // 实体名称（清洗后的文本，如 "张三"、"阿里巴巴"）

	// Properties 存储扩展特征，标准化 key：
	// - "confidence": float32 - 抽取置信度（0~1，来自 LLM/规则）
	// - "frequency": int - 跨文档出现次数
	// - "vectors": []float32 - 语义 embedding 向量
	// - "aliases": []string - 别名
	// - 其他自定义字段
	Properties map[string]any `json:"properties,omitempty"`

	// 源绑定——遵循 Microsoft GraphRAG 设计：图作为索引层
	SourceChunkIDs []string `json:"source_chunk_ids,omitempty"` // 源分片 ID 列表
	SourceDocIDs   []string `json:"source_doc_ids,omitempty"`   // 源文档 ID 列表
}

// Edge 表示知识图谱中两个节点之间的边（关系）。
// 边捕获实体之间的关系，并绑定到源文本分片以支持可追溯与证据检索。
type Edge struct {
	ID        string `json:"id"`                  // 边唯一标识
	Type      string `json:"type"`                // 边类型（如 WORKS_FOR、LOCATED_IN、BELONGS_TO）
	Source    string `json:"source"`              // 源节点 ID（主语实体）
	Target    string `json:"target"`              // 目标节点 ID（宾语实体）
	Predicate string `json:"predicate,omitempty"` // 关系类型别名（如 "就职于"、"属于"）

	// Properties 存储扩展特征，标准化 key：
	// - "confidence": float32 - 抽取置信度（0~1，来自 LLM/规则）
	// - "score": float32 - 关系强度分数
	// - "evidence": string - 关系的文本证据
	// - 其他自定义字段
	Properties     map[string]any `json:"properties,omitempty"`
	SourceChunkIDs []string       `json:"source_chunk_ids,omitempty"` // 源分片 ID 列表
	SourceDocIDs   []string       `json:"source_doc_ids,omitempty"`   // 源文档 ID 列表
}

// Label 常量：Graph 内节点的 Labels 标识
// Chunk 不作为 Node 写入 GraphStore，只存在于 VectorStore（语义线）；
// 实体 Node 存在于 GraphStore（关系线），二者通过 Node.SourceChunkIDs 双向关联。
const (
	LabelRegion = "Region" // Region 节点：对应目录 README.md 的知识库分区
)

// Edge Type 常量：Graph 内边的 Type 标识
// Chunk 不在 Graph 中，分块树只在 VectorStore.Metadata 中通过 parent_id 体现。
const (
	EdgeBelongsTo = "BELONGS_TO" // 实体 → Region：标识实体所属的知识库分区
)
