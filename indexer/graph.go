package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"sync"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/extractor"
	"github.com/DotNetAge/gorag/query"
)

// GraphSearchResult 图搜索结果，用于序列化为 Hit.Content
type GraphSearchResult struct {
	Query     string       `json:"query,omitempty"`     // 原始查询
	Entities  []*core.Node `json:"entities,omitempty"`  // 匹配的实体节点
	Relations []*core.Edge `json:"relations,omitempty"` // 关联的边
	ChunkIDs  []string     `json:"chunk_ids,omitempty"` // 关联的 chunk ID 列表
	DocIDs    []string     `json:"doc_ids,omitempty"`   // 关联的文档 ID 列表
}

// GraphIndexer 知识图谱索引器
// 与语义/全文索引器不同，GraphRAG 的索引是构建实体-关系图，查询是图遍历
// 支持两种查询模式：
// 1. 独立 GraphRAG 模式（需要 client）：Query → LLM提取实体 → 图遍历
// 2. 混合模式（通过 Chunk IDs）：Query → Semantic搜索 → Chunks → 关联 Nodes/Edges
type GraphIndexer struct {
	store  core.GraphStore
	client chat.Client       // LLM client，用于独立 GraphRAG 模式的实体提取
	cache  core.CacheStore   // 实体提取结果缓存（可选）
}

// GraphOption GraphIndexer 的可选配置
type GraphOption func(*GraphIndexer)

// WithCache 设置实体提取缓存（依赖注入 core.CacheStore 接口）
func WithCache(cache core.CacheStore) GraphOption {
	return func(g *GraphIndexer) {
		g.cache = cache
	}
}

// NewGraphIndexer 创建知识图谱索引器
// client 参数可选：
//   - 提供 client：支持独立 GraphRAG 模式（索引 + 查询都用 LLM）
//   - 不提供 client：仅支持混合模式（索引时不用 LLM，查询时通过 Chunk IDs）
// opts 可选配置，如 WithCache
func NewGraphIndexer(store core.GraphStore, opts ...any) *GraphIndexer {
	g := &GraphIndexer{
		store: store,
	}
	for _, opt := range opts {
		switch o := opt.(type) {
		case chat.Client:
			g.client = o
		case GraphOption:
			o(g)
		}
	}
	return g
}

// Name 返回索引器名称
func (g *GraphIndexer) Name() string {
	return "graph"
}

// Type 返回索引器类型
func (g *GraphIndexer) Type() string {
	return "graph"
}

// Add 从内容构建知识图谱（实现 core.Indexer 接口）
// 流程：分块 → LLM实体关系提取 → 图存储
// 注意：如果没有 client，会跳过实体提取
func (g *GraphIndexer) Add(ctx context.Context, content string) (*core.Chunk, error) {
	chunks, err := GetChunks(content)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	if err := g.IndexChunks(ctx, chunks); err != nil {
		return nil, err
	}
	return chunks[0], nil
}

// AddFile 从文件构建知识图谱（实现 core.Indexer 接口）
// 注意：如果没有 client，会跳过实体提取
func (g *GraphIndexer) AddFile(ctx context.Context, filePath string) (*core.Chunk, error) {
	chunks, err := GetFileChunks(filePath)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	if err := g.IndexChunks(ctx, chunks); err != nil {
		return nil, err
	}
	return chunks[0], nil
}

// NewQuery 创建图查询（实现 core.Indexer 接口）
func (g *GraphIndexer) NewQuery(terms string) core.Query {
	return query.NewGraphQuery(terms)
}

