// Package core 提供 goRAG 框架的基础类型与接口。
package core

// Node/Edge 属性键名常量。
// 用于 Node.Properties 和 Edge.Properties 的键名，消除字符串散落。
const (
	PropDir        = "dir"        // 目录路径
	PropFileName   = "file"       // 文件名
	PropSignature  = "signature"  // 代码符号签名
	PropVisibility = "visibility" // 代码符号可见性
	PropReceiver   = "receiver"   // 代码符号接收者
	PropPackage    = "package"    // 代码包名
	PropFrequency  = "frequency"  // 跨文档出现次数
	PropConfidence = "confidence" // 抽取置信度（0~1）
	PropScore      = "score"      // 关系强度分数
	PropEvidence   = "evidence"   // 关系的文本证据
	PropAliases    = "aliases"    // 别名列表
	PropVectors    = "vectors"    // 语义 embedding 向量
)

// Node 表示 RAG 知识图谱中的图节点实体。
// 在 GraphRAG 中，节点从文本分片派生，作为增强检索能力的索引层，
// 表示从文档中抽取的实体。
type Node struct {
	ID     string   `json:"id"`     // 节点唯一标识
	Labels []string `json:"labels"` // 类型/类别（如 ["Person", "Organization"]），与 gograph.Node.Labels 对齐
	Name   string   `json:"name"`   // 实体名称（清洗后的文本，如 "张三"、"阿里巴巴"）

	// Properties 存储扩展特征，使用 Prop* 常量键名
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

	// Properties 存储扩展特征，使用 Prop* 常量键名
	Properties     map[string]any `json:"properties,omitempty"`
	SourceChunkIDs []string       `json:"source_chunk_ids,omitempty"` // 源分片 ID 列表
	SourceDocIDs   []string       `json:"source_doc_ids,omitempty"`   // 源文档 ID 列表
}

// Label 常量：Graph 内节点的 Labels 标识
// Chunk 不作为 Node 写入 GraphStore，只存在于 VectorStore（语义线）；
// 实体 Node 存在于 GraphStore（关系线），二者通过 Node.SourceChunkIDs 双向关联。
//
// 根节点类别约定：Document/Code/Image/DataFile 四类为「文档根节点」标签，
// 由 Chunker 在分块时产出，不参与实体抽取的类别体系（Person/Organization 等）。
// 取值与 Chunk 的 content_type（core.ContentType*）同源。
const (
	LabelDocument = ContentTypeDocument // 文档根节点
	LabelCode     = ContentTypeCode     // 代码文档根节点
	LabelImage    = ContentTypeImage    // 图片根节点
	LabelDataFile = ContentTypeDataFile // 数据文件根节点
	LabelRegion   = "Region"            // Region 节点：对应目录 README.md 的知识库分区
)

// IsRootLabel 判断标签是否为文档根节点类别之一（Document/Code/Image/DataFile）。
func IsRootLabel(label string) bool {
	switch label {
	case LabelDocument, LabelCode, LabelImage, LabelDataFile:
		return true
	}
	return false
}

// Edge Type 常量：Graph 内边的 Type 标识
// Chunk 不在 Graph 中，分块树只在 VectorStore.Metadata 中通过 parent_id 体现。
const (
	EdgeBelongsTo = "BELONGS_TO" // 实体 → Region：标识实体所属的知识库分区
)
