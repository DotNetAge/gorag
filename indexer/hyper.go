package indexer

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/v2/chunker"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/result"
)

// =====================================================================
// HyperIndexer 复合索引器
// =====================================================================
//
// HyperIndexer 是双线结合的契机——协调 SemanticIndexer（语义线）和 GraphIndexer（关系线）。
//
// 核心设计：
//   - GraphIndexer 职责独立，不做分块+向量化
//   - SemanticIndexer 职责独立，不做实体提取+图结构化
//   - HyperIndexer 编排双线：读文件→结构化→分块→分流到 SemanticIndexer + GraphIndexer
//
// 工作流（AddFile）：
//  1. document.Open(path) → RawDoc（读文件 + 归一化）
//  2. core.NewStructuredDoc(raw) → StructuredDoc（结构化容器）
//  3. 路由 Chunker（注入的或 chunker.New(raw)）；chunkerImpl.Chunk(raw) → ChunkResult（含 Chunks/Nodes/Edges）
//  4. doc.SetChunks/SetNodes/SetEdges（把三类产物分别存入 StructuredDoc）
//  5. semantic.Save(doc)（语义线：向量化+写入 VectorStore）
//  6. graph.Save(doc)（关系线：实体+CONTAINS 边+写入 GraphStore，若 graph 存在）
//
// 工作流（Search）：
//  1. semantic.Search(q) → semHit（Chunks 填充）
//  2. graph.SearchGraph(q) → graphHit（Nodes/Edges 填充，若 graph 存在）
//  3. result.RRF(semHit, graphHit) → 融合 Hit（三类齐全）
//
// 扩展能力（通过 type-assert）：
//   - hyper.(IndexerAdmin)   → 委托 semantic（语义线持有 VectorStore）
//   - hyper.(IndexerCloser)  → 双线联动关闭
//   - hyper.(TreeViewBuilder)→ HyperIndexer 自身实现（先取 Region→Document，再补齐 Chunk）
//   - hyper.(GraphSearcher)  → 委托 graph
type HyperIndexer struct {
	chunker  chunker.Chunker // 分块器（按 RawDoc.Type 路由，同时产出 Nodes/Edges）
	semantic Indexer         // 语义线（必注入）
	graph    Indexer         // 关系线（可选，nil 则不启用图功能）
	logger   logging.Logger
}

// HyperOption HyperIndexer 配置选项
type HyperOption func(*HyperIndexer)

// WithHyperLogger 设置日志记录器
func WithHyperLogger(logger logging.Logger) HyperOption {
	return func(h *HyperIndexer) {
		if logger != nil {
			h.logger = logger
		}
	}
}

// WithHyperChunker 设置自定义分块器（默认按 RawDoc.Type 自动路由）
func WithHyperChunker(c chunker.Chunker) HyperOption {
	return func(h *HyperIndexer) {
		if c != nil {
			h.chunker = c
		}
	}
}

