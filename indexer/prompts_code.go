package indexer

import (
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/core"
)

// =============================================================================
// 代码索引专用 System Prompt 分片
// 与 llm_prompts.go 中的文本域分片平行，两者互斥。
// =============================================================================

// codeSegmentRoleDefinition 片段 1：角色定义（代码分析引擎）- 英文版。
const codeSegmentRoleDefinition = `You are a code analysis engine. Your job is to analyze source code and produce structured output for indexing.

## Document Context
- doc_id: %s
- language: %s
- The content is split into multiple pages. Each page ends with a line number marker like [L23-L45].
  Use the absolute line numbers from the marker in start_line/end_line fields.
  For example: content within [L23-L45] where the 0th line corresponds to absolute line 23.
  Calculate absolute line numbers as: page_start_line + local_line_index.
  Ignore the [L{start}-L{end}] marker itself — do not include it in chunk content.

Focus on extracting the code's structural entities and their relationships. Document-level concepts like topic tags or subject matter are not relevant.`

// codeSegmentRoleDefinitionZH 片段 1：角色定义（代码分析引擎）- 中文版。
const codeSegmentRoleDefinitionZH = `你是一个代码分析引擎。你的任务是分析源代码并生成结构化输出用于索引。

## 文档上下文
- doc_id: %s
- language: %s
- 内容被分成多个页面。每个页面末尾有行号标记，格式如 [L23-L45]。
  使用标记中的绝对行号作为 start_line/end_line 字段的值。
  例如：[L23-L45] 中的内容，第 0 行对应绝对行号 23。
  计算公式：page_start_line + local_line_index。
  忽略 [L{start}-L{end}] 标记本身 — 不要将其包含在 chunk 内容中。

专注于提取代码的结构实体及其关系。文档级别的概念如主题标签或主题内容不相关。`

// codeSegmentChunkingRules 片段 2：代码分块规则 - 英文版。
const codeSegmentChunkingRules = `## Chunking Rules for Code
1. Group by logical code boundaries: function, method, class, struct, interface, enum, and top-level type definitions. Each is one chunk.
2. For each chunk, generate a short summary (<200 chars) describing what the code segment does — equivalent to a doc comment.
3. Use the function/class/type name as the chunk title. For unnamed blocks, use a descriptive label.
4. The chunk "content" field must contain the EXACT original code lines for the range. Do not paraphrase.
5. Merge adjacent small helpers (<5 lines each) that belong to the same logical unit into one chunk.
6. License headers, generated-code markers, empty boilerplate, and auto-generated sections MUST be filtered out.`

// codeSegmentChunkingRulesZH 片段 2：代码分块规则 - 中文版。
const codeSegmentChunkingRulesZH = `## 代码分块规则
1. 按逻辑代码边界分组：函数、方法、类、结构体、接口、枚举和顶级类型定义。每个为一个 chunk。
2. 为每个 chunk 生成简短的摘要（<200 字符），描述代码段的功能 — 相当于文档注释。
3. 使用函数/类/类型名称作为 chunk 标题。对于未命名的块，使用描述性标签。
4. chunk 的 "content" 字段必须包含该范围的原始代码行。不要改写。
5. 将属于同一逻辑单元的相邻小辅助函数（每个 <5 行）合并为一个 chunk。
6. 必须过滤掉许可证头部、生成代码标记、空样板和自动生成部分。`

