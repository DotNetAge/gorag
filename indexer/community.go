package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/extractor"
)

// CommunityIndexer 社区索引器
// 负责：社区检测 → 摘要生成 → 存储 → Global Search 查询
type CommunityIndexer struct {
	store    core.GraphStore
	client   chat.Client       // LLM client，用于社区摘要生成
	detector core.CommunityDetector // 社区检测算法
}

// NewCommunityIndexer 创建社区索引器
// detector 参数可选，如果为 nil 则使用默认的连通分量检测器
func NewCommunityIndexer(store core.GraphStore, client chat.Client, detector ...core.CommunityDetector) *CommunityIndexer {
	ci := &CommunityIndexer{
		store:  store,
		client: client,
	}
	if len(detector) > 0 && detector[0] != nil {
		ci.detector = detector[0]
	}
	return ci
}

// Build 执行完整的社区构建流程
// 1. 社区检测（纯图算法）
// 2. 社区摘要生成（LLM）
// 3. 摘要存储到图数据库
// maxLevel: 层次聚类最大深度（0 = 单层）
func (ci *CommunityIndexer) Build(ctx context.Context, maxLevel int) error {
	if ci.client == nil {
		return fmt.Errorf("client is required for community summarization")
	}

	// 1. 社区检测
	communities, err := ci.detect(ctx)
	if err != nil {
		return fmt.Errorf("community detection failed: %w", err)
	}

	if len(communities) == 0 {
		return nil
	}

	// 2. 收集所有节点和边
	allNodeIDs := make([]string, 0)
	nodeIDSet := make(map[string]bool)
	allEdgeIDs := make([]string, 0)
	edgeIDSet := make(map[string]bool)
	for _, c := range communities {
		for _, nid := range c.NodeIDs {
			if !nodeIDSet[nid] {
				nodeIDSet[nid] = true
				allNodeIDs = append(allNodeIDs, nid)
			}
		}
		for _, eid := range c.EdgeIDs {
			if !edgeIDSet[eid] {
				edgeIDSet[eid] = true
				allEdgeIDs = append(allEdgeIDs, eid)
			}
		}
	}

	var allNodes []*core.Node
	if len(allNodeIDs) > 0 {
		// 通过 Cypher 查询所有节点
		rows, err := ci.store.Query(ctx, `MATCH (n) WHERE n.ID IN $ids RETURN n`, map[string]any{"ids": allNodeIDs})
		if err == nil {
			allNodes = ci.rowsToNodes(rows)
		}
	}

	var allEdges []*core.Edge
	if len(allEdgeIDs) > 0 {
		// 通过 Cypher 查询所有边
		edgeRows, err := ci.store.Query(ctx, `MATCH ()-[r]->() WHERE r.ID IN $ids RETURN r`, map[string]any{"ids": allEdgeIDs})
		if err == nil {
			allEdges = ci.rowsToEdges(edgeRows)
		}
	}

	// 3. 批量生成摘要
	communities, err = extractor.SummarizeMultipleCommunities(ctx, ci.client, communities, allNodes, allEdges)
	if err != nil {
		return fmt.Errorf("community summarization failed: %w", err)
	}

	// 4. 存储社区摘要到图数据库
	for _, c := range communities {
		if err := ci.storeCommunity(ctx, c); err != nil {
			return fmt.Errorf("failed to store community %s: %w", c.ID, err)
		}
	}

	return nil
}

// detect 执行社区检测
func (ci *CommunityIndexer) detect(ctx context.Context) ([]*core.Community, error) {
	if ci.detector != nil {
		return ci.detector.Detect(ctx, ci.store)
	}

	// 默认实现：使用 Cypher 查询连通分量
	return ci.defaultDetect(ctx)
}

