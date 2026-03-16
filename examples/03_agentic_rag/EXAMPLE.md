# Agentic RAG - Agent 自主决策检索示例

Agentic RAG 让 AI Agent 能够自主决定何时检索、检索什么、以及如何利用检索结果。

## Pipeline 流程图

```
Query
  ↓
┌─────────────────────────────────────┐
│  Agent Loop (最多 N 次迭代)          │
│                                     │
│  1. ReasoningStep                   │
│     思考：我需要什么信息？           │
│                                     │
│  2. ActionSelectionStep             │
│     选择行动：检索 or 生成？         │
│                                     │
│  3. TerminationCheckStep            │
│     检查：是否满足终止条件？         │
│        - 已找到答案                 │
│        - 达到最大迭代次数            │
│                                     │
│  4. ParallelRetriever               │
│     执行检索（如果需要）             │
│                                     │
│  5. ObservationStep                 │
│     观察：检索结果说明了什么？       │
│                                     │
└─────────────────────────────────────┘
  ↓
[Final Pipeline]
  ↓
RerankStep (可选)
  ↓
GenerationStep → 最终答案
```

## 核心概念

### 1. Agent 循环（Agent Loop）

Agent 会在一个循环中反复执行以下步骤：

1. **Reasoning（推理）**: 分析当前状态，确定需要什么信息
2. **Action Selection（行动选择）**: 决定下一步做什么
   - `retrieve`: 需要更多信息，执行检索
   - `generate`: 信息充足，生成答案
3. **Termination Check（终止检查）**: 判断是否可以结束
4. **Observation（观察）**: 分析检索结果，更新认知

### 2. 关键 Steps

#### ReasoningStep
```go
agenticstep.NewReasoningStep(reasoner, logger)
```
- **输入**: 当前查询 + 历史对话 + 已检索内容
- **输出**: 推理结果（需要什么信息、为什么需要）
- **示例输出**: "用户询问产品对比，但我还没有获取两个产品的详细信息，需要先检索"

#### ActionSelectionStep
```go
agenticstep.NewActionSelectionStep(selector, maxIterations, logger)
```
- **输入**: 推理结果
- **输出**: 行动决策（retrieve / generate）
- **决策依据**: 
  - 信息是否充足？
  - 是否已达到最大迭代次数？
  - 是否需要进一步检索？

#### ParallelRetriever
```go
agenticstep.NewParallelRetriever(retriever, topK, logger)
```
- **功能**: 并行执行多个检索任务
- **支持**: 同时检索多个知识源

#### ObservationStep
```go
agenticstep.NewObservationStep(logger)
```
- **功能**: 分析检索结果，提取关键信息
- **输出**: 更新后的上下文和认知状态

## 代码示例

### 基础版本

```go
package main

import (
    "context"
    "github.com/DotNetAge/gorag/infra/searcher/agentic"
    agenticstep "github.com/DotNetAge/gorag/infra/steps/agentic"
    retrievalstep "github.com/DotNetAge/gorag/infra/steps/retrieval"
)

func buildBasicAgenticRAG() *agentic.Searcher {
    searcher := agentic.New(
        agentic.WithReasoner(reasoner),
        agentic.WithActionSelector(selector),
        agentic.WithGenerator(llm),
        agentic.WithMaxIterations(5),
    )
    
    // 构建 Agent 循环体
    loop := pipeline.New[*entity.PipelineState]()
    
    // 1. 推理步骤
    loop.AddStep(agenticstep.NewReasoningStep(reasoner, logger))
    
    // 2. 行动选择
    loop.AddStep(agenticstep.NewActionSelectionStep(selector, 5, logger))
    
    // 3. 终止检查
    loop.AddStep(agenticstep.NewTerminationCheckStep(logger))
    
    // 4. 检索步骤（仅在 action == retrieve 时执行）
    loop.AddStep(retrievalstep.NewVectorSearchStep(embedder, vectorStore, 5))
    
    // 5. 观察步骤
    loop.AddStep(agenticstep.NewObservationStep(logger))
    
    // 最终生成管道
    final := pipeline.New[*entity.PipelineState]()
    final.AddStep(poststep.NewGenerator(llm, logger))
    
    return searcher
}
```

### 进阶版本（带多路检索）

```go
func buildAdvancedAgenticRAG() *agentic.Searcher {
    searcher := agentic.New(
        agentic.WithReasoner(reasoner),
        agentic.WithActionSelector(selector),
        agentic.WithRetriever(multiSourceRetriever),
        agentic.WithGenerator(llm),
        agentic.WithReranker(reranker),
        agentic.WithMaxIterations(5),
    )
    
    // === Agent 循环体 ===
    loop := pipeline.New[*entity.PipelineState]()
    
    // Step 1: 推理
    loop.AddStep(agenticstep.NewReasoningStep(reasoner, logger))
    
    // Step 2: 行动选择
    loop.AddStep(agenticstep.NewActionSelectionStep(selector, 5, logger))
    
    // Step 3: 终止检查
    loop.AddStep(agenticstep.NewTerminationCheckStep(logger))
    
    // Step 4: 并行检索（多知识源）
    if multiSourceRetriever != nil {
        loop.AddStep(agenticstep.NewParallelRetriever(
            multiSourceRetriever, 
            5, 
            logger,
        ))
    } else {
        // Fallback: 单向量库检索
        loop.AddStep(retrievalstep.NewVectorSearchStep(
            embedder, 
            vectorStore, 
            5,
        ))
    }
    
    // Step 5: 观察分析
    loop.AddStep(agenticstep.NewObservationStep(logger))
    
    // === 最终处理管道 ===
    final := pipeline.New[*entity.PipelineState]()
    
    // 重排序（可选）
    if reranker != nil {
        final.AddStep(poststep.NewRerankStep(reranker, 5))
    }
    
    // 生成答案
    final.AddStep(poststep.NewGenerator(llm, logger))
    
    // 自我评估（可选）
    if selfJudge != nil {
        final.AddStep(&selfRAGStep{
            judge:     selfJudge,
            strict:    true,
            threshold: 0.8,
        })
    }
    
    return searcher
}
```

