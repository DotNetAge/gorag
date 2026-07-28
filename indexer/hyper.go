package indexer

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	chunker       chunker.Chunker               // 分块器（按 RawDoc.Type 路由，同时产出 Nodes/Edges）
	semantic      Indexer                       // 语义线（必注入）
	graph         Indexer                       // 关系线（可选，nil 则不启用图功能）
	summarizer    llm.Summarizer                // 批量摘要器（可选，nil 则不摘要）
	refiller      llm.Refiller                  // 实体提取兜底（可选，nil 则不提取）
	schemasByPath map[string][]llm.EntitySchema // Schema 注册表（key 为源目录 path）
	hooks         hooks                         // 事件扩展 Hook 聚合
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

// WithHyperSummarizer 注入摘要器，在分块后为文档类分片逐 chunk 生成 title/summary。
// 不传则不调用 Summarizer，title/summary 由 Chunker 默认策略产出。
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
// 可传入任意数量的 Hook（OnBeforeChunkHook、OnChunkedHook、OnSummarizedHook、
// OnChunkedSavedHook、OnExtractedHook、OnNodesSavedHook），WithHooks 按类型自动归入对应切片。
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
//     [1a] OnBeforeChunkHook：文件类型白名单、前置过滤
//  2. core.NewStructuredDoc(raw) → StructuredDoc 容器
//  3. 路由 Chunker → chunkerImpl.Chunk(raw) → ChunkResult
//  4. doc.SetChunks/SetNodes/SetEdges
//     [4a] OnChunkedHook：补充标签、审计分片结果
//     4.5 [语义线加工] 若注入了 Summarizer，对文档类分片逐 chunk 摘要
//     [4b] OnSummarizedHook：逐 chunk 摘要完成通知
//  5. semantic.Save(doc)（向量化 + 写入 VectorStore）
//     [5a] OnChunkedSavedHook：外部 API 增强、补充元数据
//     5.5 [关系线加工] 若注入了 Refiller + 已注册 Schema，调用 Refiller 提取实体和关系
//     [5b] OnExtractedHook：审计实体提取结果
//  6. graph.Save(doc)（实体 + CONTAINS 边 + 写入 GraphStore，若 graph 存在）
//     [6a] OnNodesSavedHook：图数据通知、审计日志
//
// 原子化管线：AddFile 内部依次完成「分块→逐 chunk 摘要→向量化存储→实体提取→图存储」，
// 每个文件独立完成全部处理。不再依赖外部分阶段编排（如 Update 中的 LLM 批量处理）。
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

	// [1a] OnBeforeChunkHook：文件类型白名单、前置过滤
	if len(h.hooks.onBeforeChunk) > 0 {
		raw, err = runOnBeforeChunkHooks(ctx, raw, h.hooks.onBeforeChunk)
		if err != nil {
			h.logger.Error("复合索引器: OnBeforeChunkHook 失败", err, "file", filePath)
			return nil, fmt.Errorf("HyperIndexer: OnBeforeChunkHook 失败: %w", err)
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

	// [4a] OnChunkedHook：完成分片，可补充标签
	if len(h.hooks.onChunked) > 0 {
		doc, err = runOnChunkedHooks(ctx, doc, h.hooks.onChunked)
		if err != nil {
			h.logger.Error("复合索引器: OnChunkedHook 失败", err, "file", filePath)
			return nil, fmt.Errorf("HyperIndexer: OnChunkedHook 失败: %w", err)
		}
		result.Chunks = doc.Chunks()
	}

	// 4b. [增量跳过] 对照已有分片，避免不必要的 LLM 调用
	chunks := doc.Chunks()
	skipRefill := false

	if admin, ok := h.semantic.(IndexerAdmin); ok {
		existing, gErr := admin.GetChunks(ctx, raw.ID())
		if gErr == nil && len(existing) == len(chunks) {
			switch diffChunks(existing, chunks) {
			case chunkDiffSkipAll:
				h.logger.Info("复合索引器: 文件无变化，跳过索引", "file", filePath)
				return existing, nil
			case chunkDiffSkipRefill:
				skipRefill = true
				// 从已有 chunks 恢复已存在的 title/summary/tags
				for i := range chunks {
					if existing[i].Summary != "" {
						chunks[i].Title = existing[i].Title
						chunks[i].Summary = existing[i].Summary
						chunks[i].Tags = existing[i].Tags
					}
				}
				h.logger.Info("复合索引器: 分片结构无变化，"+
					"跳过实体提取（仅补充缺失摘要）",
					"file", filePath)
			case chunkDiffFull:
				// 有变化，全量处理
			}
			doc.SetChunks(chunks)
		}
	}

	// 4c. [语义线加工] 逐 chunk 调用 Summarizer（仅文档类，内容≥minContentForLLM 字符）
	if h.summarizer != nil && raw.Type() == document.RawDocDoc {
		for i := range chunks {
			if utf8.RuneCountInString(chunks[i].Content) < minContentForLLM {
				continue
			}
			title, summary, sErr := h.summarizer.Summarize(ctx, chunks[i].Content)
			if sErr != nil {
				h.logger.Warn("复合索引器: Summarizer 调用失败，跳过该分片",
					"chunk_id", chunks[i].ID, "error", sErr)
				continue
			}
			if title != "" {
				chunks[i].Title = title
			}
			if summary != "" {
				chunks[i].Summary = summary
			}
			// [4c] OnSummarizedHook：逐 chunk 摘要完成
			if len(h.hooks.onSummarized) > 0 {
				runOnSummarizedHooks(ctx, &chunks[i], h.hooks.onSummarized)
			}
		}
		doc.SetChunks(chunks)
		h.logger.Info("复合索引器: Summarizer 逐 chunk 完成", "file", filePath)
	}

	// ── 5. 语义线：向量化 + 写入 VectorStore ───────────────────────
	if store, ok := h.semantic.(IndexerStore); ok {
		if err := store.Save(ctx, doc); err != nil {
			h.logger.Error("复合索引器: 语义分块保存失败", err, "file", filePath)
			return nil, fmt.Errorf("HyperIndexer: 语义分块保存失败: %w", err)
		}
		h.logger.Debug("复合索引器: 语义分块保存完成", "chunks", len(chunks))
	} else {
		h.logger.Warn("复合索引器: semantic 未实现 IndexerStore，跳过语义线保存")
	}

	// [5a] OnChunkedSavedHook：分片已持久化
	if len(h.hooks.onChunkedSaved) > 0 {
		if err := runOnChunkedSavedHooks(ctx, chunks, h.hooks.onChunkedSaved); err != nil {
			h.logger.Warn("复合索引器: OnChunkedSavedHook 失败（不阻塞）", "error", err)
		}
	}

	// ── 6. Refiller 实体提取（skipRefill 时跳过）───────────────────
	if !skipRefill && h.refiller != nil && len(h.schemasByPath) > 0 {
		var schemas []llm.EntitySchema
		for _, ss := range h.schemasByPath {
			schemas = append(schemas, ss...)
		}
		if len(schemas) > 0 {
			// 仅对内容长度 ≥ minContentForLLM 的分片进行实体提取（短内容无足够语义）
			var refillChunks []core.Chunk
			for _, c := range chunks {
				if utf8.RuneCountInString(c.Content) >= minContentForLLM {
					refillChunks = append(refillChunks, c)
				}
			}
			if len(refillChunks) == 0 {
				h.logger.Debug("复合索引器: 所有分片内容过短，跳过实体提取",
					"file", filePath)
			} else {
				refillInput := chunker.ChunkResult{Chunks: refillChunks}
				refilled, rErr := h.refiller.Refill(ctx, refillInput, schemas)
				if rErr != nil {
					h.logger.Warn("复合索引器: Refiller 调用失败（不阻塞语义线）",
						"file", filePath, "error", rErr.Error())
				} else {
					doc.SetNodes(append(doc.Nodes(), refilled.Nodes...))
					doc.SetEdges(append(doc.Edges(), refilled.Edges...))
					h.logger.Info("复合索引器: Refiller 完成",
						"实体数", len(refilled.Nodes),
						"关系数", len(refilled.Edges))

					// [6a] OnExtractedHook：实体提取完成
					if len(h.hooks.onExtracted) > 0 {
						runOnExtractedHooks(ctx, chunks, refilled.Nodes, refilled.Edges, h.hooks.onExtracted)
					}
				}
			}
		}
	}

	// ── 7. 关系线：写入 GraphStore ─────────────────────────────────
	if h.graph != nil {
		if store, ok := h.graph.(IndexerStore); ok {
			if err := store.Save(ctx, doc); err != nil {
				h.logger.Warn("复合索引器: 关系线保存失败（不阻塞语义线）",
					"file", filePath, "error", err.Error())
			} else {
				h.logger.Info("复合索引器: 关系线保存完成",
					"实体数", len(doc.Nodes()),
					"关系数", len(doc.Edges()))

				// [7a] OnNodesSavedHook：节点已持久化
				if len(h.hooks.onNodesSaved) > 0 {
					runOnNodesSavedHooks(ctx, doc.Nodes(), h.hooks.onNodesSaved)
				}
			}
		} else {
			h.logger.Warn("复合索引器: graph 未实现 IndexerStore，跳过关系线保存")
		}
	} else {
		h.logger.Debug("复合索引器: 未启用关系线，跳过图保存")
	}

	// ── 8. 返回结果 ────────────────────────────────────────────────
	resultChunks := make([]*core.Chunk, 0, len(chunks))
	for i := range chunks {
		c := chunks[i]
		resultChunks = append(resultChunks, &c)
	}

	h.logger.Info("复合索引器: 索引文件完成",
		"file", filePath, "chunks", len(resultChunks))
	return resultChunks, nil
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

	// 4. Tag/Title/Summary 关键词重排序增强
	// 在语义召回的基础上，对 Tags、Title、Summary 匹配查询关键词的分片给予分数加成，
	// 提升相关性高的结果排序，同时不改变召回集合（避免过度精确导致结果过少）。
	boostByKeywords(fused, q)

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
		h.removeGraphData(ctx, chunkID)
	}
	h.logger.Debug("复合索引器: 删除分片完成", "chunk_id", chunkID)
	return nil
}

