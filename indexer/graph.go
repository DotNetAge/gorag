// Package indexer 定义索引器的接口与实现。
//
// 本文件实现 GraphIndexer：图索引器，职责单一化为「图结构化」。
//
// 设计要点：
//   - 不做分块（Chunker 由 HyperIndexer 调用；独立使用时 AddFile 内部调用 Chunker）
//   - 不做向量化（SemanticIndexer 负责）
//   - 只持有 GraphStore，从 doc.Nodes()/Edges() 读取实体/关系并写入
//   - Chunk 不作为 Node 写入 GraphStore
//   - 不实现 IndexerAdmin（Chunk 管理由 SemanticIndexer 通过 VectorStore 负责）
package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/DotNetAge/gorag/v2/chunker"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/query"
	"github.com/DotNetAge/gorag/v2/utils"
)

// minContentLength 是图索引最小内容长度（按字符数，非 token）。
// 短于此长度的文本直接静默丢弃，避免浪费 I/O 与存储。
const minContentLength = 20

// IndexError 包含索引失败的详细信息，供外部错误处理使用。
type IndexError struct {
	DocID     string        // 文档 ID
	Err       error         // 原始错误
	ErrorType string        // 错误分类: network | timeout | rate_limit | auth | api | unknown
	Attempts  int           // 重试次数
	Duration  time.Duration // 总耗时
}

// regionContextKey 用于在 context 中传递 region_id。
// 设置后，GraphIndexer 在写入时使用此值作为 region 标识，
// 而非从源文件路径推导。
//
// 用法（编排层）：
//
//	ctx := indexer.WithRegionID(ctx, regionID)
//	indexer.AddFile(ctx, filePath)
type regionContextKey struct{}

// WithRegionID 将 region ID 附加到 context。
// 传递给 AddFile / Save 时，相关元数据会携带此 region_id。
func WithRegionID(ctx context.Context, regionID string) context.Context {
	return context.WithValue(ctx, regionContextKey{}, regionID)
}

// RegionIDFromContext 从 context 中提取 region ID，未设置时返回空字符串。
func RegionIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(regionContextKey{}).(string)
	return id
}

// EntityDef 描述一种实体类型的提示词与 Schema 定义。
// 由 WithSchemas / WithSchemasFromFS 注入到 GraphIndexer。

// GraphIndexer 图索引器：职责单一化为「图结构化」。
//
// 设计要点：
//   - 不做分块（Chunker 由 HyperIndexer 调用，独立使用时 AddFile 内部调用 Chunker）
//   - 不做向量化（SemanticIndexer 负责）
//   - 只持有 GraphStore，从 doc.Nodes()/Edges() 读取实体/关系并写入
//   - Chunk 不作为 Node 写入 GraphStore
//   - 不实现 IndexerAdmin（Chunk 管理由 SemanticIndexer 通过 VectorStore 负责）
//
// 独立使用模式（纯图谱模式）：
//   - AddFile 内部调用 Chunker 分块，构造 StructuredDoc，调用 Save
//   - Search 只走图遍历（不走向量检索）
//
// HyperIndexer 编排模式：
//   - HyperIndexer 完成分块后，通过 Save(ctx, doc) 注入已分块的 StructuredDoc
//   - GraphIndexer 只负责写入实体/关系 + 维护 Region→Document 的 CONTAINS 边
type GraphIndexer struct {
	graphDB core.GraphStore
	logger  logging.Logger
	mu      sync.Mutex
	// entityDefs       []EntityDef            // 来自 WithSchemas 的全局实体类型定义
	// regionEntityDefs map[string][]EntityDef // 按 regionID 隔离的实体类型定义
	// 统计计数器（累积值，跨多次 AddFile/Save 调用）
	entitiesCreated int
	relsCreated     int
	statsMu         sync.Mutex
}

// GraphOption 配置 GraphIndexer 的可选参数。
type GraphOption func(*GraphIndexer)

// WithLogger 为 GraphIndexer 附加日志记录器。
func WithLogger(logger logging.Logger) GraphOption {
	return func(idx *GraphIndexer) {
		if logger != nil {
			idx.logger = logger
		}
	}
}

