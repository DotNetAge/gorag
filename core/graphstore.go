package core

import "context"

// GraphStore 图存储接口：只负责实体 Node、关系 Edge 的写入与图查询。
//
// 设计要点：
//   - GraphStore 只保存 Node 与 Edge，不保存 Chunk
//   - Chunk 不作为 Node 写入 GraphStore；Chunk 的内容/元数据由 VectorStore 管理
//   - 实体 Node 通过 SourceChunkIDs 反向引用 Chunk，形成双向关联
//   - GetByLabels：按 Label 查询节点（如查询所有 Region 节点），
//     支撑 HyperIndexer.Tree() 实现
//   - Region 节点：作为 Graph 内的分区抽象存在（Label="Region"），
//     实体通过 BELONGS_TO 边关联到 Region
//   - Document 节点：由 Chunker 在分块时生成，Label="Document"，
//     通过 CONTAINS 边与 Region 关联
//
// 实现可基于 Neo4j / Nebula / gograph 等。
type GraphStore interface {
	// UpsertNodes 批量插入或更新实体 Node。
	UpsertNodes(ctx context.Context, nodes []*Node) error

	// UpsertEdges 批量插入或更新关系 Edge。
	UpsertEdges(ctx context.Context, edges []*Edge) error

	// GetNode 按 ID 检索单个实体 Node。
	GetNode(ctx context.Context, id string) (*Node, error)

	// GetNeighbors 从指定节点出发进行邻居遍历，返回邻居节点与关联边。
	// depth 控制跳数（1=直接邻居），limit 限制返回数量。
	GetNeighbors(ctx context.Context, nodeID string, depth, limit int) ([]*Node, []*Edge, error)

	// GetByChunkIDs 通过 ChunkID 反查引用该 Chunk 的实体 Node 及其关联 Edge。
	// 一次调用同时返回 Nodes 与 Edges，
	// 用于语义检索命中 Chunk 后扩展到关系网络。
	GetByChunkIDs(ctx context.Context, chunkIDs []string) ([]*Node, []*Edge, error)

	// GetByLabels 按 Label 查询节点（如查询所有 Label="Region" 的节点）。
	// 用于 HyperIndexer.Tree() 基于 Region 节点组装知识树；GraphStore 中不存在 Chunk 节点。
	GetByLabels(ctx context.Context, labels []string, limit int) ([]*Node, error)

	// DeleteNode 按 ID 删除单个 Node。
	DeleteNode(ctx context.Context, id string) error

	// DeleteEdge 按 ID 删除单个 Edge。
	DeleteEdge(ctx context.Context, id string) error

	// Query 执行图查询（如 Cypher / GQL），返回原始行结果。
	// 由具体实现解析 query 语法（Neo4j 用 Cypher，Nebula 用 GQL）。
	Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)

	// Clear 清空所有 Node 与 Edge 数据，清空后可立即接收新数据。
	Clear(ctx context.Context) error

	// Close 优雅关闭连接，释放底层资源。
	Close(ctx context.Context) error
}
