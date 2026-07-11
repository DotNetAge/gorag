package indexer

import (
	"fmt"

	chat "github.com/DotNetAge/gochat/core"
)

// =============================================================================
// System Prompt 共享分片
// 拆成多条 SystemMessage 以利于 LLM 提供商的 KV Cache 复用。
// 支持中英文双语，通过 promptLang 参数切换。
// =============================================================================

// segmentRoleDefinition 片段 1：角色定义 + 文档上下文（含动态参数）- 英文版
const segmentRoleDefinition = `You are a knowledge base administrator. Your job is to analyze content and produce structured output for indexing.

## Document Context
- doc_id: %s
- language: %s
- The content is split into multiple pages. Each page ends with a line number marker like [L23-L45].
  Use the absolute line numbers from the marker in start_line/end_line fields.
  For example: content within [L23-L45] where the 0th line corresponds to absolute line 23.
  Calculate absolute line numbers as: page_start_line + local_line_index.
  Ignore the [L{start}-L{end}] marker itself — do not include it in chunk content.`

// segmentRoleDefinitionZH 片段 1：角色定义 + 文档上下文（含动态参数）- 中文版
const segmentRoleDefinitionZH = `你是一个知识库管理员。你的任务是分析内容并生成结构化输出用于索引。

## 文档上下文
- doc_id: %s
- language: %s
- 内容被分成多个页面。每个页面末尾有行号标记，格式如 [L23-L45]。
  使用标记中的绝对行号作为 start_line/end_line 字段的值。
  例如：[L23-L45] 中的内容，第 0 行对应绝对行号 23。
  计算公式：page_start_line + local_line_index。
  忽略 [L{start}-L{end}] 标记本身 — 不要将其包含在 chunk 内容中。`

// segmentChunkingRules 片段 2：分块与摘要规则 - 英文版
const segmentChunkingRules = `## Chunking & Title Rules
1. Group content by semantic boundaries (functions, classes, sections, paragraphs, topics).
2. For each chunk, generate a concise summary (<300 chars) capturing its semantic essence.
3. Generate a short, descriptive title (<60 chars) for each chunk, based on its main topic.
4. The chunk "content" field must contain the EXACT original text lines for the chunk range. Do not paraphrase, do not summarize the content field itself — only the "title" and "summary" fields are for condensation.
5. Filter out meaningless content: license headers, template comments, empty boilerplate, navigation links. Do not chunk these.
6. Merge adjacent lines that belong to the same logical topic. Avoid tiny fragments.
7. The FIRST chunk MUST be a document-level summary covering the entire document. Set metadata.type = "document" for this chunk, and metadata.type = "segment" for all other chunks. This document chunk's content is a 1-2 sentence document overview, and its summary describes the document as a whole. Include all key entities from the document in this chunk's entity_ids.
8. For each non-root chunk, set metadata.parent_ordinal to the 0-based array index of its parent chunk. The root chunk (index 0, type "document") has no parent_ordinal. Top-level sections set parent_ordinal = 0 (root chunk). Nested subsections reference their direct parent chunk's index. This creates a hierarchical parent-child chain through the document structure.`

// segmentChunkingRulesZH 片段 2：分块与摘要规则 - 中文版
const segmentChunkingRulesZH = `## 分块与标题规则
1. 按语义边界（函数、类、章节、段落、主题）对内容进行分组。
2. 为每个 chunk 生成简洁的摘要（<300 字符），捕捉其语义精髓。
3. 为每个 chunk 生成简短的描述性标题（<60 字符），基于其主要主题。
4. chunk 的 "content" 字段必须包含该 chunk 范围内的原始文本行。不要改写，不要总结 content 字段本身 — 只有 "title" 和 "summary" 字段用于浓缩。
5. 过滤无意义内容：许可证头部、模板注释、空样板、导航链接。不要对这些进行分块。
6. 合并属于同一逻辑主题的相邻行。避免过小的片段。
7. 第一个 chunk 必须是覆盖整个文档的文档级摘要。为此 chunk 设置 metadata.type = "document"，其他所有 chunk 设置 metadata.type = "segment"。此文档 chunk 的内容是 1-2 句文档概述，其 summary 描述整个文档。在此 chunk 的 entity_ids 中包含文档中的所有关键实体。
8. 对于每个非根 chunk，设置 metadata.parent_ordinal 为其父 chunk 的 0 基数组索引。根 chunk（索引 0，类型 "document"）没有 parent_ordinal。顶级章节设置 parent_ordinal = 0（根 chunk）。嵌套子章节引用其直接父 chunk 的索引。这通过文档结构创建层次化的父子链。`

// segmentOutputFormat 片段 4：输出格式（含 positions 说明）- 英文版
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

