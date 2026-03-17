# goRAG 整洁架构设计

## 目录结构

```
gorag/
├── pkg/                         # 公共包 (外部可导入)
│   ├── domain/                  # 领域模型 (Entities)
│   │   ├── abstraction/         # 抽象接口
│   │   │   ├── metrics.go       # Metrics 接口
│   │   │   ├── vectorstore.go   # VectorStore 接口
│   │   │   └── graphstore.go    # GraphStore 接口
│   │   └── entity/              # 核心实体
│   │       ├── document.go      # Document 实体
│   │       ├── chunk.go         # Chunk 实体
│   │       ├── query.go         # Query 实体
│   │       └── pipeline_state.go # PipelineState 实体
│   ├── usecase/                 # 业务用例 (Use Cases)
│   │   ├── dataprep/            # 数据准备用例
│   │   │   ├── indexer.go       # Indexer 接口
│   │   │   ├── parser.go        # Parser 接口
│   │   │   └── chunker.go       # Chunker 接口
│   │   ├── retrieval/           # 检索用例
│   │   │   ├── searcher.go      # Searcher 接口
│   │   │   └── retriever.go     # Retriever 接口
│   │   └── evaluation/          # 评估用例
│   │       └── evaluator.go     # Evaluator 接口
│   ├── di/                      # 依赖注入容器
│   │   └── container.go         # DI Container 实现
│   └── utils/                   # 工具函数
├── infra/                       # 框架与驱动 (Frameworks & Drivers)
│   ├── indexer/                 # 索引器实现
│   │   ├── default_indexer.go   # DefaultIndexer 核心
│   │   └── watcher.go           # 文件监控器
│   ├── indexing/                # 索引管线状态
│   │   └── state.go             # Pipeline State 定义
│   ├── steps/                   # Pipeline Steps（三阶段）
│   │   ├── pre_retrieval/       # 预检索步骤
│   │   │   ├── query_rewrite_step.go
│   │   │   ├── hyde_step.go
│   │   │   └── step_back_step.go
│   │   ├── retrieval/           # 检索步骤
│   │   │   ├── vector_search_step.go
│   │   │   ├── graph_local_search_step.go
│   │   │   └── hybrid_search_step.go
│   │   └── post_retrieval/      # 后检索步骤
│   │       ├── rerank_step.go
│   │       └── generation_step.go
│   ├── parser/                  # 解析器实现（22 种格式）
│   │   ├── pdf/
│   │   ├── markdown/
│   │   ├── json/
│   │   └── ... (其他解析器)
│   ├── vectorstore/             # 向量存储实现
│   │   ├── govector/            # 本地向量存储
│   │   ├── milvus/              # Milvus 实现
│   │   ├── qdrant/              # Qdrant 实现
│   │   └── factory.go           # VectorStore 工厂
│   ├── graphstore/              # 图存储实现
│   │   ├── neo4j/               # Neo4j 实现
│   │   └── factory.go           # GraphStore 工厂
│   ├── chunker/semantic/        # 语义分块器
│   │   └── factory.go           # SemanticChunker 工厂
│   ├── enhancer/                # 结果增强器
│   │   ├── cross_encoder_reranker.go
│   │   └── context_pruner.go
│   ├── generation/              # 生成器
│   │   └── llm_generator.go
│   ├── fusion/                  # 融合器
│   │   └── rrf_fusion.go
│   ├── evaluation/              # 评估器
│   │   └── rag_evaluator.go
│   ├── cache/                   # 缓存层
│   │   └── semantic_cache.go
│   ├── tools/                   # 工具集
│   │   └── calculator.go
│   └── service/                 # 服务层
│       └── rag_service.go
├── examples/                    # 示例代码
│   └── quickstart/
│       └── main.go              # QuickStart 示例
├── cmd/                         # 命令行工具
├── configs/                     # 配置
├── specs/                       # 设计文档
│   ├── 00-RAG 概述.md
│   ├── 01-RAG 的基本概念.md
│   ├── ...
│   └── 22-GoRAG高级开发指南.md
├── .docs/                       # 内部文档
├── go.mod                       # Go 模块文件
└── go.sum                       # Go 依赖校验文件
```

## 架构分层

### 1. Domain Layer (领域层) - `pkg/domain/`

**职责**: 定义核心业务接口和实体，不依赖任何外部实现

#### 抽象接口 (`abstraction/`)
- `Metrics`: 指标收集接口
- `VectorStore`: 向量存储接口
- `GraphStore`: 图存储接口

#### 核心实体 (`entity/`)
- `Document`: 文档实体
- `Chunk`: 文档分块
- `Query`: 查询实体
- `PipelineState`: 管线共享状态

---

### 2. Use Case Layer (用例层) - `pkg/usecase/`

**职责**: 定义业务用例接口，描述"做什么"而非"怎么做"

#### 数据准备用例 (`dataprep/`)
- `Indexer`: 索引器接口
  - `Index() error` - 增量索引
  - `IndexAll() error` - 全量索引
  - `IndexDirectory(ctx, dir, recursive) error` - 目录索引
  - `IndexFile(ctx, file) error` - 单文件索引
- `Parser`: 解析器接口
- `SemanticChunker`: 分块器接口