// WithSchemas 为 GraphIndexer 指定实体类型定义。

// WithSchemasFromFS 从文件系统（如 embed.FS）中读取匹配 glob 模式的实体类型配置文件，
// 解析后注入到 GraphIndexer。匹配多个文件时会合并所有实体类型定义。
//
// 用法：
//
//	//go:embed settings/entities-*.yml
//	var runtimeFS embed.FS
//
//	idx, _ := New(graphDB,
//	    WithSchemasFromFS(runtimeFS, "settings/entities-*.yml"),
//	)

// New 创建 GraphIndexer，返回 Indexer 接口。
//
// 参数：
//   - graphDB: 图存储（写入实体/关系，用于知识图谱检索）
//   - opts:    可选配置（WithLogger、WithSchemas、WithSchemasFromFS 等）
func New(
	graphDB core.GraphStore,
	opts ...GraphOption,
) (Indexer, error) {
	if graphDB == nil {
		return nil, fmt.Errorf("图索引器: graphDB 不能为空")
	}
	idx := &GraphIndexer{
		graphDB: graphDB,
		logger:  logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(idx)
	}
	return idx, nil
}

// CheckReady 检查 GraphIndexer 的核心存储组件是否已就绪。
func (idx *GraphIndexer) CheckReady() error {
	if idx.graphDB == nil {
		err := fmt.Errorf("图索引器: GraphDB 为空")
		idx.logger.Error("图索引器: 图存储组件未就绪", err)
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Indexer 接口实现
// ---------------------------------------------------------------------------

// Name 返回索引器名称。
func (idx *GraphIndexer) Name() string { return "graph" }

// SetEntityDefs 运行时更新全局实体类型定义列表。
// 用于用户在界面上保存知识标签选择后，同步到正在运行的 GraphIndexer。

// SetEntityDefsByRegion 设置指定 region 的实体类型定义。
// regionID 通常为项目目录的 SHA256 哈希。

// AddFile 索引一个文件，返回产生的 Chunks。
//
// 独立使用模式：内部用 Chunker 分块，构造 StructuredDoc，调用 Save。
// HyperIndexer 编排模式下不应直接调用 AddFile，应由 HyperIndexer 负责协调。
//
// 流程：
//  1. document.Open(filePath) → rawDoc
//  2. core.NewStructuredDoc(rawDoc) → doc
//  3. chunker.New(rawDoc) → chunkerImpl
//  4. chunkerImpl.Chunk(rawDoc) → result (含 Chunks/Nodes/Edges)
//  5. doc.SetChunks/SetNodes/SetEdges
//  6. idx.Save(ctx, doc)
//  7. 返回 result.Chunks 转换为 []*core.Chunk
func (idx *GraphIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error) {
	if filePath == "" {
		return nil, fmt.Errorf("图索引器: 文件路径不能为空")
	}
	idx.logger.Debug("图索引器: 开始索引文件", "file", filePath)

	// 文件大小预检 — 避免对过小文件做无意义的 I/O
	if fi, err := os.Stat(filePath); err == nil && fi.Size() < int64(minContentLength) {
		idx.logger.Debug("图索引器: 文件过小，跳过",
			"file", filePath,
			"size", fi.Size(),
			"min_length", minContentLength)
		return []*core.Chunk{}, nil
	}

	// 1. 检查存储组件就绪
	if err := idx.CheckReady(); err != nil {
		return nil, err
	}

	// 2. document.Open(filePath) → rawDoc
	rawDoc, err := document.Open(filePath)
	if err != nil {
		idx.logger.Error("图索引器: 打开文件失败", err, "file", filePath)
		return nil, fmt.Errorf("图索引器: 打开文件 %s 失败: %w", filePath, err)
	}

	// 内容长度预检（按 rune 计数）
	if utf8.RuneCountInString(rawDoc.Content()) < minContentLength {
		idx.logger.Debug("图索引器: 文件内容过短，跳过",
			"file", filePath,
			"min_length", minContentLength)
		return []*core.Chunk{}, nil
	}
	idx.logger.Debug("图索引器: 文件归一化完成",
		"file", filePath,
		"doc_type", rawDoc.Type(),
		"doc_id", rawDoc.ID())

	// 3. core.NewStructuredDoc(rawDoc) → doc
	doc, err := core.Structurize(rawDoc)
	if err != nil {
		idx.logger.Error("图索引器: 创建 StructuredDoc 失败", err, "file", filePath)
		return nil, fmt.Errorf("图索引器: 创建 StructuredDoc 失败: %w", err)
	}

	// 4. chunker.New(rawDoc) → chunkerImpl
	chunkerImpl, err := chunker.New(rawDoc)
	if err != nil {
		idx.logger.Error("图索引器: 创建 Chunker 失败", err,
			"file", filePath, "doc_type", rawDoc.Type())
		return nil, fmt.Errorf("图索引器: 创建 Chunker 失败: %w", err)
	}

	// 5. chunkerImpl.Chunk(rawDoc) → result (含 Chunks/Nodes/Edges)
	result, err := chunkerImpl.Chunk(rawDoc)
	if err != nil {
		idx.logger.Error("图索引器: 分块失败", err, "file", filePath)
		return nil, fmt.Errorf("图索引器: 分块失败: %w", err)
	}

	idx.logger.Info("图索引器: 开始索引文件",
		"file", filePath,
		"doc_id", rawDoc.ID(),
		"chunks", len(result.Chunks),
		"nodes", len(result.Nodes),
		"edges", len(result.Edges))

	// 6. doc.SetChunks/SetNodes/SetEdges
	doc.SetChunks(result.Chunks)
	doc.SetNodes(result.Nodes)
	doc.SetEdges(result.Edges)

	// 7. idx.Save(ctx, doc)
	if err := idx.Save(ctx, doc); err != nil {
		return nil, err
	}

	// 8. 返回 result.Chunks 转换为 []*core.Chunk
	chunks := make([]*core.Chunk, 0, len(result.Chunks))
	for i := range result.Chunks {
		ch := result.Chunks[i] // 复制一份，取地址
		chunks = append(chunks, &ch)
	}
	idx.logger.Debug("图索引器: 索引文件完成",
		"file", filePath, "chunks", len(chunks))
	return chunks, nil
}

// Search 实现 Indexer 接口：执行图遍历检索，返回 *core.Hit 容器。
//
// GraphIndexer 独立使用时不持有 VectorStore/Embedder，无法走向量检索。
// Search 只走图遍历（searchGraphOnly），返回的 Hit 仅包含 Nodes/Edges。
// 生产场景应通过 HyperIndexer 使用，由 SemanticIndexer 提供向量检索能力。
func (idx *GraphIndexer) Search(ctx context.Context, qry core.Query) (*core.Hit, error) {
	if qry == nil {
		return nil, fmt.Errorf("图索引器: 查询不能为空")
	}
	return idx.searchGraphOnly(ctx, qry)
}

// NewQuery 构造查询对象，默认查询类型为 semantic。
// 调用方可在查询前优化阶段通过 SetType 识别并修改为 graph / hybrid / keyword。
func (idx *GraphIndexer) NewQuery(terms string) core.Query {
	return query.New(terms)
}

// ---------------------------------------------------------------------------
// IndexerStore 接口实现
// ---------------------------------------------------------------------------

// Save 实现 IndexerStore 接口：保存已结构化的文档到 GraphStore。
//
//   - 从 doc.Nodes() 读取实体，写入 graphDB.UpsertNodes
//   - 从 doc.Edges() 读取关系，写入 graphDB.UpsertEdges
//   - 调用 writeContainsEdges 维护 Region→Document 的 CONTAINS 边
//   - 不写 Chunk 节点到 GraphStore
//   - 不写 Document→Chunk 的 CONTAINS 边（由 TreeViewBuilder 在视图层组装）
//
// 与 SemanticIndexer.Save 的差异：
//   - SemanticIndexer.Save 写 VectorStore（向量化）
//   - GraphIndexer.Save 写 GraphStore（实体 + Region→Document 边，不向量化）
func (idx *GraphIndexer) Save(ctx context.Context, doc core.StructuredDoc) error {
	if doc == nil {
		return fmt.Errorf("图索引器: Save doc 不能为空")
	}
	if err := idx.CheckReady(); err != nil {
		return err
	}
	idx.logger.Debug("图索引器: 开始保存",
		"nodes", len(doc.Nodes()),
		"edges", len(doc.Edges()))

	// 获取文件路径，用于为 Document 节点添加 source_file 属性
	var sourceFile string
	if raw := doc.Raw(); raw != nil {
		sourceFile = raw.FileName()
	}

	// 1. 从 doc.Nodes() 读取，转换为 []*core.Node，调用 graphDB.UpsertNodes
	rawNodes := doc.Nodes()
	if len(rawNodes) > 0 {
		nodes := make([]*core.Node, 0, len(rawNodes))
		for i := range rawNodes {
			n := rawNodes[i] // 复制一份，取地址
			// 为 Document 节点补充 source_file 属性（供 CountByRegion 查询）
			if sourceFile != "" {
				for _, l := range n.Labels {
					if l == "Document" {
						if n.Properties == nil {
							n.Properties = map[string]any{}
						}
						n.Properties[core.PropSourceFile] = sourceFile
						break
					}
				}
			}
			nodes = append(nodes, &n)
		}
		if err := idx.graphDB.UpsertNodes(ctx, nodes); err != nil {
			idx.logger.Error("图索引器: 写入节点失败", err,
				"node_count", len(nodes))
			return fmt.Errorf("图索引器: 写入节点失败: %w", err)
		}
		idx.statsMu.Lock()
		idx.entitiesCreated += len(nodes)
		idx.statsMu.Unlock()
		idx.logger.Debug("图索引器: 节点写入完成", "nodes", len(nodes))
	}

	// 2. 从 doc.Edges() 读取，转换为 []*core.Edge，调用 graphDB.UpsertEdges
	rawEdges := doc.Edges()
	if len(rawEdges) > 0 {
		edges := make([]*core.Edge, 0, len(rawEdges))
		for i := range rawEdges {
			e := rawEdges[i] // 复制一份，取地址
			edges = append(edges, &e)
		}
		if err := idx.graphDB.UpsertEdges(ctx, edges); err != nil {
			idx.logger.Error("图索引器: 写入边失败", err,
				"edge_count", len(edges))
			return fmt.Errorf("图索引器: 写入边失败: %w", err)
		}
		idx.statsMu.Lock()
		idx.relsCreated += len(edges)
		idx.statsMu.Unlock()
		idx.logger.Debug("图索引器: 边写入完成", "edges", len(edges))
	}

	// 3. 维护 Region→Document 的 CONTAINS 边
	//    CONTAINS 边失败不阻塞主流程，仅记录警告
	if err := idx.writeContainsEdges(ctx, doc); err != nil {
		idx.logger.Warn("图索引器: 维护 CONTAINS 边失败",
			"error", err.Error())
	}

	idx.logger.Debug("图索引器: 保存完成",
		"nodes", len(rawNodes),
		"edges", len(rawEdges))
	return nil
}

// writeContainsEdges 维护 Region→Document 的 CONTAINS 边。
//
// 流程：
//   - 从 doc.Raw() 获取文件路径
//   - 创建 Region 节点（如果不存在）
//   - 创建 Region→Document 的 CONTAINS 边
//   - Document 节点应该已经在 doc.Nodes() 中（由 Chunker 生成），UpsertNodes 会处理
//   - 不创建 Document→Chunk 的边（由 TreeViewBuilder 在视图层组装）
func (idx *GraphIndexer) writeContainsEdges(ctx context.Context, doc core.StructuredDoc) error {
	raw := doc.Raw()
	if raw == nil {
		return nil
	}
	fileName := raw.FileName()
	if fileName == "" {
		return nil
	}
	dir := filepath.Dir(fileName)
	regionName := filepath.Base(dir)
	if regionName == "" || regionName == "." || regionName == "/" {
		return nil
	}

	// Region 节点 ID：取 "region:" + dir 的 SHA256
	regionID := utils.GenerateID([]byte("region:" + dir))

	// 查找 Document 节点 ID（由 Chunker 创建，应在 doc.Nodes() 中）
	var docNodeID string
	for _, n := range doc.Nodes() {
		for _, l := range n.Labels {
			if l == "Document" {
				docNodeID = n.ID
				break
			}
		}
		if docNodeID != "" {
			break
		}
	}
	if docNodeID == "" {
		// Chunker 未生成 Document 节点，跳过 CONTAINS 边创建
		return nil
	}

	// 创建/更新 Region 节点
	regionNode := &core.Node{
		ID:         regionID,
		Labels:     []string{core.LabelRegion},
		Name:       regionName,
		Properties: map[string]any{"dir": dir},
	}
	if err := idx.graphDB.UpsertNodes(ctx, []*core.Node{regionNode}); err != nil {
		return fmt.Errorf("图索引器: 写入 Region 节点失败: %w", err)
	}

	// 创建 Region→Document 的 CONTAINS 边
	containsEdge := &core.Edge{
		ID:        utils.GenerateID([]byte(regionID + ":CONTAINS:" + docNodeID)),
		Type:      "CONTAINS",
		Source:    regionID,
		Target:    docNodeID,
		Predicate: "CONTAINS",
	}
	if err := idx.graphDB.UpsertEdges(ctx, []*core.Edge{containsEdge}); err != nil {
		return fmt.Errorf("图索引器: 写入 Region→Document CONTAINS 边失败: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// GraphSearcher 接口实现
// ---------------------------------------------------------------------------

// SearchGraph 实现 GraphSearcher 接口：执行图查询，返回 *Hit（仅 Nodes/Edges，Chunks 为空）。
//
// 与 Search 的区别：
//   - Search 在 HyperIndexer 编排下可能融合多源结果
//   - SearchGraph 明确只走图遍历，返回的 Hit 仅包含 Nodes/Edges
//
// 客户端可对 Search 和 SearchGraph 的结果直接做 Fusion 融合。
func (idx *GraphIndexer) SearchGraph(ctx context.Context, q core.Query) (*core.Hit, error) {
	if q == nil {
		return nil, fmt.Errorf("图索引器: 查询不能为空")
	}
	return idx.searchGraphOnly(ctx, q)
}

// searchGraphOnly 仅做图查询：基于 GraphStore 的 Cypher 查询 → 关联 Nodes/Edges → *Hit（无 Chunks）。
//
//   - 通过 GraphStore.Query 执行 Cypher 模糊匹配节点名称，作为图遍历的起点
//   - 返回的 Hit 仅包含 Nodes/Edges，Chunks 为空
func (idx *GraphIndexer) searchGraphOnly(ctx context.Context, q core.Query) (*core.Hit, error) {
	if q == nil || q.Raw() == "" {
		return nil, nil
	}
	if idx.graphDB == nil {
		return nil, fmt.Errorf("图索引器: graphDB 未初始化")
	}
	idx.logger.Debug("图索引器: 开始图查询", "query", q.Raw())

	// 1. 通过 Cypher 模糊匹配节点名称（双向匹配，覆盖关键词和实体名）
	//    限制返回 20 个节点，避免大图全表扫描。
	cypher := `MATCH (n) WHERE n.name CONTAINS $keyword OR $keyword CONTAINS n.name RETURN n LIMIT 20`
	rows, err := idx.graphDB.Query(ctx, cypher, map[string]any{"keyword": q.Raw()})
	if err != nil {
		idx.logger.Error("图索引器: 图查询失败", err, "query", q.Raw())
		return nil, fmt.Errorf("图索引器: 图查询失败: %w", err)
	}
	if len(rows) == 0 {
		idx.logger.Debug("图索引器: 图查询无结果", "query", q.Raw())
		return nil, nil
	}

	// 2. 解析查询结果，收集节点 ID
	nodeIDs := make([]string, 0, len(rows))
	nodes := make([]*core.Node, 0, len(rows))
	for _, row := range rows {
		node := extractNodeFromRow(row)
		if node == nil || node.ID == "" {
			continue
		}
		nodes = append(nodes, node)
		nodeIDs = append(nodeIDs, node.ID)
	}
	if len(nodeIDs) == 0 {
		idx.logger.Debug("图索引器: 图查询结果解析后无有效节点", "query", q.Raw())
		return nil, nil
	}
	idx.logger.Debug("图索引器: 图查询命中节点", "nodes", len(nodeIDs))

	// 3. 构建命中 Hit
	hit := &core.Hit{Query: q}

	// 填充 NodeHit（按 scoreNode 评分）
	nodeMap := make(map[string]*core.Node, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		nodeMap[n.ID] = n
		hit.Nodes = append(hit.Nodes, core.NodeHit{
			Node:  n,
			Score: scoreNode(n),
		})
	}

	// 4. 查询这些节点的邻居边（depth=1）
	//    使用 GetNeighbors 逐个查询，合并去重。
	edgeMap := make(map[string]*core.Edge)
	for _, nodeID := range nodeIDs {
		neighbors, edges, err := idx.graphDB.GetNeighbors(ctx, nodeID, 1, 10)
		if err != nil {
			idx.logger.Warn("图索引器: 查询节点邻居失败",
				"node_id", nodeID, "error", err.Error())
			continue
		}
		for _, e := range edges {
			if e != nil {
				edgeMap[e.ID] = e
			}
		}
		for _, n := range neighbors {
			if n == nil {
				continue
			}
			if _, exists := nodeMap[n.ID]; !exists {
				nodeMap[n.ID] = n
				hit.Nodes = append(hit.Nodes, core.NodeHit{
					Node:  n,
					Score: scoreNode(n),
				})
			}
		}
	}

	for _, e := range edgeMap {
		hit.Edges = append(hit.Edges, core.EdgeHit{
			Edge:  e,
			Score: scoreEdge(e),
		})
	}

	// 5. Hit.Score = topNodeScore + topEdgeScore（无 Chunks 贡献）
	var topNode, topEdge float32
	if len(hit.Nodes) > 0 {
		topNode = hit.Nodes[0].Score
	}
	if len(hit.Edges) > 0 {
		topEdge = hit.Edges[0].Score
	}
	hit.Score = topNode + topEdge

	idx.logger.Debug("图索引器: 图查询完成",
		"query", q.Raw(),
		"nodes", len(hit.Nodes),
		"edges", len(hit.Edges),
		"score", hit.Score)
	return hit, nil
}

// ---------------------------------------------------------------------------
// IndexerCloser 接口实现
// ---------------------------------------------------------------------------

// Close 关闭底层图存储。
func (idx *GraphIndexer) Close(ctx context.Context) error {
	if idx.graphDB == nil {
		return nil
	}
	idx.logger.Debug("图索引器: 关闭图存储")
	if err := idx.graphDB.Close(ctx); err != nil {
		idx.logger.Error("图索引器: 关闭图存储失败", err)
		return err
	}
	idx.logger.Debug("图索引器: 图存储已关闭")
	return nil
}

// ---------------------------------------------------------------------------
// 内部方法
// ---------------------------------------------------------------------------

// regionTree 返回 Region→Document 树（不含 Chunk 子节点）。
// HyperIndexer.Tree() 通过此方法取得骨架，再从 VectorStore 补齐 Chunk 子节点。
//
// 参数：
//   - regionID 为空时返回全局树（所有 Region 为第一级子节点）
//   - regionID 非空时返回该 Region 的子树（仅 Document 子节点）
//
// 注意：此方法为未导出方法，供 HyperIndexer 内部组合使用。
func (idx *GraphIndexer) regionTree(ctx context.Context, regionID string) (*core.TreeNode, error) {
	if idx.graphDB == nil {
		return nil, fmt.Errorf("图索引器: graphDB 未初始化")
	}
	idx.logger.Debug("图索引器: 构建 Region 树", "region_id", regionID)

	root := &core.TreeNode{
		ID:   "root",
		Type: "root",
		Name: "知识库",
	}

	// 1. 查询 Region 节点
	var regionNodes []*core.Node
	if regionID != "" {
		// 查询指定 Region 节点
		node, err := idx.graphDB.GetNode(ctx, regionID)
		if err != nil {
			idx.logger.Error("图索引器: 查询 Region 节点失败", err, "region_id", regionID)
			return nil, fmt.Errorf("图索引器: 查询 Region 节点失败: %w", err)
		}
		if node == nil {
			idx.logger.Warn("图索引器: Region 不存在", "region_id", regionID)
			return nil, fmt.Errorf("图索引器: Region %s 不存在", regionID)
		}
		regionNodes = []*core.Node{node}
	} else {
		// 查询所有 Region 节点
		nodes, err := idx.graphDB.GetByLabels(ctx, []string{core.LabelRegion}, -1)
		if err != nil {
			idx.logger.Error("图索引器: 查询所有 Region 节点失败", err)
			return nil, fmt.Errorf("图索引器: 查询所有 Region 节点失败: %w", err)
		}
		regionNodes = nodes
	}
	idx.logger.Debug("图索引器: Region 节点查询完成", "count", len(regionNodes))

	// 2. 对每个 Region，通过 GetNeighbors 查询 CONTAINS 边指向的 Document 节点
	for _, regionNode := range regionNodes {
		if regionNode == nil {
			continue
		}
		regionTN := nodeToTreeNode(regionNode, "region")

		neighbors, edges, err := idx.graphDB.GetNeighbors(ctx, regionNode.ID, 1, -1)
		if err != nil {
			idx.logger.Warn("图索引器: 查询 Region 邻居失败",
				"region_id", regionNode.ID, "error", err)
			continue
		}

		// 构建 nodeID→node 映射
		nodeMap := make(map[string]*core.Node, len(neighbors))
		for _, n := range neighbors {
			if n != nil {
				nodeMap[n.ID] = n
			}
		}

		// 遍历 edges，找 Region→Document 的 CONTAINS 边
		for _, edge := range edges {
			if edge == nil || edge.Type != "CONTAINS" || edge.Source != regionNode.ID {
				continue
			}
			docNode, ok := nodeMap[edge.Target]
			if !ok {
				continue
			}
			// 检查是否为 Document 节点
			isDocument := false
			for _, l := range docNode.Labels {
				if l == "Document" {
					isDocument = true
					break
				}
			}
			if !isDocument {
				continue
			}
			docTN := nodeToTreeNode(docNode, "document")
			regionTN.AddChild(docTN)
		}

		root.AddChild(regionTN)
	}

	return root, nil
}

// ---------------------------------------------------------------------------
// 扩展方法
// ---------------------------------------------------------------------------

// GraphDB 返回 GraphIndexer 持有的图数据库实例。
// 外部可通过此方法直接操作图存储（如自定义 Cypher 查询、图分析等）。
func (idx *GraphIndexer) GraphDB() core.GraphStore {
	return idx.graphDB
}

// CypherQuery 执行原始的 Cypher 查询，供外部 Agent/LLM 生成高级图查询。
// 参数 params 为 Cypher 查询的命名参数映射。
func (idx *GraphIndexer) CypherQuery(ctx context.Context, q string, params map[string]any) ([]map[string]any, error) {
	if idx.graphDB == nil {
		return nil, fmt.Errorf("图索引器: graphDB 未初始化")
	}
	return idx.graphDB.Query(ctx, q, params)
}

// CountByRegion 返回指定路径下（source_file 前缀匹配）的 Document 节点总数。
// Document 节点的 source_file 属性由 Save 方法在写入时填充。
func (idx *GraphIndexer) CountByRegion(ctx context.Context, path string) (int, error) {
	if idx.graphDB == nil {
		return 0, nil
	}
	if path == "" {
		return 0, nil
	}
	cypher := `MATCH (d:Document) WHERE d.source_file STARTS WITH $path RETURN count(d) AS cnt`
	rows, err := idx.graphDB.Query(ctx, cypher, map[string]any{"path": path})
	if err != nil {
		idx.logger.Error("图索引器: CountByRegion 查询失败", err, "path", path)
		return 0, fmt.Errorf("图索引器: CountByRegion 查询失败: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if cnt, ok := rows[0]["cnt"].(int64); ok {
		idx.logger.Debug("图索引器: CountByRegion 完成",
			"path", path, "count", cnt)
		return int(cnt), nil
	}
	return 0, nil
}

// EntityStats 返回自上次 ResetEntityStats 以来累计创建的实体和关系数量。
func (idx *GraphIndexer) EntityStats() (entities, rels int) {
	idx.statsMu.Lock()
	defer idx.statsMu.Unlock()
	return idx.entitiesCreated, idx.relsCreated
}

// ResetEntityStats 将实体/关系计数器归零（通常在每次 Sync 开始前调用）。
func (idx *GraphIndexer) ResetEntityStats() {
	if idx == nil {
		return
	}
	idx.statsMu.Lock()
	defer idx.statsMu.Unlock()
	idx.entitiesCreated = 0
	idx.relsCreated = 0
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// extractNodeFromRow 从 Cypher 查询结果行中提取 Node。
// 兼容两种返回格式：
//   - {n: {id, name, labels, properties}}
//   - {n.id, n.name, n.labels}
func extractNodeFromRow(row map[string]any) *core.Node {
	// 格式 1: n 是嵌套 map
	if r, ok := row["n"].(map[string]any); ok {
		id, _ := r["id"].(string)
		name, _ := r["name"].(string)
		if id == "" {
			return nil
		}
		n := &core.Node{
			ID:   id,
			Name: name,
		}
		if labels, ok := r["labels"].([]any); ok {
			for _, l := range labels {
				if s, ok := l.(string); ok {
					n.Labels = append(n.Labels, s)
				}
			}
		}
		if props, ok := r["properties"].(map[string]any); ok {
			n.Properties = props
		}
		return n
	}
	// 格式 2: 拍平字段
	id, _ := row["n.id"].(string)
	if id == "" {
		return nil
	}
	name, _ := row["n.name"].(string)
	n := &core.Node{
		ID:   id,
		Name: name,
	}
	if labels, ok := row["n.labels"].([]any); ok {
		for _, l := range labels {
			if s, ok := l.(string); ok {
				n.Labels = append(n.Labels, s)
			}
		}
	}
	return n
}

// nodeToTreeNode 将 graphDB Node 转换为 TreeNode。
// Document 节点的 source_file 映射到 Path；Region 节点的 dir 映射到 Path。
func nodeToTreeNode(n *core.Node, nodeType string) *core.TreeNode {
	tn := &core.TreeNode{
		ID:   n.ID,
		Type: nodeType,
		Name: n.Name,
	}
	if n.Properties != nil {
		if sourceFile, ok := n.Properties[core.PropSourceFile].(string); ok && sourceFile != "" {
			tn.Path = sourceFile
		}
		if dir, ok := n.Properties[core.PropDir].(string); ok && dir != "" {
			tn.Path = dir
		}
	}
	return tn
}

// scoreNode 为节点计算相关性分数。
// 基于节点频率（frequency）和置信度（confidence）的加权组合。
func scoreNode(n *core.Node) float32 {
	if n == nil {
		return 0
	}
	score := float32(0.3) // 基础分
	if n.Properties != nil {
		if f, ok := n.Properties[core.PropFrequency].(int); ok && f > 0 {
			score += float32(f) * 0.01
		}
		if c, ok := n.Properties[core.PropConfidence].(float64); ok {
			score += float32(c) * 0.1
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// scoreEdge 为边计算相关性分数。
// 基于边强度（score）和置信度（confidence）的加权组合。
func scoreEdge(e *core.Edge) float32 {
	if e == nil {
		return 0
	}
	score := float32(0.2) // 基础分
	if e.Properties != nil {
		if s, ok := e.Properties[core.PropScore].(float64); ok {
			score += float32(s) * 0.1
		}
		if c, ok := e.Properties[core.PropConfidence].(float64); ok {
			score += float32(c) * 0.1
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// generateChunkID 生成 chunk ID。
// 格式：chunk_{docID}_{index}_{hash8}
// 保留此函数供 AddFile 独立模式或外部调用使用。
func generateChunkID(docID string, index int, content string) string {
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])[:8]
	return fmt.Sprintf("chunk_%s_%d_%s", docID, index, hashStr)
}