// defaultDetect 默认社区检测：查询所有实体并按关系连通性分组
func (ci *CommunityIndexer) defaultDetect(ctx context.Context) ([]*core.Community, error) {
	// 查询所有节点及其 ID
	rows, err := ci.store.Query(ctx, `MATCH (n) WHERE n.ID IS NOT NULL RETURN n.ID AS id, n.name AS name`, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	// 构建邻接表
	nodeIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if id, ok := row["id"].(string); ok && id != "" {
			nodeIDs = append(nodeIDs, id)
		}
	}

	if len(nodeIDs) == 0 {
		return nil, nil
	}

	// 查询所有边来构建邻接表
	adjacency := make(map[string][]string)
	nodeEdgeMap := make(map[string][]string) // nodeID → edgeIDs
	for _, id := range nodeIDs {
		adjacency[id] = make([]string, 0)
		nodeEdgeMap[id] = make([]string, 0)
	}

	edgeRows, err := ci.store.Query(ctx, `
		MATCH (a)-[r]->(b)
		WHERE a.ID IS NOT NULL AND b.ID IS NOT NULL
		RETURN a.ID AS source, b.ID AS target, r.ID AS edge_id
	`, nil)
	if err == nil {
		for _, row := range edgeRows {
			src, _ := row["source"].(string)
			tgt, _ := row["target"].(string)
			edgeID, _ := row["edge_id"].(string)
			if src != "" && tgt != "" {
				if _, ok := adjacency[src]; ok {
					adjacency[src] = append(adjacency[src], tgt)
				}
				if _, ok := adjacency[tgt]; ok {
					adjacency[tgt] = append(adjacency[tgt], src)
				}
				// 记录边到两个端点
				if edgeID != "" {
					nodeEdgeMap[src] = append(nodeEdgeMap[src], edgeID)
					nodeEdgeMap[tgt] = append(nodeEdgeMap[tgt], edgeID)
				}
			}
		}
	}

	// BFS 找连通分量
	visited := make(map[string]bool)
	var communities []*core.Community
	communityID := 0

	for _, startID := range nodeIDs {
		if visited[startID] {
			continue
		}

		// BFS
		componentNodes := make([]string, 0)
		edgeIDSet := make(map[string]bool)
		queue := []string{startID}
		visited[startID] = true

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			componentNodes = append(componentNodes, current)

			for _, eID := range nodeEdgeMap[current] {
				edgeIDSet[eID] = true
			}

			for _, neighbor := range adjacency[current] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}

		if len(componentNodes) > 0 {
			communityID++
			edgeIDs := make([]string, 0, len(edgeIDSet))
			for eID := range edgeIDSet {
				edgeIDs = append(edgeIDs, eID)
			}
			communities = append(communities, &core.Community{
				ID:      fmt.Sprintf("community_%d", communityID),
				Level:   0,
				NodeIDs: componentNodes,
				EdgeIDs: edgeIDs,
			})
		}
	}

	return communities, nil
}

// storeCommunity 将社区摘要存储到图数据库
func (ci *CommunityIndexer) storeCommunity(ctx context.Context, community *core.Community) error {
	// 将社区作为 Community 节点存储
	props := make(map[string]any)
	props["summary"] = community.Summary
	props["level"] = community.Level
	if len(community.Keywords) > 0 {
		props["keywords"] = strings.Join(community.Keywords, ",")
	}
	if len(community.NodeIDs) > 0 {
		props["nodes"] = strings.Join(community.NodeIDs, ",")
	}
	if len(community.SourceChunkIDs) > 0 {
		props["source_chunk_ids"] = community.SourceChunkIDs
	}

	// 使用 MERGE 创建或更新 Community 节点
	// 动态构建 SET 子句，仅更新非零值字段，避免 nil 覆盖已有属性
	setClauses := []string{"c.summary = $summary", "c.level = $level"}
	params := map[string]any{
		"id":      community.ID,
		"summary": community.Summary,
		"level":   community.Level,
	}
	if v, ok := props["keywords"]; ok {
		setClauses = append(setClauses, "c.keywords = $keywords")
		params["keywords"] = v
	}
	if v, ok := props["nodes"]; ok {
		setClauses = append(setClauses, "c.nodes = $nodes")
		params["nodes"] = v
	}
	if v, ok := props["source_chunk_ids"]; ok {
		setClauses = append(setClauses, "c.source_chunk_ids = $source_chunk_ids")
		params["source_chunk_ids"] = v
	}
	cypher := fmt.Sprintf("MERGE (c:Community {id: $id}) SET %s", strings.Join(setClauses, ", "))
	_, err := ci.store.Query(ctx, cypher, params)
	return err
}