#### 检索用例 (`retrieval/`)
- `Searcher`: 搜索器接口
- `Retriever`: 检索器接口

#### 评估用例 (`evaluation/`)
- `Evaluator`: 评估器接口

---

### 3. Interface Adapters Layer (接口适配器层) - `pkg/di/`

**职责**: 依赖注入容器，连接领域层和基础设施层

#### 依赖注入容器
- `Container`: DI 容器实现
  - `RegisterInstance(interface{}, impl)` - 注册实例
  - `Resolve(interface{}) (interface{}, error)` - 解析依赖
  - `ResolveTyped[T any](c *Container) (T, error)` - 泛型解析

---

### 4. Infrastructure Layer (基础设施层) - `infra/`

**职责**: 具体实现，依赖领域层的接口

#### 索引器实现 (`indexer/`)
- `defaultIndexer`: 默认索引器实现
  - 基于 `gochat/pkg/pipeline` 的泛型 Pipeline
  - 支持文件监控和增量索引
  - 使用 DI 容器管理组件生命周期

#### Pipeline Steps (`steps/`)

**预检索阶段** (`pre_retrieval/`):
- `QueryRewriteStep`: 查询改写
- `HyDEStep`: 假设性文档生成
- `StepBackStep`: 抽象化提问

**检索阶段** (`retrieval/`):
- `VectorSearchStep`: 向量检索
- `GraphLocalSearchStep`: 图谱本地检索 (1-2 跳)
- `GraphGlobalSearchStep`: 图谱全局检索 (3+ 跳)
- `HybridSearchStep`: 混合检索
- `FusionStep`: RRF 融合

**后检索阶段** (`post_retrieval/`):
- `RerankStep`: 重排序
- `CrossEncoderRerankStep`: CrossEncoder 精排
- `GenerationStep`: 答案生成

#### 基础步骤 (`steps/` Root):
- `ParseStep`: 多解析器自动选择
- `ChunkStep`: 语义分块
- `EmbeddingStep`: 批量嵌入
- `StoreStep`: 向量存储写入
- `EntityExtractStep`: 实体抽取
- `MultimodalEmbeddingStep`: 多模态嵌入

#### 解析器实现 (`parser/`)
支持 22 种文件格式:
- 文本类：TXT, MD, LOG, CSV, JSON, XML, YAML, HTML, SQL
- 代码类：Go, Python, Java, TypeScript, JavaScript
- 文档类：PDF, DOCX, EXCEL, PPT
- 其他：EMAIL, IMAGE, DBSCHEMA

#### 向量存储实现 (`vectorstore/`)
- `govector`: 本地 SQLite 向量存储（零依赖）
- `milvus`: Milvus 分布式向量数据库
- `qdrant`: Qdrant Rust 实现
- `pinecone`: Pinecone 云服务
- `weaviate`: Weaviate 云原生向量库

#### 图存储实现 (`graphstore/`)
- `neo4j`: Neo4j 图数据库
- `memory`: 内存图存储（测试用）

#### 增强器实现 (`enhancer/`)
- `CrossEncoderReranker`: CrossEncoder 重排序
- `ContextPruner`: 上下文剪枝
- `ParentDocExpander`: 父文档扩展
- `SentenceWindowExpander`: 句子窗口扩展

#### 生成器实现 (`generation/`)
- `LLMGenerator`: LLM 答案生成

#### 融合器实现 (`fusion/`)
- `RRFFusion`: 倒数排名融合算法

#### 评估器实现 (`evaluation/`)
- `RAGEvaluator`: RAG 质量评估（Ragas 指标）

#### 缓存实现 (`cache/`)
- `SemanticCache`: 语义缓存

#### 工具实现 (`tools/`)
- `Calculator`: 数学计算工具
- `CodeInterpreter`: 代码解释器

---

## 数据流示例

### 索引流程

```
文件 → FileDiscoveryStep → ParseStep → ChunkStep → EmbeddingStep → StoreStep → VectorStore
                                    ↓
                              EntityExtractStep → GraphStore
```

### 检索流程

```
查询 → QueryRewriteStep → HyDEStep → VectorSearchStep
                                     ↓
                              GraphLocalSearchStep
                                     ↓
                              FusionStep (RRF) → RerankStep → GenerationStep → 答案
```

## 设计原则

### 依赖倒置
- 领域层定义接口（如 `VectorStore`）
- 基础设施层实现接口（如 `govector`, `milvus`）
- 高层模块不依赖低层模块，都依赖抽象

### 单一职责
- 每个 Step 只负责一个职责
- Indexer 只负责索引编排，不负责具体处理逻辑
- 使用 DI 容器解耦组件依赖

### 开箱即用
- `DefaultIndexer()` 提供默认配置
- `DefaultSearcher()` 提供默认配置
- 支持通过 Option Pattern 自定义行为

### 渐进式抽象
- **Level 1**: Indexer + Searcher（5 行代码，生产首选）
- **Level 2**: Pipeline + Steps（20 行代码，灵活编排）
- **Level 3**: 自定义 Step（50 行代码，领域特定）
- **Level 4**: 核心包调用（100+ 行代码，极致性能）
