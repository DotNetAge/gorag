package gorag

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"sync"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/indexer"
	"github.com/DotNetAge/gorag/logging"
	querypkg "github.com/DotNetAge/gorag/query"
	"github.com/DotNetAge/gorag/result"
)

// HybridIndexer 混合索引器
// 将多个索引器（语义、BM25等）组合，实现查询结果融合与重排
type HybridIndexer struct {
	indexers map[string]core.Indexer
	weights  map[string]float32
	mu       sync.RWMutex
	logger   logging.Logger
	client   chat.Client
}

// NewHybridIndexer 创建混合索引器
func NewHybridIndexer(
	logger logging.Logger,
	vectorStore core.VectorStore,
	graphStore core.GraphStore,
	docStore core.FullTextStore,
	client chat.Client,
	embedder core.Embedder) (*HybridIndexer, error) {

	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if vectorStore == nil {
		return nil, fmt.Errorf("vectorStore is required")
	}

	if docStore == nil {
		return nil, fmt.Errorf("docStore is required")
	}

	h := &HybridIndexer{
		indexers: make(map[string]core.Indexer),
		weights:  make(map[string]float32),
		logger:   logger,
		client:   client,
	}

	semanticIndexer := indexer.NewSemanticIndexer(
		vectorStore,
		embedder,
	)

	fulltextIndexer, err := indexer.NewFulltextIndexer(docStore)

	if err != nil {
		logger.Error("Failed to init fulltext indexer", err)
		return nil, err
	}

	if client != nil && graphStore != nil {
		graphIndexer := indexer.NewGraphIndexer(graphStore, client)
		h.AddIndexer(semanticIndexer, 0.7)
		h.AddIndexer(fulltextIndexer, 0.2)
		h.AddIndexer(graphIndexer, 0.1)
	} else {
		h.AddIndexer(semanticIndexer, 0.8)
		h.AddIndexer(fulltextIndexer, 0.2)
	}

	return h, nil
}

func (h *HybridIndexer) GetWeights() map[string]float32 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	cpy := make(map[string]float32, len(h.weights))
	maps.Copy(cpy, h.weights)
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

	chunks, err := indexer.GetChunks(content)
	if err != nil {
		return nil, fmt.Errorf("failed to generate chunks: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks generated from content")
	}

	var partialErrs []error
	for _, idx := range indexers {
		if chunkIndexer, ok := idx.(core.ChunkIndexer); ok {
			if err := chunkIndexer.IndexChunks(ctx, chunks); err != nil {
				h.logger.Warn("partial index failure", "indexer", idx.Name(), "error", err)
				partialErrs = append(partialErrs, err)
			}
		} else {
			for _, chunk := range chunks {
				if err := idx.IndexChunk(ctx, chunk); err != nil {
					h.logger.Warn("partial index failure", "indexer", idx.Name(), "chunkID", chunk.ID, "error", err)
					partialErrs = append(partialErrs, err)
					break
				}
			}
		}
	}

	if len(partialErrs) > 0 {
		return chunks[0], fmt.Errorf("index succeeded partially (%d/%d indexers failed): %w",
			len(partialErrs), len(indexers), partialErrs[0])
	}

	return chunks[0], nil
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

	chunks, err := indexer.GetFileChunks(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate chunks from file: %w", err)
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	var partialErrs []error
	for _, idx := range indexers {
		if chunkIndexer, ok := idx.(core.ChunkIndexer); ok {
			if err := chunkIndexer.IndexChunks(ctx, chunks); err != nil {
				h.logger.Warn("partial index failure", "indexer", idx.Name(), "error", err)
				partialErrs = append(partialErrs, err)
			}
		} else {
			for _, chunk := range chunks {
				if err := idx.IndexChunk(ctx, chunk); err != nil {
					h.logger.Warn("partial index failure", "indexer", idx.Name(), "chunkID", chunk.ID, "error", err)
					partialErrs = append(partialErrs, err)
					break
				}
			}
		}
	}

	if len(partialErrs) > 0 {
		return chunks[0], fmt.Errorf("indexfile succeeded partially (%d/%d indexers failed): %w",
			len(partialErrs), len(indexers), partialErrs[0])
	}

	return chunks[0], nil
}