// segmentOutputFormatZH 片段 4：输出格式（含 positions 说明）- 中文版
const segmentOutputFormatZH = `## 输出格式 — 仅 JSON，不要 markdown，不要额外文本
ID 使用整数 (1, 2, 3...) — 见约束条件中的具体规则。

重要：chunk_meta.positions 中的每个 [start_line, end_line] 必须使用绝对行号，计算方式为 page_start_line + local_line_index。
chunk.content 必须是这些行号的原始文本。不要包含 [L{start}-L{end}] 标记在内容中。

{
  "chunks": [
    {
      "content": "原始文本行（第一个/根 chunk — 文档级概述）",
      "metadata": {
        "type": "document",
        "title": "简短描述性标题（<60 字符）",
        "summary": "语义含义的简洁摘要（<300 字符）",
        "tags": ["主题标签1", "主题标签2", "主题标签3"],
        "entity_ids": [1, 2, 3]
      },
      "chunk_meta": {
        "positions": [[start_line, end_line], [start_line, end_line]]
      }
    },
    {
      "content": "原始文本行（后续 chunk — 片段）",
      "metadata": {
        "type": "segment",
        "title": "简短描述性标题（<60 字符）",
        "summary": "语义含义的简洁摘要（<300 字符）",
        "tags": ["主题标签1", "主题标签2", "主题标签3"],
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
      "name": "实体名称（例如：单一职责原则）",
      "properties": {
        "description": "概念的简短描述",
        "domain": "主题领域（例如：软件设计）"
      }
    },
    {
      "id": 2,
      "type": "Tool",
      "name": "实体名称（例如：Jenkins）",
      "properties": {
        "description": "工具的功能",
        "purpose": "主要用途",
        "platform": "运行平台"
      }
    },
    {
      "id": 3,
      "type": "Person",
      "name": "实体名称（例如：Martin Fowler）",
      "properties": {
        "description": "人物简介",
        "role": "角色或头衔",
        "expertise": "专业领域"
      }
    }
  ],
  "relations": [
    {
      "source": 1,
      "target": 2,
      "type": "RELATION_TYPE（例如：CONTAINS, DEPENDS_ON）",
      "predicate": "文档语言中的关系短语",
      "properties": {
        "description": "在此内容背景下它们如何关联",
        "TYPE_SPECIFIC_FIELDS": "见实体 Schema 中的关系类型属性"
      }
    }
  ]
}`

// segmentConstraints 片段 5：约束条件（含语言一致性的铁则 + 序数 ID 规则）- 英文版
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

// segmentConstraintsZH 片段 5：约束条件（含语言一致性的铁则 + 序数 ID 规则）- 中文版
const segmentConstraintsZH = `## 约束条件 — 严格规则。违反将导致拒绝。

### 铁律：语言一致性
所有输出文本必须使用"%s"：摘要、实体名称、描述、元数据值。不允许混合语言内容。

### 实体 ID 规则
- 实体 ID 必须是简单整数 (1, 2, 3...)。在每个文档切片内顺序且唯一。
- chunk 的 "entity_ids" 数组引用这些整数 ID。关系的 "source"/"target" 使用它们。
- 永远不要使用字符串名称或任何其他格式作为实体 ID。

### JSON 规则
- 仅输出有效的 JSON。前后不要有文本。
- 如果没有实体或关系，返回空数组。

### 标签规则
- 每个 chunk 必须有 3-5 个主题标签，使用文档的语言。
- 标签是简短、具体的关键词（每个 1-4 个词），描述 chunk 的内容。
- 标签应覆盖：主要主题、关键概念、技术或讨论的实体。
- 不要使用通用标签如 "introduction"、"overview"、"conclusion"。要具体。`

// buildSystemMessages 构建多条 SystemMessage 分片，以利于 KV Cache 复用。
// promptLang 控制指令模板语言（LangEN | LangZH），lang 控制输出语言约束。
// entityDefs 为 EntityDef 列表，由 WithSchemas 提供，会合并入 Prompt 的实体定义部分。
// 调用方只需 append UserMessage 即可完成 Messages 组装。
func buildSystemMessages(docID, lang, promptLang string, entityDefs []EntityDef) []chat.Message {
	if promptLang == LangEN {
		return []chat.Message{
			chat.NewSystemMessage(fmt.Sprintf(segmentRoleDefinition, docID, lang)),
			chat.NewSystemMessage(segmentChunkingRules),
			chat.NewSystemMessage(buildOntology(entityDefs, promptLang)),
			chat.NewSystemMessage(segmentOutputFormat),
			chat.NewSystemMessage(fmt.Sprintf(segmentConstraints, lang)),
		}
	}
	// 默认中文
	return []chat.Message{
		chat.NewSystemMessage(fmt.Sprintf(segmentRoleDefinitionZH, docID, lang)),
		chat.NewSystemMessage(segmentChunkingRulesZH),
		chat.NewSystemMessage(buildOntology(entityDefs, promptLang)),
		chat.NewSystemMessage(segmentOutputFormatZH),
		chat.NewSystemMessage(fmt.Sprintf(segmentConstraintsZH, lang)),
	}
}
