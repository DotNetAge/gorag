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
- The content is split into multiple pages. Each page ends with a line number marker like [L23-L45].
  Use the absolute line numbers from the marker in start_line/end_line fields.
  For example: content within [L23-L45] where the 0th line corresponds to absolute line 23.
  Calculate absolute line numbers as: page_start_line + local_line_index.
  Ignore the [L{start}-L{end}] marker itself — do not include it in chunk content.`

// segmentChunkingRules 片段 2：分块与摘要规则
const segmentChunkingRules = `## Chunking & Title Rules
1. Group content by semantic boundaries (functions, classes, sections, paragraphs, topics).
2. For each chunk, generate a concise summary (<300 chars) capturing its semantic essence.
3. Generate a short, descriptive title (<60 chars) for each chunk, based on its main topic.
4. The chunk "content" field must contain the EXACT original text lines for the chunk range. Do not paraphrase, do not summarize the content field itself — only the "title" and "summary" fields are for condensation.
5. Filter out meaningless content: license headers, template comments, empty boilerplate, navigation links. Do not chunk these.
6. Merge adjacent lines that belong to the same logical topic. Avoid tiny fragments.
7. The FIRST chunk MUST be a document-level summary covering the entire document. Set metadata.type = "document" for this chunk, and metadata.type = "segment" for all other chunks. This document chunk's content is a 1-2 sentence document overview, and its summary describes the document as a whole. Include all key entities from the document in this chunk's entity_ids.
8. For each non-root chunk, set metadata.parent_ordinal to the 0-based array index of its parent chunk. The root chunk (index 0, type "document") has no parent_ordinal. Top-level sections set parent_ordinal = 0 (root chunk). Nested subsections reference their direct parent chunk's index. This creates a hierarchical parent-child chain through the document structure.`

// segmentOutputFormat 片段 4：输出格式（含 positions 说明）
const segmentOutputFormat = `## Output Format — JSON only, no markdown, no additional text
IDs use integers (1, 2, 3...) — see Constraints for the exact rule.

Important: Each [start_line, end_line] in chunk_meta.positions MUST use absolute line numbers calculated as page_start_line + local_line_index.
The chunk.content MUST be the exact original text for those line numbers. Do not include the [L{start}-L{end}] marker in content.

{
  "chunks": [
    {
      "content": "exact original text lines (first/root chunk — document-level overview)",
      "metadata": {
        "type": "document",
        "title": "short descriptive title (<60 chars)",
        "summary": "concise summary of semantic meaning (<300 chars)",
        "tags": ["topic_tag1", "topic_tag2", "topic_tag3"],
        "entity_ids": [1, 2, 3]
      },
      "chunk_meta": {
        "positions": [[start_line, end_line], [start_line, end_line]]
      }
    },
    {
      "content": "exact original text lines (subsequent chunk — segment)",
      "metadata": {
        "type": "segment",
        "title": "short descriptive title (<60 chars)",
        "summary": "concise summary of semantic meaning (<300 chars)",
        "tags": ["topic_tag1", "topic_tag2", "topic_tag3"],
        "entity_ids": [1, 2, 3],
        "parent_ordinal": 0
      },
      "chunk_meta": {
        "positions": [[start_line, end_line], [start_line, end_line]]
      }
    }
  ],
  "entities": [
    {
      "id": 1,
      "type": "Concept",
      "name": "entity name (e.g. Single Responsibility Principle)",
      "properties": {
        "description": "brief description of the concept",
        "domain": "subject domain (e.g. software design)"
      }
    },
    {
      "id": 2,
      "type": "Tool",
      "name": "entity name (e.g. Jenkins)",
      "properties": {
        "description": "what the tool does",
        "purpose": "its main use case",
        "platform": "where it runs"
      }
    },
    {
      "id": 3,
      "type": "Person",
      "name": "entity name (e.g. Martin Fowler)",
      "properties": {
        "description": "who this person is",
        "role": "their role or title",
        "expertise": "their area of knowledge"
      }
    }
  ],
  "relations": [
    {
      "source": 1,
      "target": 2,
      "type": "RELATION_TYPE (e.g. CONTAINS, DEPENDS_ON)",
      "predicate": "relationship phrase in the document language",
      "properties": {
        "description": "how they relate in context of this content",
        "TYPE_SPECIFIC_FIELDS": "see Entity Schema for relation type properties"
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
// entityDefs 为 EntityDef 列表，由 WithSchemas 提供，会合并入 Prompt 的实体定义部分。
// 调用方只需 append UserMessage 即可完成 Messages 组装。
func buildSystemMessages(docID, lang string, entityDefs []EntityDef) []chat.Message {
	return []chat.Message{
		chat.NewSystemMessage(fmt.Sprintf(segmentRoleDefinition, docID, lang)),
		chat.NewSystemMessage(segmentChunkingRules),
		chat.NewSystemMessage(buildOntology(entityDefs)),
		chat.NewSystemMessage(segmentOutputFormat),
		chat.NewSystemMessage(fmt.Sprintf(segmentConstraints, lang)),
	}
}
