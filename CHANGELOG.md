# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased] - 2026-03-17

### 📚 Documentation - Major Writing Overhaul

#### **New Advanced Guides (Specs 17-22)**
- **Spec 17: Advanced RAG Deep Dive** - Comprehensive guide covering Pre-Retrieval, During Retrieval, and Post-Retrieval optimization techniques
  - Query Rewriting with LLM
  - HyDE (Hypothetical Document Embeddings)
  - Step-Back Prompting for abstract reasoning
  - Multi-granularity indexing strategies
  
- **Spec 18: Graph RAG Deep Dive** - Production-ready guide for knowledge graph integration
  - Entity extraction and relation extraction pipelines
  - Graph local search (1-2 hops) vs global search (3+ hops)
  - Hybrid search combining vector + graph retrieval
  - Real-world cases: supply chain analysis, academic recommendations, medical diagnosis
  
- **Spec 19: Agentic RAG Deep Dive** - Autonomous decision-making RAG systems
  - Reasoning module for task decomposition
  - Tool system (Retrieval, Calculator, Search, Code Interpreter)
  - Parallel execution and dependency management
  - Self-reflection and quality checking
  
- **Spec 20: Multimodal RAG Deep Dive** - Cross-modal text-image retrieval
  - CLIP dual-tower architecture explained
  - Unified vector space projection principles
  - Chinese-CLIP integration guide
  - Embedding model selection criteria
  
- **Spec 21: Multimodal RAG Query and Multi-way Recall** - Three-way recall strategy
  - Text branch, Image branch, Hybrid branch
  - RRF (Reciprocal Rank Fusion) algorithm implementation
  - Performance benchmarks: Recall@10 improved from 0.65 to 0.86
  
- **Spec 22: GoRAG Advanced Developer Guide** - Four-layer progressive capability model ⭐
  - **Level 1**: Indexer + Searcher (5 lines, production-first)
  - **Level 2**: Pipeline + Steps (20 lines, flexible orchestration)
  - **Level 3**: Custom Steps (50-100 lines, domain-specific logic)
  - **Level 4**: Core packages (100+ lines, ultimate performance)

#### **Reference Manuals**
- **GoRAG_Steps_Reference_Manual.md** - Complete dictionary of all 40 Steps
  - 7 Base Steps (Parse, Chunk, Embedding, Store, etc.)
  - 6 Pre-Retrieval Steps (QueryRewrite, HyDE, StepBack, etc.)
  - 8 Retrieval Steps (VectorSearch, ImageSearch, GraphLocal/Global, Fusion, etc.)
  - 11 Post-Retrieval Steps (Rerank, Generation, Compression, Pruning, etc.)
  - 8 Agentic Steps (Reasoning, ActionSelection, ToolExecutor, etc.)
  - Each Step includes: function signature, options, usage examples, performance data

### ✨ Enhanced Developer Experience

#### **Indexer/Searcher First Approach** 
- All guides now prioritize `DefaultIndexer` + `DefaultSearcher` as the simplest entry point
- Added comparison tables showing 4-20x code reduction vs manual Pipeline assembly
- Clear migration path from Level 1 → Level 2 → Level 3 → Level 4

#### **Code Authenticity Guarantee**
- Removed all fabricated pseudo-code from documentation
- Verified every Step against actual `infra/steps/` source code
- Ensured all package paths are real and importable
- Fixed type mismatches (`pipeline.PipelineState` → `entity.PipelineState`)

#### **Unified Code Style**
- Standardized on `pipeline.AddSteps()` for elegant chain assembly
- Consistent Option Pattern usage across all examples
- Realistic performance benchmarks included in every section

### 🔧 Technical Corrections