// SearchGlobal 通过社区摘要回答全局性问题 (Global Search)
// 流程：获取社区摘要 → LLM Map-Reduce 生成答案
// 返回匹配的社区信息，不生成最终答案（答案生成由调用方完成）
func (ci *CommunityIndexer) SearchGlobal(ctx context.Context, query string, level int) ([]core.CommunityMatch, error) {
	// 1. 获取所有社区摘要
	summaries, err := ci.store.GetCommunitySummaries(ctx, level)
	if err != nil {
		return nil, fmt.Errorf("failed to get community summaries: %w", err)
	}

	if len(summaries) == 0 {
		return nil, nil
	}

	if ci.client == nil {
		// 没有 client 时，返回所有社区（无排序）
		matches := make([]core.CommunityMatch, len(summaries))
		for i, s := range summaries {
			matches[i] = core.CommunityMatch{
				CommunityID: idAsString(s["id"]),
				Score:       1.0,
				Summary:     idAsString(s["summary"]),
				Keywords:    splitKeywords(s["keywords"]),
			}
		}
		return matches, nil
	}

	// 2. 用 LLM 对社区摘要进行相关性排序
	return ci.rankCommunities(ctx, query, summaries)
}

// rankCommunities 使用 LLM 对社区摘要进行相关性排序
func (ci *CommunityIndexer) rankCommunities(ctx context.Context, query string, summaries []map[string]any) ([]core.CommunityMatch, error) {
	// 构建社区摘要列表文本
	communityTexts := make([]string, 0, len(summaries))
	for i, s := range summaries {
		summary := idAsString(s["summary"])
		keywords := splitKeywords(s["keywords"])
		kwStr := strings.Join(keywords, ", ")
		communityTexts = append(communityTexts,
			fmt.Sprintf("[%d] Summary: %s | Keywords: %s", i+1, summary, kwStr))
	}

	prompt := fmt.Sprintf(`You are a community relevance ranking expert. Given a user query and a list of community summaries, rank the communities by relevance.

IMPORTANT: You MUST output in the same language as the user query.

## User Query
%s

## Community Summaries
%s

## Task
Select the communities most relevant to the query. Return their indices (1-based) in order of relevance.

## Output Format
Output strictly in the following JSON format:
{
  "relevant": [
    {"index": 1, "reason": "brief reason why relevant"},
    {"index": 3, "reason": "brief reason why relevant"}
  ]
}

Only include communities with relevance score > 0.3. Return at most 5 communities.
Output the JSON directly:`, query, strings.Join(communityTexts, "\n\n"))

	messages := []chat.Message{
		chat.NewSystemMessage(prompt),
	}

	response, err := ci.client.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("community ranking failed: %w", err)
	}

	// 解析排名结果
	var result struct {
		Relevant []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"relevant"`
	}

	jsonStr := strings.TrimSpace(response.Content)
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimPrefix(jsonStr, "```")
	jsonStr = strings.TrimSuffix(jsonStr, "```")
	jsonStr = strings.TrimSpace(jsonStr)

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse ranking result: %w", err)
	}

	// 构建匹配结果
	matches := make([]core.CommunityMatch, 0, len(result.Relevant))
	totalRelevant := float32(len(result.Relevant))
	for i, r := range result.Relevant {
		idx := r.Index - 1 // 转为 0-based
		if idx < 0 || idx >= len(summaries) {
			continue
		}
		s := summaries[idx]
		// 相关性分数：排名越前分数越高
		score := float32(0.5) + float32(totalRelevant-float32(i))/float32(totalRelevant)*0.5
		if score > 1.0 {
			score = 1.0
		}
		matches = append(matches, core.CommunityMatch{
			CommunityID: idAsString(s["id"]),
			Score:       score,
			Summary:     idAsString(s["summary"]),
			Keywords:    splitKeywords(s["keywords"]),
		})
	}

	return matches, nil
}

