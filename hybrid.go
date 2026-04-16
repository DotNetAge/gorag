package gorag

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/indexer"
	"github.com/DotNetAge/gorag/logging"
	querypkg "github.com/DotNetAge/gorag/query"
	"github.com/DotNetAge/gorag/result"
)

// HybridIndexer 混合索引器
// 将多个索引器（语义、BM25等）组合，实现查询结果融合与重排
type HybridIndexer struct {
	indexers map[string]core.Indexer // 索引器映射，按名称索引
	weights  map[string]float32      // 各索引器的权重
	mu       sync.RWMutex
	logger   logging.Logger
}

// NewHybridIndexer 创建混合索引器
func NewHybridIndexer(
	vectorStore core.VectorStore,
	graphStore core.GraphStore,
	docStore core.FullTextStore,
	embedder core.Embedder) (*HybridIndexer, error) {

	h := &HybridIndexer{
		indexers: make(map[string]core.Indexer),
		weights:  make(map[string]float32),
	}

	semanticIndexer := indexer.NewSemanticIndexer(
		vectorStore,
		embedder,
	)

	fulltextIndexer, err := indexer.NewFulltextIndexer(docStore)

	if err != nil {
		slog.Error("Failed to init fulltext indexer", "error", err)
		return nil, err
	}

	graphIndexer := indexer.NewGraphIndexer(graphStore)

	h.AddIndexer(semanticIndexer, 0.7)
	h.AddIndexer(fulltextIndexer, 0.2)
	h.AddIndexer(graphIndexer, 0.1)

	return h, nil
}

func (h *HybridIndexer) GetWeights() map[string]float32 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	cpy := make(map[string]float32, len(h.weights))
	for k, v := range h.weights {
		cpy[k] = v
	}
	return cpy
}

// AddIndexer 向混合索引器添加索引器
func (h *HybridIndexer) AddIndexer(indexer core.Indexer, weight float32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.indexers[indexer.Name()] = indexer
	if h.weights == nil {
		h.weights = make(map[string]float32)
	}
	h.weights[indexer.Name()] = weight
}

// RemoveIndexer 移除索引器
func (h *HybridIndexer) RemoveIndexer(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.indexers, name)
	delete(h.weights, name)
}

// GetIndexer 获取索引器
func (h *HybridIndexer) GetIndexer(name string) (core.Indexer, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	idx, ok := h.indexers[name]
	return idx, ok
}

// ListIndexers 列出所有索引器名称（按字母排序，保证确定性输出）
func (h *HybridIndexer) ListIndexers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	names := make([]string, 0, len(h.indexers))
	for name := range h.indexers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Add 将内容添加到所有索引器
func (h *HybridIndexer) Add(ctx context.Context, content string) (*core.Chunk, error) {
	h.mu.RLock()
	indexers := make([]core.Indexer, 0, len(h.indexers))
	for _, idx := range h.indexers {
		indexers = append(indexers, idx)
	}
	h.mu.RUnlock()

	if len(indexers) == 0 {
		return nil, nil
	}

	// 使用第一个索引器生成分块（保持一致性）
	chunk, err := indexers[0].Add(ctx, content)
	if err != nil {
		return nil, err
	}

	// 分发给其余索引器
	var partialErrs []error
	for i := 1; i < len(indexers); i++ {
		if _, err := indexers[i].Add(ctx, content); err != nil {
			slog.Warn("partial add failure", "indexer", indexers[i].Name(), "error", err)
			partialErrs = append(partialErrs, err)
		}
	}

	if len(partialErrs) > 0 {
		return chunk, fmt.Errorf("add succeeded partially (%d/%d indexers failed): %w",
			len(partialErrs), len(indexers), partialErrs[0])
	}

	return chunk, nil
}

// AddFile 将文件添加到所有索引器
func (h *HybridIndexer) AddFile(ctx context.Context, filePath string) (*core.Chunk, error) {
	h.mu.RLock()
	indexers := make([]core.Indexer, 0, len(h.indexers))
	for _, idx := range h.indexers {
		indexers = append(indexers, idx)
	}
	h.mu.RUnlock()

	if len(indexers) == 0 {
		return nil, nil
	}

	chunk, err := indexers[0].AddFile(ctx, filePath)
	if err != nil {
		return nil, err
	}

	var partialErrs []error
	for i := 1; i < len(indexers); i++ {
		if _, err := indexers[i].AddFile(ctx, filePath); err != nil {
			slog.Warn("partial addfile failure", "indexer", indexers[i].Name(), "error", err)
			partialErrs = append(partialErrs, err)
		}
	}

	if len(partialErrs) > 0 {
		return chunk, fmt.Errorf("addfile succeeded partially (%d/%d indexers failed): %w",
			len(partialErrs), len(indexers), partialErrs[0])
	}

	return chunk, nil
}

