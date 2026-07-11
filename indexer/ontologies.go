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

const globalRelationTypesZH = `### 关系类型
**结构关系**: IS_A, PART_OF, CONTAINS
**语义关系**: DESCRIBES, CITES, RELATED_TO
**逻辑关系**: IMPLIES, PRECEDES, DEPENDS_ON`

// =============================================================================
// 全局统一的提取约束 — 所有领域共享。
// =============================================================================

const globalExtractionConstraints = `### Extraction Constraints
- Each entity's "type" field becomes its node Label in the graph — use this to match via MATCH (n:TypeName).
- Extract entities only from the types defined below. Normalize abbreviations.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids. This creates the bidirectional link: chunk→entity via entity_ids, entity→chunk via SourceChunkIDs in the graph.
- Max 5 entities and 5 relations per chunk.
- Short content (<=3 lines) → extract only Topic, Term, and Concept (if applicable).
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

const globalExtractionConstraintsZH = `### 提取约束
- 每个实体的 "type" 字段成为图中的节点标签 — 使用 MATCH (n:TypeName) 进行匹配。
- 仅从下面定义的类型中提取实体。标准化缩写。
- 每个 chunk 的 entity_ids 字段必须列出从该 chunk 中提取的所有实体 ID。每个实体必须至少出现在一个 chunk 的 entity_ids 中。这创建了双向链接：chunk→entity 通过 entity_ids，entity→chunk 通过 SourceChunkIDs。
- 每个 chunk 最多 5 个实体和 5 个关系。
- 短内容（<=3 行）→ 仅提取 Topic、Term 和 Concept（如适用）。
- 为每个提取的实体创建 "Chunk DESCRIBES Entity" 边。`

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

var entityPropertyHintsZH = map[string]string{
	"Concept":  "领域或学科",
	"Term":     "定义",
	"Method":   "领域, 步骤或阶段",
	"Resource": "格式, 来源或作者",
	"Tool":     "用途, 平台",
	"Person":   "角色, 专业领域, 所属机构",
	"Topic":    "描述",
	"Event":    "时间, 地点",
	"Work":     "创作者, 格式",
	"Metric":   "测量单位, 数值范围",
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

var defaultEntityDefsZH = []EntityDef{
	{Prompt: "**Concept** — 核心概念、理论、原则、范式"},
	{Prompt: "**Term** — 领域特定术语、行话、名词"},
	{Prompt: "**Method** — 方法论、流程、技术、工作流"},
	{Prompt: "**Resource** — 文档、书籍、文章、网页、参考资料"},
	{Prompt: "**Tool** — 软件、平台、设备、工具"},
	{Prompt: "**Person** — 作者、专家、贡献者、角色"},
	{Prompt: "**Topic** — 主题、领域、分类、标签"},
	{Prompt: "**Event** — 里程碑、会议、事件、历史事件"},
	{Prompt: "**Work** — 创作成果（博客、视频、故事、艺术品、代码）"},
	{Prompt: "**Metric** — KPI、测量指标、分数、统计数据"},
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
func buildPropertyGuidance(defs []EntityDef, promptLang string) string {
	var b strings.Builder
	hints := entityPropertyHints
	if promptLang == LangZH {
		b.WriteString("### 实体属性\n")
		b.WriteString("每个实体的 \"properties\" 对象必须包含 \"description\" 以及类型特定字段。\n")
		hints = entityPropertyHintsZH
	} else {
		b.WriteString("### Entity Properties\n")
		b.WriteString("Each entity's \"properties\" object MUST include \"description\" plus type-specific fields.\n")
	}
	for _, d := range defs {
		typeName := extractTypeName(d.Prompt)
		if typeName == "" {
			continue
		}
		if d.Schema != "" {
			if promptLang == LangZH {
				b.WriteString(fmt.Sprintf("- **%s**: 使用 ### 实体 Schema 中定义的 schema\n", typeName))
			} else {
				b.WriteString(fmt.Sprintf("- **%s**: use the schema defined in ### Entity Schema\n", typeName))
			}
		} else if hint, ok := hints[typeName]; ok {
			if promptLang == LangZH {
				b.WriteString(fmt.Sprintf("- **%s**: description, %s\n", typeName, hint))
			} else {
				b.WriteString(fmt.Sprintf("- **%s**: description, %s\n", typeName, hint))
			}
		} else {
			if promptLang == LangZH {
				b.WriteString(fmt.Sprintf("- **%s**: description, 与 %s 语义相关的字段\n", typeName, typeName))
			} else {
				b.WriteString(fmt.Sprintf("- **%s**: description, fields semantically relevant to %s\n", typeName, typeName))
			}
		}
	}
	return b.String()
}

// buildOntology 组装完整的实体提取提示词。
// entityDefs 为 EntityDef 列表，每个 def.Prompt 追加在 ### Entity Types 下，
// 非空的 def.Schema 追加在 ### Entity Schema 下。
// 无实体定义时使用通用兜底定义。
// promptLang 控制提示词语言（LangEN | LangZH）。
func buildOntology(entityDefs []EntityDef, promptLang string) string {
	// 收集非空定义
	var defs []EntityDef
	for _, d := range entityDefs {
		if trimmed := strings.TrimSpace(d.Prompt); trimmed != "" {
			defs = append(defs, EntityDef{Prompt: trimmed, Schema: strings.TrimSpace(d.Schema)})
		}
	}

	// 无实体定义 → 通用兜底
	if len(defs) == 0 {
		if promptLang == LangZH {
			defs = defaultEntityDefsZH
		} else {
			defs = defaultEntityDefs
		}
	}

	var b strings.Builder
	if promptLang == LangZH {
		b.WriteString("## 实体提取规则\n\n")
		b.WriteString("### 实体类型\n")
		b.WriteString("下面列出的每个实体类型成为图中的节点标签（MATCH (n:TypeName)）。\n")
	} else {
		b.WriteString("## Entity Extraction Rules\n\n")
		b.WriteString("### Entity Types\n")
		b.WriteString("Each entity type listed below becomes a node Label (MATCH (n:TypeName)).\n")
	}
	b.WriteString(defs[0].Prompt)
	for _, d := range defs[1:] {
		b.WriteByte('\n')
		b.WriteString(d.Prompt)
	}

	// Entity Properties Guidance（动态生成，每个类型不同的属性建议）
	b.WriteString("\n\n")
	b.WriteString(buildPropertyGuidance(defs, promptLang))

	// Entity Schema 段（仅当有非空 Schema 时追加）
	hasSchema := false
	for _, d := range defs {
		if d.Schema != "" {
			hasSchema = true
			break
		}
	}
	if hasSchema {
		if promptLang == LangZH {
			b.WriteString("### 实体 Schema — 下面每个实体类型的 schema 以其标签（类型名）为键。\n")
		} else {
			b.WriteString("### Entity Schema — Each entity type's schema below is keyed by its Label (type name).\n")
		}
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
	if promptLang == LangZH {
		b.WriteString(globalRelationTypesZH)
		b.WriteString("\n\n")
		b.WriteString(globalExtractionConstraintsZH)
	} else {
		b.WriteString(globalRelationTypes)
		b.WriteString("\n\n")
		b.WriteString(globalExtractionConstraints)
	}

	return b.String()
}
