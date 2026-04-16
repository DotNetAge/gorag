package extractor

import (
	"context"
	"encoding/json"
	"fmt"

	gochatcore "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/utils"
)

// ExtractNodesAndEdges 从 Chunk 中提取 Node 和 Edge
// 使用 LLM 进行实体和关系抽取，直接返回 GraphRAG 原语结构
func ExtractNodesAndEdges(ctx context.Context, client gochatcore.Client, chunk *core.Chunk) ([]core.Node, []core.Edge, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("client is required")
	}
	if chunk == nil || chunk.Content == "" {
		return nil, nil, nil
	}

	// 构建 LLM 提示
	prompt := `You are an expert entity extraction specialist. Extract entities and relationships from the given text.
		
IMPORTANT: You MUST output in the same language as the user query.

## Tasks
1. Identify named entities (person names, locations, organizations, terms, etc.)
2. Identify relationships between entities

## Output Format
Output strictly in the following JSON format, no other content:
{
  "entities": [
    {
      "name": "entity name",
      "entity_type": "entity type (PERSON/LOCATION/ORGANIZATION/TECHNOLOGY/PRODUCT/EVENT/OTHER)"
    }
  ],
  "relations": [
    {
      "subject": "subject entity name",
      "predicate": "relationship type (e.g., works_at, belongs_to, contains, develops, creates)",
      "object": "object entity name"
    }
  ]
}

## Notes
- Only extract entities explicitly mentioned in the text, do not infer
- Use the exact text from the original for entity names
- Subject and object of relations must be extracted entities
- Return empty arrays if no entities or relations found`

	messages := []gochatcore.Message{
		gochatcore.NewSystemMessage(prompt),
		gochatcore.NewUserMessage(chunk.Content),
	}

	// 调用 LLM
	response, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// 解析 JSON 响应
	return parseResponse(response.Content, chunk)
}

// llmResponse LLM 返回的 JSON 结构
type llmResponse struct {
	Entities  []llmEntity   `json:"entities"`
	Relations []llmRelation `json:"relations"`
}

type llmEntity struct {
	Name       string `json:"name"`
	EntityType string `json:"entity_type"`
}

type llmRelation struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// parseResponse 解析 LLM 响应并转换为 Node 和 Edge
func parseResponse(content string, chunk *core.Chunk) ([]core.Node, []core.Edge, error) {
	// 提取 JSON 部分（可能在 markdown 代码块中）
	jsonStr := extractJSON(content)

	var resp llmResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// 构建实体映射（名称 -> Node）
	entityMap := make(map[string]*core.Node)
	nodes := make([]core.Node, 0, len(resp.Entities))

	for _, ent := range resp.Entities {
		if ent.Name == "" {
			continue
		}

		// 检查是否已存在（去重）
		if _, exists := entityMap[ent.Name]; exists {
			continue
		}

		// 创建 Node
		node := core.Node{
			ID:   utils.GenerateID([]byte(ent.Name + chunk.DocID)),
			Type: ent.EntityType,
			Name: ent.Name,
			Properties: map[string]any{
				"confidence": 0.9, // LLM 抽取默认置信度
			},
			SourceChunkIDs: []string{chunk.ID},
			SourceDocIDs:   []string{chunk.DocID},
		}

		entityMap[ent.Name] = &node
		nodes = append(nodes, node)
	}

	// 构建关系（Edge）
	edges := make([]core.Edge, 0, len(resp.Relations))
	for _, rel := range resp.Relations {
		subjectNode, hasSubject := entityMap[rel.Subject]
		objectNode, hasObject := entityMap[rel.Object]

		if !hasSubject || !hasObject {
			continue
		}

		// 创建 Edge
		edge := core.Edge{
			ID:        utils.GenerateID([]byte(rel.Subject + rel.Predicate + rel.Object + chunk.DocID)),
			Type:      rel.Predicate,
			Source:    subjectNode.ID,
			Target:    objectNode.ID,
			Predicate: rel.Predicate,
			Properties: map[string]any{
				"confidence": 0.9, // LLM 抽取默认置信度
			},
			SourceChunkIDs: []string{chunk.ID},
			SourceDocIDs:   []string{chunk.DocID},
		}

		edges = append(edges, edge)
	}

	return nodes, edges, nil
}

// extractJSON 从可能包含 markdown 代码块的响应中提取 JSON
func extractJSON(content string) string {
	// 尝试直接解析
	var v any
	if err := json.Unmarshal([]byte(content), &v); err == nil {
		return content
	}

	// 尝试从 markdown 代码块中提取
	start := 0
	if idx := indexOf(content, "```json"); idx >= 0 {
		start = idx + 7
	} else if idx := indexOf(content, "```"); idx >= 0 {
		start = idx + 3
	}

	end := len(content)
	if idx := indexOf(content[start:], "```"); idx >= 0 {
		end = start + idx
	}

	return content[start:end]
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
