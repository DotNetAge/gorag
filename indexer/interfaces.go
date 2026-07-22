// Package indexer 定义索引器的接口与实现。
//
// 本文件定义 indexer 包对外的 6 个核心接口（参考 io 包的小接口组合风格）：
//   - Indexer         核心接口：Name / AddFile / Search / NewQuery（所有索引器必实现）
//   - IndexerStore    存储接口：Save（各 Indexer 实现自动路由到各自存储）
//   - IndexerAdmin    管理接口：List / GetChunks / Count / Remove / Clear
//   - IndexerCloser   资源管理接口：Close
//   - TreeViewBuilder 导航接口：Tree（仅 HyperIndexer 实现）
//   - GraphSearcher   图查询接口：SearchGraph（仅 GraphIndexer 实现）
//
// 设计要点：
//   - 接口分离原则（ISP）：调用方按需 type-assert，不依赖不需要的方法
//   - 组合优于继承：HyperIndexer 通过组合 SemanticIndexer + GraphIndexer 实现双线协同
//   - 只支持文件输入：Indexer 职责单一化为「索引文件」，所有内容必须通过 AddFile 索引
//   - Hit 与 StructuredDoc 对称：StructuredDoc 是索引过程容器，Hit 是检索过程容器
//   - StructuredDoc 类型定义在 core 包，indexer 包不依赖 structurizer 包
package indexer

import (
	"context"

	"github.com/DotNetAge/gorag/v2/core"
)

// Indexer 索引器核心接口：负责文件索引和检索。
// 所有索引器必须实现此接口。
//
// 设计要点：
//   - 只支持文件输入（AddFile），不支持字符串输入
//   - 所有内容必须先落盘为文件，通过 document.Open 归一化
//   - source_file / region_id 等元数据依赖文件路径
//   - Search 返回 *core.Hit 容器（持有 Chunks/Nodes/Edges），与 StructuredDoc 对称
//
// 实现类型：
//   - SemanticIndexer：分块 + 向量化 + 语义检索
//   - GraphIndexer：分块 + 图结构化（独立使用为纯图谱模式）
//   - HyperIndexer：双线协同（语义线 + 关系线）
type Indexer interface {
	// Name 返回索引器名称（如 "semantic" / "graph" / "hyper"）。
	Name() string

	// AddFile 从文件读取内容后执行索引全流程。
	// filePath 必须为绝对路径，内部通过 document.Open 归一化后分块、向量化、写入存储。
	// 返回本次索引生成的 Chunks（调用方可用于追踪或后续处理）。
	AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error)

	// Search 执行检索，返回命中的 Hit 容器（持有 Chunks/Nodes/Edges）。
	// Hit 与 StructuredDoc 对称——StructuredDoc 是存，Hit 是取。
	Search(ctx context.Context, query core.Query) (*core.Hit, error)

	// NewQuery 构造查询对象，承载查询前优化与查询类型识别。
	NewQuery(terms string) core.Query
}

// IndexerStore 存储接口：保存 StructuredDoc 到各自存储。
//
// 各 Indexer 的 Save 实现自动路由到各自存储：
//   - SemanticIndexer.Save → 从 doc.Chunks() 读取，按 Title/Summary/Content 生成多维度向量，写入 VectorStore
//   - GraphIndexer.Save → 从 doc.Nodes()/Edges() 读取实体/关系，维护 Region→Document 的 CONTAINS 边，写入 GraphStore
//
// 语义：
//   - Save 接收的 StructuredDoc 已完成「读文件 + 归一化 + 分块 + 结构化」
//   - Chunks/Nodes/Edges 已由 HyperIndexer 调用 Chunker 填充到 doc 中
//   - 各 Indexer 从 doc 读取各自需要的数据（向量化 / 图结构化）
//   - Save 只负责「保存各自需要的数据」，不返回 Chunks
//
// HyperIndexer 不实现此接口——它是组合器，通过调用 semantic.Save + graph.Save 实现存储路由。
type IndexerStore interface {
	// Save 保存已结构化的文档到各自存储。
	// 不返回 Chunks——Chunks 已在 doc 中，各 Indexer 只负责保存。
	Save(ctx context.Context, doc core.StructuredDoc) error
}