// Search 执行图搜索（实现 core.Indexer 接口）
// 独立 GraphRAG 模式：LLM实体提取 → 图遍历 → 节点/边序列化 → Hit
// 注意：需要 client，否则返回空结果
func (g *GraphIndexer) Search(ctx context.Context, qry core.Query) ([]core.Hit, error) {
	if g.client == nil {
		// 没有 client，无法进行独立 GraphRAG 查询
		return nil, nil
	}

	// 1. 从查询中提取实体（使用静态方法）
	queryChunk := &core.Chunk{
		ID:      "query_" + qry.Raw(),
		Content: qry.Raw(),
	}

	entityNodes, _, err := extractor.ExtractNodesAndEdges(ctx, g.client, queryChunk)
	if err != nil || len(entityNodes) == 0 {
		return nil, err
	}

	// 转换为指针切片
	entities := make([]*core.Node, len(entityNodes))
	for i := range entityNodes {
		entities[i] = &entityNodes[i]
	}

	// 获取 depth 和 limit（如果有）
	gq, ok := qry.(*query.GraphQuery)
	depth := 1
	limit := 10
	if ok {
		depth = gq.Depth
		limit = gq.Limit
	}

	// 2. 收集所有关联的 chunkID 和 docID
	chunkIDs := make([]string, 0)
	docIDs := make([]string, 0)
	seenChunk := make(map[string]bool)
	seenDoc := make(map[string]bool)

	for _, entity := range entities {
		for _, chunkID := range entity.SourceChunkIDs {
			if !seenChunk[chunkID] {
				chunkIDs = append(chunkIDs, chunkID)
				seenChunk[chunkID] = true
			}
		}
		for _, docID := range entity.SourceDocIDs {
			if !seenDoc[docID] {
				docIDs = append(docIDs, docID)
				seenDoc[docID] = true
			}
		}
	}

	// 3. 并发获取关联的边
	type edgeResult struct {
		edges []*core.Edge
		err   error
	}
	edgeCh := make(chan edgeResult, len(entities))
	var wg sync.WaitGroup
	for _, entity := range entities {
		wg.Add(1)
		go func(entityID string) {
			defer wg.Done()
			_, edges, err := g.store.GetNeighbors(ctx, entityID, depth, limit)
			edgeCh <- edgeResult{edges: edges, err: err}
		}(entity.ID)
	}

	// 并发收集结果
	go func() {
		wg.Wait()
		close(edgeCh)
	}()

	relations := make([]*core.Edge, 0)
	for result := range edgeCh {
		if result.err == nil {
			relations = append(relations, result.edges...)
		}
	}

	// 4. 构建 GraphSearchResult 并序列化为 JSON
	result := GraphSearchResult{
		Query:     qry.Raw(),
		Entities:  entities,
		Relations: relations,
		ChunkIDs:  chunkIDs,
		DocIDs:    docIDs,
	}

	content, err := json.Marshal(result)
	if err != nil {
		// 记录错误但继续处理，返回降级结果
		var buf strings.Builder
		fmt.Fprintf(&buf, `{"error": "failed to serialize graph result: %v"}`, err)
		content = []byte(buf.String())
	}

	// 5. 构建 Hit，按 chunkID 分组返回
	hits := make([]core.Hit, 0, len(chunkIDs))
	seenChunkHit := make(map[string]bool)

	for _, chunkID := range chunkIDs {
		if seenChunkHit[chunkID] {
			continue
		}
		seenChunkHit[chunkID] = true

		docID := ""
		for _, e := range entities {
			for _, cid := range e.SourceChunkIDs {
				if cid == chunkID && len(e.SourceDocIDs) > 0 {
					docID = e.SourceDocIDs[0]
					break
				}
			}
			if docID != "" {
				break
			}
		}

		docEntities := make([]*core.Node, 0)
		for _, e := range entities {
			if slices.Contains(e.SourceChunkIDs, chunkID) {
				docEntities = append(docEntities, e)
			}
		}

		docRelations := make([]*core.Edge, 0)
		for _, r := range relations {
			if slices.Contains(r.SourceChunkIDs, chunkID) {
				docRelations = append(docRelations, r)
			}
		}

		chunkResult := GraphSearchResult{
			Query:     qry.Raw(),
			Entities:  docEntities,
			Relations: docRelations,
			ChunkIDs:  []string{chunkID},
			DocIDs:    []string{docID},
		}

		chunkContent, _ := json.Marshal(chunkResult)
		graphScore := scoreGraphResult(docEntities, docRelations)
		hits = append(hits, core.Hit{
			ID:      chunkID,
			Score:   graphScore,
			DocID:   docID,
			Content: string(chunkContent),
		})
	}

	// 如果没有 chunkID，至少返回一个包含所有信息的 Hit
	if len(hits) == 0 && len(entities) > 0 {
		chunkID := ""
		if len(entities[0].SourceChunkIDs) > 0 {
			chunkID = entities[0].SourceChunkIDs[0]
		}
		fallbackScore := scoreGraphResult(entities, relations)
		hits = append(hits, core.Hit{
			ID:      chunkID,
			Score:   fallbackScore,
			DocID:   "",
			Content: string(content),
		})
	}

	return hits, nil
}