// NewHyperIndexer 创建复合索引器，返回 Indexer 接口。
//
// 参数：
//   - semantic: 语义线索引器（必传，通常由 NewSemanticIndexer 创建）
//   - graph:    关系线索引器（可选，传 nil 则不启用图功能；通常由 New（GraphIndexer）创建）
//   - opts:     可选配置（WithHyperLogger、WithHyperChunker）
//
// 设计要点：
//   - semantic 为 nil 时返回 error（语义线是必传的核心能力）
//   - graph 为 nil 时降级为纯语义模式（Search 只返回 Chunks）
//   - 默认分块器按需懒创建（每次 AddFile 时根据 RawDoc.Type 路由）
func NewHyperIndexer(semantic Indexer, graph Indexer, opts ...HyperOption) (Indexer, error) {
	if semantic == nil {
		return nil, fmt.Errorf("NewHyperIndexer: semantic 不能为空")
	}
	h := &HyperIndexer{
		semantic: semantic,
		graph:    graph,
		logger:   logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

// Name 返回索引器名称
func (h *HyperIndexer) Name() string { return "hyper" }

// AddFile 实现 Indexer 接口：对外统一入口，编排双线索引。
//
// 工作流：
//  1. document.Open(filePath) → RawDoc
//  2. core.NewStructuredDoc(raw) → StructuredDoc 容器
//  3. 路由 Chunker（注入优先，否则 chunker.New(raw)）→ chunkerImpl.Chunk(raw) → ChunkResult
//  4. doc.SetChunks/SetNodes/SetEdges（三类产物分别写入 doc）
//  5. semantic.Save(doc)（向量化 + 写入 VectorStore）
//  6. graph.Save(doc)（实体 + CONTAINS 边 + 写入 GraphStore，若 graph 存在）
//
// 返回本次索引生成的 Chunks（用于调用方追踪）。
// 关系线失败不阻塞语义线，仅记录警告。
func (h *HyperIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error) {
	if filePath == "" {
		return nil, fmt.Errorf("HyperIndexer: 文件路径不能为空")
	}
	h.logger.Debug("复合索引器: 开始索引文件", "file", filePath)

	// 1. 读文件 + 归一化
	raw, err := document.Open(filePath)
	if err != nil {
		h.logger.Error("复合索引器: 打开文件失败", err, "file", filePath)
		return nil, fmt.Errorf("HyperIndexer: 打开文件 %s 失败: %w", filePath, err)
	}
	h.logger.Debug("复合索引器: 文件归一化完成",
		"file", filePath, "doc_type", raw.Type(), "doc_id", raw.ID())

	// 2. 结构化为 StructuredDoc 容器（Chunks/Nodes/Edges 此时均为空，由 Chunker 填充）
	doc, err := core.Structurize(raw)
	if err != nil {
		h.logger.Error("复合索引器: 结构化文档失败", err, "file", filePath)
		return nil, fmt.Errorf("HyperIndexer: 结构化文档失败: %w", err)
	}

	// 3. 路由 Chunker（注入优先，否则按 RawDoc.Type 自动路由）
	chunkerImpl := h.chunker
	if chunkerImpl == nil {
		chunkerImpl, err = chunker.New(raw)
		if err != nil {
			h.logger.Error("复合索引器: 创建分块器失败", err,
				"file", filePath, "doc_type", raw.Type())
			return nil, fmt.Errorf("HyperIndexer: 创建分块器失败: %w", err)
		}
	}

	// 4. 分块：返回 ChunkResult（同时包含 Chunks/Nodes/Edges）
	result, err := chunkerImpl.Chunk(raw)
	if err != nil {
		h.logger.Error("复合索引器: 分块失败", err, "file", filePath)
		return nil, fmt.Errorf("HyperIndexer: 分块失败: %w", err)
	}
	h.logger.Debug("复合索引器: 分块完成",
		"file", filePath,
		"chunks", len(result.Chunks),
		"nodes", len(result.Nodes),
		"edges", len(result.Edges))

	// 5. 把三类产物分别存入 StructuredDoc（供 Save 读取）
	doc.SetChunks(result.Chunks)
	doc.SetNodes(result.Nodes)
	doc.SetEdges(result.Edges)

	// 6. 语义线：向量化 + 写入 VectorStore（失败返回 error）
	if store, ok := h.semantic.(IndexerStore); ok {
		if err := store.Save(ctx, doc); err != nil {
			h.logger.Error("复合索引器: 语义线保存失败", err, "file", filePath)
			return nil, fmt.Errorf("HyperIndexer: 语义线保存失败: %w", err)
		}
		h.logger.Debug("复合索引器: 语义线保存完成", "chunks", len(result.Chunks))
	} else {
		h.logger.Warn("复合索引器: semantic 未实现 IndexerStore，跳过语义线保存")
	}

	// 7. 关系线：实体 + CONTAINS 边 + 写入 GraphStore（若 graph 存在）
	//    关系线失败不阻塞语义线结果，仅记录警告
	if h.graph != nil {
		if store, ok := h.graph.(IndexerStore); ok {
			if err := store.Save(ctx, doc); err != nil {
				h.logger.Warn("复合索引器: 关系线保存失败（不阻塞语义线）",
					"file", filePath, "error", err.Error())
			} else {
				h.logger.Debug("复合索引器: 关系线保存完成",
					"nodes", len(result.Nodes),
					"edges", len(result.Edges))
			}
		} else {
			h.logger.Warn("复合索引器: graph 未实现 IndexerStore，跳过关系线保存")
		}
	} else {
		h.logger.Debug("复合索引器: 未启用关系线，跳过图保存")
	}

	// 8. 转换 []core.Chunk → []*core.Chunk 返回（保持 AddFile 签名一致）
	chunks := make([]*core.Chunk, 0, len(result.Chunks))
	for i := range result.Chunks {
		ch := result.Chunks[i]
		chunks = append(chunks, &ch)
	}
	h.logger.Debug("复合索引器: 索引文件完成",
		"file", filePath, "chunks", len(chunks))
	return chunks, nil
}

// Search 实现 Indexer 接口：双线融合检索。
//
// 工作流：
//  1. semantic.Search(q) → semHit（Chunks 填充）
//  2. graph.SearchGraph(q) → graphHit（Nodes/Edges 填充，若 graph 存在）
//  3. result.RRF(semHit, graphHit) → 融合 Hit（三类齐全）
//
// 设计要点：
//   - graph 为 nil 时跳过图检索，直接返回 semHit
//   - graphHit 为 nil 时 RRF 内部自动跳过，不影响融合
//   - 融合后 Hit.Score = topChunkScore + topNodeScore + topEdgeScore
func (h *HyperIndexer) Search(ctx context.Context, q core.Query) (*core.Hit, error) {
	if q == nil {
		return nil, fmt.Errorf("HyperIndexer: 查询不能为空")
	}
	h.logger.Debug("复合索引器: 开始检索", "query", q.Raw(), "type", q.Type())

	// 1. 语义线检索
	semHit, err := h.semantic.Search(ctx, q)
	if err != nil {
		h.logger.Error("复合索引器: 语义检索失败", err, "query", q.Raw())
		return nil, fmt.Errorf("HyperIndexer: 语义检索失败: %w", err)
	}
	h.logger.Debug("复合索引器: 语义检索完成",
		"chunks", len(semHit.Chunks))

	// 2. 关系线检索（若 graph 存在）
	var graphHit *core.Hit
	if h.graph != nil {
		if gs, ok := h.graph.(GraphSearcher); ok {
			graphHit, err = gs.SearchGraph(ctx, q)
			if err != nil {
				h.logger.Warn("复合索引器: 图检索失败（不阻塞语义结果）",
					"query", q.Raw(), "error", err.Error())
				// 图检索失败不阻塞语义结果，继续融合
			} else if graphHit != nil {
				h.logger.Debug("复合索引器: 图检索完成",
					"nodes", len(graphHit.Nodes),
					"edges", len(graphHit.Edges))
			}
		}
	} else {
		h.logger.Debug("复合索引器: 未启用关系线，跳过图检索")
	}

	// 3. 融合：语义 Hit + 图 Hit → 综合 Hit
	//    Chunks 来自语义线，Nodes/Edges 来自关系线
	//    Score = topChunk + topNode + topEdge（缺失类别贡献 0）
	fused, err := result.RRF(
		result.NewSource("semantic", 1.0, semHit),
		result.NewSource("graph", 0.7, graphHit),
	)
	if err != nil {
		// 融合失败时降级返回语义结果
		h.logger.Warn("复合索引器: 融合失败，降级返回语义结果",
			"error", err.Error())
		return semHit, nil
	}
	h.logger.Debug("复合索引器: 融合完成",
		"chunks", len(fused.Chunks),
		"nodes", len(fused.Nodes),
		"edges", len(fused.Edges),
		"score", fused.Score)

	// 保留 Query（从 semHit 继承）
	if fused.Query == nil {
		fused.Query = q
	}

	return fused, nil
}

// NewQuery 实现 Indexer 接口：委托语义线构造查询。
func (h *HyperIndexer) NewQuery(terms string) core.Query {
	return h.semantic.NewQuery(terms)
}

// ---------------------------------------------------------------------------
// 扩展接口委托（通过 type-assert）
// ---------------------------------------------------------------------------

// List 实现 IndexerAdmin 接口：委托语义线（VectorStore 持有全部 Chunk 数据）。
func (h *HyperIndexer) List(ctx context.Context, offset, limit int, filters []core.FilterCondition) ([]core.Chunk, int, error) {
	if a, ok := h.semantic.(IndexerAdmin); ok {
		return a.List(ctx, offset, limit, filters)
	}
	return nil, 0, fmt.Errorf("HyperIndexer: semantic 未实现 IndexerAdmin")
}

// GetChunks 实现 IndexerAdmin 接口：委托语义线。
func (h *HyperIndexer) GetChunks(ctx context.Context, docID string) ([]*core.Chunk, error) {
	if a, ok := h.semantic.(IndexerAdmin); ok {
		return a.GetChunks(ctx, docID)
	}
	return nil, fmt.Errorf("HyperIndexer: semantic 未实现 IndexerAdmin")
}

// Count 实现 IndexerAdmin 接口：委托语义线。
func (h *HyperIndexer) Count(ctx context.Context) (int, error) {
	if a, ok := h.semantic.(IndexerAdmin); ok {
		return a.Count(ctx)
	}
	return 0, fmt.Errorf("HyperIndexer: semantic 未实现 IndexerAdmin")
}

// Remove 实现 IndexerAdmin 接口：双线联动删除。
// 语义线和关系线都需要按 chunkID 删除关联数据。
// 关系线删除失败不阻塞语义线，仅记录警告。
func (h *HyperIndexer) Remove(ctx context.Context, chunkID string) error {
	h.logger.Debug("复合索引器: 删除分片", "chunk_id", chunkID)
	// 1. 语义线删除
	if a, ok := h.semantic.(IndexerAdmin); ok {
		if err := a.Remove(ctx, chunkID); err != nil {
			h.logger.Error("复合索引器: 语义线删除失败", err, "chunk_id", chunkID)
			return fmt.Errorf("HyperIndexer: 语义线删除失败: %w", err)
		}
	}
	// 2. 关系线删除（若 graph 存在）
	if h.graph != nil {
		if a, ok := h.graph.(IndexerAdmin); ok {
			if err := a.Remove(ctx, chunkID); err != nil {
				h.logger.Warn("复合索引器: 关系线删除失败（不阻塞）",
					"chunk_id", chunkID, "error", err.Error())
				// 关系线删除失败不阻塞，仅记录警告
			}
		}
	}
	h.logger.Debug("复合索引器: 删除分片完成", "chunk_id", chunkID)
	return nil
}

// Clear 实现 IndexerAdmin 接口：双线联动清空。
func (h *HyperIndexer) Clear(ctx context.Context) error {
	h.logger.Debug("复合索引器: 清空索引")
	// 1. 语义线清空
	if a, ok := h.semantic.(IndexerAdmin); ok {
		if err := a.Clear(ctx); err != nil {
			h.logger.Error("复合索引器: 语义线清空失败", err)
			return fmt.Errorf("HyperIndexer: 语义线清空失败: %w", err)
		}
	}
	// 2. 关系线清空（若 graph 存在）
	if h.graph != nil {
		if a, ok := h.graph.(IndexerAdmin); ok {
			if err := a.Clear(ctx); err != nil {
				h.logger.Error("复合索引器: 关系线清空失败", err)
				return fmt.Errorf("HyperIndexer: 关系线清空失败: %w", err)
			}
		}
	}
	h.logger.Info("复合索引器: 索引已清空")
	return nil
}

// Close 实现 IndexerCloser 接口：双线联动关闭。
// 任何一线关闭失败都返回 error，但会尝试关闭所有线后再返回。
func (h *HyperIndexer) Close(ctx context.Context) error {
	h.logger.Debug("复合索引器: 关闭存储")
	var firstErr error
	// 1. 语义线关闭
	if c, ok := h.semantic.(IndexerCloser); ok {
		if err := c.Close(ctx); err != nil {
			h.logger.Error("复合索引器: 语义线关闭失败", err)
			firstErr = fmt.Errorf("HyperIndexer: 语义线关闭失败: %w", err)
		}
	}
	// 2. 关系线关闭（若 graph 存在）
	if h.graph != nil {
		if c, ok := h.graph.(IndexerCloser); ok {
			if err := c.Close(ctx); err != nil {
				if firstErr == nil {
					h.logger.Error("复合索引器: 关系线关闭失败", err)
					firstErr = fmt.Errorf("HyperIndexer: 关系线关闭失败: %w", err)
				} else {
					h.logger.Warn("复合索引器: 关系线关闭失败（语义线也已失败）",
						"error", err.Error())
				}
			}
		}
	}
	h.logger.Debug("复合索引器: 存储已关闭")
	return firstErr
}

// ---------------------------------------------------------------------------
// TreeViewBuilder 接口实现（HyperIndexer 自身实现）
// ---------------------------------------------------------------------------

// Tree 实现 TreeViewBuilder 接口：构建 Region → Document → Chunk 三层知识树。
//
// 实现流程：
//  1. 从 GraphIndexer 取 Region→Document 树（通过未导出方法 regionTree）
//  2. 对每个 Document 节点，通过 semantic.(IndexerAdmin).GetChunks 从 VectorStore 读取 Chunk 列表
//  3. 将 Chunk 挂载为对应 Document 的子节点
//  4. 返回完整的 Region→Document→Chunk 树
//
// 设计要点：
//   - 仅 HyperIndexer 实现此接口——它需要同时访问 GraphStore 和 VectorStore
//   - GraphIndexer 不实现 TreeViewBuilder，仅暴露未导出的 regionTree 供 HyperIndexer 调用
//   - Chunk 不写入 GraphStore，而是在视图层通过 VectorStore.Metadata[core.VecMetaDocID] 动态组装
//
// graph 为 nil 或非 *GraphIndexer 时返回 error。
func (h *HyperIndexer) Tree(ctx context.Context, regionID string) (*core.TreeNode, error) {
	h.logger.Debug("复合索引器: 构建知识树", "region_id", regionID)
	if h.graph == nil {
		return nil, fmt.Errorf("HyperIndexer: 关系线未启用，无法构建知识树")
	}
	// 1. 从 GraphIndexer 取 Region→Document 树
	g, ok := h.graph.(*GraphIndexer)
	if !ok {
		return nil, fmt.Errorf("HyperIndexer: 关系线不是 GraphIndexer，无法获取 Region 树")
	}
	tree, err := g.regionTree(ctx, regionID)
	if err != nil {
		h.logger.Error("复合索引器: 获取 Region 树失败", err, "region_id", regionID)
		return nil, err
	}
	// 2. 为每个 Document 节点从 VectorStore 补齐 Chunk 子节点
	admin, ok := h.semantic.(IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("HyperIndexer: 语义线未实现 IndexerAdmin，无法获取 Chunk")
	}
	h.populateChunks(ctx, tree, admin)
	h.logger.Debug("复合索引器: 知识树构建完成",
		"region_id", regionID,
		"regions", len(tree.Children))
	return tree, nil
}

// populateChunks 递归为 Document 节点补齐 Chunk 子节点。
// 遇到 Document 节点时通过 admin.GetChunks 读取该文档的全部 Chunk 并挂载为子节点；
// 其他类型节点（root/region）递归处理其子节点。
func (h *HyperIndexer) populateChunks(ctx context.Context, node *core.TreeNode, admin IndexerAdmin) {
	if node == nil {
		return
	}
	if node.Type == "document" {
		chunks, err := admin.GetChunks(ctx, node.ID)
		if err != nil {
			h.logger.Warn("复合索引器: 获取文档 Chunk 失败",
				"doc_id", node.ID, "error", err.Error())
			return
		}
		for _, chunk := range chunks {
			child := &core.TreeNode{
				ID:   chunk.ID,
				Type: "chunk",
				Name: chunk.Title,
			}
			node.AddChild(child)
		}
		h.logger.Debug("复合索引器: 文档节点补充 Chunk 子节点",
			"doc_id", node.ID, "chunks", len(chunks))
	}
	for _, child := range node.Children {
		h.populateChunks(ctx, child, admin)
	}
}

// ---------------------------------------------------------------------------
// GraphSearcher 接口实现（委托 graph）
// ---------------------------------------------------------------------------

// SearchGraph 实现 GraphSearcher 接口：委托关系线。
// graph 为 nil 或未实现 GraphSearcher 时返回 error。
func (h *HyperIndexer) SearchGraph(ctx context.Context, q core.Query) (*core.Hit, error) {
	if h.graph == nil {
		return nil, fmt.Errorf("HyperIndexer: 关系线未启用，无法执行图查询")
	}
	if g, ok := h.graph.(GraphSearcher); ok {
		h.logger.Debug("复合索引器: 委托图查询", "query", q.Raw())
		return g.SearchGraph(ctx, q)
	}
	return nil, fmt.Errorf("HyperIndexer: 关系线未实现 GraphSearcher")
}