#### **Fixed Documentation Errors**
- ❌ Removed fake `graphstep` package (doesn't exist)
- ✅ Replaced with real `retrievalstep.GraphLocalSearchStep` and `GraphGlobalSearchStep`
- ❌ Removed fabricated `RelationExtractStep`, `GraphEmbeddingStep`
- ✅ Simplified to use existing `EntityExtractStep` + `EmbeddingStep`
- ❌ Removed overly complex manual Pipeline examples
- ✅ Added simple `Indexer.WithQueryRewrite()`, `Searcher.WithHyDE()` alternatives

#### **Type Safety Improvements**
- Corrected all state types to use `*entity.PipelineState` consistently
- Fixed return type annotations in code examples
- Ensured all imports match actual package structures

### 📊 Documentation Metrics

| Category | Count | Total Size |
|----------|-------|------------|
| **Total Specs** | 22 documents | ~500KB |
| **Advanced Guides** | 6 docs (17-22) | ~150KB |
| **Step Reference** | 40 Steps documented | ~50KB |
| **Code Examples** | 100+ runnable examples | - |
| **Performance Benchmarks** | 20+ comparison tables | - |

### 🎯 Design Philosophy Updates

**"Simple is better than complex, but flexibility must not be sacrificed"**

- **80% scenarios**: Use Level 1 (Indexer + Searcher) - 5 lines
- **15% scenarios**: Use Level 2 (Pipeline orchestration) - 20 lines  
- **4% scenarios**: Use Level 3 (Custom Steps) - 50 lines
- **1% scenarios**: Use Level 4 (Core packages) - 100+ lines

**Key Principle**: Always start with the highest abstraction. Drop down only when necessary.

---

## [1.0.0] - 2026-03-10

### 🚀 Major Architectural Updates
- **Complete LLM Engine Migration**: Deprecated and removed the internal `llm` package in favor of the unified [`gochat`](https://github.com/DotNetAge/gochat) SDK. This brings enterprise-grade stability, unified message structures, streaming events, and out-of-the-box support for OpenAI, Anthropic, Ollama, Azure, and multiple domestic Chinese LLMs (Kimi, DeepSeek, GLM-4, etc.).
- **Native Vector Store Integration**: Introduced [`govector`](https://github.com/DotNetAge/govector) as a first-class citizen. `GoRAG` now ships with a pure Go, zero-dependency embedded vector database, allowing developers to run a full RAG pipeline locally without setting up external databases like Milvus, Qdrant, or Pinecone.
- **Parser Ecosystem Decoupling**: Moved heavy CGO-dependent parsers (Audio, Video, Webpage) into independent plugin repositories (`gorag-audio`, `gorag-video`, `gorag-webpage`) to keep the core framework ultra-lightweight and compilation times fast.

### ✨ Added
- **Ollama Client Upgrades**: Native integration via `gochat`, providing robust support for running local open-source models (Llama 3, Qwen, Mistral).
- **16 Native Parsers**: Built-in, streaming-supported, pure Go parsers for `txt`, `md`, `csv`, `json`, `yaml`, `html`, `xml`, `log`, `sql`, and various programming languages (`go`, `py`, `js`, `ts`, `java`, `email`).
- **Concurrent Directory Indexing**: Added a powerful 10-worker concurrent processing engine (`IndexDirectory` and `AsyncIndexDirectory`) capable of ingesting entire codebases or 100M+ files rapidly.
- **Advanced RAG Features**: Native implementation of Multi-hop RAG, Agentic RAG, Semantic Chunking, HyDE (Hypothetical Document Embeddings), and RAG-Fusion.
- **Resilience Mechanisms**: Added Circuit Breaker, rate-limiting, connection pooling, and graceful degradation strategies for high-availability production deployments.

### 🧹 Removed
- **Internal `llm` package**: Completely deleted. Replaced by `github.com/DotNetAge/gochat/pkg/core` interfaces.
- Legacy prompt formatting wrappers that didn't align with the standard multi-turn Chat structures.

### 🐛 Fixed
- Resolved integration test flakiness with Testcontainers for Milvus, Qdrant, and Weaviate.
- Fixed `mockLLM` implementations in test suites to correctly emulate `gochat`'s new stream chunk structures (`gochatcore.StreamEvent`).
- Fixed vector dimension mismatches and improved test coverage to reliably stay above 85%.



1. GraphRAG 检索器实现 (pkg/retriever/graph)：
    * 多路召回：实现了实体提取（Entity Extraction）和知识图谱遍历（Neighbors Search）。
    * 混合检索：将图谱的结构化关系与向量库的非结构化分块相结合。
    * 模版化生成：自定义了生成步骤，通过 text/template 将图谱上下文和文档分块精准喂给 LLM。
    * 验证：已完成单元测试，确保了从查询到实体、再到图谱检索和生成的链路通畅。
2. AgenticRAG 检索器实现 (pkg/retriever/agentic)：
    * 自主推理：构建了基于 Agent 接口的检索器，支持 LLM 决定何时使用何种工具。
    * 追踪能力：在检索结果中集成了“推理步骤”（Thought/Action/Observation）的追踪，方便调试和审计。
    * 统一上下文：增强了核心 RetrievalContext，支持智能体在多轮迭代中保存中间状态。
3. 基础设施建设
   * 新增 Enrichment 步骤：在 pkg/steps/enrich/docstore.go 中实现了通用的 EnrichWithDocStore 插件。该步骤能自动识别检索到的 Chunk 及其关联的 DocumentID，并从 DocStore 中实时召回父文档全文。
4. 高级检索器升级，我对以下三个核心检索器进行了改造，使其支持 WithDocStore 选项：
   * GraphRAG：在图谱检索后，利用 DocStore 补全实体的原始文本背景，解决图谱节点描述过于抽象的问题。
   * CRAG：当评估结果为“模糊（Ambiguous）”时，优先触发 DocStore 的深度证据挖掘，尝试在本域内解决语境缺失，减少昂贵的 Web 搜索调用。
   * Self-RAG：在自我反思（Reflection）环节，若发现生成的回答不完整，会自动触发 DocStore 召回父文档，为 Refinement 循环提供更宏观的语义支持。

---

技术亮点：父子文档检索 (PDR)

在本项目中，DocStore 不再仅仅是一个静态存储库，而是通过 Enrichment Step 变成了一个动态语境增强器。这解决了 RAG 的经典痛点：检索时需要小分块（精确），生成时需要大上下文（全面）。


---

1. 新增的存储驱动
   * SQLite GraphStore: 推荐的默认本地方案。利用递归 CTE 实现高效的多跳关系遍历，无需额外安装数据库。
   * BoltDB GraphStore: 纯 Go 实现的嵌入式 K/V 方案。通过内置邻接表索引，提供极高的本地读写性能。
   * Neo4j GraphStore: 工业级方案。支持完整的 Cypher 查询语言，适用于大规模、复杂的企业级知识图谱。
2. 核心架构优化
   * 标准化接口: 所有驱动均严格遵循 store.GraphStore 接口，支持 UpsertNodes、UpsertEdges 和 GetNeighbors。
   * Cypher 扩展: Neo4j 实现中直接支持 Query 方法执行原生 Cypher 语句，为未来的复杂图算法（如 PageRank 或社区发现）留下了接口。
   * GraphRAG 深度集成: 这些存储驱动已与 pkg/retriever/graph 检索器无缝对接。现在在 GraphRAG 模式下，系统会根据提取的实体，自动从这些存储中检索子图上下文。
3. 交付物
   * pkg/indexing/store/sqlite/graphstore.go & graphstore_test.go
   * pkg/indexing/store/bolt/graphstore.go & graphstore_test.go
   * pkg/indexing/store/neo4j/graphstore.go (新增 Neo4j 官方驱动依赖)
   * pkg/retriever/graph/README.md (更新了存储配置文档)


---

本次更新的关键特性：


1. 新增 CypherStep (pkg/retriever/graph/cypher_step.go):
    * 这是一个专门用于“逻辑跳跃”的检索步骤。它会利用你定义的 Cypher 模板（如：找 CEO、找竞争对手、找风险关联），在图数据库中自动运行并提取推理结果。
    * 提取到的结果会自动注入到 graph_context 中，以 [Deep Reasoning Insights] 的形式提供给 LLM，极大地增强了 AI 处理复杂逻辑问题的能力。
2. 灵活的流水线扩展:
    * 更新了 NewRetriever 的设计，现在支持 WithCustomStep 选项。
    * 这意味着你可以根据不同的业务场景，在检索器中注入无限多个 Cypher 模板步骤。例如，你可以同时运行一个“找合作伙伴”的模板和一个“找潜在风险”的模板。
3. 文档同步更新:
    * 在 pkg/retriever/graph/README.md 中补充了如何配置和使用 Cypher 模板的具体代码示例。

现在的架构优势：
你的 GoRAG 框架现在不仅能搜到“文档里写了什么”，还能通过图谱推理出“文档里没直说、但逻辑上存在的关系”。


目前我们已经具备了：
  * 本地嵌入式图存储 (SQLite/BoltDB)
  * 工业级图存储 (Neo4j)
  * Cypher 模板检索
  * 父子文档原文增强 (PDR)


---

核心可观测性升级总结：


1. 上下文贯通：在 IndexingContext 和 RetrievalContext 中原生集成了 Tracer 和 Span。这使得流水线中的每一个 Step 都能轻松开启子 Span。
2. GraphRAG 全程追踪：
    * Retrieve 方法会开启一个名为 GraphRAG.Retrieve 的根 Span。
    * EntityExtraction 步骤会开启子 Span，并记录提取到的实体数量和具体名称。
    * GraphSearch 步骤会记录每一跳找到的节点数和边数。
3. 零侵入性：默认使用 NoopTracer，对不使用追踪的用户完全透明且无性能损耗。
4. 标准化接口：用户可以通过注入符合 observability.Tracer 接口的实现（如接入 Jaeger, Honeycomb 或 OpenTelemetry），瞬间获得整个 RAG 系统的高级链路图。


目前项目的状态：
我们已经攻克了：
* 自动化图构建（indexing 侧）。
* 智能意图路由（retrieval 侧）。
* 深度可观测性（framework 侧）。


---


核心改进概览


1. Indexer 接口统一
    * 统一了 pkg/indexing/indexing.go 与 pkg/indexer/builder.go 中的 Indexer 接口定义。
    * IndexFile 现在返回 (*core.IndexingContext, error)，以便获取索引过程中的上下文和元数据。
    * 新增了 IndexDirectory(ctx, path, recursive) 方法。
2. 实现高效并发索引
    * 在 defaultIndexer 中实现了 IndexDirectory。
    * 引入了 Worker Pool 模式（基于 errgroup），支持多协程并发处理目录下的文件。
    * 默认并发数为 10（可配置），显著提升了大规模文档库的初始化索引速度。
3. 增强型工厂函数与配置选项 (Functional Options)
    * 实现了 DefaultIndexer 工厂函数，支持以下配置选项：
        * WithConcurrency(bool) / WithWorkers(int)：控制并发性能。
        * WithAllParsers()：一键加载 20+ 内置解析器。
        * WithWatchDir(dirs...)：配置监控目录。
        * WithStore, WithGraph, WithEmbedding 等：支持 DI 风格的组件注入。
4. 解析器兼容性补全 (Bug Fix)
    * 修复了多个流式解析器（tscode, xml, yaml, log, config）未完全实现 core.Parser 接口的问题。
    * 为这些解析器补全了 Parse(ctx, content, metadata) 和 Supports(contentType) 方法。
    * 更新了 ParserRegistry，确保其返回类型统一为 core.Parser，解决了之前的类型转换编译错误。
5. 文件监控 (Watcher) 适配
    * 更新了 pkg/indexing/watcher.go，使其完美适配新的 Indexer 接口。
    * 在 defaultIndexer 中增加了 Start() 方法，支持阻塞式启动目录监控。


验证情况
* 单元测试：新建了 pkg/indexer/builder_test.go，验证了 Init、WithAllParsers 以及自动初始化逻辑，全部通过。
* 编译检查：解决了由于接口不一致导致的 pkg/indexer 编译失败问题。
* 代码规范：所有修改均符合 GEMINI.md 定义的模块化和高性能要求。