// SearchByChunkIDs 通过 Chunk IDs 查询关联的图结构
// 混合模式：由 HybridIndexer 调用，无需 LLM
// 流程：Chunk IDs → 查询关联 Nodes → 多跳遍历（可选边类型过滤） → 路径评分 → Hit
//
// 支持选项：
//   - depth: 遍历深度（默认 1，即直接邻居）
//   - limit: 返回结果数量上限
//   - edgeTypes: 关系类型过滤，仅遍历指定类型的边
func (g *GraphIndexer) SearchByChunkIDs(ctx context.Context, chunkIDs []string, depth, limit int, edgeTypes ...[]string) ([]core.Hit, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	// 1. 查询关联的 Nodes
	nodes, err := g.store.GetNodesByChunkIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by chunk IDs: %w", err)
	}

	if len(nodes) == 0 {
		return nil, nil
	}

	// 2. 收集起始节点 ID
	nodeIDs := make([]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}

	// 3. 多跳遍历（使用 GetMultiHopPaths 支持边类型过滤）
	var types []string
	if len(edgeTypes) > 0 && len(edgeTypes[0]) > 0 {
		types = edgeTypes[0]
	}

	hopNodes, hopEdges := []*core.Node{}, []*core.Edge{}
	if depth > 0 {
		hopNodes, hopEdges, err = g.store.GetMultiHopPaths(ctx, nodeIDs, types, depth, limit)
		if err != nil {
			// 多跳遍历失败时降级为直接查询关联边
			hopEdges, err = g.store.GetEdgesByChunkIDs(ctx, chunkIDs)
			if err != nil {
				hopEdges = nil
			}
		}
	}

	// 4. 合并：直接关联的 edges + 多跳发现的 edges
	// 多跳成功时，多跳结果已包含直接边，无需再查；失败降级时 hopEdges 已是直接边
	edgeMap := make(map[string]*core.Edge)
	for _, e := range hopEdges {
		edgeMap[e.ID] = e
	}
	if depth <= 0 {
		// 无多跳时，直接查询关联边
		directEdges, err := g.store.GetEdgesByChunkIDs(ctx, chunkIDs)
		if err == nil {
			for _, e := range directEdges {
				if _, exists := edgeMap[e.ID]; !exists {
					edgeMap[e.ID] = e
				}
			}
		}
	}

	// 合并节点
	nodeMap := make(map[string]*core.Node)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}
	for _, n := range hopNodes {
		if _, exists := nodeMap[n.ID]; !exists {
			nodeMap[n.ID] = n
		}
	}

	allNodes := make([]*core.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		allNodes = append(allNodes, n)
	}
	allEdges := make([]*core.Edge, 0, len(edgeMap))
	for _, e := range edgeMap {
		allEdges = append(allEdges, e)
	}

	// 5. 收集所有 Chunk IDs 和 Doc IDs
	allChunkIDs := make([]string, 0)
	allDocIDs := make([]string, 0)
	seenChunk := make(map[string]bool)
	seenDoc := make(map[string]bool)

	for _, node := range allNodes {
		for _, chunkID := range node.SourceChunkIDs {
			if !seenChunk[chunkID] {
				allChunkIDs = append(allChunkIDs, chunkID)
				seenChunk[chunkID] = true
			}
		}
		for _, docID := range node.SourceDocIDs {
			if !seenDoc[docID] {
				allDocIDs = append(allDocIDs, docID)
				seenDoc[docID] = true
			}
		}
	}

	// 6. 构建 Hit，带路径评分
	hits := make([]core.Hit, 0, len(chunkIDs))
	seenChunkHit := make(map[string]bool)

	for _, chunkID := range chunkIDs {
		if seenChunkHit[chunkID] {
			continue
		}
		seenChunkHit[chunkID] = true

		docID := ""
		for _, node := range allNodes {
			for _, cid := range node.SourceChunkIDs {
				if cid == chunkID && len(node.SourceDocIDs) > 0 {
					docID = node.SourceDocIDs[0]
					break
				}
			}
			if docID != "" {
				break
			}
		}

		chunkNodes := make([]*core.Node, 0)
		for _, node := range allNodes {
			if slices.Contains(node.SourceChunkIDs, chunkID) {
				chunkNodes = append(chunkNodes, node)
			}
		}

		chunkEdges := make([]*core.Edge, 0)
		for _, edge := range allEdges {
			if slices.Contains(edge.SourceChunkIDs, chunkID) {
				chunkEdges = append(chunkEdges, edge)
			}
		}

		result := GraphSearchResult{
			Entities:  chunkNodes,
			Relations: chunkEdges,
			ChunkIDs:  []string{chunkID},
			DocIDs:    []string{docID},
		}

		content, _ := json.Marshal(result)
		score := scoreGraphResult(chunkNodes, chunkEdges)
		hits = append(hits, core.Hit{
			ID:      chunkID,
			Score:   score,
			DocID:   docID,
			Content: string(content),
		})
	}

	return hits, nil
}