// removeGraphData 按 chunkID 删除 GraphStore 中关联的 Nodes 和 Edges。
func (h *HyperIndexer) removeGraphData(ctx context.Context, chunkID string) {
	// 优先通过 IndexerAdmin 接口删除
	if a, ok := h.graph.(IndexerAdmin); ok {
		if err := a.Remove(ctx, chunkID); err != nil {
			h.logger.Warn("复合索引器: 关系线删除失败（不阻塞）",
				"chunk_id", chunkID, "error", err.Error())
		}
		return
	}

	// 回退：直接操作 GraphStore（GraphIndexer 不实现 IndexerAdmin）
	gi, ok := h.graph.(*GraphIndexer)
	if !ok {
		h.logger.Debug("复合索引器: graph 不支持删除，跳过", "chunk_id", chunkID)
		return
	}
	gdb := gi.GraphDB()
	if gdb == nil {
		return
	}

	nodes, edges, err := gdb.GetByChunkIDs(ctx, []string{chunkID})
	if err != nil {
		h.logger.Warn("复合索引器: 查询 chunk 关联的图数据失败", "chunk_id", chunkID, "error", err)
		return
	}

	// 先删除边，再删除节点
	for _, e := range edges {
		if dErr := gdb.DeleteEdge(ctx, e.ID); dErr != nil {
			h.logger.Warn("复合索引器: 删除图边失败", "edge_id", e.ID, "error", dErr)
		}
	}
	for _, n := range nodes {
		if dErr := gdb.DeleteNode(ctx, n.ID); dErr != nil {
			h.logger.Warn("复合索引器: 删除图节点失败", "node_id", n.ID, "error", dErr)
		}
	}
	h.logger.Debug("复合索引器: 图数据清理完成",
		"chunk_id", chunkID,
		"节点数", len(nodes),
		"边数", len(edges))
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

// Neighbors 实现 GraphNavigator 接口：委托关系线执行多跳邻居遍历。
// 若关系线未启用或未实现 GraphNavigator，返回错误。
func (h *HyperIndexer) Neighbors(ctx context.Context, nodeID string, depth, limit int) ([]*core.Node, []*core.Edge, error) {
	if h.graph == nil {
		return nil, nil, fmt.Errorf("HyperIndexer: 关系线未启用，无法执行图导航")
	}
	if g, ok := h.graph.(GraphNavigator); ok {
		h.logger.Debug("复合索引器: 委托图导航", "nodeID", nodeID, "depth", depth)
		return g.Neighbors(ctx, nodeID, depth, limit)
	}
	return nil, nil, fmt.Errorf("HyperIndexer: 关系线未实现 GraphNavigator")
}

// GetNode 实现 GraphNavigator 接口：委托关系线按 ID 获取单个节点。
func (h *HyperIndexer) GetNode(ctx context.Context, nodeID string) (*core.Node, error) {
	if h.graph == nil {
		return nil, fmt.Errorf("HyperIndexer: 关系线未启用，无法执行图导航")
	}
	if g, ok := h.graph.(GraphNavigator); ok {
		h.logger.Debug("复合索引器: 委托获取节点", "nodeID", nodeID)
		return g.GetNode(ctx, nodeID)
	}
	return nil, fmt.Errorf("HyperIndexer: 关系线未实现 GraphNavigator")
}

// ExploreRegion 实现 GraphExplorer 接口：委托关系线执行目录级图探索。
func (h *HyperIndexer) ExploreRegion(ctx context.Context, dir string, depth, limit int) (*RegionGraphView, error) {
	if h.graph == nil {
		return nil, fmt.Errorf("HyperIndexer: 关系线未启用，无法执行图探索")
	}
	if g, ok := h.graph.(GraphExplorer); ok {
		h.logger.Debug("复合索引器: 委托目录级图探索", "dir", dir, "depth", depth)
		return g.ExploreRegion(ctx, dir, depth, limit)
	}
	return nil, fmt.Errorf("HyperIndexer: 关系线未实现 GraphExplorer")
}

// ExploreFile 实现 FileExplorer 接口：委托关系线执行文件级图探索。
func (h *HyperIndexer) ExploreFile(ctx context.Context, filePath string, depth, limit int) (*RegionGraphView, error) {
	if h.graph == nil {
		return nil, fmt.Errorf("HyperIndexer: 关系线未启用，无法执行文件级图探索")
	}
	if f, ok := h.graph.(FileExplorer); ok {
		h.logger.Debug("复合索引器: 委托文件级图探索", "file", filePath, "depth", depth)
		return f.ExploreFile(ctx, filePath, depth, limit)
	}
	return nil, fmt.Errorf("HyperIndexer: 关系线未实现 FileExplorer")
}

// CypherQuery 执行原始 Cypher 查询，委托给关系线的 GraphIndexer。
// 仅当索引器支持图存储时可用。
func (h *HyperIndexer) CypherQuery(ctx context.Context, q string, params map[string]any) ([]map[string]any, error) {
	if h.graph == nil {
		return nil, fmt.Errorf("HyperIndexer: 关系线未启用，无法执行 Cypher 查询")
	}
	if g, ok := h.graph.(interface {
		CypherQuery(ctx context.Context, q string, params map[string]any) ([]map[string]any, error)
	}); ok {
		h.logger.Debug("复合索引器: 委托 Cypher 查询", "query", q)
		return g.CypherQuery(ctx, q, params)
	}
	return nil, fmt.Errorf("HyperIndexer: 关系线不支持 Cypher 查询")
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
// 查询结果重排序
// ---------------------------------------------------------------------------

// keywordBoostBase 是单条关键词命中的基础加成比例。
const keywordBoostBase = 0.15

// keywordBoostMaxMatches 是参与计算的最大命中关键词数。
const keywordBoostMaxMatches = 3

// keywordBoostMaxRatio 是最大累计加成比例（1.0 表示最多提升 100%）。
const keywordBoostMaxRatio = 1.0

// boostByKeywords 对 Hit 中的 Chunk 按查询关键词匹配度进行分数增强。
//
// 匹配区域包括：Tags（完全匹配）、Title（子串）、Summary（子串）。
// 每命中一个关键词，分数乘以 (1 + keywordBoostBase)；命中上限 keywordBoostMaxMatches。
// 该方法只调整排序和分数，不剔除任何召回结果，避免过度精确导致召回不足。
func boostByKeywords(hit *core.Hit, q core.Query) {
	if hit == nil || q == nil || len(hit.Chunks) == 0 {
		return
	}

	keywords := q.Keywords()
	if len(keywords) == 0 {
		return
	}

	for i := range hit.Chunks {
		matchCount := countKeywordMatches(&hit.Chunks[i], keywords)
		if matchCount > 0 {
			if matchCount > keywordBoostMaxMatches {
				matchCount = keywordBoostMaxMatches
			}
			ratio := float32(matchCount) * keywordBoostBase
			if ratio > keywordBoostMaxRatio {
				ratio = keywordBoostMaxRatio
			}
			hit.Chunks[i].Score *= (1 + ratio)
		}
	}

	// 按增强后的分数重新排序
	sort.Slice(hit.Chunks, func(i, j int) bool {
		return hit.Chunks[i].Score > hit.Chunks[j].Score
	})

	// 保持融合后的综合分数语义不变，不因 chunk 重排而覆盖原 Score
}

// countKeywordMatches 统计查询关键词在 Chunk Tags/Title/Summary 中的命中数。
// Tags 要求完全匹配（忽略大小写），Title/Summary 使用子串匹配（忽略大小写）。
func countKeywordMatches(ch *core.ChunkHit, keywords []string) int {
	if ch == nil || ch.Chunk == nil {
		return 0
	}
	count := 0
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		lowerKw := strings.ToLower(kw)
		if keywordInTags(ch.Tags, lowerKw) ||
			strings.Contains(strings.ToLower(ch.Title), lowerKw) ||
			strings.Contains(strings.ToLower(ch.Summary), lowerKw) {
			count++
		}
	}
	return count
}

// keywordInTags 判断标签列表中是否存在与关键词完全匹配（忽略大小写）的项。
func keywordInTags(tags []string, lowerKeyword string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, lowerKeyword) {
			return true
		}
	}
	return false
}