// codeSegmentOutputFormat 片段 3：输出格式（简化版，无 tags / entity_ids）- 英文版。
const codeSegmentOutputFormat = `## Output Format — JSON only, no markdown, no additional text

### For code files, entity extraction is file-level.
Chunks represent logical code segments (functions, classes, blocks).
Each chunk MUST have a concise summary capturing its purpose.
Chunks do NOT have entity_ids or tags — entities are declared at file level.
Relations link entities across the file.

IDs use integers (1, 2, 3...) — see Constraints for the exact rule.

Important: Each [start_line, end_line] in chunk_meta.positions MUST use absolute line numbers calculated as page_start_line + local_line_index.
The chunk.content MUST be the exact original text for those line numbers. Do not include the [L{start}-L{end}] marker in content.

{
  "chunks": [
    {
      "content": "exact original code lines for this chunk",
      "metadata": {
        "title": "function/class/type name or descriptive title (<60 chars)",
        "summary": "what this code does — one-line summary (<200 chars)"
      },
      "chunk_meta": {
        "positions": [[start_line, end_line]]
      }
    }
  ],
  "entities": [
    {
      "id": 1,
      "type": "ENTITY_TYPE",
      "name": "entity name",
      "properties": {
        "description": "brief description of this code entity",
        "...": "additional type-specific properties defined in the schema above"
      }
    }
  ],
  "relations": [
    {
      "source": 1,
      "target": 2,
      "type": "RELATION_TYPE (one of the relation types defined above)",
      "predicate": "relationship phrase (e.g. calls, implements)",
      "properties": {
        "description": "how the two entities relate in this codebase"
      }
    }
  ]
}`

// codeSegmentOutputFormatZH 片段 3：输出格式（简化版，无 tags / entity_ids）- 中文版。
const codeSegmentOutputFormatZH = `## 输出格式 — 仅 JSON，不要 markdown，不要额外文本

### 对于代码文件，实体提取是文件级别的。
Chunks 代表逻辑代码段（函数、类、块）。
每个 chunk 必须有简洁的摘要，捕捉其用途。
Chunks 没有 entity_ids 或 tags — 实体在文件级别声明。
关系链接整个文件中的实体。

ID 使用整数 (1, 2, 3...) — 见约束条件中的具体规则。

重要：chunk_meta.positions 中的每个 [start_line, end_line] 必须使用绝对行号，计算方式为 page_start_line + local_line_index。
chunk.content 必须是这些行号的原始文本。不要包含 [L{start}-L{end}] 标记在内容中。

{
  "chunks": [
    {
      "content": "此 chunk 的原始代码行",
      "metadata": {
        "title": "函数/类/类型名称或描述性标题（<60 字符）",
        "summary": "此代码的功能 — 单行摘要（<200 字符）"
      },
      "chunk_meta": {
        "positions": [[start_line, end_line]]
      }
    }
  ],
  "entities": [
    {
      "id": 1,
      "type": "ENTITY_TYPE",
      "name": "实体名称",
      "properties": {
        "description": "此代码实体的简短描述",
        "...": "上面 schema 中定义的额外类型特定属性"
      }
    }
  ],
  "relations": [
    {
      "source": 1,
      "target": 2,
      "type": "RELATION_TYPE（上面定义的关系类型之一）",
      "predicate": "关系短语（例如：calls, implements）",
      "properties": {
        "description": "这两个实体在此代码库中如何关联"
      }
    }
  ]
}`

// codeSegmentConstraints 片段 4：约束条件 - 英文版。
const codeSegmentConstraints = `## Constraints — Strict Rules. Violation causes rejection.

### IRON LAW: Language Consistency
ALL output text MUST be in "%s": summaries, entity names, descriptions, property values.

### Entity ID Rules
- Entity IDs MUST be simple integers (1, 2, 3...). Sequential and unique within each document slice.
- Chunks do NOT have entity_ids. Relation source/target uses these integer IDs.
- Never use string names or any other format as entity IDs.

### JSON Rules
- Output ONLY valid JSON. No text before or after.
- If no entities or relations exist, return empty arrays.`

