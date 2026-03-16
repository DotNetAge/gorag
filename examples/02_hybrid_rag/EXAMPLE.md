# Steps 组合使用示例 - Hybrid RAG

本示例展示如何组合多个 Steps 实现混合检索。

## Pipeline 流程图

```
Query
  ↓
[QueryToFilterStep] ──→ 提取元数据过滤器
  ↓
[StepBackStep] ────────→ 生成抽象查询
  ↓
[HyDEStep] ───────────→ 生成假设文档
  ↓
┌─────────────────────────────┐
│ VectorSearchStep (并行)     │
│ SparseSearchStep (并行)     │
└─────────────────────────────┘
  ↓
[RAGFusionStep] ─────────→ 融合结果 (RRF)
  ↓
[RerankStep] ────────────→ 重排序
  ↓
[GenerationStep] ────────→ 生成答案
```

## 代码示例

### 1. 基础版本（仅核心 Steps）

```go
package main

import (
    "context"
    "github.com/DotNetAge/gorag/infra/searcher/hybrid"
    retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
    poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
)

func main() {
    // 初始化 searcher
    searcher := hybrid.New(
        hybrid.WithEmbedding(embedder),
        hybrid.WithVectorStore(vectorStore),
        hybrid.WithGenerator(llm),
        hybrid.WithDenseTopK(10),
    )
    
    // 执行查询
    answer, _ := searcher.Search(ctx, query)
}
```

### 2. 进阶版本（完整 Steps 组合）

```go
package main

import (
    "context"
    "github.com/DotNetAge/gorag/infra/searcher/hybrid"
    "github.com/DotNetAge/gorag/infra/enhancer"
    prestep "github.com/DotNetAge/gorag/infra/steps/pre_retrieval"
    retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
    poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
    "github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

func buildAdvancedHybridRAG() *hybrid.Searcher {
    // 1. 初始化增强组件
    filterExtractor := enhancer.NewFilterExtractor(llm)
    stepBackGen := enhancer.NewStepBackGenerator(llm)
    hydeGen := enhancer.NewHyDEGenerator(llm)
    
    // 2. 初始化工具组件
    fusionEngine := retrieval.NewRRFEngine(60)
    
    // 3. 构建 searcher
    searcher := hybrid.New(
        hybrid.WithEmbedding(embedder),
        hybrid.WithVectorStore(vectorStore),
        hybrid.WithSparseStore(bm25Store),
        hybrid.WithFusionEngine(fusionEngine),
        hybrid.WithGenerator(llm),
        hybrid.WithReranker(reranker),
        
        // 启用高级功能
        hybrid.WithFilterExtractor(filterExtractor),
        hybrid.WithStepBackGen(stepBackGen),
        hybrid.WithHyDEGen(hydeGen),
        
        // 配置参数
        hybrid.WithDenseTopK(10),
        hybrid.WithSparseTopK(10),
        hybrid.WithFusionTopK(20),
        hybrid.WithRerankTopK(5),
    )
    
    return searcher
}
```

### 3. 手动 Pipeline 组装（学习原理）

```go
func buildManualPipeline() *pipeline.Pipeline[*entity.PipelineState] {
    p := pipeline.New[*entity.PipelineState]()
    
    // === Pre-Retrieval 阶段 ===
    
    // Step 1: 提取过滤器
    p.AddStep(prestep.NewQueryToFilterStep(filterExtractor, logger))
    
    // Step 2: StepBack 抽象化
    p.AddStep(prestep.NewStepBackStep(stepBackGen, logger))
    
    // Step 3: HyDE 假设文档
    p.AddStep(prestep.NewHyDEStep(hydeGen, logger))
    
    // === Retrieval 阶段 ===
    
    // Step 4: 稠密向量检索
    p.AddStep(retrievalstep.NewVectorSearchStep(embedder, vectorStore, 10))
    
    // Step 5: 稀疏检索（BM25）
    p.AddStep(retrievalstep.NewSparseSearchStep(bm25Store, 10, logger))
    
    // Step 6: 转并行结果
    p.AddStep(chunksToParallelResultsStep{})
    
    // Step 7: RAG Fusion 融合
    p.AddStep(retrievalstep.NewRAGFusionStep(fusionEngine, 20))
    
    // === Post-Retrieval 阶段 ===
    
    // Step 8: Cross-Encoder 重排序
    p.AddStep(poststep.NewRerankStep(reranker, 5))
    
    // Step 9: LLM 生成答案
    p.AddStep(poststep.NewGenerator(llm, logger))
    
    return p
}
```