// rowsToNodes 将查询行转换为 Node 列表
func (ci *CommunityIndexer) rowsToNodes(rows []map[string]any) []*core.Node {
	nodes := make([]*core.Node, 0, len(rows))
	for _, row := range rows {
		n, ok := row["n"].(map[string]any)
		if !ok {
			continue
		}
		props, _ := n["properties"].(map[string]any)
		if props == nil {
			continue
		}

		var id, name string
		if idVal, ok := props["ID"].(string); ok {
			id = idVal
		}
		if nameVal, ok := props["name"].(string); ok {
			name = nameVal
		}

		nodeType := ""
		if labels, ok := n["labels"].([]string); ok && len(labels) > 0 {
			nodeType = labels[0]
		}

		var sourceChunkIDs, sourceDocIDs []string
		if cids, ok := props["source_chunk_ids"].([]string); ok {
			sourceChunkIDs = cids
		}
		if dids, ok := props["source_doc_ids"].([]string); ok {
			sourceDocIDs = dids
		}

		// 清理内部字段
		delete(props, "ID")
		delete(props, "name")
		delete(props, "source_chunk_ids")
		delete(props, "source_doc_ids")

		nodes = append(nodes, &core.Node{
			ID:             id,
			Type:           nodeType,
			Name:           name,
			Properties:     props,
			SourceChunkIDs: sourceChunkIDs,
			SourceDocIDs:   sourceDocIDs,
		})
	}
	return nodes
}

// rowsToEdges 将查询行转换为 Edge 列表
func (ci *CommunityIndexer) rowsToEdges(rows []map[string]any) []*core.Edge {
	edges := make([]*core.Edge, 0, len(rows))
	for _, row := range rows {
		r, ok := row["r"].(map[string]any)
		if !ok {
			continue
		}

		props, _ := r["properties"].(map[string]any)
		if props == nil {
			props = make(map[string]any)
		}

		var id string
		if idVal, ok := props["ID"].(string); ok {
			id = idVal
		}

		edgeType, _ := r["type"].(string)
		predicate := getStringProp(props, "predicate")
		source, _ := r["startNodeID"].(string)
		target, _ := r["endNodeID"].(string)

		var sourceChunkIDs, sourceDocIDs []string
		if cids, ok := props["source_chunk_ids"].([]string); ok {
			sourceChunkIDs = cids
		}
		if dids, ok := props["source_doc_ids"].([]string); ok {
			sourceDocIDs = dids
		}

		// 清理内部字段
		delete(props, "ID")
		delete(props, "predicate")
		delete(props, "source_chunk_ids")
		delete(props, "source_doc_ids")

		edges = append(edges, &core.Edge{
			ID:             id,
			Type:           edgeType,
			Source:         source,
			Target:         target,
			Predicate:      predicate,
			Properties:     props,
			SourceChunkIDs: sourceChunkIDs,
			SourceDocIDs:   sourceDocIDs,
		})
	}
	return edges
}

// getStringProp 安全提取 string 属性（避免与外部同名函数冲突）
func getStringProp(props map[string]any, key string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
func idAsString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// splitKeywords 将关键词字符串转为 []string
func splitKeywords(v any) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		// 支持逗号分隔
		parts := strings.Split(val, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	case []string:
		return val
	}
	return nil
}
