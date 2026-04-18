package indexer

import (
	"context"
	"encoding/json"
	"fmt"
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
	client chat.Client // LLM client，用于独立 GraphRAG 模式的实体提取
}

// NewGraphIndexer 创建知识图谱索引器
// client 参数可选：
//   - 提供 client：支持独立 GraphRAG 模式（索引 + 查询都用 LLM）
//   - 不提供 client：仅支持混合模式（索引时不用 LLM，查询时通过 Chunk IDs）
func NewGraphIndexer(store core.GraphStore, client ...chat.Client) *GraphIndexer {
	g := &GraphIndexer{
		store: store,
	}
	if len(client) > 0 && client[0] != nil {
		g.client = client[0]
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

	// 基于匹配实体数和关系数计算相关性分数
	baseScore := float32(0.5)
	entityBonus := float32(len(entities)) * 0.05
	relationBonus := float32(len(relations)) * 0.02
	graphScore := baseScore + entityBonus + relationBonus
	if graphScore > 1.0 {
		graphScore = 1.0
	}

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
			for _, cid := range e.SourceChunkIDs {
				if cid == chunkID {
					docEntities = append(docEntities, e)
					break
				}
			}
		}

		docRelations := make([]*core.Edge, 0)
		for _, r := range relations {
			for _, cid := range r.SourceChunkIDs {
				if cid == chunkID {
					docRelations = append(docRelations, r)
					break
				}
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
		fallbackScore := float32(0.5)
		if len(entities) > 0 {
			fallbackScore += float32(len(entities)) * 0.05
			if fallbackScore > 1.0 {
				fallbackScore = 1.0
			}
		}
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
// 流程：Chunk IDs → 查询关联 Nodes/Edges → 图遍历 → Hit
func (g *GraphIndexer) SearchByChunkIDs(ctx context.Context, chunkIDs []string, depth, limit int) ([]core.Hit, error) {
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

	// 2. 查询关联的 Edges
	edges, err := g.store.GetEdgesByChunkIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges by chunk IDs: %w", err)
	}

	// 3. 并发获取邻居节点（如果 depth > 0）
	var relations []*core.Edge
	if depth > 0 {
		type edgeResult struct {
			edges []*core.Edge
			err   error
		}
		edgeCh := make(chan edgeResult, len(nodes))
		var wg sync.WaitGroup
		for _, node := range nodes {
			wg.Add(1)
			go func(nodeID string) {
				defer wg.Done()
				_, neighborEdges, err := g.store.GetNeighbors(ctx, nodeID, depth, limit)
				edgeCh <- edgeResult{edges: neighborEdges, err: err}
			}(node.ID)
		}

		go func() {
			wg.Wait()
			close(edgeCh)
		}()

		for result := range edgeCh {
			if result.err == nil {
				relations = append(relations, result.edges...)
			}
		}
	} else {
		relations = edges
	}

	// 4. 收集所有 Chunk IDs 和 Doc IDs
	allChunkIDs := make([]string, 0)
	allDocIDs := make([]string, 0)
	seenChunk := make(map[string]bool)
	seenDoc := make(map[string]bool)

	for _, node := range nodes {
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

	// 5. 构建 Hit
	hits := make([]core.Hit, 0, len(chunkIDs))
	seenChunkHit := make(map[string]bool)

	// 基于节点数和关系数计算相关性分数
	baseScore := float32(0.5)
	nodeBonus := float32(len(nodes)) * 0.05
	relationBonus := float32(len(relations)) * 0.02
	graphScore := baseScore + nodeBonus + relationBonus
	if graphScore > 1.0 {
		graphScore = 1.0
	}

	for _, chunkID := range chunkIDs {
		if seenChunkHit[chunkID] {
			continue
		}
		seenChunkHit[chunkID] = true

		// 查找关联的 docID
		docID := ""
		for _, node := range nodes {
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

		// 筛选该 chunk 关联的 nodes 和 edges
		chunkNodes := make([]*core.Node, 0)
		for _, node := range nodes {
			for _, cid := range node.SourceChunkIDs {
				if cid == chunkID {
					chunkNodes = append(chunkNodes, node)
					break
				}
			}
		}

		chunkEdges := make([]*core.Edge, 0)
		for _, edge := range relations {
			for _, cid := range edge.SourceChunkIDs {
				if cid == chunkID {
					chunkEdges = append(chunkEdges, edge)
					break
				}
			}
		}

		// 构建结果
		result := GraphSearchResult{
			Entities:  chunkNodes,
			Relations: chunkEdges,
			ChunkIDs:  []string{chunkID},
			DocIDs:    []string{docID},
		}

		content, _ := json.Marshal(result)
		hits = append(hits, core.Hit{
			ID:      chunkID,
			Score:   graphScore,
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

// IndexChunk indexes a pre-generated chunk (implements core.Indexer interface)
func (g *GraphIndexer) IndexChunk(ctx context.Context, chunk *core.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk cannot be nil")
	}
	return g.buildChunk(ctx, chunk)
}

// IndexChunks indexes multiple pre-generated chunks in batch (implements core.ChunkIndexer interface)
func (g *GraphIndexer) IndexChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	for _, chunk := range chunks {
		if err := g.buildChunk(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

// Ensure implementation of core.ChunkIndexer interface
var _ core.ChunkIndexer = (*GraphIndexer)(nil)

// Close 关闭图存储
func (g *GraphIndexer) Close(ctx context.Context) error {
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
