# GoRAG 集成示例 - Steps 组合使用指南

本指南展示如何将 GoRAG 的 Steps 组合成完整的 RAG 应用。

## 目录

1. [基础示例](#基础示例)
2. [高级示例](#高级示例)
3. [实战场景](#实战场景)
4. [最佳实践](#最佳实践)

---

## 基础示例

### 1. Native RAG - 最简单的检索增强生成

**场景**: 快速构建一个标准的 RAG 系统

**Steps 组合**:
```
QueryRewriteStep → VectorSearchStep → GenerationStep
```

**代码示例**:
```go
package main

import (
    "context"
    "github.com/DotNetAge/gorag/infra/searcher/native"
    prestep "github.com/DotNetAge/gorag/infra/steps/pre_retrieval"
    retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
    poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
    "github.com/DotNetAge/gochat/pkg/pipeline"
)

func buildNativeRAG() *native.Searcher {
    searcher := native.New(
        native.WithEmbedding(embedder),
        native.WithVectorStore(vectorStore),
        native.WithGenerator(llm),
        native.WithTopK(5),
    )
    
    // 手动构建 Pipeline（可选）
    p := pipeline.New[*entity.PipelineState]()
    p.AddStep(prestep.NewQueryRewriteStep(queryRewriter))
    p.AddStep(retrievalstep.NewVectorSearchStep(embedder, vectorStore, 5))
    p.AddStep(poststep.NewGenerator(llm, logger))
    
    return searcher
}
```

**适用场景**:
- 快速原型开发
- 简单的问答系统
- 文档检索

---

### 2. Hybrid RAG - 混合检索提升准确率

**场景**: 需要高精度的检索结果

**Steps 组合**:
```
QueryToFilter → StepBack → HyDE → 
[VectorSearch + SparseSearch] → 
RAGFusion → Rerank → Generation
```

**代码示例**:
```go
package main

import (
    "github.com/DotNetAge/gorag/infra/searcher/hybrid"
    prestep "github.com/DotNetAge/gorag/infra/steps/pre_retrieval"
    retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
    poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
)

func buildHybridRAG() *hybrid.Searcher {
    searcher := hybrid.New(
        hybrid.WithEmbedding(embedder),
        hybrid.WithVectorStore(vectorStore),
        hybrid.WithSparseStore(bm25Store),
        hybrid.WithFusionEngine(rrfEngine),
        hybrid.WithGenerator(llm),
        hybrid.WithReranker(crossEncoder),
        hybrid.WithFilterExtractor(filterExtractor),
        hybrid.WithStepBackGen(stepBackGen),
        hybrid.WithHyDEGen(hydeGen),
    )
    
    p := pipeline.New[*entity.PipelineState]()
    
    // 预检索优化
    p.AddStep(prestep.NewQueryToFilterStep(filterExtractor, logger))
    p.AddStep(prestep.NewStepBackStep(stepBackGen, logger))
    p.AddStep(prestep.NewHyDEStep(hydeGen, logger))
    
    // 并行检索
    p.AddStep(retrievalstep.NewVectorSearchStep(embedder, vectorStore, 10))
    p.AddStep(retrievalstep.NewSparseSearchStep(bm25Store, 10, logger))
    p.AddStep(chunksToParallelResultsStep{})
    
    // 融合与优化
    p.AddStep(retrievalstep.NewRAGFusionStep(fusionEngine, 20))
    p.AddStep(poststep.NewRerankStep(reranker, 5))
    p.AddStep(poststep.NewGenerator(llm, logger))
    
    return searcher
}
```

**适用场景**:
- 企业知识库搜索
- 法律/医疗等专业领域
- 需要高精度排序的场景

---

## 高级示例

### 3. Agentic RAG - Agent 自主决策检索

**场景**: 复杂问题需要多轮检索和推理

**Steps 组合**:
```
Loop:
  ReasoningStep → ActionSelectionStep → TerminationCheck
  → ParallelRetriever → ObservationStep
Final:
  Rerank → Generation
```

**代码示例**:
```go
package main

import (
    "github.com/DotNetAge/gorag/infra/searcher/agentic"
    agenticstep "github.com/DotNetAge/gorag/infra/steps/agentic"
    retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
    poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
)

func buildAgenticRAG() *agentic.Searcher {
    searcher := agentic.New(
        agentic.WithReasoner(reasoner),
        agentic.WithActionSelector(selector),
        agentic.WithRetriever(retriever),
        agentic.WithGenerator(llm),
        agentic.WithMaxIterations(5),
    )
    
    // 循环体
    loop := pipeline.New[*entity.PipelineState]()
    loop.AddStep(agenticstep.NewReasoningStep(reasoner, logger))
    loop.AddStep(agenticstep.NewActionSelectionStep(selector, 5, logger))
    loop.AddStep(agenticstep.NewTerminationCheckStep(logger))
    
    if retriever != nil {
        loop.AddStep(agenticstep.NewParallelRetriever(retriever, 5, logger))
    } else {
        loop.AddStep(retrievalstep.NewVectorSearchStep(embedder, vectorStore, 5))
    }
    
    loop.AddStep(agenticstep.NewObservationStep(logger))
    
    // 最终处理
    final := pipeline.New[*entity.PipelineState]()
    final.AddStep(poststep.NewRerankStep(reranker, 5))
    final.AddStep(poststep.NewGenerator(llm, logger))
    
    return searcher
}
```

**适用场景**:
- 复杂问答系统
- 研究助手
- 数据分析代理

---

### 4. Graph RAG - 知识图谱增强

**场景**: 需要利用实体关系进行推理

**Steps 组合**:
```
QueryRewrite → EntityExtract → 
GraphLocalSearch / GraphGlobalSearch → 
Generation
```

**代码示例**:
```go
package main

import (
    "github.com/DotNetAge/gorag/infra/searcher/graphlocal"
    "github.com/DotNetAge/gorag/infra/steps"
    retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
    poststep "github.com/DotNetAge/gorag/infra/steps/post_retrieval"
)

func buildGraphRAG() *graphlocal.Searcher {
    searcher := graphlocal.New(
        graphlocal.WithEntityExtractor(extractor),
        graphlocal.WithGraphLocalSearcher(localSearcher),
        graphlocal.WithGenerator(llm),
        graphlocal.WithMaxHops(2),
    )
    
    p := pipeline.New[*entity.PipelineState]()
    
    // 查询优化
    p.AddStep(prestep.NewQueryRewriteStep(rewriter))
    
    // 图谱检索
    p.AddStep(steps.NewEntityExtractor(extractor, logger))
    p.AddStep(retrievalstep.NewGraphLocalSearchStep(localSearcher, 2, 10))
    
    // 可选：与传统检索融合
    if vectorStore != nil {
        p.AddStep(retrievalstep.NewVectorSearchStep(embedder, vectorStore, 10))
        p.AddStep(chunksToParallelResultsStep{})
        p.AddStep(retrievalstep.NewRAGFusionStep(fusionEngine, 10))
    }
    
    p.AddStep(poststep.NewGenerator(llm, logger))
    
    return searcher
}
```

**适用场景**:
- 知识图谱查询
- 关系推理
- 推荐系统

---

## 实战场景

### 5. 智能客服系统

**完整配置**:
```go
func buildCustomerSupportBot() *hybrid.Searcher {
    // 1. 初始化组件
    embedder := initEmbedder()
    vectorStore := initVectorStore()
    bm25Store := initBM25Store()
    llm := initLLM()
    reranker := initReranker()
    
    // 2. 初始化增强组件
    filterExtractor := enhancer.NewFilterExtractor(llm)
    stepBackGen := enhancer.NewStepBackGenerator(llm)
    
    // 3. 构建 Searcher
    searcher := hybrid.New(
        hybrid.WithEmbedding(embedder),
        hybrid.WithVectorStore(vectorStore),
        hybrid.WithSparseStore(bm25Store),
        hybrid.WithFusionEngine(retrieval.NewRRFEngine(60)),
        hybrid.WithGenerator(llm),
        hybrid.WithReranker(reranker),
        hybrid.WithFilterExtractor(filterExtractor),
        hybrid.WithStepBackGen(stepBackGen),
        hybrid.WithDenseTopK(10),
        hybrid.WithSparseTopK(10),
        hybrid.WithFusionTopK(20),
        hybrid.WithRerankTopK(5),
    )
    
    return searcher
}

// 使用示例
func main() {
    bot := buildCustomerSupportBot()
    
    ctx := context.Background()
    query := "我的订单为什么还没发货？"
    
    answer, err := bot.Search(ctx, query)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(answer)
}
```

---

### 6. 代码搜索引擎

**配置要点**:
```go
func buildCodeSearchEngine() *native.Searcher {
    // 使用专门的代码 embedding
    embedder := embedding.WithCodeBERT()
    
    // 代码分块器
    chunker := parser.NewCodeChunker(
        parser.WithLanguage("go"),
        parser.WithFunctionLevel(true),
    )
    
    searcher := native.New(
        native.WithEmbedding(embedder),
        native.WithVectorStore(codeVectorStore),
        native.WithGenerator(codeLLM),
        native.WithQueryRewriter(service.NewQueryRewriter(llm)),
        native.WithTopK(10),
    )
    
    return searcher
}
```

---

## 最佳实践

### 1. 选择合适的 RAG 模式

| 需求 | 推荐模式 | 理由 |
|------|---------|------|
| 快速原型 | Native RAG | 简单、快速 |
| 高精度 | Hybrid RAG | 多路召回 + 重排序 |
| 复杂推理 | Agentic RAG | 多轮决策 |
| 关系查询 | Graph RAG | 利用实体关系 |

### 2. Steps 选择建议

**Pre-Retrieval**:
- 简单查询：不需要
- 模糊查询：QueryRewrite
- 复杂问题：StepBack + HyDE
- 有元数据：QueryToFilter

**Retrieval**:
- 通用：VectorSearch
- 高精度：VectorSearch + SparseSearch + RAGFusion
- 专业领域：GraphSearch

**Post-Retrieval**:
- 基础：Generation
- 高质量：Rerank + Generation
- 可解释：Evaluation + Generation

### 3. 性能优化技巧

```go
// 1. 并发检索
p.AddStep(chunksToParallelResultsStep{})

// 2. 缓存热点查询
if cacheService != nil {
    p.AddStep(prestep.NewSemanticCacheChecker(cacheService, logger))
    // ... 检索步骤 ...
    p.AddStep(prestep.NewCacheResponseWriter(cacheService, logger))
}

// 3. 动态调整 TopK
searcher.SetTopK(calculateDynamicTopK(query))
```

### 4. 错误处理

```go
answer, err := searcher.Search(ctx, query)
if err != nil {
    // 优雅降级
    if fallbackSearcher != nil {
        answer, err = fallbackSearcher.Search(ctx, query)
    }
    if err != nil {
        return "抱歉，我暂时无法回答这个问题。", err
    }
}
```

---

## 下一步

- 查看具体示例代码：`./examples/` 目录
- API 文档：https://pkg.go.dev/github.com/DotNetAge/gorag
- 更多教程：https://github.com/DotNetAge/gorag/wiki