// Search 从所有索引器搜索并融合结果
func (h *HybridIndexer) Search(ctx context.Context, query core.Query) ([]core.Hit, error) {
	// 0. 加锁快照，避免遍历 map 时的 data race (修复 #1.1)
	h.mu.RLock()
	type indexerSnap struct {
		name string
		idx  core.Indexer
	}
	snaps := make([]indexerSnap, 0, len(h.indexers))
	for name, idx := range h.indexers {
		snaps = append(snaps, indexerSnap{name: name, idx: idx})
	}
	weightsSnap := make(map[string]float32, len(h.weights))
	for k, v := range h.weights {
		weightsSnap[k] = v
	}
	h.mu.RUnlock()

	if len(snaps) == 0 {
		return nil, nil
	}

	// 1. 并发分发搜索请求
	type searchResult struct {
		indexerName string
		hits        []core.Hit
		err         error
	}

	resultCh := make(chan searchResult, len(snaps))
	for _, s := range snaps {
		go func(name string, idx core.Indexer) {
			hits, err := idx.Search(ctx, query)
			resultCh <- searchResult{indexerName: name, hits: hits, err: err}
		}(s.name, s.idx)
	}

	// 2. 收集结果
	results := []result.FusionSource{}
	var errs []error
	for i := 0; i < len(snaps); i++ {
		r := <-resultCh
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		results = append(results,
			*result.NewSource(r.indexerName,
				weightsSnap[r.indexerName],
				r.hits))
	}

	// 所有 indexer 都失败了，返回错误
	if len(results) == 0 && len(errs) > 0 {
		return nil, errs[0]
	}

	fusionHits, err := result.RRF(results...)
	if err != nil {
		return nil, err
	}

	// 如果 query 是 SemanticQuery 类型，进行重排
	if sq, ok := query.(*querypkg.SemanticQuery); ok {
		rerankedHits, err := result.Rerank(sq, fusionHits)
		if err != nil {
			return nil, err
		}
		return rerankedHits, nil
	}

	// 非语义查询，跳过重排
	return fusionHits, nil
}

// Remove 从所有索引器移除（best-effort：部分失败仅记录警告，不阻断整体）
func (h *HybridIndexer) Remove(ctx context.Context, chunkID string) error {
	h.mu.RLock()
	indexers := make([]core.Indexer, 0, len(h.indexers))
	for _, idx := range h.indexers {
		indexers = append(indexers, idx)
	}
	h.mu.RUnlock()

	var errs []error
	for _, idx := range indexers {
		if err := idx.Remove(ctx, chunkID); err != nil {
			slog.Warn("remove partial failure", "indexer", idx.Name(), "chunkID", chunkID, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", idx.Name(), err))
		}
	}
	if len(errs) == len(indexers) && len(errs) > 0 {
		return fmt.Errorf("remove failed from all %d indexers: %v", len(errs), errs)
	}
	if len(errs) > 0 {
		slog.Warn("remove completed with partial failures", "total", len(indexers), "failed", len(errs))
	}
	return nil
}

// Close 关闭所有索引器持有的资源
func (h *HybridIndexer) Close(ctx context.Context) error {
	h.mu.RLock()
	indexers := make([]core.Indexer, 0, len(h.indexers))
	for _, idx := range h.indexers {
		indexers = append(indexers, idx)
	}
	h.mu.RUnlock()

	var errs []error
	for _, idx := range indexers {
		if closer, ok := idx.(interface{ Close(context.Context) error }); ok {
			if err := closer.Close(ctx); err != nil {
				slog.Warn("close indexer failed", "indexer", idx.Name(), "error", err)
				errs = append(errs, fmt.Errorf("%s: %w", idx.Name(), err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close failed for %d indexers: %v", len(errs), errs)
	}
	return nil
}

// Name 返回索引器名称
func (h *HybridIndexer) Name() string {
	return "hybrid"
}

// Type 返回索引器类型
func (h *HybridIndexer) Type() string {
	return "hybrid"
}

// 确保实现 core.Indexer 接口
var _ core.Indexer = (*HybridIndexer)(nil)

// NewQuery 混合索引器不支持创建单类型查询
// 使用者应根据需求选择具体索引器的 NewQuery 方法
// 返回权重最高的索引器的查询；若权重均为0，返回第一个索引器的查询
func (h *HybridIndexer) NewQuery(terms string) core.Query {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var primaryIndexer core.Indexer
	var maxWeight float32
	for name, idx := range h.indexers {
		w, ok := h.weights[name]
		if !ok {
			w = 0
		}
		if w > maxWeight || primaryIndexer == nil {
			maxWeight = w
			primaryIndexer = idx
		}
	}

	if primaryIndexer != nil {
		return primaryIndexer.NewQuery(terms)
	}
	return nil
}
