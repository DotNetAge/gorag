# Agentic RAG 示例 - Agent 自主决策检索

## 概述

Agentic RAG 引入**Agent 决策循环**，让系统能够自主判断何时检索、何时反思、何时停止，实现类似人类的思考过程。

## Pipeline 组成（ReAct 模式）

```
Query 
  ↓
┌──────────────────────────────────────┐
│  Agent Loop (最多 max_iterations)    │
│  ┌────────────────────────────┐     │
│  │ ReasoningStep              │     │  分析当前状态
│  │ "我应该检索还是直接回答？"   │     │
│  └────────────────────────────┘     │
│            ↓                         │
│  ┌────────────────────────────┐     │
│  │ ActionSelectionStep        │     │  选择行动
│  │ Retrieve / Reflect / Finish│     │
│  └────────────────────────────┘     │
│            ↓                         │
│  ┌────────────────────────────┐     │
│  │ TerminationCheckStep       │     │  检查是否结束
│  └────────────────────────────┘     │
│            ↓                         │
│  ┌────────────────────────────┐     │
│  │ VectorSearchStep           │     │  执行检索（仅当 Retrieve）
│  └────────────────────────────┘     │
│            ↓                         │
│  ┌────────────────────────────┐     │
│  │ ObservationStep            │     │  记录观察结果
│  └────────────────────────────┘     │
└──────────────────────────────────────┘
  ↓
GenerationStep                ← 生成最终答案
  ↓
[SelfRAGStep]                 ← 自我评估（可选）
  ↓
Answer
```

### 涉及的 Steps

| Step | 包路径 | 作用 | 调用次数 |
|------|--------|------|---------|
| ReasoningStep | `infra/steps/agentic` | 分析状态，产生推理链 | 每轮 1 次 |
| ActionSelectionStep | `infra/steps/agentic` | 选择行动（Retrieve/Reflect/Finish） | 每轮 1 次 |
| TerminationCheckStep | `infra/steps/agentic` | 检查终止条件 | 每轮 1 次 |
| VectorSearchStep | `infra/steps/retrieval` | 执行向量检索 | 仅 Retrieve 时 |
| ObservationStep | `infra/steps/agentic` | 记录观察和迭代计数 | 每轮 1 次 |
| GenerationStep | `infra/steps/post_retrieval` | 生成最终答案 | 1 次 |
| SelfRAGStep | `infra/steps/post_retrieval` | 评估答案可信度 | 0-1 次 |

## 使用方式

```go
package main

import (
	"github.com/DotNetAge/gorag/infra/searcher/agentic"
)

func main() {
	// 创建 Agentic RAG Searcher
	searcher := agentic.New(
		// 必需组件（Agent 决策核心）
		agentic.WithReasoner(reasoner),         // LLM 推理
		agentic.WithActionSelector(selector),   // 行动选择器
		agentic.WithGenerator(generator),       // 答案生成
		
		// 检索组件（二选一）
		agentic.WithRetriever(retriever),       // 优先：并行检索器
		// 或
		agentic.WithEmbedding(embedder),        // 备选：向量检索
		agentic.WithVectorStore(store),
		
		// 可选增强
		agentic.WithReranker(reranker),         // 重排序
		agentic.WithSelfRAGJudge(judge, true),  // Self-RAG 评估
		agentic.WithMaxIterations(5),           // 最大迭代次数
		agentic.WithTopK(10),                   // 检索数量
	)
	
	// 执行查询（自动进行多轮决策）
	answer, _ := searcher.Search(ctx, "Go 语言的并发模型与 Java 有什么区别？")
}
```

## Agent 决策流程详解

### 第 1 轮迭代示例

**用户查询**: "Go 语言的并发模型与 Java 有什么区别？"

#### 1. ReasoningStep

**输入**:
```
当前问题：Go 语言的并发模型与 Java 有什么区别？
已检索内容：无
已尝试次数：0
```

**LLM 推理输出**:
```json
{
  "thought": "这是一个比较型问题，需要 Go 和 Java 两方面的知识。当前没有任何检索内容，无法准确回答。",
  "action": "retrieve",
  "action_input": {
    "query": "Go 语言 并发模型 goroutine channel"
  }
}
```

#### 2. ActionSelectionStep

