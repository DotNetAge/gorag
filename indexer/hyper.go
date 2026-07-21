package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"unicode/utf8"

	"github.com/DotNetAge/gorag/v2/chunker"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	"github.com/DotNetAge/gorag/v2/llm"
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
	chunker       chunker.Chunker           // 分块器（按 RawDoc.Type 路由，同时产出 Nodes/Edges）
	semantic      Indexer                   // 语义线（必注入）
	graph         Indexer                   // 关系线（可选，nil 则不启用图功能）
	summarizer    llm.Summarizer            // 批量摘要器（可选，nil 则不摘要）
	refiller      llm.Refiller              // 实体提取兜底（可选，nil 则不提取）
	schemasByPath map[string][]llm.EntitySchema // Schema 注册表（key 为源目录 path）
	hooks         hooks                     // 事件扩展 Hook 聚合
	logger        logging.Logger
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

// WithHyperSummarizer 注入批量摘要器，在分块后为文档类分片批量生成 title/summary。
// 不传则不调用 Summarizer，title/summary 由 Chunker 默认策略产出。
// 若注入的 llm.Summarizer 同时实现了 SummarizeBatch 方法，优先使用批量模式；
// 否则回退到逐分片 Summarize 模式。
func WithHyperSummarizer(s llm.Summarizer) HyperOption {
	return func(h *HyperIndexer) {
		if s != nil {
			h.summarizer = s
		}
	}
}

// WithHyperRefiller 注入实体提取兜底，在语义线保存后对分片执行 LLM 实体提取。
// 提取结果追加到 doc.Nodes/Edges，再写入 GraphStore。
// 不传则不执行实体提取，关系线只走 Chunker 代码解析器产出的 Nodes/Edges。
func WithHyperRefiller(r llm.Refiller) HyperOption {
	return func(h *HyperIndexer) {
		if r != nil {
			h.refiller = r
		}
	}
}

// WithHooks 注入事件扩展 Hook。
//
// 可传入任意数量的 Hook（OnFileOpenedHook、OnChunkHook、OnBeforeSemanticSaveHook、
// OnIndexCompleteHook），WithHooks 按类型自动归入对应切片。
// 单一类型可注册多个 Hook，按注册顺序执行。
func WithHooks(hooksList ...any) HyperOption {
	return func(h *HyperIndexer) {
		for _, hook := range hooksList {
			h.hooks.register(hook)
		}
	}
}

