package query

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
)

// GraphQuery 图查询
type GraphQuery struct {
	BaseQuery
	Depth     int
	Limit     int
	EdgeTypes []string         // 关系类型过滤（可选），空表示不过滤
	Mode      core.SearchMode  // 搜索模式: "local" | "global" | "hybrid"
	cypher    string           // Text2Cypher 生成的 Cypher 查询语句
}

// NewGraphQuery creates a new graph query from the given terms.
func NewGraphQuery(terms string) core.Query {
	return &GraphQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
		Depth: 1,
		Limit: 10,
		Mode:  core.SearchModeLocal,
	}
}

// SetDepth 设置图遍历深度
func (q *GraphQuery) SetDepth(depth int) {
	q.Depth = depth
}

// SetLimit 设置返回结果数量限制
func (q *GraphQuery) SetLimit(limit int) {
	q.Limit = limit
}

// SetEdgeTypes 设置关系类型过滤（仅遍历指定类型的边）
func (q *GraphQuery) SetEdgeTypes(types []string) {
	q.EdgeTypes = types
}

// SetMode 设置搜索模式
func (q *GraphQuery) SetMode(mode core.SearchMode) {
	q.Mode = mode
}

// Cypher 返回 Text2Cypher 生成的 Cypher 查询语句
func (q *GraphQuery) Cypher() string {
	return q.cypher
}

// Text2Cypher 将自然语言查询转换为 Cypher 图查询语句
// 使用 LLM 理解用户意图并生成对应的 Cypher 查询
// 生成的 Cypher 应返回与实体/关系相关的节点和边信息
func (q *GraphQuery) Text2Cypher(client chat.Client) error {
	if client == nil {
		return fmt.Errorf("client is required for Text2Cypher")
	}
	if q.raw == "" {
		return fmt.Errorf("query cannot be empty")
	}

	ctx := context.Background()

	// 构建图 schema 提示，让 LLM 了解可用的节点和关系类型
	prompt := fmt.Sprintf(`You are a Cypher query generation expert for a knowledge graph database (GraphRAG).

## Graph Schema
The graph contains the following types of nodes and relationships:

### Node Types (labels)
- PERSON: 人名
- ORGANIZATION: 组织机构
- LOCATION: 地点
- TECHNOLOGY: 技术
- PRODUCT: 产品
- EVENT: 事件
- OTHER: 其他

### Relationship Types (edge types)
- Common predicates include: works_at, belongs_to, contains, develops, creates, located_in, participates_in, related_to, etc.
- Edges have properties: "confidence" (float), "score" (float), "evidence" (string)

### Node Properties
- name: string (entity name)
- Properties may include: "confidence", "frequency", "aliases"

### Important Notes
- Node IDs are stored as property "ID" on nodes
- Node names are stored as property "name" on nodes
- Source chunk IDs are stored as property "source_chunk_ids" ([]string)
- Source doc IDs are stored as property "source_doc_ids" ([]string)

## Your Task
Convert the following natural language query into a valid Cypher query.

Requirements:
1. The query should return relevant nodes (entities) and their relationships
2. Use MATCH, WHERE, RETURN, OPTIONAL MATCH, WITH clauses as needed
3. If the query mentions specific entity names, match them against the "name" property
4. Use LIMIT %d to control result size
5. Output ONLY the Cypher query, no explanation, no markdown code blocks
6. The Cypher must be compatible with gograph (a Go-based graph database supporting Cypher-like syntax)

## User Query
%s

Output the Cypher query directly:`, q.Limit, q.raw)

	messages := []chat.Message{
		chat.NewSystemMessage(prompt),
	}

	response, err := client.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("Text2Cypher failed: %w", err)
	}

	// 清理响应：移除可能的 markdown 代码块标记
	cypher := strings.TrimSpace(response.Content)
	cypher = strings.TrimPrefix(cypher, "```cypher")
	cypher = strings.TrimPrefix(cypher, "```")
	cypher = strings.TrimSuffix(cypher, "```")
	cypher = strings.TrimSpace(cypher)

	// 基本验证：Cypher 应该包含 MATCH 和 RETURN
	upper := strings.ToUpper(cypher)
	if !strings.Contains(upper, "MATCH") || !strings.Contains(upper, "RETURN") {
		return fmt.Errorf("generated text is not a valid Cypher query: missing MATCH or RETURN clause")
	}

	q.cypher = cypher
	return nil
}