**解析推理结果**:
- Action Type: `retrieve`
- Sub Query: "Go 语言 并发模型 goroutine channel"

**更新 State**:
```go
state.Agentic.Custom["agent_action"] = &retrieval.AgentAction{
	Type:  retrieval.ActionRetrieve,
	Query: "Go 语言 并发模型 goroutine channel",
}
```

#### 3. VectorSearchStep（执行检索）

**检索内容**:
```go
chunks := vectorStore.Search(
	query="Go 语言 并发模型 goroutine channel",
	topK=10,
)
```

**结果存入 State**:
```go
state.RetrievedChunks = [][]*Chunk{
	{chunk1, chunk2, ..., chunk10},
}
```

#### 4. ObservationStep

**记录观察**:
```go
state.Agentic.Custom["iteration"] = 0
state.Agentic.Custom["last_action"] = "retrieve"
logger.Info("Retrieved 10 chunks about Go concurrency")
```

---

### 第 2 轮迭代

#### ReasoningStep

**输入**:
```
当前问题：Go 语言的并发模型与 Java 有什么区别？
已检索内容：10 个关于 Go 并发的文档块
已尝试次数：1
```

**LLM 推理输出**:
```json
{
  "thought": "已经获取了 Go 方面的信息，但缺少 Java 的知识。需要继续检索 Java 相关内容才能进行比较。",
  "action": "retrieve",
  "action_input": {
    "query": "Java 并发模型 线程池 Executor CompletableFuture"
  }
}
```

#### VectorSearchStep（第二次检索）

**新增检索**:
```go
chunks := vectorStore.Search(
	query="Java 并发模型 线程池 Executor",
	topK=10,
)
```

**累积结果**:
```go
state.RetrievedChunks = [][]*Chunk{
	{go_chunk1, ..., go_chunk10},      // 第 1 轮
	{java_chunk1, ..., java_chunk10},  // 第 2 轮
}
```

---

### 第 3 轮迭代

#### ReasoningStep

**输入**:
```
当前问题：Go 语言的并发模型与 Java 有什么区别？
已检索内容：20 个文档块（Go 10 个 + Java 10 个）
已尝试次数：2
```

**LLM 推理输出**:
```json
{
  "thought": "现在已经收集了 Go 和 Java 两方面的足够信息，可以生成完整的对比答案了。",
  "action": "finish",
  "final_answer_outline": "从 goroutine vs 线程、channel vs 锁、CSP vs 面向对象等方面对比"
}
```

#### TerminationCheckStep

**检测到 Finish 动作**:
```go
state.Agentic.Custom["finished"] = true
```

**退出循环**

---

### 生成最终答案

#### GenerationStep

**构建 Prompt**:
```
系统：你是一个专业的技术顾问。请基于以下检索到的内容，详细对比 Go 和 Java 的并发模型。

检索到的 Go 相关内容:
[1] Go 使用 goroutine，轻量级线程，栈内存仅 2KB...
[2] channel 用于 goroutine 间通信，遵循 CSP 原则...
[3] ...

检索到的 Java 相关内容:
[11] Java 使用重量级线程，由操作系统管理...
[12] Java 使用 synchronized、Lock 等同步机制...
[13] ...

问题：Go 语言的并发模型与 Java 有什么区别？

答案：
```

**输出**:
```
Go 语言和 Java 的并发模型有以下主要区别：

1. **线程模型**
   - Go: goroutine，用户态线程，栈内存 2KB，可创建数十万
   - Java: 操作系统线程，栈内存 1MB，通常数千个

2. **通信机制**
   - Go: channel，"通过通信来共享内存"
   - Java: 锁机制，"通过共享内存来通信"
   
3. **编程范式**
   - Go: CSP（Communicating Sequential Processes）
   - Java: 面向对象的多线程模型
   
4. **调度方式**
   - Go: M:N 调度，Go 运行时调度器管理
   - Java: 1:1 映射，操作系统调度
...
```

#### SelfRAGStep（可选）

**评估答案可信度**:
```go
score, reason := judge.EvaluateFaithfulness(
	ctx, query, allChunks, answer,
)
// score = 0.92 (高可信度)
```

**如果分数低 (< 0.8)**:
- **严格模式**: 返回错误，提示答案可能不准确
- **非严格模式**: 在答案后追加警告标记