// Remove 移除与指定 chunk 关联的所有实体和关系（实现 core.Indexer 接口）
func (g *GraphIndexer) Remove(ctx context.Context, chunkID string) error {
	q := `MATCH (n) WHERE $chunkID IN n.source_chunk_ids DETACH DELETE n`
	_, err := g.store.Query(ctx, q, map[string]any{"chunkID": chunkID})
	return err
}

// SearchGlobal 通过社区摘要回答全局性问题 (Global Search)
// 适用于："数据集主要讨论了哪些主题？" 这类宏观问题
// 需要 client 和已构建的社区摘要
func (g *GraphIndexer) SearchGlobal(ctx context.Context, query string, level int) ([]core.Hit, error) {
	if g.client == nil {
		return nil, nil
	}

	ci := NewCommunityIndexer(g.store, g.client)
	matches, err := ci.SearchGlobal(ctx, query, level)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, nil
	}

	// 将社区匹配转换为 Hit
	hits := make([]core.Hit, 0, len(matches))
	for _, m := range matches {
		// 将社区信息序列化为 JSON
		content, _ := json.Marshal(map[string]any{
			"community_id": m.CommunityID,
			"summary":      m.Summary,
			"keywords":     m.Keywords,
		})

		hits = append(hits, core.Hit{
			ID:      m.CommunityID,
			Score:   m.Score,
			DocID:   "",
			Content: string(content),
		})
	}

	return hits, nil
}

// IndexChunk indexes a pre-generated chunk (implements core.Indexer interface)
func (g *GraphIndexer) IndexChunk(ctx context.Context, chunk *core.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk cannot be nil")
	}
	return g.buildChunk(ctx, chunk)
}

// IndexChunks indexes multiple pre-generated chunks in batch (implements core.ChunkIndexer interface)
//
// 三重优化流水线：
//  1. 大块优先：将 ParentDoc 子块聚合到父块，减少总 chunk 数
//  2. 分级提取：跳过低价值 chunk（纯描述、过短内容），仅对高价值 chunk 调用 LLM
//  3. 并发提取：远程 LLM 全并发；本地 LLM（Ollama 等）串行提取但并发写入图数据库
func (g *GraphIndexer) IndexChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	if g.client == nil {
		return nil
	}

	// 过滤有效 chunk
	validChunks := make([]*core.Chunk, 0, len(chunks))
	for _, c := range chunks {
		if c != nil && c.Content != "" {
			validChunks = append(validChunks, c)
		}
	}
	if len(validChunks) == 0 {
		return nil
	}

	// ---- 阶段1：大块优先 — 聚合子块到父块 ----
	mergedChunks := mergeToParentChunks(validChunks)

	// ---- 阶段2：分级提取 — 过滤低价值 chunk ----
	highValueChunks := make([]*core.Chunk, 0, len(mergedChunks))
	skippedCount := 0
	for _, c := range mergedChunks {
		if chunkValueScore(c) >= 0.25 {
			highValueChunks = append(highValueChunks, c)
		} else {
			skippedCount++
		}
	}
	if skippedCount > 0 && len(highValueChunks) == 0 {
		// 全部被跳过时，保留评分最高的 chunk
		var best *core.Chunk
		bestScore := -1.0
		for _, c := range mergedChunks {
			s := chunkValueScore(c)
			if s > bestScore {
				bestScore = s
				best = c
			}
		}
		if best != nil {
			highValueChunks = append(highValueChunks, best)
		}
	}
	if len(highValueChunks) == 0 {
		return nil
	}

	// ---- 阶段3：带缓存的批量实体提取 ----
	// 远程 LLM：并发执行多个批次；本地 LLM：串行提取但下面阶段4并发写入
	local := isLocalClient(g.client)
	var allNodes []core.Node
	var allEdges []core.Edge
	var err error

	if local {
		// 本地 LLM（Ollama 等）：串行批量提取
		allNodes, allEdges, err = extractor.ExtractBatchWithCache(ctx, g.client, highValueChunks, g.cache)
		if err != nil {
			return fmt.Errorf("batch entity extraction failed: %w", err)
		}
	} else {
		// 远程 LLM：并发执行多批次提取，大幅降低总等待时间
		allNodes, allEdges, err = g.extractConcurrently(ctx, highValueChunks)
		if err != nil {
			return fmt.Errorf("concurrent extraction failed: %w", err)
		}
	}

	// 对合并块：将提取结果中的节点/边关联到所有原始子块 ID
	allNodes, allEdges = expandMergedNodes(allNodes, allEdges, highValueChunks)

	// ---- 阶段4：并发写入图数据库 ----
	// 本地和远程 LLM 都可以并发写入图数据库
	return g.storeBatchConcurrent(ctx, allNodes, allEdges)
}

