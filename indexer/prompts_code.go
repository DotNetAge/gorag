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

// codeSegmentRoleDefinition 片段 1：角色定义（代码分析引擎）。
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

// codeSegmentChunkingRules 片段 2：代码分块规则。
const codeSegmentChunkingRules = `## Chunking Rules for Code
1. Group by logical code boundaries: function, method, class, struct, interface, enum, and top-level type definitions. Each is one chunk.
2. For each chunk, generate a short summary (<200 chars) describing what the code segment does — equivalent to a doc comment.
3. Use the function/class/type name as the chunk title. For unnamed blocks, use a descriptive label.
4. The chunk "content" field must contain the EXACT original code lines for the range. Do not paraphrase.
5. Merge adjacent small helpers (<5 lines each) that belong to the same logical unit into one chunk.
6. License headers, generated-code markers, empty boilerplate, and auto-generated sections MUST be filtered out.`

// codeSegmentOutputFormat 片段 3：输出格式（简化版，无 tags / entity_ids）。
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

// codeSegmentConstraints 片段 4：约束条件。
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

// buildCodeSystemMessages 构建代码文档专用的 SystemMessage 分片列表。
// 与 buildSystemMessages 平行。
// 调用方只需 append UserMessage 即可完成 Messages 组装。
//
// 与文本域的关键区别：
//   - 不使用用户选择的 entityDefs，始终使用内置的 codeEntityDefs
//   - 实体类型为代码域专属（Interface, Class, Function 等）
//   - Chunk 无 entity_ids 和 tags
//   - 关系类型为代码域专属（IMPLEMENTS, CALLS, DEFINES 等）
func buildCodeSystemMessages(docID, lang string) []chat.Message {
	return []chat.Message{
		chat.NewSystemMessage(fmt.Sprintf(codeSegmentRoleDefinition, docID, lang)),
		chat.NewSystemMessage(codeSegmentChunkingRules),
		chat.NewSystemMessage(buildCodeEntityOntology()),
		chat.NewSystemMessage(codeSegmentOutputFormat),
		chat.NewSystemMessage(fmt.Sprintf(codeSegmentConstraints, lang)),
	}
}

// buildCodeEntityOntology 构建代码域的 Entity Types + Relation Types + Entity Schema 段。
//
// 被 buildCodeSystemMessages 内部调用，插入到分片 2 和分片 3 之间。
// 单独抽出以便测试。
func buildCodeEntityOntology() string {
	var b strings.Builder
	b.WriteString("## Entity Extraction Rules\n\n")
	b.WriteString("### Entity Types\n")
	b.WriteString("Each entity type listed below becomes a node Label (MATCH (n:TypeName)).\n")

	// 写第一项
	if len(codeEntityDefs) > 0 {
		b.WriteString(codeEntityDefs[0].Prompt)
	}
	// 写其余项
	for _, d := range codeEntityDefs[1:] {
		b.WriteByte('\n')
		b.WriteString(d.Prompt)
	}

	// Entity Schema 段
	b.WriteString("\n\n### Entity Schema — Each entity type's schema below is keyed by its Label (type name).\n")
	for _, d := range codeEntityDefs {
		if d.Schema == "" {
			continue
		}
		b.WriteString("```json\n")
		b.WriteString(d.Schema)
		b.WriteString("\n```\n")
	}

	b.WriteString("\n\n")
	b.WriteString(codeRelationTypes)

	return b.String()
}
