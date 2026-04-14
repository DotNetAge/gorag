package extractor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/utils"
	gochatcore "github.com/DotNetAge/gochat/pkg/core"
)

// LLMExtractor 基于 LLM 的实体抽取器
// 通过注入 gochat Client 实现结构化实体抽取
type LLMExtractor struct {
	client gochatcore.Client
	prompt string
}

// NewLLMExtractor 创建基于 LLM 的实体抽取器
func NewLLMExtractor(client gochatcore.Client) *LLMExtractor {
	return &LLMExtractor{
		client: client,
		prompt: `You are an expert entity extraction specialist. Extract entities and relationships from the given text.

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
- Return empty arrays if no entities or relations found`,
	}
}

// Extract 实现 core.Extractor 接口
// 从结构化文档中抽取实体和关系
func (e *LLMExtractor) Extract(structured *core.StructuredDocument) ([]*core.Entity, []*core.Relation, error) {
	if structured == nil || structured.Root == nil {
		return nil, nil, nil
	}

	// 提取文档文本内容
	text := e.extractText(structured.Root)

	// 构建提示
	messages := []gochatcore.Message{
		gochatcore.NewSystemMessage(e.prompt),
		gochatcore.NewUserMessage(text),
	}

	// 调用 LLM
	response, err := e.client.Chat(context.Background(), messages)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// 解析 JSON 响应
	return e.parseResponse(response.Content, structured)
}

// extractText 递归提取结构化文档中的文本
func (e *LLMExtractor) extractText(node *core.StructureNode) string {
	if node == nil {
		return ""
	}

	var text string
	if node.Text != "" {
		text = node.Text
	}

	// 递归处理子节点
	for _, child := range node.Children {
		if childText := e.extractText(child); childText != "" {
			if text != "" {
				text += "\n"
			}
			text += childText
		}
	}

	return text
}

// llmResponse LLM 返回的 JSON 结构
type llmResponse struct {
	Entities  []llmEntity  `json:"entities"`
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

// parseResponse 解析 LLM 响应并转换为 Entity 和 Relation
func (e *LLMExtractor) parseResponse(content string, structured *core.StructuredDocument) ([]*core.Entity, []*core.Relation, error) {
	// 提取 JSON 部分（可能在 markdown 代码块中）
	jsonStr := e.extractJSON(content)

	var resp llmResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// 构建实体映射（名称 -> Entity）
	entityMap := make(map[string]*core.Entity)
	entities := make([]*core.Entity, 0, len(resp.Entities))

	for _, ent := range resp.Entities {
		if ent.Name == "" {
			continue
		}

		// 检查是否已存在
		if _, exists := entityMap[ent.Name]; exists {
			continue
		}

		entity := &core.Entity{
			ID:         utils.GenerateID([]byte(ent.Name + structured.ID())),
			Name:       ent.Name,
			EntityType: ent.EntityType,
			Confidence: 0.9, // LLM 抽取默认置信度
			SourceNode: structured.Root.ID(),
		}
		entityMap[ent.Name] = entity
		entities = append(entities, entity)
	}

	// 构建关系
	relations := make([]*core.Relation, 0, len(resp.Relations))
	for _, rel := range resp.Relations {
		subject, hasSubject := entityMap[rel.Subject]
		object, hasObject := entityMap[rel.Object]

		if !hasSubject || !hasObject {
			continue
		}

		relation := &core.Relation{
			ID:        utils.GenerateID([]byte(rel.Subject + rel.Predicate + rel.Object + structured.ID())),
			Subject:   subject,
			Predicate: rel.Predicate,
			Object:    object,
			Score:     0.9,
		}
		relations = append(relations, relation)
	}

	return entities, relations, nil
}

// extractJSON 从可能包含 markdown 代码块的响应中提取 JSON
func (e *LLMExtractor) extractJSON(content string) string {
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

// 确保实现 core.Extractor 接口
var _ core.Extractor = (*LLMExtractor)(nil)
