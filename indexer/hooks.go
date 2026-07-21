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
// 所有 Hook 按"修改型 / 通知型"分类：
//   - 修改型 Hook 返回 error 时阻塞管线，调用方需处理
//   - 通知型 Hook 返回 error 时仅记录日志，不阻塞管线

// OnFileOpenedHook 修改型 Hook。
// 在 document.Open 之后、Chunker 之前触发，可修改 RawDoc。
type OnFileOpenedHook interface {
	OnFileOpened(ctx context.Context, doc document.RawDoc) (document.RawDoc, error)
}

// OnChunkHook 修改型 Hook。
// 在 Chunker 产出每个 Chunk 后触发，可修改 *core.Chunk。
type OnChunkHook interface {
	OnChunk(ctx context.Context, chunk *core.Chunk) (*core.Chunk, error)
}

// OnBeforeSemanticSaveHook 修改型 Hook。
// 在 Summarizer 之后、semantic.Save 之前触发，可修改 StructuredDoc 的 Chunks。
type OnBeforeSemanticSaveHook interface {
	OnBeforeSemanticSave(ctx context.Context, doc core.StructuredDoc) (core.StructuredDoc, error)
}

// OnIndexCompleteHook 通知型 Hook。
// 在 AddFile 所有步骤完成后触发，仅通知，不阻塞返回。
type OnIndexCompleteHook interface {
	OnIndexComplete(ctx context.Context, result []*core.Chunk) error
}

// ---------------------------------------------------------------------------
// 简化注册：WithHooks 接受任意数量的 Hook，按类型自动分类
// ---------------------------------------------------------------------------

// hooks 聚合 HyperIndexer 的所有 Hook 切片。
type hooks struct {
	onFileOpened        []OnFileOpenedHook
	onChunk             []OnChunkHook
	onBeforeSemantic    []OnBeforeSemanticSaveHook
	onIndexComplete     []OnIndexCompleteHook
}

// register 按接口类型将 hook 归入对应切片。
// 不认识的类型静默忽略，方便未来扩展。
func (h *hooks) register(hook any) {
	switch v := hook.(type) {
	case OnFileOpenedHook:
		h.onFileOpened = append(h.onFileOpened, v)
	case OnChunkHook:
		h.onChunk = append(h.onChunk, v)
	case OnBeforeSemanticSaveHook:
		h.onBeforeSemantic = append(h.onBeforeSemantic, v)
	case OnIndexCompleteHook:
		h.onIndexComplete = append(h.onIndexComplete, v)
	}
}

// ---------------------------------------------------------------------------
// Hook 执行辅助方法
// ---------------------------------------------------------------------------

// runOnFileOpenedHooks 依次执行所有 OnFileOpenedHook。
// 任一失败则阻塞并返回 error。
func runOnFileOpenedHooks(ctx context.Context, doc document.RawDoc, hs []OnFileOpenedHook) (document.RawDoc, error) {
	var err error
	for _, h := range hs {
		doc, err = h.OnFileOpened(ctx, doc)
		if err != nil {
			return doc, fmt.Errorf("Hook OnFileOpened 失败: %w", err)
		}
	}
	return doc, nil
}

// runOnChunkHooks 依次执行所有 OnChunkHook。
// 任一失败则阻塞并返回 error。
func runOnChunkHooks(ctx context.Context, chunk *core.Chunk, hs []OnChunkHook) (*core.Chunk, error) {
	var err error
	for _, h := range hs {
		chunk, err = h.OnChunk(ctx, chunk)
		if err != nil {
			return chunk, fmt.Errorf("Hook OnChunk 失败: %w", err)
		}
	}
	return chunk, nil
}

// runOnBeforeSemanticSaveHooks 依次执行所有 OnBeforeSemanticSaveHook。
// 任一失败则阻塞并返回 error。
func runOnBeforeSemanticSaveHooks(ctx context.Context, doc core.StructuredDoc, hs []OnBeforeSemanticSaveHook) (core.StructuredDoc, error) {
	var err error
	for _, h := range hs {
		doc, err = h.OnBeforeSemanticSave(ctx, doc)
		if err != nil {
			return doc, fmt.Errorf("Hook OnBeforeSemanticSave 失败: %w", err)
		}
	}
	return doc, nil
}

// runOnIndexCompleteHooks 依次执行所有 OnIndexCompleteHook。
// 任一失败仅记录日志，不阻塞（通知型 Hook）。
func runOnIndexCompleteHooks(ctx context.Context, result []*core.Chunk, hs []OnIndexCompleteHook, logger interface{ Warn(msg string, keysAndValues ...any) }) {
	for _, h := range hs {
		if err := h.OnIndexComplete(ctx, result); err != nil {
			if logger != nil {
				logger.Warn("Hook OnIndexComplete 失败（不阻塞）", "error", err)
			}
		}
	}
}
