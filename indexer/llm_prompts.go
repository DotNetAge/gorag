package indexer

import (
	"fmt"

	chat "github.com/DotNetAge/gochat/core"
)

// =============================================================================
// System Prompt 共享分片
// 拆成多条 SystemMessage 以利于 LLM 提供商的 KV Cache 复用。
// 指令部分使用纯英文；领域专有名词例词保留原文不翻译。
// =============================================================================

// segmentRoleDefinition 片段 1：角色定义 + 文档上下文（含动态参数）。
const segmentRoleDefinition = `You are a knowledge base administrator. Your job is to analyze content and produce structured output for indexing.

## Document Context
- doc_id: %s
- language: %s
- Content lines are prefixed with absolute line numbers (e.g. "42: func foo()").
  Use these line numbers in start_line/end_line fields. Never invent line numbers.`

// segmentChunkingRules 片段 2：分块与摘要规则
const segmentChunkingRules = `## Chunking & Title Rules
1. Group content by semantic boundaries (functions, classes, sections, paragraphs, topics).
2. For each chunk, generate a concise summary (<300 chars) capturing its semantic essence.
3. Generate a short, descriptive title (<60 chars) for each chunk, based on its main topic.
4. The chunk "content" field must contain the EXACT original text lines for the chunk range. Do not paraphrase, do not summarize the content field itself — only the "title" and "summary" fields are for condensation.
5. Filter out meaningless content: license headers, template comments, empty boilerplate, navigation links. Do not chunk these.
6. Merge adjacent lines that belong to the same logical topic. Avoid tiny fragments.`

// segmentOutputFormat 片段 4：输出格式
const segmentOutputFormat = `## Output Format — JSON only, no markdown, no additional text
IDs use integers (1, 2, 3...) — see Constraints for the exact rule.

{
  "chunks": [
    {
      "content": "exact original text lines for this chunk",
      "metadata": {
        "title": "short descriptive title (<60 chars)",
        "summary": "concise summary of semantic meaning (<300 chars)",
        "tags": ["topic_tag1", "topic_tag2", "topic_tag3"],
        "entity_ids": [1, 2, 3]
      },
      "chunk_meta": {
        "positions": [[start_line, end_line], [start_line, end_line]]
      }
    }
  ],
  "entities": [
    {
      "id": 1,
      "type": "ENTITY_TYPE (one of the types defined above)",
      "name": "normalized entity name",
      "properties": {
        "description": "brief description relevant to this document"
      }
    }
  ],
  "relations": [
    {
      "source": 1,
      "target": 2,
      "type": "RELATION_TYPE (one of the relation types defined above)",
      "predicate": "relationship phrase in the document language",
      "properties": {
        "description": "how they relate in context of this content"
      }
    }
  ]
}`

// segmentConstraints 片段 5：约束条件（含语言一致性的铁则 + 序数 ID 规则）
const segmentConstraints = `## Constraints — Strict Rules. Violation causes rejection.

### IRON LAW: Language Consistency
ALL output text MUST be in "%s": summaries, entity names, descriptions, metadata values. No mixed-language content.

### Entity ID Rules
- Entity IDs MUST be simple integers (1, 2, 3...). Sequential and unique within each document slice.
- Chunk "entity_ids" array references these integer IDs. Relation "source"/"target" uses them.
- Never use string names or any other format as entity IDs.

### JSON Rules
- Output ONLY valid JSON. No text before or after.
- If no entities or relations exist, return empty arrays.

### Tag Rules
- Each chunk MUST have exactly 3-5 topic tags in the document's language.
- Tags are short, specific keywords (1-4 words each) that describe the chunk's content.
- Tags should cover: main topics, key concepts, techniques, or entities discussed.
- Do NOT use generic tags like "introduction", "overview", "conclusion". Be specific.`

// buildSystemMessages 构建多条 SystemMessage 分片，以利于 KV Cache 复用。
// ontology 参数传入预设名（如 "general", "tech"），支持逗号分隔的多个名称（如 "tech,finance"）。
// customPrompts 为自定义提示文本，会追加在预设 ontology 之后。
// 调用方只需 append UserMessage 即可完成 Messages 组装。
func buildSystemMessages(docID, lang, ontology string, customPrompts ...string) []chat.Message {
	return []chat.Message{
		chat.NewSystemMessage(fmt.Sprintf(segmentRoleDefinition, docID, lang)),
		chat.NewSystemMessage(segmentChunkingRules),
		chat.NewSystemMessage(selectOntology(ontology, customPrompts...)),
		chat.NewSystemMessage(segmentOutputFormat),
		chat.NewSystemMessage(fmt.Sprintf(segmentConstraints, lang)),
	}
}
