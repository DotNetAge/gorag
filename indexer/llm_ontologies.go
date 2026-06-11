package indexer

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
// 实体提取规则 — 8 个领域预设
// 指令部分使用纯英文；领域专有名词例词保留原文。
// 每个预设包含：Entity Types（含描述）、Relation Types、Extraction Constraints。
// =============================================================================

// ontologyGeneral 通用型 — 适合大多数个人知识管理场景，不限定领域。
const ontologyGeneral = `## Entity Extraction Rules

### Entity Types (10 Types)
Only use the following entity types for general personal knowledge management:

**Knowledge** — Abstract and methodological knowledge: Concept, Term, Method
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
- Metric: KPI, measurement, score, statistic

### Relation Types (9 Types)
**Structural**: IS_A, PART_OF, CONTAINS
**Semantic**: DESCRIBES, CITES, RELATED_TO
**Logical**: IMPLIES, PRECEDES, DEPENDS_ON

### Extraction Constraints
- Extract entities only from the 10 types above. Normalize abbreviations.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids. This creates the bidirectional link: chunk→entity via entity_ids, entity→chunk via SourceChunkIDs in the graph.
- Max 5 entities and 5 relations per chunk.
- Short content (<=3 lines) → extract only Topic, Term, and Concept.
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// ontologyTech 技术文档 — 适用于代码、架构、方法论、技术知识管理。
const ontologyTech = `## Entity Extraction Rules

### Entity Taxonomy (18 Types, 5 Categories)
Only use the following entity types:

**Concept** — Abstract top-level knowledge: CoreTheory, Term, Definition, Principle, Model
**KnowledgeUnit** — Independent knowledge points: Method, Process, Technique, Formula, Framework
**Resource** — Document/chunk layer: Document, Section, Chunk
**Practice** — Ground-level actionable knowledge: Tool, Step, Problem, Solution, Note
**Association** — Auxiliary knowledge: Person, Reference, Version, Tag

### Relation Whitelist (14 Types)
**Hierarchy**: IS_A, PART_OF, CONTAINS, CLASSIFIED_AS
**Content**: DESCRIBES, CITES, EXEMPLIFIES
**Logic**: IMPLIES, EQUIVALENT_TO, CONTRADICTS, EXTENDS
**Dependency**: PRECEDES, DEPENDS_ON, COMPLEMENTS
**Practice**: APPLIES_TO, SOLVES, DEMONSTRATES

### Extraction Constraints
- Extract only within the entity taxonomy above. Use only the 14 relation names above.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids.
- Short chunks (<=3 lines) → extract Terms only; paragraphs → all applicable categories.
- Normalize aliases/abbreviations to standard names (e.g. "k8s" → "Kubernetes").
- Max 5 entities and max 5 relations per chunk.
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// ontologyMedia 自媒体运营 — 适用于短视频、图文、直播等内容创作与分发管理。
const ontologyMedia = `## Entity Extraction Rules

### Entity Types (10 Types)
Only use the following entity types for content/media management:

**Platform** — Distribution channels (e.g. 抖音, 公众号, B站, YouTube, 小红书)
**Format** — Content format (e.g. 短视频, 图文, 直播, 播客, 长视频)
**Topic** — Subject, trend, content idea (e.g. 热点话题, 选题, 趋势)
**Audience** — Target audience, follower persona, demographics
**Metric** — Performance metrics (e.g. 播放量, 点赞, 转化率, 粉丝数)
**Method** — Operation strategies (e.g. 运营策略, 排版技巧, 文案套路)
**Tool** — Tools and software (e.g. 剪映, 编辑器, 数据分析工具)
**Work** — Published content piece (e.g. 视频, 文章, 帖子)
**Event** — Activities and releases (e.g. 活动, 发布, 热点事件)
**Person** — Key individuals (e.g. KOL, 竞品账号, 创作者)

### Relation Types (8 Types)
PUBLISHED_ON — published on a platform
TARGETS — targets a specific audience
BELONGS_TO — belongs to a topic/category
GENERATES — generates metrics/results
RELATED_TO — related to another entity
CITES — cites or references
PRECEDES — precedes in timeline
COMPLEMENTS — complements another work

### Extraction Constraints
- Extract entities only from the 10 types above.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids.
- Normalize platform/account names to standard form.
- Max 5 entities and 5 relations per chunk.
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// ontologyWriting 文学写作/创作 — 适用于小说、剧本、诗歌等文学创作分析。
const ontologyWriting = `## Entity Extraction Rules