## 核心 Step API

### 1. ReasoningStep

```go
reasoner := service.NewAgentReasoner(llm)
step := agenticstep.NewReasoningStep(reasoner, logger)
```

**Prompt 模板**:
```
你是一个 RAG 系统的决策者。请分析当前状态，决定下一步行动。

可用行动:
1. retrieve: 检索更多信息
2. reflect: 反思已有信息，重新组织思路
3. finish: 信息充足，生成最终答案

当前问题：{query}
已检索内容：{chunks_count} 个文档块
已尝试次数：{iteration}

请以 JSON 格式输出你的决策。
```

### 2. ActionSelectionStep

```go
selector := service.NewAgentActionSelector(llm)
step := agenticstep.NewActionSelectionStep(selector, maxIterations, logger)
```

**验证逻辑**:
- 确保 Action 类型合法
- 提取 Sub Query（如果是 Retrieve）
- 检查是否超过最大迭代次数

### 3. ParallelRetriever（高级）

```go
retriever := retrieval.NewParallelRetriever(queries, topK)
step := agenticstep.NewParallelRetriever(retriever, topK, logger)
```

**优势**: 一次检索多个子查询，提升效率

## State 数据流转

```go
type PipelineState struct {
	Query *Query
	Agentic *AgenticMetadata {
		Custom: map[string]any {
			"iteration":     0, 1, 2, ...
			"agent_action":  &AgentAction{Type, Query}
			"finished":      true/false
			"reasoning_trace": [...]
		}
	}
	RetrievedChunks [][]*Chunk  // 多轮累积
	Answer string
}
```

## 配置建议

### MaxIterations

```go
// 简单问题：2-3 轮
agentic.WithMaxIterations(3)

// 复杂分析：5-7 轮
agentic.WithMaxIterations(7)

// 默认值：5 轮（平衡效果和成本）
```

### TopK 策略

```go
// 每轮检索量不宜过大，避免上下文爆炸
agentic.WithTopK(5)  // 每轮 5 个文档块
```

## 典型应用场景

✅ **适用场景**:
- 复杂多跳问答（需要多次检索）
- 开放域问答（不确定需要哪些信息）
- 研究型问题（需要多角度信息）
- 对话系统（需要多轮交互）

❌ **不适用场景**:
- 简单事实查询（Native RAG 即可）
- 实时性要求极高（多轮迭代延迟高）
- 成本敏感（多次 LLM 调用）

## 调试技巧

### 启用详细日志

```go
logger := logging.NewZapLogger(logging.DebugLevel)
searcher := agentic.New(
	agentic.WithLogger(logger),
)
```

**输出示例**:
```
[DEBUG] Agent iteration 1 started
[DEBUG] Reasoning: Need more information about Java
[DEBUG] Action selected: retrieve
[DEBUG] Sub-query: "Java 并发模型"
[DEBUG] Retrieved 10 chunks
[DEBUG] Observation recorded
```

### 查看推理链

```go
// 从 State 中提取完整推理过程
for i := 0; i < maxIterations; i++ {
	reasoning := state.Agentic.Custom["reasoning_trace_"+strconv.Itoa(i)]
	fmt.Printf("第%d轮推理：%s\n", i, reasoning)
}
```

## 性能优化

### 1. 缓存推理结果

对相似查询缓存 Agent 决策路径，避免重复推理

### 2. 提前终止检测

设置置信度阈值，达到即停止

```go
if state.SelfRagScore > 0.9 {
	break  // 提前结束
}
```

### 3. 批量检索

将多轮的 Sub Query 合并，一次性检索

## 与其他模式对比

| 特性 | Native | Hybrid | **Agentic** |
|------|--------|--------|-------------|
| 检索轮数 | 1 轮 | 1 轮 | **多轮** |
| 决策能力 | ❌ | ❌ | ✅ |
| 适应性 | 低 | 中 | **高** |
| 延迟 | 低 | 中 | 高 |
| 成本 | 低 | 中 | 高 |

## 参考资源

- [ReAct 论文](https://arxiv.org/abs/2210.03629)
- [Self-RAG 论文](https://arxiv.org/abs/2310.11511)
- [Agentic RAG 设计文档](../../specs/agentic_rag.md)
