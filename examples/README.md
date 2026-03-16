# GoRAG Examples - Steps 组合使用示例

本目录包含一系列完整的示例，展示如何组合使用 GoRAG 的 Steps 构建强大的 RAG 应用。

## 📚 示例列表

### 基础示例

| 示例 | 描述 | Steps 组合 | 难度 |
|------|------|-----------|------|
| [**01_native_rag**](./01_native_rag/) | 标准 Native RAG 流程 | QueryRewrite → VectorSearch → Generation | ⭐ |
| [**02_hybrid_rag**](./02_hybrid_rag/) | 混合检索（稠密 + 稀疏） | QueryToFilter + StepBack + HyDE → VectorSearch + SparseSearch → RAGFusion → Rerank → Generation | ⭐⭐⭐ |
| [**03_agentic_rag**](./03_agentic_rag/) | Agent 自主决策 RAG | Reasoning ↔ ActionSelection ↔ TerminationCheck ↔ ParallelRetriever ↔ Observation | ⭐⭐⭐⭐ |

### 高级示例

| 示例 | 描述 | Steps 组合 | 难度 |
|------|------|-----------|------|
| [**04_graph_rag**](./04_graph_rag/) | 图谱增强 RAG | EntityExtract → GraphLocal/GlobalSearch → (Fusion) → Generation | ⭐⭐⭐ |
| [**05_multiagent_rag**](./05_multiagent_rag/) | 多 Agent 协作 RAG | Multiple Agents + Coordinator + Aggregator | ⭐⭐⭐⭐⭐ |
| [**06_stepback_hyde**](./06_stepback_hyde/) | StepBack + HyDE 组合 | StepBack + HyDE → VectorSearch → Fusion | ⭐⭐ |

### 实战场景

| 示例 | 描述 | 适用场景 |
|------|------|---------|
| [**07_document_qa**](./07_document_qa/) | 文档问答系统 | 企业知识库、智能客服 |
| [**08_code_search**](./08_code_search/) | 代码语义搜索 | IDE 插件、代码审查 |
| [**09_knowledge_base**](./09_knowledge_base/) | 知识库构建与查询 | 文档管理系统 |

## 🚀 快速开始

### 前置准备

```bash
# 1. 克隆项目
git clone https://github.com/DotNetAge/gorag.git
cd gorag

# 2. 安装依赖
go mod download

# 3. 配置环境变量（可选）
export OPENAI_API_KEY="your-api-key"
```

### 运行示例

```bash
# 基础 Native RAG 示例（推荐从这里开始）
cd examples/01_native_rag
go run main.go

# 混合检索示例
cd examples/02_hybrid_rag
go run main.go

# Agent RAG 示例
cd examples/03_agentic_rag
go run main.go
```

## 📖 学习路径

### 初学者路线
```
01_native_rag → 02_hybrid_rag → INTEGRATION_EXAMPLES.md
```

1. **第一步**: 理解 Native RAG 的基本流程
2. **第二步**: 学习 Hybrid RAG 的多路检索策略
3. **第三步**: 阅读集成指南，掌握 Steps 组合技巧

### 进阶路线
```
02_hybrid_rag → 03_agentic_rag → 04_graph_rag → 05_multiagent_rag
```

1. **第一步**: 掌握高级检索技术（HyDE, StepBack, RRF）
2. **第二步**: 学习 Agent 自主决策机制
3. **第三步**: 探索图谱和多 Agent 协作

### 实战路线
```
01_native_rag → 07_document_qa → 08_code_search → 09_knowledge_base
```

1. **第一步**: 快速搭建原型
2. **第二步**: 应用到具体业务场景
3. **第三步**: 优化和扩展功能

## 🎯 Steps 速查表

### Pre-Retrieval Steps（预检索）

| Step | 包路径 | 作用 |
|------|--------|------|
| `QueryRewriteStep` | `pre_retrieval` | 改写查询，提升清晰度 |
| `QueryToFilterStep` | `pre_retrieval` | 提取元数据过滤器 |
| `StepBackStep` | `pre_retrieval` | 生成抽象查询（因果推理） |
| `HyDEStep` | `pre_retrieval` | 生成假设性文档 |
| `SemanticCacheChecker` | `pre_retrieval` | 语义缓存检查 |

### Retrieval Steps（检索）

| Step | 包路径 | 作用 |
|------|--------|------|
| `VectorSearchStep` | `retrieval` | 稠密向量检索 |
| `SparseSearchStep` | `retrieval` | 稀疏检索（BM25） |
| `RAGFusionStep` | `retrieval` | 多路结果融合（RRF） |
| `GraphLocalSearchStep` | `retrieval` | 图谱局部搜索 |
| `GraphGlobalSearchStep` | `retrieval` | 图谱全局搜索 |
| `EntityExtractor` | `steps` | 实体抽取 |

