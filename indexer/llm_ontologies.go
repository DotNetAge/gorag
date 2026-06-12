package indexer

import "strings"

// =============================================================================
// 全局统一的关系类型 — 所有领域共享，保证图数据库关系语义一致。
// =============================================================================

const globalRelationTypes = `### Relation Types (9 Types)
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

// buildOntology 组装完整的实体提取提示词。
// entityDefs 是实体类型定义行列表，每个元素作为一行追加在 ### Entity Types 下。
// 无实体定义时使用通用兜底定义。
//
// 输出结构：
//
//	## Entity Extraction Rules
//	### Entity Types               ← 只输出一次
//	[entityDefs 类型定义行]
//	### Relation Types              ← 全局统一
//	### Extraction Constraints      ← 全局统一
func buildOntology(entityDefs []string) string {
	// 收集非空定义
	var defs []string
	for _, d := range entityDefs {
		if trimmed := strings.TrimSpace(d); trimmed != "" {
			defs = append(defs, trimmed)
		}
	}

	// 无实体定义 → 通用兜底
	if len(defs) == 0 {
		return "## Entity Extraction Rules\n\n" +
			"### Entity Types\n" +
			"**Concept** — core idea, theory, principle, paradigm\n" +
			"**Term** — domain-specific term, jargon, noun\n" +
			"**Method** — methodology, process, technique, workflow\n" +
			"**Resource** — document, book, article, webpage, reference\n" +
			"**Tool** — software, platform, device, utility\n" +
			"**Person** — author, expert, contributor, role\n" +
			"**Topic** — subject, domain, category, tag\n" +
			"**Event** — milestone, meeting, occurrence, historical event\n" +
			"**Work** — creative output (blog, video, story, artwork, code)\n" +
			"**Metric** — KPI, measurement, score, statistic\n\n" +
			globalRelationTypes + "\n\n" +
			globalExtractionConstraints
	}

	var b strings.Builder
	b.WriteString("## Entity Extraction Rules\n\n")
	b.WriteString("### Entity Types\n")
	b.WriteString(defs[0])
	for _, d := range defs[1:] {
		b.WriteByte('\n')
		b.WriteString(d)
	}
	b.WriteString("\n\n")
	b.WriteString(globalRelationTypes)
	b.WriteString("\n\n")
	b.WriteString(globalExtractionConstraints)

	return b.String()
}
