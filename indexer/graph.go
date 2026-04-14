package indexer

import (
	"context"
	"encoding/json"

	"github.com/DotNetAge/gorag/core"
)

// GraphSearchResult 图搜索结果，用于序列化为 Hit.Content
type GraphSearchResult struct {
	Query     string       `json:"query,omitempty"`     // 原始查询
	Entities  []*core.Node `json:"entities,omitempty"`  // 匹配的实体节点
	Relations []*core.Edge `json:"relations,omitempty"` // 关联的边
	ChunkIDs  []string     `json:"chunk_ids,omitempty"` // 关联的 chunk ID 列表
	DocIDs    []string     `json:"doc_ids,omitempty"`   // 关联的文档 ID 列表
}

// graphIndexer 知识图谱索引器
// 与语义/全文索引器不同，GraphRAG 的索引是构建实体-关系图，查询是图遍历
type graphIndexer struct {
	store     core.GraphStore
	extractor core.GraphExtractor
}

// NewGraphIndexer 创建知识图谱索引器
func NewGraphIndexer(store core.GraphStore) *graphIndexer {
	return &graphIndexer{
		store:     store,
		extractor: nil, // 待注入 GraphExtractor 实现
	}
}

// SetExtractor 设置实体关系提取器
func (g *graphIndexer) SetExtractor(extractor core.GraphExtractor) {
	g.extractor = extractor
}

// Name 返回索引器名称
func (g *graphIndexer) Name() string {
	return "graph"
}

// Type 返回索引器类型
func (g *graphIndexer) Type() string {
	return "graph"
}

// Add 从内容构建知识图谱（实现 core.Indexer 接口）
// 流程：分块 → 实体关系提取 → 图存储
func (g *graphIndexer) Add(ctx context.Context, content string) (*core.Chunk, error) {
	chunks, err := GetChunks(content)
	if err != nil {
		return nil, err
	}
	for _, chunk := range chunks {
		if err := g.buildChunk(ctx, chunk); err != nil {
			return nil, err
		}
	}
	if len(chunks) > 0 {
		return chunks[0], nil
	}
	return nil, nil
}

// AddFile 从文件构建知识图谱（实现 core.Indexer 接口）
func (g *graphIndexer) AddFile(ctx context.Context, filePath string) (*core.Chunk, error) {
	chunks, err := GetFileChunks(filePath)
	if err != nil {
		return nil, err
	}
	for _, chunk := range chunks {
		if err := g.buildChunk(ctx, chunk); err != nil {
			return nil, err
		}
	}
	if len(chunks) > 0 {
		return chunks[0], nil
	}
	return nil, nil
}

// NewQuery 创建图查询（实现 core.Indexer 接口）
func (g *graphIndexer) NewQuery(terms string) core.Query {
	return GraphQuery(terms)
}

// Search 执行图搜索（实现 core.Indexer 接口）
// 流程：实体提取 → 图遍历 → 节点/边序列化 → Hit
func (g *graphIndexer) Search(ctx context.Context, query core.Query) ([]core.Hit, error) {
	if g.extractor == nil {
		return nil, nil
	}

	// 1. 从查询中提取实体
	queryChunk := &core.Chunk{
		ID:      "query_" + query.Raw(),
		Content: query.Raw(),
	}

	entityNodes, _, err := g.extractor.Extract(ctx, queryChunk)
	if err != nil || len(entityNodes) == 0 {
		return nil, err
	}

	// 转换为指针切片
	entities := make([]*core.Node, len(entityNodes))
	for i := range entityNodes {
		entities[i] = &entityNodes[i]
	}

	// 获取 depth 和 limit（如果有）
	gq, ok := query.(*graphQuery)
	depth := 1
	limit := 10
	if ok {
		depth = gq.depth
		limit = gq.limit
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

	// 3. 获取关联的边
	relations := make([]*core.Edge, 0)
	for _, entity := range entities {
		_, edges, err := g.store.GetNeighbors(ctx, entity.ID, depth, limit)
		if err == nil {
			relations = append(relations, edges...)
		}
	}

	// 4. 构建 GraphSearchResult 并序列化为 JSON
	result := GraphSearchResult{
		Query:     query.Raw(),
		Entities:  entities,
		Relations: relations,
		ChunkIDs:  chunkIDs,
		DocIDs:    docIDs,
	}

	content, err := json.Marshal(result)
	if err != nil {
		content = []byte(`{"error": "failed to serialize graph result"}`)
	}

	// 5. 构建 Hit，按 docID 分组返回
	hits := make([]core.Hit, 0, len(docIDs))
	seenDocHit := make(map[string]bool)

	for _, docID := range docIDs {
		if seenDocHit[docID] {
			continue
		}
		seenDocHit[docID] = true

		// 筛选属于此 docID 的 entities 和 relations
		docEntities := make([]*core.Node, 0)
		for _, e := range entities {
			for _, sid := range e.SourceDocIDs {
				if sid == docID {
					docEntities = append(docEntities, e)
					break
				}
			}
		}

		docRelations := make([]*core.Edge, 0)
		for _, r := range relations {
			for _, sid := range r.SourceDocIDs {
				if sid == docID {
					docRelations = append(docRelations, r)
					break
				}
			}
		}

		docResult := GraphSearchResult{
			Query:     query.Raw(),
			Entities:  docEntities,
			Relations: docRelations,
			ChunkIDs:  chunkIDs,
			DocIDs:    []string{docID},
		}

		docContent, _ := json.Marshal(docResult)
		hits = append(hits, core.Hit{
			ID:      docID,
			Score:   1.0,
			DocID:   docID,
			Content: string(docContent),
		})
	}

	// 如果没有 docID，至少返回一个包含所有信息的 Hit
	if len(hits) == 0 {
		hits = append(hits, core.Hit{
			ID:      entities[0].SourceChunkIDs[0],
			Score:   1.0,
			DocID:   "",
			Content: string(content),
		})
	}

	return hits, nil
}

// Remove 移除与指定 chunk 关联的所有实体和关系（实现 core.Indexer 接口）
func (g *graphIndexer) Remove(ctx context.Context, chunkID string) error {
	q := `MATCH (n) WHERE $chunkID IN n.source_chunk_ids DETACH DELETE n`
	_, err := g.store.Query(ctx, q, map[string]any{"chunkID": chunkID})
	return err
}

// Close 关闭图存储
func (g *graphIndexer) Close(ctx context.Context) error {
	return g.store.Close(ctx)
}

// buildChunk 内部方法：从 chunk 提取实体关系并存储
func (g *graphIndexer) buildChunk(ctx context.Context, chunk *core.Chunk) error {
	if g.extractor == nil {
		return nil // 未设置提取器时跳过
	}

	// 1. 提取实体和关系
	nodes, edges, err := g.extractor.Extract(ctx, chunk)
	if err != nil {
		return err
	}

	// 2. 绑定来源信息并转换为指针切片
	nodePtrs := make([]*core.Node, len(nodes))
	for i := range nodes {
		nodes[i].SourceChunkIDs = append(nodes[i].SourceChunkIDs, chunk.ID)
		nodes[i].SourceDocIDs = append(nodes[i].SourceDocIDs, chunk.DocID)
		nodePtrs[i] = &nodes[i]
	}

	edgePtrs := make([]*core.Edge, len(edges))
	for i := range edges {
		edges[i].SourceChunkIDs = append(edges[i].SourceChunkIDs, chunk.ID)
		edges[i].SourceDocIDs = append(edges[i].SourceDocIDs, chunk.DocID)
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
