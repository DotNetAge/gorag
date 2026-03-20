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
