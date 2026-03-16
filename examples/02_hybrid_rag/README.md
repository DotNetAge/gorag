# Hybrid RAG 示例 - 混合检索策略

## 概述

Hybrid RAG 结合了**稠密向量检索**和**稀疏检索（BM25）**的优势，通过 RAG-Fusion 融合两种结果，提供更准确的召回。

## Pipeline 组成

```
Query 
  ↓
[QueryToFilterStep]      ← 提取元数据过滤器
  ↓
[StepBackStep]           ← 生成抽象查询（可选）
  ↓
[HyDEStep]               ← 生成假设文档（可选）
  ↓
VectorSearchStep         ← 稠密检索路径
  ↓
ChunksToParallelResults  ← 暂存稠密结果
  ↓
SparseSearchStep         ← 稀疏检索路径（BM25）
  ↓
RAGFusionStep            ← RRF 融合两种结果
  ↓
RerankStep               ← 交叉编码器重排序（可选）
  ↓
GenerationStep           ← LLM 生成答案
```

### 涉及的 Steps

| Step | 包路径 | 作用 | 是否必需 |
|------|--------|------|---------|
| QueryToFilterStep | `infra/steps/pre_retrieval` | 从查询提取过滤条件 | 可选 |
| StepBackStep | `infra/steps/pre_retrieval` | 生成抽象查询 | 可选 |
| HyDEStep | `infra/steps/pre_retrieval` | 生成假设性文档 | 可选 |
| VectorSearchStep | `infra/steps/retrieval` | 稠密向量检索 | **必需** |
| SparseSearchStep | `infra/steps/retrieval` | BM25 关键词检索 | 可选（推荐） |
| RAGFusionStep | `infra/steps/retrieval` | RRF 融合算法 | **必需** |
| RerankStep | `infra/steps/post_retrieval` | 交叉编码器重排序 | 可选 |
| GenerationStep | `infra/steps/post_retrieval` | LLM 生成答案 | **必需** |

## 使用方式

```go
package main

import (
	"github.com/DotNetAge/gorag/infra/searcher/hybrid"
	"github.com/DotNetAge/gorag/infra/enhancer"
)

func main() {
	// 创建 Hybrid RAG Searcher
	searcher := hybrid.New(
		// 必需配置
		hybrid.WithEmbedding(embedder),
		hybrid.WithVectorStore(store),
		hybrid.WithGenerator(generator),
		
		// 可选增强（强烈推荐）
		hybrid.WithSparseStore(sparseStore),     // BM25 检索
		hybrid.WithFilterExtractor(extractor),   // 过滤器提取
		hybrid.WithStepBack(stepBackGen),        // StepBack 扩展
		hybrid.WithHyDE(hydeGen),                // HyDE 扩展
		
		// 参数配置
		hybrid.WithDenseTopK(20),    // 稠密检索数量
		hybrid.WithSparseTopK(20),   // 稀疏检索数量
		hybrid.WithFusionTopK(10),   // 融合后输出数量
		hybrid.WithRerankTopK(5),    // 重排序后保留数量
	)
	
	answer, _ := searcher.Search(ctx, "在 2023 年发布的 Go 语言新特性")
}
```

## 核心 Step 详解

### 1. QueryToFilterStep

**作用**: 从查询中提取结构化过滤条件

**示例**:
```
输入："查找作者为 Alice 的关于机器学习的文档"
输出：
  - 查询文本："机器学习的文档"
  - 过滤器：{author: "Alice"}
```

**代码**:
```go
extractor := enhancer.NewFilterExtractor(llm)
step := prestep.NewQueryToFilterStep(extractor, logger)
```

### 2. StepBackStep

**作用**: 生成更抽象、上位的查询，扩大检索范围

**示例**:
```
原始查询："Transformer 模型中的多头注意力机制"
StepBack 查询："深度学习中的注意力机制原理"
```

**代码**:
```go
stepBackGen := enhancer.NewStepBackGenerator(llm)
step := prestep.NewStepBackStep(stepBackGen, logger)
```

### 3. HyDEStep (Hypothetical Document Embeddings)

**作用**: 生成假设性答案文档，用其向量进行检索

**工作流程**:
1. LLM 生成假设性答案（可能不准确）
2. 将假设答案转为向量
3. 搜索相似的真实文档

**优势**: 解决查询与文档的语义鸿沟问题

**代码**:
```go
hydeGen := enhancer.NewHyDEGenerator(llm)
step := prestep.NewHyDEStep(hydeGen, logger)
```

### 4. SparseSearchStep

**作用**: 执行 BM25 关键词检索

**与向量检索对比**:
- **稠密检索**: 理解语义，但可能漏掉关键词匹配
- **稀疏检索**: 精确匹配关键词，但不理解语义
- **混合检索**: 两者互补，召回率更高

**代码**:
```go
step := retrievalstep.NewSparseSearchStep(
	sparseStore,  // BM25 索引
	20,           // topK
	logger,
)
```

