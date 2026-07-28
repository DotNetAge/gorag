package indexer

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
)

// =====================================================================
// 事件 Hook 接口定义
// =====================================================================
//
// 所有 Hook 按触发时机命名，分为修改型（返回 error 阻塞管线）
// 和通知型（仅记录日志，不阻塞管线）。

// OnBeforeChunkHook 修改型 Hook。
// 在 document.Open 之后、Chunker.Chunk 之前触发，可修改 RawDoc。
// 应用场景：文件类型白名单、前置过滤。
type OnBeforeChunkHook interface {
	OnBeforeChunk(ctx context.Context, raw document.RawDoc) (document.RawDoc, error)
}

// OnChunkedHook 修改型 Hook。
// 在 Chunker 完成分片后、Summarizer 之前触发，可修改 StructuredDoc。
// 应用场景：补充标签、审计分片结果。
type OnChunkedHook interface {
	OnChunked(ctx context.Context, doc core.StructuredDoc) (core.StructuredDoc, error)
}

// OnSummarizedHook 通知型 Hook。
// 在每个 chunk 完成 LLM 摘要后触发，不阻塞管线。
// 应用场景：逐 chunk 进度通知。
type OnSummarizedHook interface {
	OnSummarized(ctx context.Context, chunk *core.Chunk) error
}

// OnChunkedSavedHook 修改型 Hook。
// 在 semantic.Save（向量化+存储）完成后、Refiller 之前触发。
// 应用场景：外部 API 增强、补充元数据。
type OnChunkedSavedHook interface {
	OnChunkedSaved(ctx context.Context, chunks []core.Chunk) error
}

// OnExtractedHook 通知型 Hook。
// 在 Refiller 完成实体提取后、graph.Save 之前触发，不阻塞管线。
// 应用场景：审计实体提取结果。
type OnExtractedHook interface {
	OnExtracted(ctx context.Context, chunks []core.Chunk, nodes []core.Node, edges []core.Edge) error
}

// OnNodesSavedHook 通知型 Hook。
// 在 graph.Save（图存储）完成后触发，不阻塞管线。
// 应用场景：图数据通知、审计日志。
type OnNodesSavedHook interface {
	OnNodesSaved(ctx context.Context, nodes []core.Node) error
}

// ---------------------------------------------------------------------------
// 简化注册：WithHooks 接受任意数量的 Hook，按类型自动分类
// ---------------------------------------------------------------------------

// hooks 聚合 HyperIndexer 的所有 Hook 切片。
type hooks struct {
	onBeforeChunk  []OnBeforeChunkHook
	onChunked      []OnChunkedHook
	onSummarized   []OnSummarizedHook
	onChunkedSaved []OnChunkedSavedHook
	onExtracted    []OnExtractedHook
	onNodesSaved   []OnNodesSavedHook
}

// register 按接口类型将 hook 归入对应切片。
func (h *hooks) register(hook any) {
	switch v := hook.(type) {
	case OnBeforeChunkHook:
		h.onBeforeChunk = append(h.onBeforeChunk, v)
	case OnChunkedHook:
		h.onChunked = append(h.onChunked, v)
	case OnSummarizedHook:
		h.onSummarized = append(h.onSummarized, v)
	case OnChunkedSavedHook:
		h.onChunkedSaved = append(h.onChunkedSaved, v)
	case OnExtractedHook:
		h.onExtracted = append(h.onExtracted, v)
	case OnNodesSavedHook:
		h.onNodesSaved = append(h.onNodesSaved, v)
	}
}

// ---------------------------------------------------------------------------
// Hook 执行辅助方法（新版）
// ---------------------------------------------------------------------------

func runOnBeforeChunkHooks(ctx context.Context, raw document.RawDoc, hs []OnBeforeChunkHook) (document.RawDoc, error) {
	var err error
	for _, h := range hs {
		raw, err = h.OnBeforeChunk(ctx, raw)
		if err != nil {
			return raw, fmt.Errorf("Hook OnBeforeChunk 失败: %w", err)
		}
	}
	return raw, nil
}

func runOnChunkedHooks(ctx context.Context, doc core.StructuredDoc, hs []OnChunkedHook) (core.StructuredDoc, error) {
	var err error
	for _, h := range hs {
		doc, err = h.OnChunked(ctx, doc)
		if err != nil {
			return doc, fmt.Errorf("Hook OnChunked 失败: %w", err)
		}
	}
	return doc, nil
}

func runOnSummarizedHooks(ctx context.Context, chunk *core.Chunk, hs []OnSummarizedHook) {
	for _, h := range hs {
		_ = h.OnSummarized(ctx, chunk)
	}
}

func runOnChunkedSavedHooks(ctx context.Context, chunks []core.Chunk, hs []OnChunkedSavedHook) error {
	for _, h := range hs {
		if err := h.OnChunkedSaved(ctx, chunks); err != nil {
			return fmt.Errorf("Hook OnChunkedSaved 失败: %w", err)
		}
	}
	return nil
}

func runOnExtractedHooks(ctx context.Context, chunks []core.Chunk, nodes []core.Node, edges []core.Edge, hs []OnExtractedHook) {
	for _, h := range hs {
		_ = h.OnExtracted(ctx, chunks, nodes, edges)
	}
}

func runOnNodesSavedHooks(ctx context.Context, nodes []core.Node, hs []OnNodesSavedHook) {
	for _, h := range hs {
		_ = h.OnNodesSaved(ctx, nodes)
	}
}