// NewHyperIndexer 创建复合索引器，返回 Indexer 接口。
//
// 参数：
//   - semantic: 语义线索引器（必传，通常由 NewSemanticIndexer 创建）
//   - graph:    关系线索引器（可选，传 nil 则不启用图功能；通常由 New（GraphIndexer）创建）
//   - opts:     可选配置（WithHyperLogger、WithHyperChunker、WithHyperSummarizer、WithHyperRefiller）
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
		semantic:      semantic,
		graph:         graph,
		schemasByPath: make(map[string][]llm.EntitySchema),
		logger:        logging.DefaultNoopLogger(),
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
// 工作流（完整）：
//  1. document.Open(filePath) → RawDoc
//     [1a] OnFileOpenedHook：文件类型白名单、前置过滤
//  2. core.NewStructuredDoc(raw) → StructuredDoc 容器
//  3. 路由 Chunker → chunkerImpl.Chunk(raw) → ChunkResult
//  4. doc.SetChunks/SetNodes/SetEdges
//     [4a] OnChunkHook：对每个 Chunk 执行敏感词过滤、补充标签
//  4.5 [语义线加工] 若注入了 Summarizer，对文档类分片批量摘要
//     [4b] OnBeforeSemanticSaveHook：批量审核、外部 API 增强
//  5. semantic.Save(doc)（向量化 + 写入 VectorStore）
//  5.5 [关系线加工] 若注入了 Refiller + 已注册 Schema，调用 Refiller 提取实体和关系
//  6. graph.Save(doc)（实体 + CONTAINS 边 + 写入 GraphStore，若 graph 存在）
//     [6a] OnIndexCompleteHook：通知下游、审计日志
//
// 返回本次索引生成的 Chunks（用于调用方追踪）。
// 关系线失败不阻塞语义线，仅记录警告。
func (h *HyperIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error) {
	if filePath == "" {
		return nil, fmt.Errorf("HyperIndexer: 文件路径不能为空")
	}
	h.logger.Info("复合索引器: 开始索引文件", "file", filePath)

	// 1. 读文件 + 归一化
	raw, err := document.Open(filePath)
	if err != nil {
		h.logger.Error("复合索引器: 打开文件失败", err, "file", filePath)
		return nil, fmt.Errorf("HyperIndexer: 打开文件 %s 失败: %w", filePath, err)
	}
	h.logger.Debug("复合索引器: 文件归一化完成",
		"file", filePath, "doc_type", raw.Type(), "doc_id", raw.ID())

	// [1a] OnFileOpenedHook：文件类型白名单、前置过滤
	if len(h.hooks.onFileOpened) > 0 {
		var hookErr error
		raw, hookErr = runOnFileOpenedHooks(ctx, raw, h.hooks.onFileOpened)
		if hookErr != nil {
			h.logger.Error("复合索引器: OnFileOpenedHook 失败", hookErr, "file", filePath)
			return nil, fmt.Errorf("HyperIndexer: OnFileOpenedHook 失败: %w", hookErr)
		}
	}

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
	h.logger.Info("复合索引器: 分块完成",
		"file", filePath,
		"chunks", len(result.Chunks),
		"nodes", len(result.Nodes),
		"edges", len(result.Edges))

	// 4a. 把三类产物分别存入 StructuredDoc（供 Save 读取）
	doc.SetChunks(result.Chunks)
	doc.SetNodes(result.Nodes)
	doc.SetEdges(result.Edges)

	// [4a] OnChunkHook：对每个 Chunk 执行敏感词过滤、补充标签
	if len(h.hooks.onChunk) > 0 {
		for i := range result.Chunks {
			modified, hookErr := runOnChunkHooks(ctx, &result.Chunks[i], h.hooks.onChunk)
			if hookErr != nil {
				h.logger.Error("复合索引器: OnChunkHook 失败", hookErr,
					"chunk_id", result.Chunks[i].ID)
				return nil, fmt.Errorf("HyperIndexer: OnChunkHook 失败: %w", hookErr)
			}
			if modified != nil {
				result.Chunks[i] = *modified
			}
		}
		doc.SetChunks(result.Chunks)
	}

	// 4b. [语义线加工] 若注入了 Summarizer，对文档类分片批量摘要
	if h.summarizer != nil && raw.Type() == document.RawDocDoc {
		var toSummarize []core.Chunk
		for _, c := range result.Chunks {
			if utf8.RuneCountInString(c.Content) >= minSummaryContentLength {
				toSummarize = append(toSummarize, c)
			}
		}
		if len(toSummarize) > 0 {
			h.logger.Info("复合索引器: 调用 Summarizer", "分片数", len(toSummarize))
			// 优先使用批量模式（通过接口断言检测 SummarizeBatch 方法）
			var updated []core.Chunk
			var err error
			if bs, ok := h.summarizer.(interface {
				SummarizeBatch(context.Context, []core.Chunk) ([]core.Chunk, error)
			}); ok {
				updated, err = bs.SummarizeBatch(ctx, toSummarize)
			} else {
				updated, err = h.summarizer.Summarize(ctx, toSummarize)
			}
			if err != nil {
				h.logger.Warn("复合索引器: Summarizer 调用失败，使用原始分片继续", "error", err)
			} else {
				updatedByID := make(map[string]core.Chunk, len(updated))
				for _, u := range updated {
					updatedByID[u.ID] = u
				}
				chunks := doc.Chunks()
				for i := range chunks {
					if u, ok := updatedByID[chunks[i].ID]; ok {
						chunks[i] = u
					}
				}
				doc.SetChunks(chunks)
				h.logger.Info("复合索引器: Summarizer 完成", "已摘要", len(updated))
			}
		} else {
			h.logger.Info("复合索引器: 所有分片内容过短，跳过 Summarizer",
				"最短长度", minSummaryContentLength)
		}
	}

	// [4b] OnBeforeSemanticSaveHook：批量审核、外部 API 增强
	if len(h.hooks.onBeforeSemantic) > 0 {
		var hookErr error
		doc, hookErr = runOnBeforeSemanticSaveHooks(ctx, doc, h.hooks.onBeforeSemantic)
		if hookErr != nil {
			h.logger.Error("复合索引器: OnBeforeSemanticSaveHook 失败", hookErr, "file", filePath)
			return nil, fmt.Errorf("HyperIndexer: OnBeforeSemanticSaveHook 失败: %w", hookErr)
		}
	}

	// 5. 语义线：向量化 + 写入 VectorStore（失败返回 error）
	if store, ok := h.semantic.(IndexerStore); ok {
		if err := store.Save(ctx, doc); err != nil {
			h.logger.Error("复合索引器: 语义线保存失败", err, "file", filePath)
			return nil, fmt.Errorf("HyperIndexer: 语义线保存失败: %w", err)
		}
		h.logger.Debug("复合索引器: 语义线保存完成", "chunks", len(result.Chunks))
	} else {
		h.logger.Warn("复合索引器: semantic 未实现 IndexerStore，跳过语义线保存")
	}

	// 5b. [关系线加工] 若注入了 Refiller 且有已注册 Schema，调用 Refiller 提取实体和关系
	if h.refiller != nil && len(h.schemasByPath) > 0 {
		// 将注册表中所有 Schema 合并为一个列表传给 Refiller
		var schemas []llm.EntitySchema
		for _, ss := range h.schemasByPath {
			schemas = append(schemas, ss...)
		}
		if len(schemas) > 0 {
			refilled, rErr := h.refiller.Refill(ctx, result, schemas)
			if rErr != nil {
				h.logger.Warn("复合索引器: Refiller 调用失败（不阻塞语义线）",
					"file", filePath, "error", rErr.Error())
			} else {
				// 将新增的 Nodes/Edges 合并到 doc
				doc.SetNodes(append(doc.Nodes(), refilled.Nodes...))
				doc.SetEdges(append(doc.Edges(), refilled.Edges...))
				h.logger.Info("复合索引器: Refiller 完成",
					"实体数", len(refilled.Nodes),
					"关系数", len(refilled.Edges))
			}
		}
	}

	// 6. 关系线：实体 + CONTAINS 边 + 写入 GraphStore（若 graph 存在）
	//    关系线失败不阻塞语义线结果，仅记录警告
	if h.graph != nil {
		if store, ok := h.graph.(IndexerStore); ok {
			if err := store.Save(ctx, doc); err != nil {
				h.logger.Warn("复合索引器: 关系线保存失败（不阻塞语义线）",
					"file", filePath, "error", err.Error())
			} else {
				h.logger.Info("复合索引器: 关系线保存完成",
					"实体数", len(doc.Nodes()),
					"关系数", len(doc.Edges()))
			}
		} else {
			h.logger.Warn("复合索引器: graph 未实现 IndexerStore，跳过关系线保存")
		}
	} else {
		h.logger.Debug("复合索引器: 未启用关系线，跳过图保存")
	}

	// 7. 转换 []core.Chunk → []*core.Chunk 返回（保持 AddFile 签名一致）
	chunks := make([]*core.Chunk, 0, len(result.Chunks))
	for i := range result.Chunks {
		ch := result.Chunks[i]
		chunks = append(chunks, &ch)
	}

	// [6a] OnIndexCompleteHook：通知下游、审计日志（不阻塞管线）
	if len(h.hooks.onIndexComplete) > 0 {
		runOnIndexCompleteHooks(ctx, chunks, h.hooks.onIndexComplete, h.logger)
	}

	h.logger.Info("复合索引器: 索引文件完成",
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

// ---------------------------------------------------------------------------
// Schema 注册
// ---------------------------------------------------------------------------

// AddSchemas 注册外部实体 Schema 到 HyperIndexer。
//
// path 是文件系统路径，指向该组 Schema 的源目录（如 "schemas/general"），
// 保留溯源语义——调用方可追踪这些 Schema 来自哪个配置文件目录。
// schemas 使用 llm/schema.go 中已定义的 EntitySchema 类型，
// 可经由 llm.LoadEntitySchemasFromDir 或自定义加载函数解析后传入。
//
// path 不用于文件读取（schemas 已经是解析好的），仅作为注册标识。
// HyperIndexer 将 path 作为 key 存储在内部注册表中，供 Refiller 消费时合并。
func (h *HyperIndexer) AddSchemas(path string, schemas []llm.EntitySchema) {
	if path == "" || len(schemas) == 0 {
		return
	}
	if h.schemasByPath == nil {
		h.schemasByPath = make(map[string][]llm.EntitySchema)
	}
	h.schemasByPath[path] = schemas
	h.logger.Debug("复合索引器: Schema 注册完成", "path", path, "count", len(schemas))
}

// SetSummarizer 在运行时注入或替换 Summarizer。
// 若传入 nil，则禁用 Summarizer 功能。
func (h *HyperIndexer) SetSummarizer(s llm.Summarizer) {
	h.summarizer = s
	if s != nil {
		h.logger.Info("复合索引器: Summarizer 已注入")
	} else {
		h.logger.Info("复合索引器: Summarizer 已移除")
	}
}

// SetRefiller 在运行时注入或替换 Refiller。
// 若传入 nil，则禁用 Refiller 功能。
func (h *HyperIndexer) SetRefiller(r llm.Refiller) {
	h.refiller = r
	if r != nil {
		h.logger.Info("复合索引器: Refiller 已注入")
	} else {
		h.logger.Info("复合索引器: Refiller 已移除")
	}
}

// SetLogger 在运行时设置日志记录器。
func (h *HyperIndexer) SetLogger(logger logging.Logger) {
	if logger != nil {
		h.logger = logger
	}
}

// ---------------------------------------------------------------------------
// 多文件实体关系发现
// ---------------------------------------------------------------------------

// ProcessChunks 对指定分片执行增量 LLM 处理（摘要 + 实体提取）。
//
// 流程：
//  1. [摘要] 若注入了 Summarizer，对所有内容足够长的分片调用 Summarizer
//  2. [向量更新] 摘要后的分片重新向量化并写入 VectorStore
//  3. [实体提取] 若注入了 Refiller + 已注册 Schema，对所有分片调用 Refiller
//  4. [图更新] 新提取的 Nodes/Edges 写入 GraphStore
//
// 参数：
//   - ctx: 上下文
//   - chunks: 需要处理的分片（必须已从 VectorStore 或其他来源加载完整数据）
//
// 返回值：
//   - processedChunks: 处理后的分片列表（Summary/Title 已更新）
//   - addedNodes: 新提取的实体节点
//   - addedEdges: 新提取的关系边
//   - error: 整体错误（单个阶段失败不阻塞后续阶段，仅记录警告）
//
// 典型调用方：IndexingService.Update，从 meta.db 查询需要 LLM 处理的分片，
// 从 VectorStore 加载完整数据后传给此方法。
func (h *HyperIndexer) ProcessChunks(ctx context.Context, chunks []core.Chunk) (processedChunks []core.Chunk, addedNodes []core.Node, addedEdges []core.Edge, err error) {
	h.logger.Info("复合索引器: 开始增量 LLM 处理", "chunks", len(chunks))
	if len(chunks) == 0 {
		return chunks, nil, nil, nil
	}

	// 1. [摘要] Summarizer 阶段
	if h.summarizer != nil {
		var toSummarize []core.Chunk
		for _, c := range chunks {
			if utf8.RuneCountInString(c.Content) >= minSummaryContentLength {
				toSummarize = append(toSummarize, c)
			}
		}
		if len(toSummarize) > 0 {
			h.logger.Info("复合索引器: ProcessChunks 调用 Summarizer", "分片数", len(toSummarize))
			var result []core.Chunk
			var sErr error
			if bs, ok := h.summarizer.(interface {
				SummarizeBatch(context.Context, []core.Chunk) ([]core.Chunk, error)
			}); ok {
				result, sErr = bs.SummarizeBatch(ctx, toSummarize)
			} else {
				result, sErr = h.summarizer.Summarize(ctx, toSummarize)
			}
			if sErr != nil {
				h.logger.Warn("复合索引器: ProcessChunks Summarizer 失败", "error", sErr)
			} else {
				// 按 ID 回写到 processedChunks
				updatedByID := make(map[string]core.Chunk, len(result))
				for _, u := range result {
					updatedByID[u.ID] = u
				}
				processedChunks = make([]core.Chunk, len(chunks))
				for i, c := range chunks {
					if u, ok := updatedByID[c.ID]; ok {
						processedChunks[i] = u
					} else {
						processedChunks[i] = c
					}
				}
				h.logger.Info("复合索引器: ProcessChunks Summarizer 完成", "已摘要", len(result))
			}
		}
	}

	// 2. [向量更新] 将摘要后的分片重新向量化
	if len(processedChunks) > 0 {
		if si, ok := h.semantic.(*semanticIndexer); ok {
			for i := range processedChunks {
				if err := si.saveOneChunk(ctx, &processedChunks[i]); err != nil {
					h.logger.Warn("复合索引器: 更新分片向量失败",
						"chunk_id", processedChunks[i].ID, "error", err)
				}
			}
			h.logger.Info("复合索引器: 向量更新完成", "chunks", len(processedChunks))
		} else {
			h.logger.Warn("复合索引器: semantic 不是 *semanticIndexer，无法更新向量")
		}
	} else if len(chunks) > 0 {
		// 没有经过摘要处理，直接使用原始分片
		processedChunks = make([]core.Chunk, len(chunks))
		copy(processedChunks, chunks)
	}

	// 3. [实体提取] Refiller 阶段
	if h.refiller != nil && len(h.schemasByPath) > 0 {
		var schemas []llm.EntitySchema
		for _, ss := range h.schemasByPath {
			schemas = append(schemas, ss...)
		}
		if len(schemas) > 0 {
			result := chunker.ChunkResult{Chunks: chunks}
			refilled, rErr := h.refiller.Refill(ctx, result, schemas)
			if rErr != nil {
				h.logger.Warn("复合索引器: ProcessChunks Refiller 失败", "error", rErr)
			} else {
				addedNodes = refilled.Nodes
				addedEdges = refilled.Edges
				h.logger.Info("复合索引器: ProcessChunks Refiller 完成",
					"实体数", len(addedNodes), "关系数", len(addedEdges))
			}
		}
	}

	// 4. [图更新] 将新实体/关系写入 GraphStore
	if h.graph != nil && (len(addedNodes) > 0 || len(addedEdges) > 0) {
		doc := core.NewStructuredDocFromParts(addedNodes, addedEdges)
		if store, ok := h.graph.(IndexerStore); ok {
			if gErr := store.Save(ctx, doc); gErr != nil {
				h.logger.Warn("复合索引器: ProcessChunks 图保存失败", "error", gErr)
			} else {
				h.logger.Info("复合索引器: ProcessChunks 图更新完成",
					"实体数", len(addedNodes), "关系数", len(addedEdges))
			}
		}
	}

	h.logger.Info("复合索引器: 增量 LLM 处理完成",
		"总分片", len(chunks),
		"已摘要", len(processedChunks),
		"实体数", len(addedNodes),
		"关系数", len(addedEdges))
	return
}

// ---------------------------------------------------------------------------
// 内部方法
// ---------------------------------------------------------------------------

// resolveRegionID 从文件路径推导 RegionID。
//
// RegionID 使用目录路径的 SHA256 哈希的前 16 位十六进制字符作为标识，
// 同时横跨 Chunk（语义线）和 Node（关系线）。
// 此方法通常在 AddFile 过程中由内部逻辑调用。
func (h *HyperIndexer) resolveRegionID(filePath string) string {
	dir := filepath.Dir(filePath)
	sum := sha256.Sum256([]byte(dir))
	return hex.EncodeToString(sum[:])[:16]
}

// minSummaryContentLength 是触发 Summarizer 的最小内容长度（按字符数）。
// 短于此长度的分块没有摘要化的必要，直接跳过以节省 LLM 调用。
const minSummaryContentLength = 100