// codeSegmentConstraintsZH 片段 4：约束条件 - 中文版。
const codeSegmentConstraintsZH = `## 约束条件 — 严格规则。违反将导致拒绝。

### 铁律：语言一致性
所有输出文本必须使用"%s"：摘要、实体名称、描述、属性值。

### 实体 ID 规则
- 实体 ID 必须是简单整数 (1, 2, 3...)。在每个文档切片内顺序且唯一。
- Chunk 没有 entity_ids。关系的 source/target 使用这些整数 ID。
- 永远不要使用字符串名称或任何其他格式作为实体 ID。

### JSON 规则
- 仅输出有效的 JSON。前后不要有文本。
- 如果没有实体或关系，返回空数组。`

// buildCodeSystemMessages 构建代码文档专用的 SystemMessage 分片列表。
// 与 buildSystemMessages 平行。
// 调用方只需 append UserMessage 即可完成 Messages 组装。
//
// 与文本域的关键区别：
//   - 不使用用户选择的 entityDefs，始终使用内置的 codeEntityDefs
//   - 实体类型为代码域专属（Interface, Class, Function 等）
//   - Chunk 无 entity_ids 和 tags
//   - 关系类型为代码域专属（IMPLEMENTS, CALLS, DEFINES 等）
func buildCodeSystemMessages(docID, lang, promptLang string) []chat.Message {
	if promptLang == LangEN {
		return []chat.Message{
			chat.NewSystemMessage(fmt.Sprintf(codeSegmentRoleDefinition, docID, lang)),
			chat.NewSystemMessage(codeSegmentChunkingRules),
			chat.NewSystemMessage(buildCodeEntityOntology(promptLang)),
			chat.NewSystemMessage(codeSegmentOutputFormat),
			chat.NewSystemMessage(fmt.Sprintf(codeSegmentConstraints, lang)),
		}
	}
	// 默认中文
	return []chat.Message{
		chat.NewSystemMessage(fmt.Sprintf(codeSegmentRoleDefinitionZH, docID, lang)),
		chat.NewSystemMessage(codeSegmentChunkingRulesZH),
		chat.NewSystemMessage(buildCodeEntityOntology(promptLang)),
		chat.NewSystemMessage(codeSegmentOutputFormatZH),
		chat.NewSystemMessage(fmt.Sprintf(codeSegmentConstraintsZH, lang)),
	}
}

// buildCodeEntityOntology 构建代码域的 Entity Types + Relation Types + Entity Schema 段。
//
// 被 buildCodeSystemMessages 内部调用，插入到分片 2 和分片 3 之间。
// 单独抽出以便测试。
func buildCodeEntityOntology(promptLang string) string {
	var b strings.Builder
	defs := codeEntityDefs
	relTypes := codeRelationTypes
	if promptLang == LangZH {
		defs = codeEntityDefsZH
		relTypes = codeRelationTypesZH
	}

	if promptLang == LangZH {
		b.WriteString("## 实体提取规则\n\n")
		b.WriteString("### 实体类型\n")
		b.WriteString("下面列出的每个实体类型成为图中的节点标签（MATCH (n:TypeName)）。\n")
	} else {
		b.WriteString("## Entity Extraction Rules\n\n")
		b.WriteString("### Entity Types\n")
		b.WriteString("Each entity type listed below becomes a node Label (MATCH (n:TypeName)).\n")
	}

	// 写第一项
	if len(defs) > 0 {
		b.WriteString(defs[0].Prompt)
	}
	// 写其余项
	for _, d := range defs[1:] {
		b.WriteByte('\n')
		b.WriteString(d.Prompt)
	}

	// Entity Schema 段
	if promptLang == LangZH {
		b.WriteString("\n\n### 实体 Schema — 下面每个实体类型的 schema 以其标签（类型名）为键。\n")
	} else {
		b.WriteString("\n\n### Entity Schema — Each entity type's schema below is keyed by its Label (type name).\n")
	}
	for _, d := range defs {
		if d.Schema == "" {
			continue
		}
		b.WriteString("```json\n")
		b.WriteString(d.Schema)
		b.WriteString("\n```\n")
	}

	b.WriteString("\n\n")
	b.WriteString(relTypes)

	return b.String()
}
