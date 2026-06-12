package indexer

import "strings"

// =============================================================================
// Ontology 预设名常量
// 用户设置 ModelConfig.Ontology 时引用这些常量名。
// =============================================================================

const (
	OntologyNameGeneral    = "general"
	OntologyNameTech       = "tech"
	OntologyNameMedia      = "media"
	OntologyNameWriting    = "writing"
	OntologyNameResearch   = "research"
	OntologyNameFinance    = "finance"
	OntologyNameMedical    = "medical"
	OntologyNameJournalism = "journalism"
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

// =============================================================================
// 领域实体类型定义 — 每个预设只定义纯类型内容，不含标题。
// 复合时由 selectOntology 统一添加 ### Entity Types 标题。
// =============================================================================

const entityTypesGeneral = `**Knowledge** — Abstract and methodological knowledge: Concept, Term, Method
**Entity** — Named things and references: Resource, Tool, Person
**Output** — Creative works and temporal events: Work, Event
**Metadata** — Classification and quantification: Topic, Metric

#### Type Descriptions
- Concept: core idea, theory, principle, paradigm
- Term: domain-specific term, jargon, noun
- Method: methodology, process, technique, workflow
- Resource: document, book, article, webpage, reference
- Tool: software, platform, device, utility
- Person: author, expert, contributor, role
- Work: creative output (blog, video, story, artwork, code)
- Event: milestone, meeting, occurrence, historical event
- Topic: subject, domain, category, tag
- Metric: KPI, measurement, score, statistic`

const entityTypesTech = `**Concept** — Abstract top-level knowledge: CoreTheory, Term, Definition, Principle, Model
**KnowledgeUnit** — Independent knowledge points: Method, Process, Technique, Formula, Framework
**Resource** — Document/chunk layer: Document, Section, Chunk
**Practice** — Ground-level actionable knowledge: Tool, Step, Problem, Solution, Note
**Association** — Auxiliary knowledge: Person, Reference, Version, Tag`

const entityTypesMedia = `**Platform** — Distribution channels (e.g. 抖音, 公众号, B站, YouTube, 小红书)
**Format** — Content format (e.g. 短视频, 图文, 直播, 播客, 长视频)
**Topic** — Subject, trend, content idea (e.g. 热点话题, 选题, 趋势)
**Audience** — Target audience, follower persona, demographics
**Metric** — Performance metrics (e.g. 播放量, 点赞, 转化率, 粉丝数)
**Method** — Operation strategies (e.g. 运营策略, 排版技巧, 文案套路)
**Tool** — Tools and software (e.g. 剪映, 编辑器, 数据分析工具)
**Work** — Published content piece (e.g. 视频, 文章, 帖子)
**Event** — Activities and releases (e.g. 活动, 发布, 热点事件)
**Person** — Key individuals (e.g. KOL, 竞品账号, 创作者)`

const entityTypesWriting = `**Genre** — Literary genre (e.g. 科幻, 言情, 悬疑, 现实主义, 奇幻)
**Theme** — Central theme or motif (e.g. 爱, 自由, 科技与人, 成长)
**Character** — Fictional character, role, archetype
**Setting** — World-building elements (e.g. 时间, 地点, 社会背景, 世界观)
**Plot** — Storyline, conflict, narrative arc
**Style** — Narrative voice, writing style, perspective
**Structure** — Narrative structure (e.g. 三幕式, 起承转合, 非线性)
**Technique** — Literary device (e.g. 隐喻, 伏笔, 对话, 倒叙)
**Person** — Real person (e.g. author, critic, editor)
**Work** — Creative work (e.g. novel, poem, script, story)`

const entityTypesResearch = `**Market** — Industry, sector, market category
**Product** — Product, service, offering, feature
**Competitor** — Competing company, product, or alternative
**CustomerSegment** — Customer group, user persona, demographic
**Metric** — Business metrics (e.g. market share, growth rate, DAU, NPS)
**Trend** — Market trend, pattern, direction
**Method** — Research or analysis method (e.g. SWOT, 波特五力, 问卷)
**Organization** — Company, institution, brand, agency
**Event** — Business event (e.g. funding, launch, policy change)
**Person** — Key individual (e.g. analyst, executive, influencer)`

const entityTypesFinance = `**Asset** — Financial asset (e.g. stock, bond, real estate, commodity, crypto)
**Market** — Financial market or exchange (e.g. A股, NYSE, 债券市场)
**Indicator** — Economic or technical indicator (e.g. K线, PE, GDP, CPI)
**Strategy** — Investment or trading strategy (e.g. 定投, 对冲, 价值投资)
**Risk** — Risk factor (e.g. volatility, credit risk, inflation)
**Regulation** — Regulatory body or policy (e.g. SEC, 央行, 货币政策)
**Institution** — Financial institution (e.g. bank, fund, brokerage)
**Person** — Key figure (e.g. analyst, economist, fund manager)
**Event** — Market event (e.g. rate decision, earnings report, IPO)
**Metric** — Performance metric (e.g. 收益率, 夏普比率, ROI, 波动率)`

const entityTypesMedical = `**Disease** — Disease, disorder, condition (e.g. 糖尿病, hypertension, COVID-19)
**Symptom** — Sign, symptom, clinical presentation
**Treatment** — Treatment, therapy, regimen, medication class
**Drug** — Specific drug, dosage, formulation
**Anatomy** — Anatomical structure, organ, body system
**Procedure** — Medical procedure, surgery, examination
**Diagnosis** — Diagnostic criteria, test, biomarker
**Prevention** — Prevention method, vaccine, lifestyle measure
**Organization** — Healthcare organization (e.g. hospital, WHO, FDA)
**Person** — Healthcare professional (e.g. doctor, researcher, specialist)`

const entityTypesJournalism = `**Source** — Information source, news outlet, media (e.g. Reuters, 新华社, Twitter)
**Event** — News event, incident, development
**Figure** — Person or entity involved (e.g. politician, celebrity, witness)
**Topic** — News topic, issue, beat (e.g. 外交, 经济, 科技)
**Location** — Geographic location, region, country
**Organization** — Organization, government, NGO, company
**Claim** — Statement, allegation, assertion by a figure
**Fact** — Verified fact, data point, statistic
**Bias** — Stated or observed bias, perspective, spin
**Timeline** — Chronological marker, date, sequence`

// =============================================================================
// Ontology 注册表 — 只存 Entity Types 部分（不含标题）
// =============================================================================

var entityTypeRegistry = map[string]string{
	OntologyNameGeneral:    entityTypesGeneral,
	OntologyNameTech:       entityTypesTech,
	OntologyNameMedia:      entityTypesMedia,
	OntologyNameWriting:    entityTypesWriting,
	OntologyNameResearch:   entityTypesResearch,
	OntologyNameFinance:    entityTypesFinance,
	OntologyNameMedical:    entityTypesMedical,
	OntologyNameJournalism: entityTypesJournalism,
}

// selectOntology 组装完整的实体提取提示词。
//
// names 支持逗号分隔的多个预设名（如 "tech,finance"），会合并各预设的 Entity Types。
// customPrompts 是来自 WithOntologyTech 的自定义实体类型文本，追加在预设之后。
//
// 输出结构：
//
//	## Entity Extraction Rules
//	### Entity Types               ← 只输出一次
//	[所有预设的类型定义合并]
//	### Relation Types              ← 全局统一
//	### Extraction Constraints      ← 全局统一
//
// 无预设名 + 无自定义提示 → 回退通用型（general）。
func selectOntology(names string, customPrompts ...string) string {
	var entityParts []string

	if names != "" {
		for _, n := range strings.Split(names, ",") {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			text, ok := entityTypeRegistry[n]
			if !ok {
				continue
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			entityParts = append(entityParts, strings.TrimSpace(text))
		}
	}

	for _, p := range customPrompts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			entityParts = append(entityParts, trimmed)
		}
	}

	// 无任何内容 → 回退通用型
	if len(entityParts) == 0 {
		return "## Entity Extraction Rules\n\n" +
			"### Entity Types\n" + entityTypesGeneral + "\n\n" +
			globalRelationTypes + "\n\n" +
			globalExtractionConstraints
	}

	var b strings.Builder
	b.WriteString("## Entity Extraction Rules\n\n")
	b.WriteString("### Entity Types\n")

	for i, p := range entityParts {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p)
	}

	b.WriteString("\n\n")
	b.WriteString(globalRelationTypes)
	b.WriteString("\n\n")
	b.WriteString(globalExtractionConstraints)

	return b.String()
}