### Entity Types (10 Types)
Only use the following entity types for literary and creative writing:

**Genre** — Literary genre (e.g. 科幻, 言情, 悬疑, 现实主义, 奇幻)
**Theme** — Central theme or motif (e.g. 爱, 自由, 科技与人, 成长)
**Character** — Fictional character, role, archetype
**Setting** — World-building elements (e.g. 时间, 地点, 社会背景, 世界观)
**Plot** — Storyline, conflict, narrative arc
**Style** — Narrative voice, writing style, perspective
**Structure** — Narrative structure (e.g. 三幕式, 起承转合, 非线性)
**Technique** — Literary device (e.g. 隐喻, 伏笔, 对话, 倒叙)
**Person** — Real person (e.g. author, critic, editor)
**Work** — Creative work (e.g. novel, poem, script, story)

### Relation Types (8 Types)
BELONGS_TO — belongs to a genre/category
SET_IN — set in a particular setting/world
FEATURES — features a character or technique
FOLLOWS — follows a structure or style
INSPIRED_BY — inspired by another work or person
PART_OF — part of a series or collection
PRECEDES — precedes in chronological order
COMPLEMENTS — complements another work or theme

### Extraction Constraints
- Extract entities only from the 10 types above.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids.
- Character names should use canonical form (full name).
- Max 5 entities and 5 relations per chunk.
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// ontologyResearch 市场研究 — 适用于行业分析、竞品调研、市场趋势研究。
const ontologyResearch = `## Entity Extraction Rules

### Entity Types (10 Types)
Only use the following entity types for market research and analysis:

**Market** — Industry, sector, market category
**Product** — Product, service, offering, feature
**Competitor** — Competing company, product, or alternative
**CustomerSegment** — Customer group, user persona, demographic
**Metric** — Business metrics (e.g. market share, growth rate, DAU, NPS)
**Trend** — Market trend, pattern, direction
**Method** — Research or analysis method (e.g. SWOT, 波特五力, 问卷)
**Organization** — Company, institution, brand, agency
**Event** — Business event (e.g. funding, launch, policy change)
**Person** — Key individual (e.g. analyst, executive, influencer)

### Relation Types (8 Types)
BELONGS_TO — belongs to a market/segment
COMPETES_WITH — competes with another entity
TARGETS — targets a customer segment
GENERATES — generates metrics/outcomes
CITES — cites a source or reference
DEPENDS_ON — depends on another factor
DRIVES — drives or influences an outcome
PRECEDES — precedes in sequence or timeline

### Extraction Constraints
- Extract entities only from the 10 types above.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids.
- Normalize company and product names to official form.
- Max 5 entities and 5 relations per chunk.
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// ontologyFinance 金融投资 — 适用于股票、基金、宏观经济、投资分析。
const ontologyFinance = `## Entity Extraction Rules

### Entity Types (10 Types)
Only use the following entity types for finance and investment:

**Asset** — Financial asset (e.g. stock, bond, real estate, commodity, crypto)
**Market** — Financial market or exchange (e.g. A股, NYSE, 债券市场)
**Indicator** — Economic or technical indicator (e.g. K线, PE, GDP, CPI)
**Strategy** — Investment or trading strategy (e.g. 定投, 对冲, 价值投资)
**Risk** — Risk factor (e.g. volatility, credit risk, inflation)
**Regulation** — Regulatory body or policy (e.g. SEC, 央行, 货币政策)
**Institution** — Financial institution (e.g. bank, fund, brokerage)
**Person** — Key figure (e.g. analyst, economist, fund manager)
**Event** — Market event (e.g. rate decision, earnings report, IPO)
**Metric** — Performance metric (e.g. 收益率, 夏普比率, ROI, 波动率)

### Relation Types (10 Types)
INVESTED_IN — invested in a particular asset/market
AFFECTS — affects or influences another entity
CORRELATES_WITH — correlates with another metric or asset
PREDICTS — predicts or forecasts an outcome
REGULATED_BY — regulated by a body or policy
DERIVED_FROM — derived from a metric or calculation
COMPARED_TO — compared to a benchmark or peer
DEPENDS_ON — depends on a factor or condition
PRECEDES — precedes in sequence
DRIVES — drives or leads to an outcome