// extractConcurrently 并发执行多批次实体提取（仅用于远程 LLM）
// 将 chunks 按 batchSize 分组后并发调用 LLM，每个批次独立带缓存
func (g *GraphIndexer) extractConcurrently(ctx context.Context, chunks []*core.Chunk) ([]core.Node, []core.Edge, error) {
	const batchSize = 5

	// 先做缓存查找，分离命中和未命中
	cachedNodes, cachedEdges, uncached := g.extractCachedChunks(chunks)

	if len(uncached) == 0 {
		return cachedNodes, cachedEdges, nil
	}

	// 按 batchSize 分组
	batches := make([][]*core.Chunk, 0, (len(uncached)+batchSize-1)/batchSize)
	for i := 0; i < len(uncached); i += batchSize {
		end := min(i+batchSize, len(uncached))
		batches = append(batches, uncached[i:end])
	}

	// 并发执行每个批次
	type batchResult struct {
		nodes  []core.Node
		edges  []core.Edge
		err    error
	}
	resultCh := make(chan batchResult, len(batches))
	var wg sync.WaitGroup

	for _, batch := range batches {
		wg.Add(1)
		go func(b []*core.Chunk) {
			defer wg.Done()
			// cache 传入以支持写入；读取阶段虽有少量冗余（extractCachedChunks 已查过），
			// 但保证缓存持久化一致性，且重复 Get 是 O(1) 内存操作，开销可忽略
			nodes, edges, err := extractor.ExtractBatchWithCache(ctx, g.client, b, g.cache)
			resultCh <- batchResult{nodes: nodes, edges: edges, err: err}
		}(batch)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集结果
	allNodes := cachedNodes
	allEdges := cachedEdges
	for result := range resultCh {
		if result.err != nil {
			return nil, nil, result.err
		}
		allNodes = append(allNodes, result.nodes...)
		allEdges = append(allEdges, result.edges...)
	}

	return allNodes, allEdges, nil
}

// extractCachedChunks 从 chunks 中分离缓存命中和未命中的
func (g *GraphIndexer) extractCachedChunks(chunks []*core.Chunk) (nodes []core.Node, edges []core.Edge, uncached []*core.Chunk) {
	if g.cache == nil {
		return nil, nil, chunks
	}

	for _, chunk := range chunks {
		hashKey := extractor.ContentHash(chunk.Content)
		ext, _ := extractor.GetCachedExtraction(g.cache, hashKey)
		if ext != nil {
			n, e := extractor.BuildFromExtraction(ext, chunk)
			nodes = append(nodes, n...)
			edges = append(edges, e...)
		} else {
			uncached = append(uncached, chunk)
		}
	}
	return
}

// storeBatchConcurrent 并发写入图数据库
// 将 nodes 和 edges 分组后并发写入，适用于大批量数据
func (g *GraphIndexer) storeBatchConcurrent(ctx context.Context, nodes []core.Node, edges []core.Edge) error {
	if len(nodes) == 0 && len(edges) == 0 {
		return nil
	}

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs []error
	)

	// 并发写入节点（按批次分组，每批最多 500 个）
	nodeBatchSize := 500
	nodeBatches := splitNodes(nodes, nodeBatchSize)
	for _, batch := range nodeBatches {
		ptrs := make([]*core.Node, len(batch))
		for i := range batch {
			ptrs[i] = &batch[i]
		}
		wg.Add(1)
		go func(ps []*core.Node) {
			defer wg.Done()
			if err := g.store.UpsertNodes(ctx, ps); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to upsert nodes: %w", err))
				mu.Unlock()
			}
		}(ptrs)
	}

	// 并发写入边
	edgeBatchSize := 500
	edgeBatches := splitEdges(edges, edgeBatchSize)
	for _, batch := range edgeBatches {
		ptrs := make([]*core.Edge, len(batch))
		for i := range batch {
			ptrs[i] = &batch[i]
		}
		wg.Add(1)
		go func(ps []*core.Edge) {
			defer wg.Done()
			if err := g.store.UpsertEdges(ctx, ps); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to upsert edges: %w", err))
				mu.Unlock()
			}
		}(ptrs)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("%d store errors occurred, first: %w", len(errs), errs[0])
	}
	return nil
}

