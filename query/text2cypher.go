package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/logging"
)

// Text2Cypher 自然语言转 Cypher 预实现类。
//
// 承载自然语言 → Cypher 的转换能力，作为 query 包的预实现能力保留。
// 当前可能未在主流程使用，但作为后续扩展的基础设施不删除。
// 内部依赖 LLM 客户端完成转换；构造函数接收 chat.Client 和 model 作为必传参数，返回 error。
//
// 使用方式：
//
//	t2c, err := query.NewText2Cypher(chatClient, "gpt-4", logger)
//	if err != nil { ... }
//	cypher, err := t2c.Translate(ctx, "查询所有 Person 类型的实体")
//	if err != nil { ... }
//	// 用 cypher 调用 graphDB.Query(ctx, cypher, params)
type Text2Cypher struct {
	chat   chat.Client
	model  string
	logger logging.Logger
}

// NewText2Cypher 创建 Text2Cypher 实例。
//
// 必传参数：
//   - chatClient: LLM 客户端（不能为 nil）
//   - model:      模型名称（不能为空）
//   - logger:     日志实例（可为 nil，使用 NoopLogger）
//
// 返回 error 而非 panic。
func NewText2Cypher(chatClient chat.Client, model string, logger logging.Logger) (*Text2Cypher, error) {
	if chatClient == nil {
		return nil, fmt.Errorf("NewText2Cypher: chatClient is nil")
	}
	if model == "" {
		return nil, fmt.Errorf("NewText2Cypher: model is empty")
	}
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &Text2Cypher{
		chat:   chatClient,
		model:  model,
		logger: logger,
	}, nil
}

// Translate 将自然语言查询转换为 Cypher 查询语句。
//
// 流程：
//  1. 构建 Cypher 生成 Prompt（含 gograph 数据模型说明）
//  2. 调用 LLM（最多 3 次重试，间隔 2s/4s）
//  3. 清理响应（移除 markdown 代码块标记）
//
// 失败时返回 error，包含详细的重试信息。
func (t *Text2Cypher) Translate(ctx context.Context, naturalLanguage string) (string, error) {
	if naturalLanguage == "" {
		return "", fmt.Errorf("Text2Cypher.Translate: naturalLanguage is empty")
	}

	prompt := buildText2CypherPrompt(naturalLanguage)
	messages := []chat.Message{
		chat.NewSystemMessage(prompt),
	}

	var resp *chat.Response
	var lastErr error
	for attempt := 0; attempt <= 2; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*2) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		resp, lastErr = t.chat.Chat(ctx, messages)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("Text2Cypher.Translate: LLM 调用失败，3 次尝试后: %w", lastErr)
	}

	// 清理响应：移除可能的 markdown 代码块标记
	cypher := strings.TrimSpace(resp.Content)
	cypher = strings.TrimPrefix(cypher, "```cypher")
	cypher = strings.TrimPrefix(cypher, "```")
	cypher = strings.TrimSuffix(cypher, "```")
	cypher = strings.TrimSpace(cypher)

	if cypher == "" {
		return "", fmt.Errorf("Text2Cypher.Translate: LLM 返回空 Cypher 查询")
	}

	return cypher, nil
}

// buildText2Cypher 构建 Text2Cypher 的系统提示词。
//
// 提示词包含：
//   - gograph 节点数据模型（ID/name/source_chunk_ids/source_doc_ids/confidence/frequency）
//   - gograph 边数据模型（ID/type/predicate/source_chunk_ids/source_doc_ids/confidence/score/evidence）
//   - Cypher 语法参考（MATCH/WHERE/RETURN/ORDER BY/LIMIT/DETACH DELETE）
//   - 转换规则（节点 label 而非属性、参数化查询、LIMIT 控制、纯 Cypher 输出）
//
// Prompt 设计经过 LLM 调优，便于复用已有调优经验。
func buildText2CypherPrompt(text string) string {
	return fmt.Sprintf(`You are a Cypher query generation expert for gograph, an embedded Go graph database.

## Node Data Model

Each node has a label matching its entity category (PascalCase). Query by label using MATCH (n:LabelName).
Access node properties uniformly via n.propertyName syntax.

  n.ID                -- unique identifier (string)
  n.name              -- entity name (e.g. "Zhang San", "Alibaba")
  n.source_chunk_ids  -- []string, IDs of source chunks that mention this entity
  n.source_doc_ids    -- []string, IDs of source documents
  n.confidence        -- float (optional), extraction confidence
  n.frequency         -- int (optional), occurrence count
  n.*                 -- any custom property from dynamic schema

Entity category labels: Person, Organization, Location, Technology, Product, Event, Entity

To query by type:   MATCH (n:Person) RETURN n
To query by name:   MATCH (n) WHERE n.name = $name RETURN n

## Edge (Relationship) Data Model

  r.ID                -- unique identifier (string)
  r.type              -- relationship type, e.g. 'KNOWS', 'WORKS_FOR', 'LOCATED_IN', 'BELONGS_TO', 'RELATED_TO'
  r.predicate         -- human-readable description (e.g. "works at", "located in")
  r.source_chunk_ids  -- []string
  r.source_doc_ids    -- []string
  r.confidence        -- float (optional)
  r.score             -- float (optional)
  r.evidence          -- string (optional), text evidence
  r.*                 -- any custom property

Access edge fields uniformly via r.propertyName.

## RETURN Result Shape

RETURN n gives: {id, labels: ["Person"], properties: {ID, name, source_chunk_ids, ...}}
RETURN r gives: {id, type, startNodeID, endNodeID, properties: {ID, predicate, ...}}

## Cypher Syntax Reference

  MATCH (n:Person) RETURN n                                          -- filter by label
  MATCH (n) WHERE n.name = $name RETURN n                           -- parameterized filter
  MATCH (n:Person {name: 'Zhang San'}) RETURN n                     -- label + property shorthand
  MATCH (a:Person)-[r:KNOWS]->(b:Person) RETURN a, r, b             -- relationship traversal
  MATCH (n) WHERE $cid IN n.source_chunk_ids RETURN n               -- array contains
  RETURN n.ID, n.name, n.source_chunk_ids                            -- specific fields
  ORDER BY n.name SKIP 10 LIMIT 20                                   -- pagination
  MATCH (n {ID: $id}) DETACH DELETE n                                -- delete

## Instructions

Convert the following natural language query into a valid Cypher query.

Rules:
1. Node entity category is a LABEL, not a property -- use MATCH (n:Person), never WHERE n.type = 'Person'
2. Entity names are in property n.name -- use WHERE n.name = $name or (n {name: 'Zhang San'})
3. Relationship queries use (source)-[r:TYPE]->(target) patterns
4. RETURN both nodes and relationships when relevant, e.g. RETURN a, r, b
5. Use LIMIT 20 to control result size
6. Use parameterized queries ($name, $id) when filtering by specific values
7. Output ONLY the Cypher query, no explanation, no markdown code blocks

## User Query
%s

Output the Cypher query directly:`, text)
}