### Post-Retrieval Steps（后检索）

| Step | 包路径 | 作用 |
|------|--------|------|
| `RerankStep` | `post_retrieval` | 交叉编码重排序 |
| `GenerationStep` | `post_retrieval` | 基于上下文生成答案 |
| `ContextPruningStep` | `post_retrieval` | 上下文剪枝 |

### Agentic Steps（Agent）

| Step | 包路径 | 作用 |
|------|--------|------|
| `ReasoningStep` | `agentic` | 推理思考 |
| `ActionSelectionStep` | `agentic` | 行动选择 |
| `TerminationCheckStep` | `agentic` | 终止检查 |
| `ParallelRetriever` | `agentic` | 并行检索 |
| `ObservationStep` | `agentic` | 观察分析 |

## 🔧 常见 Patterns

### Pattern 1: 简单问答
```
Query → VectorSearch → Generation
```
适用于：快速原型、简单场景

### Pattern 2: 高精度检索
```
Query → QueryRewrite → [VectorSearch + SparseSearch] 
     → RAGFusion → Rerank → Generation
```
适用于：企业知识库、专业领域

### Pattern 3: 复杂推理
```
Query → [Reasoning ↔ ActionSelection ↔ Retrieval] × N 
     → Rerank → Generation
```
适用于：研究助手、故障诊断

### Pattern 4: 图谱增强
```
Query → EntityExtract → GraphSearch 
     → [VectorSearch + Fusion] → Generation
```
适用于：知识图谱查询、关系推理

## 💡 最佳实践

### 1. 选择合适的模式

| 需求 | 推荐模式 | 理由 |
|------|---------|------|
| 快速原型 | Native RAG | 简单、快速 |
| 高精度 | Hybrid RAG | 多路召回 + 重排序 |
| 复杂推理 | Agentic RAG | 多轮决策 |
| 关系查询 | Graph RAG | 利用实体关系 |

### 2. 参数调优建议

```go
// TopK 选择
- 精确问答：TopK = 3-5
- 综合分析：TopK = 10-20
- 探索性查询：TopK = 20-50

// 迭代次数（Agentic）
- 简单问题：MaxIterations = 2-3
- 复杂问题：MaxIterations = 5-7
```

### 3. 性能优化

```go
// 启用并发
p.AddStep(chunksToParallelResultsStep{})

// 添加缓存
if cacheService != nil {
    p.AddStep(prestep.NewSemanticCacheChecker(cacheService, logger))
}

// 动态调整 TopK
searcher.SetTopK(calculateDynamicTopK(query))
```

## 📊 示例代码结构

每个示例目录包含：

```
01_native_rag/
├── README.md        # 详细说明
├── main.go          # 主程序
└── EXAMPLE.md       # 分步教程（如有）
```

## 🤝 贡献指南

欢迎提交更多示例！请遵循以下格式：

1. 在对应目录下创建 `main.go`
2. 添加详细的 `README.md` 说明
3. 包含可运行的代码和示例输出
4. 提交 Pull Request

## 📚 相关资源

- **完整文档**: [GoRAG 文档](../README.md)
- **API 参考**: [pkg.go.dev](https://pkg.go.dev/github.com/DotNetAge/gorag)
- **集成指南**: [INTEGRATION_EXAMPLES.md](./INTEGRATION_EXAMPLES.md)
- **问题反馈**: [GitHub Issues](https://github.com/DotNetAge/gorag/issues)

## 🎓 学习资源

### 入门教程
- [Native RAG 详解](./01_native_rag/EXAMPLE.md)
- [Hybrid RAG 实战](./02_hybrid_rag/EXAMPLE.md)
- [Agentic RAG 指南](./03_agentic_rag/EXAMPLE.md)

### 进阶专题
- [Steps 组合艺术](./INTEGRATION_EXAMPLES.md)
- [性能优化技巧](./INTEGRATION_EXAMPLES.md#最佳实践)
- [常见问题解答](../docs/FAQ.md)

### 论文参考
- [HyDE: Hypothetical Document Embedding](https://arxiv.org/abs/xxxx.xxxxx)
- [RAG-Fusion: Multi-query Fusion](https://arxiv.org/abs/xxxx.xxxxx)
- [Agentic RAG: Autonomous Retrieval](https://arxiv.org/abs/xxxx.xxxxx)

---

**开始你的 RAG 之旅吧！** 🚀

从 [`01_native_rag`](./01_native_rag/) 开始，逐步掌握 GoRAG 的强大功能！