// splitSlice 将切片按 batchSize 分组，Go 泛型避免 splitNodes/splitEdges 代码重复
func splitSlice[S ~[]E, E any](s S, batchSize int) []S {
	if len(s) <= batchSize {
		return []S{s}
	}
	n := (len(s) + batchSize - 1) / batchSize
	batches := make([]S, 0, n)
	for i := 0; i < len(s); i += batchSize {
		end := min(i+batchSize, len(s))
		batches = append(batches, s[i:end])
	}
	return batches
}

func splitNodes(nodes []core.Node, batchSize int) [][]core.Node {
	return splitSlice(nodes, batchSize)
}

func splitEdges(edges []core.Edge, batchSize int) [][]core.Edge {
	return splitSlice(edges, batchSize)
}

// expandMergedNodes 对合并块的 Node/Edge 扩展 SourceChunkIDs
// 如果 chunk 是合并块（metadata.is_merged=true），将 SourceChunkIDs 扩展到所有原始子块
func expandMergedNodes(nodes []core.Node, edges []core.Edge, chunks []*core.Chunk) ([]core.Node, []core.Edge) {
	// 建立 chunkID -> 原始子块 IDs 的映射
	mergedMap := make(map[string][]string)
	for _, c := range chunks {
		if merged, ok := c.Metadata["is_merged"].(bool); ok && merged {
			if childIDs, ok := c.Metadata["merged_from"].([]string); ok {
				mergedMap[c.ID] = childIDs
			}
		}
	}
	if len(mergedMap) == 0 {
		return nodes, edges
	}

	// 扩展节点的 SourceChunkIDs 和 SourceDocIDs
	for i := range nodes {
		if len(nodes[i].SourceChunkIDs) == 0 {
			continue
		}
		extraIDs := mergedMap[nodes[i].SourceChunkIDs[0]]
		if len(extraIDs) > 0 {
			seen := make(map[string]bool)
			for _, id := range nodes[i].SourceChunkIDs {
				seen[id] = true
			}
			for _, id := range extraIDs {
				if !seen[id] {
					nodes[i].SourceChunkIDs = append(nodes[i].SourceChunkIDs, id)
					seen[id] = true
				}
			}
		}
	}

	// 扩展边的 SourceChunkIDs
	for i := range edges {
		if len(edges[i].SourceChunkIDs) == 0 {
			continue
		}
		extraIDs := mergedMap[edges[i].SourceChunkIDs[0]]
		if len(extraIDs) > 0 {
			seen := make(map[string]bool)
			for _, id := range edges[i].SourceChunkIDs {
				seen[id] = true
			}
			for _, id := range extraIDs {
				if !seen[id] {
					edges[i].SourceChunkIDs = append(edges[i].SourceChunkIDs, id)
					seen[id] = true
				}
			}
		}
	}

	return nodes, edges
}

// Ensure implementation of core.ChunkIndexer interface
var _ core.ChunkIndexer = (*GraphIndexer)(nil)

// Close 关闭图存储和缓存
func (g *GraphIndexer) Close(ctx context.Context) error {
	if g.cache != nil {
		_ = g.cache.Close()
	}
	return g.store.Close(ctx)
}

// DB 返回图存储数据库
func (g *GraphIndexer) DB() core.GraphStore {
	return g.store
}

// buildChunk 内部方法：从 chunk 提取实体关系并存储
func (g *GraphIndexer) buildChunk(ctx context.Context, chunk *core.Chunk) error {
	if g.client == nil {
		return nil // 未设置 client 时跳过
	}

	// 1. 使用静态方法提取实体和关系
	nodes, edges, err := extractor.ExtractNodesAndEdges(ctx, g.client, chunk)
	if err != nil {
		return err
	}

	// 2. 转换为指针切片
	nodePtrs := make([]*core.Node, len(nodes))
	for i := range nodes {
		nodePtrs[i] = &nodes[i]
	}

	edgePtrs := make([]*core.Edge, len(edges))
	for i := range edges {
		edgePtrs[i] = &edges[i]
	}

	// 3. 存储到图数据库
	if len(nodePtrs) > 0 {
		if err := g.store.UpsertNodes(ctx, nodePtrs); err != nil {
			return err
		}
	}
	if len(edgePtrs) > 0 {
		if err := g.store.UpsertEdges(ctx, edgePtrs); err != nil {
			return err
		}
	}

	return nil
}

