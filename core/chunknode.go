package core

// ChunkNode 知识图谱树节点。
//
// Tree 方法的返回值，用于 Sidebar 导航展示。
// 分片树（Region → Document）和实体图（Entity → Entity）通过
// ChunkIDs 关联，而非直接嵌套。
type ChunkNode struct {
	ID       string         `json:"id"`                // graphDB 节点 ID
	Name     string         `json:"name"`              // 显示名称（无后缀文件名或目录名）
	Type     string         `json:"type"`              // "region" | "document"
	Source   string         `json:"source,omitempty"`  // 源路径（Document=file, Region=dir）
	Summary  string         `json:"summary,omitempty"` // 摘要（Document 级别）
	ChunkIDs []string       `json:"chunk_ids,omitempty"`// 关联的 Chunk ID，连接分片树与实体图
	Meta     map[string]any `json:"meta,omitempty"`    // 扩展元数据
	Children []*ChunkNode   `json:"children,omitempty"`// 子节点
}

// AddChild 添加子节点。
func (n *ChunkNode) AddChild(child *ChunkNode) {
	n.Children = append(n.Children, child)
}