### Extraction Constraints
- Extract entities only from the 10 types above.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids.
- Normalize ticker symbols and company names to standard form.
- Max 5 entities and 5 relations per chunk.
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// ontologyMedical 医学 — 适用于疾病、药物、诊疗、医学研究知识管理。
const ontologyMedical = `## Entity Extraction Rules

### Entity Types (10 Types)
Only use the following entity types for medical and healthcare knowledge:

**Disease** — Disease, disorder, condition (e.g. 糖尿病, hypertension, COVID-19)
**Symptom** — Sign, symptom, clinical presentation
**Treatment** — Treatment, therapy, regimen, medication class
**Drug** — Specific drug, dosage, formulation
**Anatomy** — Anatomical structure, organ, body system
**Procedure** — Medical procedure, surgery, examination
**Diagnosis** — Diagnostic criteria, test, biomarker
**Prevention** — Prevention method, vaccine, lifestyle measure
**Organization** — Healthcare organization (e.g. hospital, WHO, FDA)
**Person** — Healthcare professional (e.g. doctor, researcher, specialist)

### Relation Types (9 Types)
MANIFESTS — manifests as a symptom or sign
TREATED_BY — treated by a drug or therapy
CAUSES — causes or leads to a condition
PREVENTS — prevents a disease or symptom
DIAGNOSED_BY — diagnosed by a test or criterion
INTERACTS_WITH — interacts with another drug or substance
CONTRAINDICATES — contraindicates a treatment
ASSOCIATED_WITH — associated with a risk factor
PART_OF — part of a system or classification

### Extraction Constraints
- Extract entities only from the 10 types above.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids.
- Use standard medical terminology (ICD/SNOMED preferred form).
- Max 5 entities and 5 relations per chunk.
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// ontologyJournalism 新闻 — 适用于新闻报道、时事分析、信息核实。
const ontologyJournalism = `## Entity Extraction Rules

### Entity Types (10 Types)
Only use the following entity types for journalism and news analysis:

**Source** — Information source, news outlet, media (e.g. Reuters, 新华社, Twitter)
**Event** — News event, incident, development
**Figure** — Person or entity involved (e.g. politician, celebrity, witness)
**Topic** — News topic, issue, beat (e.g. 外交, 经济, 科技)
**Location** — Geographic location, region, country
**Organization** — Organization, government, NGO, company
**Claim** — Statement, allegation, assertion by a figure
**Fact** — Verified fact, data point, statistic
**Bias** — Stated or observed bias, perspective, spin
**Timeline** — Chronological marker, date, sequence

### Relation Types (9 Types)
REPORTED_BY — reported by a source/outlet
ALLEGES — alleges or claims by a figure
CONFIRMS — confirms a fact or claim
CONTRADICTS — contradicts another claim or fact
CONTEXTUALIZES — provides context for an event
PART_OF — part of a series or broader topic
PRECEDES — precedes in timeline
CAUSES — causes or triggers an event
RELATED_TO — related to another entity or topic

### Extraction Constraints
- Extract entities only from the 10 types above.
- Each chunk's entity_ids field MUST list all entity ids extracted from that chunk. Every entity must appear in at least one chunk's entity_ids.
- Distinguish between claims (unverified) and facts (verified).
- Max 5 entities and 5 relations per chunk.
- Always create a "Chunk DESCRIBES Entity" edge for each extracted entity.`

// =============================================================================
// Ontology 注册表与选择
// =============================================================================

var ontologyRegistry = map[string]string{
	OntologyNameGeneral:    ontologyGeneral,
	OntologyNameTech:       ontologyTech,
	OntologyNameMedia:      ontologyMedia,
	OntologyNameWriting:    ontologyWriting,
	OntologyNameResearch:   ontologyResearch,
	OntologyNameFinance:    ontologyFinance,
	OntologyNameMedical:    ontologyMedical,
	OntologyNameJournalism: ontologyJournalism,
}

// selectOntology 根据预设名选取实体提取规则的 System Prompt 文本。
// 未知名称或空值回退到通用型（general）。
func selectOntology(name string) string {
	if text, ok := ontologyRegistry[name]; ok {
		return text
	}
	return ontologyGeneral
}
