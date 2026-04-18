package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gochatcore "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
)

// SummarizeCommunity 让 LLM 为一个社区生成摘要和关键词
// 输入社区的节点和边信息，输出该社区的自然语言摘要
func SummarizeCommunity(ctx context.Context, client gochatcore.Client, community *core.Community, nodes []*core.Node, edges []*core.Edge) (string, []string, error) {
	if client == nil {
		return "", nil, fmt.Errorf("client is required")
	}
	if community == nil || len(nodes) == 0 {
		return "", nil, nil
	}

	// 构建节点和边的摘要文本
	nodeTexts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		nodeTexts = append(nodeTexts, fmt.Sprintf("[%s: %s]", n.Type, n.Name))
	}

	// 构建节点名称查找表，供边描述使用
	nodeNameMap := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeNameMap[n.ID] = n.Name
	}

	edgeTexts := make([]string, 0, len(edges))
	for _, e := range edges {
		srcName := e.Source
		if name, ok := nodeNameMap[e.Source]; ok && name != "" {
			srcName = name
		}
		tgtName := e.Target
		if name, ok := nodeNameMap[e.Target]; ok && name != "" {
			tgtName = name
		}
		edgeTexts = append(edgeTexts, fmt.Sprintf("  %s -[%s: %s]-> %s", srcName, e.Predicate, e.Type, tgtName))
	}

	prompt := fmt.Sprintf(`You are a knowledge graph community analysis expert. Summarize the following community of related entities.

IMPORTANT: You MUST output in the same language that best matches the entity names (Chinese names → Chinese output, English names → English output).

## Community Information
- Community ID: %s
- Level: %d
- Total entities: %d
- Total relationships: %d

## Entities in this community
%s

## Relationships in this community
%s

## Task
Generate:
1. A concise summary (2-4 sentences) describing what this community is about, what these entities collectively represent
2. 3-5 keywords/tags that best characterize this community

## Output Format
Output strictly in the following JSON format:
{
  "summary": "community summary here",
  "keywords": ["keyword1", "keyword2", "keyword3"]
}

Output the JSON directly:`, community.ID, community.Level, len(nodes), len(edges),
		strings.Join(nodeTexts, "\n"), strings.Join(edgeTexts, "\n"))

	messages := []gochatcore.Message{
		gochatcore.NewSystemMessage(prompt),
	}

	response, err := client.Chat(ctx, messages)
	if err != nil {
		return "", nil, fmt.Errorf("community summarization failed: %w", err)
	}

	// 解析 JSON
	var result struct {
		Summary  string   `json:"summary"`
		Keywords []string `json:"keywords"`
	}

	jsonStr := extractCommunityJSON(response.Content)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return "", nil, fmt.Errorf("failed to parse community summary: %w", err)
	}

	return result.Summary, result.Keywords, nil
}

// SummarizeMultipleCommunities 批量为多个社区生成摘要
func SummarizeMultipleCommunities(ctx context.Context, client gochatcore.Client, communities []*core.Community, nodes []*core.Node, edges []*core.Edge) ([]*core.Community, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if len(communities) == 0 {
		return nil, nil
	}

	// 构建节点和边的查找映射
	nodeMap := make(map[string]*core.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}
	edgeMap := make(map[string]*core.Edge, len(edges))
	for _, e := range edges {
		edgeMap[e.ID] = e
	}

	results := make([]*core.Community, 0, len(communities))
	for _, community := range communities {
		// 收集该社区的节点和边
		commNodes := make([]*core.Node, 0, len(community.NodeIDs))
		for _, nid := range community.NodeIDs {
			if n, ok := nodeMap[nid]; ok {
				commNodes = append(commNodes, n)
			}
		}
		commEdges := make([]*core.Edge, 0, len(community.EdgeIDs))
		for _, eid := range community.EdgeIDs {
			if e, ok := edgeMap[eid]; ok {
				commEdges = append(commEdges, e)
			}
		}

		summary, keywords, err := SummarizeCommunity(ctx, client, community, commNodes, commEdges)
		if err != nil {
			// 单个社区摘要失败不阻断整体流程
			continue
		}

		community.Summary = summary
		community.Keywords = keywords
		results = append(results, community)
	}

	return results, nil
}

// extractCommunityJSON 从响应中提取 JSON
func extractCommunityJSON(content string) string {
	var v any
	if err := json.Unmarshal([]byte(content), &v); err == nil {
		return content
	}

	// 尝试从 markdown 代码块中提取
	start := 0
	if idx := strings.Index(content, "```json"); idx >= 0 {
		start = idx + 7
	} else if idx := strings.Index(content, "```"); idx >= 0 {
		start = idx + 3
	}

	end := len(content)
	if idx := strings.Index(content[start:], "```"); idx >= 0 {
		end = start + idx
	}

	return content[start:end]
}