// IndexerAdmin 索引器管理接口：浏览、统计、维护。
//
// 调用方按需 type-assert：
//
//	if a, ok := idx.(IndexerAdmin); ok { ... }
type IndexerAdmin interface {
	// List 分页浏览已索引的 Chunk。
	// filters 为 nil 时返回全部，非 nil 时按条件过滤（多个条件之间为 AND 语义）。
	// 返回当前页的 Chunk 切片与过滤前总数。
	List(ctx context.Context, offset, limit int, filters []core.FilterCondition) ([]core.Chunk, int, error)

	// GetChunks 按 docID 获取该文档的所有 Chunk。
	GetChunks(ctx context.Context, docID string) ([]*core.Chunk, error)

	// Count 返回已索引的 Chunk 总数。
	Count(ctx context.Context) (int, error)

	// Remove 按 chunkID 移除索引项（连带删除关联的 Nodes / Edges）。
	Remove(ctx context.Context, chunkID string) error

	// Clear 清空索引。
	Clear(ctx context.Context) error
}

// IndexerCloser 资源管理接口：释放底层存储资源。
//
// 调用方按需 type-assert：
//
//	if c, ok := idx.(IndexerCloser); ok { defer c.Close(ctx) }
type IndexerCloser interface {
	// Close 释放底层资源（数据库连接、文件句柄等）。
	Close(ctx context.Context) error
}

// TreeViewBuilder 知识库导航接口：构建 Region → Document → Chunk 层级树。
//
// 仅 HyperIndexer 实现，因为它需要同时访问 GraphStore 的 Region→Document 层级
// 和 VectorStore 的 Document→Chunk 数据。
// 调用方按需 type-assert：
//
//	if t, ok := idx.(TreeViewBuilder); ok { tree, err := t.Tree(ctx, "") }
type TreeViewBuilder interface {
	// Tree 输出基于 Region 层级的知识树。
	// regionID 为空时返回整棵树；非空时返回该 Region 子树。
	// 实现流程：先从 GraphIndexer 取 Region→Document 树，
	// 再通过 SemanticIndexer 为每个 Document 补齐 Chunk 子节点。
	Tree(ctx context.Context, regionID string) (*core.TreeNode, error)
}

// GraphSearcher 图查询扩展接口：执行图检索，返回 *Hit（Nodes / Edges 填充）。
//
// 仅 GraphIndexer 实现，SemanticIndexer 不维护 GraphStore。
// 调用方按需 type-assert：
//
//	if g, ok := idx.(GraphSearcher); ok { hit, err := g.SearchGraph(ctx, q) }
//
// 与 Indexer.Search 统一返回 *Hit，便于 Fusion 融合：
//   - SearchGraph 返回的 Hit 中 Chunks 为空，仅填充 Nodes / Edges
//   - 客户端可对 Search 和 SearchGraph 的结果直接做 Fusion 融合
//   - GraphResult 类型已删除，统一用 *core.Hit 表达检索结果
type GraphSearcher interface {
	// SearchGraph 执行图查询，返回 Hit（Nodes / Edges 填充，Chunks 为空）。
	SearchGraph(ctx context.Context, query core.Query) (*core.Hit, error)
}

// GraphNavigator 图导航扩展接口：从指定节点出发进行多跳邻居遍历。
//
// 仅 GraphIndexer / HyperIndexer 实现，用于 `grag nodes` 这类目录级图探索命令。
type GraphNavigator interface {
	// Neighbors 从 nodeID 出发遍历 depth 跳邻居，返回邻居节点与关联边。
	// depth=1 表示直接邻居；limit 限制返回节点数量。
	Neighbors(ctx context.Context, nodeID string, depth, limit int) ([]*core.Node, []*core.Edge, error)

	// GetNode 按 ID 获取单个节点，用于补全路径起点（如 Region 节点本身）。
	GetNode(ctx context.Context, nodeID string) (*core.Node, error)
}

// RegionGraphView 是 GraphIndexer 返回的目录级图视图。
type RegionGraphView struct {
	RegionID   string
	RegionName string
	Region     *core.Node
	Nodes      []*core.Node
	Edges      []*core.Edge
}

// GraphExplorer 图探索扩展接口：以目录为起点查询 Region 及其多跳邻居。
//
// GraphIndexer 直接实现；HyperIndexer 委托给内部 graph。
type GraphExplorer interface {
	// ExploreRegion 从指定目录的 Region 节点出发，遍历 depth 跳邻居。
	// dir 应为绝对路径；limit 限制返回节点数量。
	ExploreRegion(ctx context.Context, dir string, depth, limit int) (*RegionGraphView, error)
}