// scoreGraphResult 基于 nodes 和 edges 的质量计算相关性分数
// 考虑因素：实体数量、关系数量、关系强度(score)、实体频率(frequency)
func scoreGraphResult(nodes []*core.Node, edges []*core.Edge) float32 {
	if len(nodes) == 0 {
		return 0
	}

	// 基础分
	baseScore := float32(0.3)

	// 实体贡献：每个实体 +0.05
	entityBonus := float32(len(nodes)) * 0.05

	// 关系贡献：每条关系 +0.03
	relationBonus := float32(len(edges)) * 0.03

	// 关系强度加成：边的 score 属性（如果存在）
	edgeScoreSum := float32(0)
	edgeScoreCount := 0
	for _, edge := range edges {
		if edge.Properties != nil {
			if s, ok := edge.Properties["score"].(float64); ok {
				edgeScoreSum += float32(s)
				edgeScoreCount++
			}
		}
	}
	strengthBonus := float32(0)
	if edgeScoreCount > 0 {
		avgStrength := edgeScoreSum / float32(edgeScoreCount)
		strengthBonus = avgStrength * 0.1
	}

	// 实体频率加成：高频实体更相关
	freqBonus := float32(0)
	for _, node := range nodes {
		if node.Properties != nil {
			if f, ok := node.Properties["frequency"].(int); ok && f > 0 {
				freqBonus += float32(math.Min(float64(f), 10)) * 0.01
			}
		}
	}

	total := baseScore + entityBonus + relationBonus + strengthBonus + freqBonus
	if total > 1.0 {
		return 1.0
	}
	return total
}

// =============================================================================
// 优化1：并发提取 — isLocalClient 判断 LLM 是否为本地部署
// =============================================================================

// isLocalClient 检测 LLM 客户端是否指向本地服务（如 Ollama、LM Studio 等）
// 本地 LLM 通常是串行处理的，并发调用会导致排队或失败
func isLocalClient(client chat.Client) bool {
	// 尝试通过 BaseClient 获取 BaseURL
	type configAccessor interface {
		Config() chat.Config
	}
	if ca, ok := client.(configAccessor); ok {
		baseURL := ca.Config().BaseURL
		u, err := url.Parse(baseURL)
		if err != nil {
			return false
		}
		host := strings.ToLower(u.Hostname())
		if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
			return true
		}
		// RFC 1918 私有地址：10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
		if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") {
			return true
		}
		if strings.HasPrefix(host, "172.") {
			// 172.16.0.0 - 172.31.255.255
			parts := strings.SplitN(host, ".", 3)
			if len(parts) >= 2 {
				minor := 0
				for _, c := range parts[1] {
					if c >= '0' && c <= '9' {
						minor = minor*10 + int(c-'0')
					} else {
						break
					}
				}
				return minor >= 16 && minor <= 31
			}
		}
		return false
	}
	return false
}

// =============================================================================
// 优化2：大块优先 — 聚合子块到父块做实体提取
// =============================================================================

// parentGroup 表示一组共享同一父块的子块
type parentGroup struct {
	parentID string
	docID    string
	chunks   []*core.Chunk
}