// Search 从所有索引器搜索并融合结果
func (h *HybridIndexer) Search(ctx context.Context, query core.Query) ([]core.Hit, error) {
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

	type searchResult struct {
		indexerName string
		hits        []core.Hit
		err         error
	}

	resultCh := make(chan searchResult, len(snaps))
	var graphIndexer *indexer.GraphIndexer

	for _, s := range snaps {
		if s.name == "graph" {
			if gi, ok := s.idx.(*indexer.GraphIndexer); ok {
				graphIndexer = gi
			}
			continue
		}

		go func(name string, idx core.Indexer) {
			hits, err := idx.Search(ctx, query)
			resultCh <- searchResult{indexerName: name, hits: hits, err: err}
		}(s.name, s.idx)
	}

	results := []result.FusionSource{}
	chunkIDs := make([]string, 0)
	seenChunk := make(map[string]bool)
	var errs []error

	for i := 0; i < len(snaps); i++ {
		if snaps[i].name == "graph" {
			continue
		}

		r := <-resultCh
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}

		results = append(results,
			*result.NewSource(r.indexerName,
				weightsSnap[r.indexerName],
				r.hits))

		for _, hit := range r.hits {
			if !seenChunk[hit.ID] {
				chunkIDs = append(chunkIDs, hit.ID)
				seenChunk[hit.ID] = true
			}
		}
	}

	if len(results) == 0 && len(errs) > 0 {
		return nil, errs[0]
	}

	if graphIndexer != nil && len(chunkIDs) > 0 {
		graphHits, err := graphIndexer.SearchByChunkIDs(ctx, chunkIDs, 1, 10)
		if err == nil && len(graphHits) > 0 {
			results = append(results,
				*result.NewSource("graph",
					weightsSnap["graph"],
					graphHits))
		}
	}

	fusionHits, err := result.RRF(results...)
	if err != nil {
		return nil, err
	}

	if sq, ok := query.(*querypkg.SemanticQuery); ok {
		rerankedHits, err := result.Rerank(sq, fusionHits)
		if err != nil {
			return nil, err
		}
		return rerankedHits, nil
	}

	return fusionHits, nil
}

// Remove 从所有索引器移除
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
			h.logger.Warn("remove partial failure", "indexer", idx.Name(), "chunkID", chunkID, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", idx.Name(), err))
		}
	}
	if len(errs) == len(indexers) && len(errs) > 0 {
		return fmt.Errorf("remove failed from all %d indexers: %v", len(errs), errs)
	}
	if len(errs) > 0 {
		h.logger.Warn("remove completed with partial failures", "total", len(indexers), "failed", len(errs))
	}
	return nil
}

// IndexChunk indexes a single pre-generated chunk across all indexers
func (h *HybridIndexer) IndexChunk(ctx context.Context, chunk *core.Chunk) error {
	h.mu.RLock()
	indexers := make([]core.Indexer, 0, len(h.indexers))
	for _, idx := range h.indexers {
		indexers = append(indexers, idx)
	}
	h.mu.RUnlock()

	var errs []error
	for _, idx := range indexers {
		if err := idx.IndexChunk(ctx, chunk); err != nil {
			h.logger.Warn("index chunk partial failure", "indexer", idx.Name(), "chunkID", chunk.ID, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", idx.Name(), err))
		}
	}
	if len(errs) == len(indexers) && len(errs) > 0 {
		return fmt.Errorf("index chunk failed from all %d indexers: %v", len(errs), errs)
	}
	if len(errs) > 0 {
		h.logger.Warn("index chunk completed with partial failures", "total", len(indexers), "failed", len(errs))
	}
	return nil
}

// IndexChunks indexes multiple pre-generated chunks across all indexers
func (h *HybridIndexer) IndexChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	h.mu.RLock()
	indexers := make([]core.Indexer, 0, len(h.indexers))
	for _, idx := range h.indexers {
		indexers = append(indexers, idx)
	}
	h.mu.RUnlock()

	var errs []error
	for _, idx := range indexers {
		if chunkIndexer, ok := idx.(core.ChunkIndexer); ok {
			if err := chunkIndexer.IndexChunks(ctx, chunks); err != nil {
				h.logger.Warn("index chunks partial failure", "indexer", idx.Name(), "error", err)
				errs = append(errs, fmt.Errorf("%s: %w", idx.Name(), err))
			}
		} else {
			for _, chunk := range chunks {
				if err := idx.IndexChunk(ctx, chunk); err != nil {
					h.logger.Warn("index chunk partial failure", "indexer", idx.Name(), "chunkID", chunk.ID, "error", err)
					errs = append(errs, fmt.Errorf("%s: %w", idx.Name(), err))
					break
				}
			}
		}
	}
	if len(errs) == len(indexers) && len(errs) > 0 {
		return fmt.Errorf("index chunks failed from all %d indexers: %v", len(errs), errs)
	}
	if len(errs) > 0 {
		h.logger.Warn("index chunks completed with partial failures", "total", len(indexers), "failed", len(errs))
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
				h.logger.Warn("close indexer failed", "indexer", idx.Name(), "error", err)
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

var _ core.Indexer = (*HybridIndexer)(nil)

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
