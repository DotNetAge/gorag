package indexer

import (
	"fmt"
	"strings"
)

// =============================================================================
// 全局统一的关系类型 — 所有领域共享，保证图数据库关系语义一致。
// =============================================================================

const globalRelationTypes = `### Relation Types
**Structural**: IS_A, PART_OF, CONTAINS
**Semantic**: DESCRIBES, CITES, RELATED_TO
**Logical**: IMPLIES, PRECEDES, DEPENDS_ON`

// =============================================================================
// 全局统一的提取约束 — 所有领域共享。
// =============================================================================

const globalExtractionConstraints = `### Extraction Constraints
- Extract entities only from the types defined below. Normalize abbreviations.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids. This creates the bidirectional link: chunk→entity via entity_ids, entity→chunk via SourceChunkIDs in the graph.
- Max 5 entities and 5 relations per chunk.
- Short content (<=3 lines) → extract only Topic, Term, and Concept (if applicable).
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// entityPropertyHints 常见实体类型的推荐属性。
// 由 buildPropertyGuidance 根据类型名查找。
var entityPropertyHints = map[string]string{
	"Concept":  "domain_or_field",
	"Term":     "definition",
	"Method":   "domain, steps_or_stages",
	"Resource": "format, source_or_author",
	"Tool":     "purpose, platform",
	"Person":   "role, expertise, affiliation",
	"Topic":    "description",
	"Event":    "date_or_time, location",
	"Work":     "creator, format",
	"Metric":   "measurement_unit, value_range",
}

// defaultEntityDefs 无自定义实体定义时的通用兜底。
var defaultEntityDefs = []EntityDef{
	{Prompt: "**Concept** — core idea, theory, principle, paradigm"},
	{Prompt: "**Term** — domain-specific term, jargon, noun"},
	{Prompt: "**Method** — methodology, process, technique, workflow"},
	{Prompt: "**Resource** — document, book, article, webpage, reference"},
	{Prompt: "**Tool** — software, platform, device, utility"},
	{Prompt: "**Person** — author, expert, contributor, role"},
	{Prompt: "**Topic** — subject, domain, category, tag"},
	{Prompt: "**Event** — milestone, meeting, occurrence, historical event"},
	{Prompt: "**Work** — creative output (blog, video, story, artwork, code)"},
	{Prompt: "**Metric** — KPI, measurement, score, statistic"},
}

// extractTypeName 从 Prompt 格式 "**Name** — description" 中提取类型名。
func extractTypeName(prompt string) string {
	// 查找 **Name** 模式
	start := strings.Index(prompt, "**")
	if start < 0 {
		return ""
	}
	start += 2
	end := strings.Index(prompt[start:], "**")
	if end < 0 {
		return ""
	}
	return prompt[start : start+end]
}

// buildPropertyGuidance 生成 ### Entity Properties 段文本。
// 对有 Schema 的类型引用其定义，对其他类型推断常见属性。
func buildPropertyGuidance(defs []EntityDef) string {
	var b strings.Builder
	b.WriteString("### Entity Properties\n")
	b.WriteString("Each entity's \"properties\" object MUST include \"description\" plus type-specific fields.\n")
	for _, d := range defs {
		typeName := extractTypeName(d.Prompt)
		if typeName == "" {
			continue
		}
		if d.Schema != "" {
			b.WriteString(fmt.Sprintf("- **%s**: use the schema defined in ### Entity Schema\n", typeName))
		} else if hint, ok := entityPropertyHints[typeName]; ok {
			b.WriteString(fmt.Sprintf("- **%s**: description, %s\n", typeName, hint))
		} else {
			b.WriteString(fmt.Sprintf("- **%s**: description, fields semantically relevant to %s\n", typeName, typeName))
		}
	}
	return b.String()
}

// buildOntology 组装完整的实体提取提示词。
// entityDefs 为 EntityDef 列表，每个 def.Prompt 追加在 ### Entity Types 下，
// 非空的 def.Schema 追加在 ### Entity Schema 下。
// 无实体定义时使用通用兜底定义。
func buildOntology(entityDefs []EntityDef) string {
	// 收集非空定义
	var defs []EntityDef
	for _, d := range entityDefs {
		if trimmed := strings.TrimSpace(d.Prompt); trimmed != "" {
			defs = append(defs, EntityDef{Prompt: trimmed, Schema: strings.TrimSpace(d.Schema)})
		}
	}

	// 无实体定义 → 通用兜底
	if len(defs) == 0 {
		defs = defaultEntityDefs
	}

	var b strings.Builder
	b.WriteString("## Entity Extraction Rules\n\n")
	b.WriteString("### Entity Types\n")
	b.WriteString(defs[0].Prompt)
	for _, d := range defs[1:] {
		b.WriteByte('\n')
		b.WriteString(d.Prompt)
	}

	// Entity Properties Guidance（动态生成，每个类型不同的属性建议）
	b.WriteString("\n\n")
	b.WriteString(buildPropertyGuidance(defs))

	// Entity Schema 段（仅当有非空 Schema 时追加）
	hasSchema := false
	for _, d := range defs {
		if d.Schema != "" {
			hasSchema = true
			break
		}
	}
	if hasSchema {
		b.WriteString("### Entity Schema — Each entity's properties MUST match its type's schema below\n")
		for _, d := range defs {
			if d.Schema == "" {
				continue
			}
			typeName := extractTypeName(d.Prompt)
			if typeName != "" {
				b.WriteString(fmt.Sprintf("**%s**\n", typeName))
			}
			b.WriteString("```json\n")
			b.WriteString(d.Schema)
			b.WriteString("\n```\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(globalRelationTypes)
	b.WriteString("\n\n")
	b.WriteString(globalExtractionConstraints)

	return b.String()
}