// minContentForLLM 是触发 LLM 处理（摘要/实体提取）的最小内容长度（按字符数）。
// 短于此长度的分块：
//   - 没有摘要化的必要（标题已起到摘要作用）
//   - 无足够的语义信息提取有意义的实体
const minContentForLLM = 200

// chunkDiffResult 新旧分片对照结果
type chunkDiffResult int

const (
	chunkDiffFull       chunkDiffResult = iota // 分片有变化 → 全量处理（摘要 + 实体提取）
	chunkDiffSkipRefill                        // 分片未变但摘要不全 → 补摘要，跳过实体提取
	chunkDiffSkipAll                           // 分片未变 + 摘要完整 → 完全跳过 LLM 处理
)

// diffChunks 对比新旧 chunks，4 维度（StartPos、EndPos、Content、Title）精准对照。
// 仅当四个维度全部匹配时视为「未变化」，否则触发全量重做。
func diffChunks(existing []*core.Chunk, newChunks []core.Chunk) chunkDiffResult {
	if len(existing) != len(newChunks) {
		return chunkDiffFull
	}
	for i := range newChunks {
		if existing[i].StartPos != newChunks[i].StartPos ||
			existing[i].EndPos != newChunks[i].EndPos ||
			existing[i].Content != newChunks[i].Content ||
			existing[i].Title != newChunks[i].Title {
			return chunkDiffFull
		}
	}
	// 结构完全相同，检查 Summary 是否都已填充
	for i := range newChunks {
		if newChunks[i].Summary == "" {
			return chunkDiffSkipRefill
		}
	}
	return chunkDiffSkipAll
}