// mergeToParentChunks 将有 ParentID 的子块聚合到父块级别
// 无 ParentID 的 chunk 保持不变
// 返回聚合后的 chunk 列表（每个"父块"包含所有子块内容合并）
func mergeToParentChunks(chunks []*core.Chunk) []*core.Chunk {
	// 分离有父块和无父块的 chunk
	orphanChunks := make([]*core.Chunk, 0)
	parentGroups := make(map[string]*parentGroup)
	groupOrder := make([]string, 0)

	for _, c := range chunks {
		if c.ParentID == "" {
			orphanChunks = append(orphanChunks, c)
			continue
		}
		key := c.ParentID + "|" + c.DocID
		group, ok := parentGroups[key]
		if !ok {
			group = &parentGroup{
				parentID: c.ParentID,
				docID:    c.DocID,
			}
			parentGroups[key] = group
			groupOrder = append(groupOrder, key)
		}
		group.chunks = append(group.chunks, c)
	}

	// 无父块子块可以直接返回
	if len(parentGroups) == 0 {
		return chunks
	}

	// 为每个父块组构建聚合 chunk
	merged := make([]*core.Chunk, 0, len(orphanChunks)+len(parentGroups))
	merged = append(merged, orphanChunks...)

	for _, key := range groupOrder {
		group := parentGroups[key]
		if len(group.chunks) == 0 {
			continue
		}

		// 按 ChunkMeta.Index 排序子块
		sortChunksByIndex(group.chunks)

		var sb strings.Builder
		childIDs := make([]string, 0, len(group.chunks))
		for _, child := range group.chunks {
			if child.Content != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				sb.WriteString(child.Content)
			}
			childIDs = append(childIDs, child.ID)
		}

		if sb.Len() == 0 {
			continue
		}

		// 合并 ChunkMeta 信息（取第一个子块的 heading 信息）
		firstMeta := group.chunks[0].ChunkMeta

		merged = append(merged, &core.Chunk{
			ID:       group.parentID,
			ParentID: "",
			DocID:    group.docID,
			Content:  sb.String(),
			Metadata: map[string]any{
				"merged_from":    childIDs,
				"child_count":    len(group.chunks),
				"is_merged":      true,
				"heading_level":  firstMeta.HeadingLevel,
				"heading_path":   firstMeta.HeadingPath,
			},
			ChunkMeta: core.ChunkMeta{
				Index:        firstMeta.Index,
				StartPos:     firstMeta.StartPos,
				EndPos:       group.chunks[len(group.chunks)-1].ChunkMeta.EndPos,
				HeadingLevel: firstMeta.HeadingLevel,
				HeadingPath:  firstMeta.HeadingPath,
			},
		})
	}

	return merged
}

func sortChunksByIndex(chunks []*core.Chunk) {
	// 简单冒泡排序（chunk 数量通常不多）
	for i := 0; i < len(chunks)-1; i++ {
		for j := i + 1; j < len(chunks); j++ {
			if chunks[i].ChunkMeta.Index > chunks[j].ChunkMeta.Index {
				chunks[i], chunks[j] = chunks[j], chunks[i]
			}
		}
	}
}

// =============================================================================
// 优化3：分级提取 — 评估 chunk 的实体提取价值
// =============================================================================

// chunkValueScore 评估 chunk 的实体提取价值 (0-1)
// 综合考虑：标题层级、内容长度、实体密度信号
// 返回值越高的 chunk 越值得用 LLM 提取
func chunkValueScore(chunk *core.Chunk) float64 {
	if chunk == nil || chunk.Content == "" {
		return 0
	}

	content := chunk.Content
	contentLen := len([]rune(content))
	score := 0.0

	// 1. 内容长度评分：太短（<50字）价值低，适中（200-2000字）价值最高
	switch {
	case contentLen < 50:
		score += 0.05
	case contentLen < 100:
		score += 0.15
	case contentLen < 200:
		score += 0.3
	case contentLen <= 2000:
		score += 0.5
	default:
		score += 0.4 // 超长文本可能包含大量噪音
	}

	// 2. 标题层级评分：标题附近的内容通常更有价值
	if chunk.ChunkMeta.HeadingLevel > 0 && len(chunk.ChunkMeta.HeadingPath) > 0 {
		score += 0.2
	}

	// 3. 实体密度信号（快速启发式，不调 LLM）
	// 检查常见实体模式的出现频率
	entitySignals := countEntitySignals(content)
	signalDensity := float64(entitySignals) / float64(contentLen) * 100

	switch {
	case signalDensity > 5:
		score += 0.3
	case signalDensity > 2:
		score += 0.2
	case signalDensity > 0.5:
		score += 0.1
	}

	// 4. 合并块加成：已合并的父块内容更完整
	if merged, ok := chunk.Metadata["is_merged"].(bool); ok && merged {
		score += 0.15
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}

// entitySignals 用于快速统计实体密度信号的常量模式
// 初始化时预转为小写，避免每次调用时重复 ToLower
var entitySignals = func() []string {
	raw := []string{
		"先生", "女士", "教授", "博士", "经理", "总裁", "部长", "院长",
		"公司", "集团", "大学", "研究所", "医院", "银行", "部门",
		"中国", "美国", "日本", "德国", "法国", "英国", "韩国",
		" Inc.", " Corp.", " Ltd.", " LLC.", " Co.",
		"University", "Institute", "Laboratory",
	}
	lowered := make([]string, len(raw))
	for i, s := range raw {
		lowered[i] = strings.ToLower(s)
	}
	return lowered
}()

// countEntitySignals 统计内容中实体信号的出现次数
func countEntitySignals(content string) int {
	count := 0
	lower := strings.ToLower(content)
	for _, signal := range entitySignals {
		count += strings.Count(lower, signal)
	}
	return count
}