### 5. RAGFusionStep

**作用**: 使用 RRF (Reciprocal Rank Fusion) 融合多路检索结果

**RRF 公式**:
```
RRF Score = Σ 1 / (k + rank_i)
其中 k=60（默认），rank_i 是第 i 路结果的排名
```

**示例**:
```
稠密检索结果: [A(1), B(2), C(3)]
稀疏检索结果: [B(1), D(2), E(3)]

RRF 分数计算:
A: 1/(60+1) = 0.0164
B: 1/(60+2) + 1/(60+1) = 0.0328  ← B 在两路都出现，分数最高
C: 1/(60+3) = 0.0159
D: 1/(60+2) = 0.0161
E: 1/(60+3) = 0.0159

融合后排序: [B, A, D, C, E]
```

**代码**:
```go
fusionEngine := fusion.NewRRFFusionEngine()  // k=60
step := retrievalstep.NewRAGFusionStep(
	fusionEngine,
	10,  // 融合后输出 TopK
)
```

### 6. RerankStep

**作用**: 使用交叉编码器（Cross-Encoder）对融合结果精细排序

**工作原理**:
- 将查询和每个文档拼接输入 LLM
- LLM 直接预测相关性分数
- 按分数重新排序

**优势**: 比向量检索更准确，但计算成本高

**代码**:
```go
reranker := reranking.NewCrossEncoderReranker(model)
step := poststep.NewRerankStep(reranker, 5)  // 保留前 5 个
```

## State 数据流转

```
初始状态:
state.Query = "Go 语言并发模型"

↓ QueryToFilterStep
state.Query.Text = "并发模型"
state.Filters = {"language": "Go"}

↓ StepBackStep (可选)
state.Agentic.StepBackQuery = "编程语言并发机制"

↓ HyDEStep (可选)
state.Agentic.HypotheticalDocument = "Go 语言使用 goroutine..."

↓ VectorSearchStep
state.RetrievedChunks = [[dense_chunk1, ..., dense_chunk20]]

↓ ChunksToParallelResults
state.ParallelResults = [[dense_chunk1, ..., dense_chunk20]]
state.RetrievedChunks = []

↓ SparseSearchStep
state.RetrievedChunks = [[sparse_chunk1, ..., sparse_chunk20]]

↓ RAGFusionStep
state.RetrievedChunks = [[fused_chunk1, ..., fused_chunk10]]

↓ RerankStep (可选)
state.RetrievedChunks = [[reranked_chunk1, ..., reranked_chunk5]]

↓ GenerationStep
state.Answer = "Go 语言的并发模型基于 CSP 理论..."
```

## 性能优化建议

### 1. TopK 配置策略

```go
// 推荐配置比例
hybrid.New(
	hybrid.WithDenseTopK(20),    // 稠密：召回为主
	hybrid.WithSparseTopK(20),   // 稀疏：召回为主
	hybrid.WithFusionTopK(10),   // 融合：筛选精华
	hybrid.WithRerankTopK(5),    // 重排序：最优结果
)
```

### 2. 何时启用增强 Steps

| 场景 | QueryToFilter | StepBack | HyDE | Rerank |
|------|--------------|----------|------|--------|
| 简单事实查询 | ❌ | ❌ | ❌ | ❌ |
| 复杂分析查询 | ✅ | ✅ | ✅ | ✅ |
| 带条件查询 | ✅ | ❌ | ❌ | ❌ |
| 概念解释查询 | ❌ | ✅ | ✅ | ❌ |

### 3. RRF 参数调优

```go
// 自定义 RRF 参数
fusionEngine := fusion.NewRRFFusionEngineWithK(40)  // 默认 k=60
// k 越小，越偏向高排名结果
// k 越大，各结果分数越接近
```

## 典型应用场景

✅ **适用场景**:
- 大规模知识库（万级文档以上）
- 对召回率要求高的场景
- 查询表达多样化
- 需要精确关键词匹配

❌ **不适用场景**:
- 文档量小（千级以下）
- 实时性要求极高（HyDE/StepBack 增加延迟）
- 资源受限环境（多个 LLM 调用）

## 与其他模式对比

| 模式 | 检索策略 | 准确率 | 召回率 | 延迟 |
|------|---------|-------|-------|------|
| **Native RAG** | 单路稠密 | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Hybrid RAG** | 稠密 + 稀疏 | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Agentic RAG** | 多轮迭代 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |

## 下一步

- **[03_agentic_rag](../03_agentic_rag/)** - 学习 Agent 自主决策
- **[06_stepback_hyde](../06_stepback_hyde/)** - 深入理解查询增强技术

## 参考资源

- [RAG-Fusion 论文](https://arxiv.org/abs/2305.16508)
- [HyDE 论文](https://arxiv.org/abs/2212.10496)
- [RRF 算法原文](https://www.microsoft.com/en-us/research/publication/the-probabilistic-relevance-framework-bm25-and-beyond/)