## 关键 Steps 说明

### Pre-Retrieval Steps

| Step | 作用 | 何时使用 |
|------|------|---------|
| `QueryToFilterStep` | 从查询中提取过滤条件 | 数据有元数据（时间、类型等） |
| `StepBackStep` | 生成更抽象的查询 | 需要因果推理或宏观视角 |
| `HyDEStep` | 生成假设性文档 | 查询简短、需要语义扩展 |
| `QueryRewriteStep` | 改写查询 | 查询模糊或有歧义 |

### Retrieval Steps

| Step | 作用 | 特点 |
|------|------|------|
| `VectorSearchStep` | 稠密向量检索 | 语义匹配 |
| `SparseSearchStep` | 稀疏检索（BM25） | 关键词匹配 |
| `GraphLocalSearchStep` | 图谱局部搜索 | 实体关系推理 |
| `GraphGlobalSearchStep` | 图谱全局搜索 | 社区级摘要 |

### Post-Retrieval Steps

| Step | 作用 | 效果 |
|------|------|------|
| `RAGFusionStep` | 多路结果融合 | RRF 算法，提升召回质量 |
| `RerankStep` | 交叉编码重排序 | 精排，提升 Top 结果准确性 |
| `GenerationStep` | 基于上下文生成 | 合成最终答案 |

## 性能优化建议

### 1. 并发检索

```go
// 在 VectorSearch 和 SparseSearch 后添加
p.AddStep(chunksToParallelResultsStep{})
p.AddStep(retrievalstep.NewRAGFusionStep(fusionEngine, 20))
```

### 2. 动态 TopK

```go
// 根据查询长度动态调整
func calculateTopK(query string) int {
    if len(query) < 10 {
        return 15 // 短查询扩大检索范围
    }
    return 10 // 长查询保持精确
}
```

### 3. 缓存热点查询

```go
if cacheService != nil {
    p.AddStep(prestep.NewSemanticCacheChecker(cacheService, logger))
    // ... 检索步骤 ...
    p.AddStep(prestep.NewCacheResponseWriter(cacheService, logger))
}
```

## 实际运行示例

```bash
# 1. 准备数据
cd examples/02_hybrid_rag
go run ingest.go  # 导入文档到向量库

# 2. 运行查询
go run main.go
```

## 输出示例

```
=== 查询：GoRAG 支持哪些高级检索策略？ ===

检索到的 chunks:
- Chunk 1: GoRAG 支持 HyDE、RAG-Fusion、StepBack 等策略...
- Chunk 2: 混合检索结合了稠密和稀疏检索的优势...
- Chunk 3: 图谱检索利用实体关系进行推理...

生成的答案:
GoRAG 支持以下高级检索策略：
1. HyDE（Hypothetical Document Embedding）
2. RAG-Fusion（多路召回融合）
3. StepBack（抽象推理）
4. 混合检索（稠密 + 稀疏）
5. 图谱检索（局部/全局）
...
```

## 下一步

- 查看 [`03_agentic_rag`](../03_agentic_rag/) - Agent 自主决策检索
- 查看 [`04_graph_rag`](../04_graph_rag/) - 知识图谱增强
- 阅读 [API 文档](https://pkg.go.dev/github.com/DotNetAge/gorag)
