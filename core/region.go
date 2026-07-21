package core

// Region 知识库分区：对应文件目录中的 README.md，是知识库分区抽象。
//
// 设计要点：
//   - Region 是 Graph 内的分区节点（Labels=["Region"]），作为 GraphStore 中的节点存在
//   - Region 对应文件目录中的 README.md，是知识库分区抽象
//   - HyperIndexer.Tree() 基于 Region 层级组装知识树，输出 TreeNode
//   - Region 的 Summary 由 LLM 对目录下所有 Chunk 的摘要聚合而成，写入目录下的 README.md
//   - 实体 Node 通过 EdgeBelongsTo 边关联到 Region（实体 → Region）
//
// 生命周期：
//  1. Chunker 在分块时计算 region_id = sha256(dir) 并存入 Chunk.Metadata["region_id"]
//  2. RegionIndexer.IndexRegion 在目录下所有文件索引完成后被显式调用：
//     - 若目录下 README.md 已存在 → 直接复用，跳过 LLM 聚合
//     - 若不存在 → 从 VectorStore 查询该区域所有 Chunk → LLM 聚合摘要 → 写入 README.md
//  3. 编排层将 README.md 通过 HyperIndexer.AddFile 索引，
//     其内容被分块、结构化，写入 VectorStore + GraphStore
//
// 注意：Chunk 不作为 Node 写入 GraphStore，所以 Region 与 Chunk 之间没有 CONTAINS 边。
// Region 与 Chunk 的关联通过 Chunk.Metadata["region_id"] 在 VectorStore 中体现。
// HyperIndexer.Tree() 方法先从 GraphStore 取 Region→Document 树，
// 再通过 VectorStore 为每个 Document 补齐 Chunk 子节点，组装完整 TreeNode。
type Region struct {
	ID      string         `json:"id"`                // sha256(dir)，作为 Graph 内 Region 节点的 ID
	Title   string         `json:"title"`             // dir 的 basename（文件夹名，无后缀）
	Summary string         `json:"summary"`           // LLM 聚合摘要，写入 README.md（复用已有文件时为空）
	Tags    []string       `json:"tags"`              // 聚合标签（合并所有子 Chunk 的 tags）
	Dir     string         `json:"dir"`               // 目录绝对路径
	Meta    map[string]any `json:"meta,omitempty"`    // 扩展元数据
}

// TreeNode 知识树节点：HyperIndexer.Tree() 的返回类型。
//
// 基于 Region 层级组装，叶子节点为 Chunk。
// 分块树（Region → Document → Chunk）的 Region→Document 在 GraphStore 中通过 CONTAINS 边关联；
// Document→Chunk 通过 VectorStore.Metadata["doc_id"] 动态组装（Chunk 不在 Graph 中）。
type TreeNode struct {
	ID       string      `json:"id"`                  // Region ID / Document ID / Chunk ID
	Type     string      `json:"type"`                // "region" / "document" / "chunk"
	Name     string      `json:"name"`                // Region 目录名 / Document 标题 / Chunk 标题
	Summary  string      `json:"summary,omitempty"`   // Region 摘要（README.md 内容）
	Path     string      `json:"path,omitempty"`      // Region 目录或文件绝对路径
	Children []*TreeNode `json:"children,omitempty"`  // 子节点（Region/Document 有）
}

// AddChild 添加子节点。
func (n *TreeNode) AddChild(child *TreeNode) {
	n.Children = append(n.Children, child)
}
