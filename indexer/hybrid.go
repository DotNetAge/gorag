package indexer

import (
	"context"
	"maps"
	"sync"

	"github.com/DotNetAge/gorag/core"
)

// HybridIndexer 混合索引器
// 将多个索引器（语义、BM25等）组合，实现查询结果融合与重排
type HybridIndexer struct {
	indexers map[string]core.Indexer // 索引器映射，按名称索引
	weights  map[string]float32      // 各索引器的权重
	k        int                     // RRF 参数，默认 60
	mu       sync.RWMutex
}

// NewHybridIndexer 创建混合索引器
func NewHybridIndexer(indexers ...core.Indexer) *HybridIndexer {
	h := &HybridIndexer{
		indexers: make(map[string]core.Indexer),
		weights:  make(map[string]float32),
		k:        60,
	}
	for _, idx := range indexers {
		h.indexers[idx.Name()] = idx
		h.weights[idx.Name()] = 1.0
	}
	return h
}

// WithWeights 设置各索引器的权重（按索引器名称）
func (h *HybridIndexer) WithWeights(weights map[string]float32) *HybridIndexer {
	h.mu.Lock()
	defer h.mu.Unlock()
	maps.Copy(h.weights, weights)
	return h
}

// WithRRFK 设置 RRF 算法参数 k
func (h *HybridIndexer) WithRRFK(k int) *HybridIndexer {
	h.k = k
	return h
}

// AddIndexer 向混合索引器添加索引器
func (h *HybridIndexer) AddIndexer(indexer core.Indexer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.indexers[indexer.Name()] = indexer
	if h.weights == nil {
		h.weights = make(map[string]float32)
	}
	if _, ok := h.weights[indexer.Name()]; !ok {
		h.weights[indexer.Name()] = 1.0
	}
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

// ListIndexers 列出所有索引器名称
func (h *HybridIndexer) ListIndexers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	names := make([]string, 0, len(h.indexers))
	for name := range h.indexers {
		names = append(names, name)
	}
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
	for i := 1; i < len(indexers); i++ {
		if _, err := indexers[i].Add(ctx, content); err != nil {
			// 记录错误但不中断
			continue
		}
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

	for i := 1; i < len(indexers); i++ {
		if _, err := indexers[i].AddFile(ctx, filePath); err != nil {
			continue
		}
	}

	return chunk, nil
}

// Search 从所有索引器搜索并融合结果
func (h *HybridIndexer) Search(ctx context.Context, query core.Query) ([]core.Hit, error) {
	h.mu.RLock()
	indexers := make([]core.Indexer, 0, len(h.indexers))
	weights := make(map[string]float32)
	for name, idx := range h.indexers {
		indexers = append(indexers, idx)
		weights[name] = h.weights[name]
	}
	k := h.k
	h.mu.RUnlock()

	if len(indexers) == 0 {
		return nil, nil
	}

	// 1. 并发分发搜索请求
	type searchResult struct {
		indexerName string
		hits        []core.Hit
		err         error
	}

	resultCh := make(chan searchResult, len(indexers))
	for _, idxr := range indexers {
		go func(name string, idx core.Indexer) {
			hits, err := idx.Search(ctx, query)
			resultCh <- searchResult{indexerName: name, hits: hits, err: err}
		}(idxr.Name(), idxr)
	}

	// 2. 收集结果
	results := make(map[string][]core.Hit)
	for i := 0; i < len(indexers); i++ {
		r := <-resultCh
		if r.err != nil {
			continue
		}
		results[r.indexerName] = r.hits
	}

	// 3. RRF 融合重排
	fusedHits := h.rrfFusion(results, weights, k)

	return fusedHits, nil
}

// rrfFusion 使用 Reciprocal Rank Fusion (RRF) 算法融合多来源结果
// RRF 公式：score(d) = Σ weight_i / (k + rank_i(d))
// k 是平滑参数（通常 60），rank_i(d) 是文档在索引源 i 中的排名（从 1 开始）
func (h *HybridIndexer) rrfFusion(results map[string][]core.Hit, weights map[string]float32, k int) []core.Hit {
	if len(results) == 0 {
		return nil
	}

	// scoreMap: chunkID -> 融合分数
	type scoreEntry struct {
		hit   core.Hit
		score float32
	}
	scoreMap := make(map[string]*scoreEntry)

	// 对每个索引源分别计算排名贡献
	for indexerName, hits := range results {
		weight := float32(1.0)
		if w, ok := weights[indexerName]; ok {
			weight = w
		}

		// 按排名计算 RRF 分数：1/(k + rank)，rank 从 1 开始
		for rank, hit := range hits {
			if _, exists := scoreMap[hit.ID]; !exists {
				scoreMap[hit.ID] = &scoreEntry{
					hit:   hit,
					score: 0,
				}
			}
			// RRF 贡献 = weight / (k + rank)
			rrfScore := weight / float32(k+rank+1)
			scoreMap[hit.ID].score += rrfScore
		}
	}

	// 转换为 slice 并按融合分数降序排序
	fused := make([]core.Hit, 0, len(scoreMap))
	for _, entry := range scoreMap {
		entry.hit.Score = entry.score
		fused = append(fused, entry.hit)
	}

	// 排序
	for i := 0; i < len(fused)-1; i++ {
		for j := i + 1; j < len(fused); j++ {
			if fused[j].Score > fused[i].Score {
				fused[i], fused[j] = fused[j], fused[i]
			}
		}
	}

	return fused
}

// Remove 从所有索引器移除
func (h *HybridIndexer) Remove(ctx context.Context, chunkID string) error {
	h.mu.RLock()
	indexers := make([]core.Indexer, 0, len(h.indexers))
	for _, idx := range h.indexers {
		indexers = append(indexers, idx)
	}
	h.mu.RUnlock()

	var lastErr error
	for _, idx := range indexers {
		if err := idx.Remove(ctx, chunkID); err != nil {
			lastErr = err
			continue
		}
	}
	return lastErr
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

func (s *HybridIndexer) NewQuery(terms string) core.Query {
	return nil
}