## 使用场景

### 场景 1: 复杂问答

**问题**: "比较特斯拉 Model 3 和比亚迪汉的优缺点，并给出购买建议"

**Agent 执行流程**:
```
Iteration 1:
  Reasoning: 需要两款车的详细信息
  Action: retrieve (检索 Model 3 信息)
  Observation: 获取到 Model 3 的性能、价格等数据
  
Iteration 2:
  Reasoning: 还需要比亚迪汉的信息
  Action: retrieve (检索比亚迪汉信息)
  Observation: 获取到比亚迪汉的数据
  
Iteration 3:
  Reasoning: 已有足够信息进行对比
  Action: generate
  Termination: ✓ 满足终止条件
  
Final: 生成对比分析和购买建议
```

### 场景 2: 研究助手

**问题**: "量子计算在药物研发中的应用前景如何？"

**Agent 执行流程**:
```
Iteration 1:
  Reasoning: 需要了解量子计算的基本原理
  Action: retrieve (检索量子计算基础)
  
Iteration 2:
  Reasoning: 需要了解药物研发的流程
  Action: retrieve (检索药物研发流程)
  
Iteration 3:
  Reasoning: 需要查找两者的结合点
  Action: retrieve (检索量子计算 + 药物研发)
  
Iteration 4:
  Reasoning: 信息充足，可以综合分析
  Action: generate
  
Final: 生成综合分析报告
```

### 场景 3: 故障诊断

**问题**: "服务器 CPU 使用率异常升高，可能的原因是什么？"

**Agent 执行流程**:
```
Iteration 1:
  Reasoning: 需要查看最近的系统变更记录
  Action: retrieve (检索变更日志)
  
Iteration 2:
  Reasoning: 需要检查常见原因列表
  Action: retrieve (检索故障知识库)
  
Iteration 3:
  Reasoning: 需要查看监控数据（调用工具）
  Action: retrieve (调用监控 API)
  
Iteration 4:
  Reasoning: 综合所有信息进行分析
  Action: generate
  
Final: 生成诊断报告和解决建议
```

## 配置参数说明

### MaxIterations（最大迭代次数）

```go
agentic.WithMaxIterations(5)
```
- **推荐值**: 3-7
- **过低**: 可能无法充分检索
- **过高**: 增加延迟，可能导致循环

### TopK（检索数量）

```go
agentic.WithTopK(5)
```
- **推荐值**: 3-10
- **根据场景调整**: 
  - 精确问答：较小的 TopK (3-5)
  - 综合分析：较大的 TopK (5-10)

## 性能优化技巧

### 1. 语义缓存

```go
// 在 Agent 循环前添加缓存检查
if cacheService != nil {
    p.AddStep(prestep.NewSemanticCacheChecker(cacheService, logger))
}
```

### 2. 早期终止

```go
// 在 ReasoningStep 中添加快速路径
type SmartReasoner struct {
    base Reasoner
}

func (r *SmartReasoner) Reason(ctx, state) error {
    // 如果查询简单直接，跳过 Agent 循环
    if isSimpleQuery(state.Query) {
        state.SetAction("generate")
        return nil
    }
    return r.base.Reason(ctx, state)
}
```

### 3. 并行检索优化

```go
// 使用 ParallelRetriever 同时检索多个源
retriever := aggregation.NewParallelRetriever(
    vectorRetriever,
    graphRetriever,
    webRetriever,
)
```

## 调试技巧

### 启用详细日志

```go
logger := logging.NewDefaultLogger(logging.WithLevel(logging.DEBUG))

searcher := agentic.New(
    agentic.WithLogger(logger),
    // ... 其他配置
)
```

### 观察 Agent 决策过程

```go
// 添加回调函数
searcher.SetIterationCallback(func(iteration int, state *entity.PipelineState) {
    fmt.Printf("=== Iteration %d ===\n", iteration)
    fmt.Printf("Reasoning: %s\n", state.Reasoning)
    fmt.Printf("Action: %s\n", state.Action)
    fmt.Printf("Context Length: %d\n", len(state.Context))
})
```

## 运行示例

```bash
cd examples/03_agentic_rag
go run main.go
```

## 输出示例

```
=== Query: 比较特斯拉 Model 3 和比亚迪汉的优缺点 ===

--- Agent Iteration 1 ---
Reasoning: 需要获取两款车型的详细信息进行对比
Action: retrieve
Retrieved 5 documents about Tesla Model 3

--- Agent Iteration 2 ---
Reasoning: 已有 Model 3 信息，还需要比亚迪汉的数据
Action: retrieve
Retrieved 5 documents about BYD Han

--- Agent Iteration 3 ---
Reasoning: 已收集足够信息，可以进行对比分析
Action: generate
Termination check: PASSED

=== Final Answer ===

特斯拉 Model 3 vs 比亚迪汉 对比分析：

1. 性能对比
   - Model 3: ...
   - 比亚迪汉：...

2. 价格对比
   ...

3. 购买建议
   ...
```

## 下一步

- 查看 [`04_graph_rag`](../04_graph_rag/) - 图谱增强检索
- 查看 [`05_multiagent_rag`](../05_multiagent_rag/) - 多 Agent 协作
- 阅读 [Agentic RAG 论文](https://arxiv.org/abs/2401.xxxxx)
