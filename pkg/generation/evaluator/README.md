# Evaluator - RAG 评估器

RAG 系统的评估模块，提供 Faithfulness、Relevance、Precision 等指标，用于衡量 RAG 答案质量。

## 是什么

评估器是 RAG 系统质量保障的关键组件，通过 LLM 作为 Judge 对生成的答案进行多维度打分。

### 核心指标

```
┌─────────────────────────────────────────────────────────┐
│                    RAG 评估指标                          │
├─────────────────┬─────────────────┬─────────────────────┤
│  Faithfulness   │   Relevance     │   Precision         │
│  (忠诚度)        │   (相关性)       │   (精确度)           │
├─────────────────┴─────────────────┴─────────────────────┤
│  答案是否基于      答案是否回答      检索到的上下文是否      │
│  给定的上下文？     了用户的问题？     真正有用？            │
└─────────────────────────────────────────────────────────┘
```

### 评估类型

| 类型 | 说明 | 用途 |
|------|------|------|
| RAGEvaluator | 标准 RAG 评估 | 评估完整 RAG 流程 |
| CRAGEvaluator | CRAG (Corrective RAG) 评估 | 判断检索结果是否相关 |
| RagasLLMJudge | RAGAS 风格 LLM 评估 | 端到端质量评估 |

---

## 有什么用

1. **质量监控**：量化 RAG 系统的输出质量
2. **迭代优化**：通过分数识别 RAG 流程中的薄弱环节
3. **A/B 测试**：比较不同配置/模型的性能差异
4. **自动化回归**：确保系统升级不降低答案质量

---

## 怎么工作的

### 评估流程

```
用户查询 + 上下文 + 答案
           ↓
      [LLM Judge]
           ↓
    ┌──────┴──────┐
    ↓             ↓
Faithfulness   Relevance
    ↓             ↓
    └──────┬──────┘
           ↓
      综合评分 + 原因
```

### RagasLLMJudge 实现

```
1. 构建 Prompt，包含：
   - 评估维度的定义
   - 评分标准 (0.0-1.0)
   - 待评估的内容
           ↓
2. 发送给 LLM (GPT-4 等)
           ↓
3. 解析响应：
   Score: 0.85
   Reason: The answer is mostly faithful...
           ↓
4. 返回分数和原因
```

### CRAG 评估标签

| 标签 | 值 | 含义 |
|------|-----|------|
| Relevant | 1 | 检索结果高度相关，直接用于生成 |
| Ambiguous | 0 | 部分相关，需要优化或重写 |
| Irrelevant | -1 | 不相关，需要重新检索 |

---

## 我们怎么实现的

### 核心接口

```go
// LLMJudge - 使用 LLM 进行评估
type LLMJudge interface {
    EvaluateFaithfulness(ctx context.Context, query string, chunks []*core.Chunk, answer string) (float32, string, error)
    EvaluateAnswerRelevance(ctx context.Context, query string, answer string) (float32, string, error)
    EvaluateContextPrecision(ctx context.Context, query string, chunks []*core.Chunk) (float32, string, error)
}
```

### 核心结构

```go
// RAGEvaluator - 标准 RAG 评估
type ragEvaluator struct {
    llm chat.Client
}

// CRAGEvaluator - CRAG 评估
type cragEvaluator struct {
    llm chat.Client
}

// RagasLLMJudge - RAGAS 风格评估
type RagasLLMJudge struct {
    judgeLLM chat.Client
}

// BenchmarkResult - 批量评估结果
type BenchmarkResult struct {
    TotalCases      int
    AvgFaithfulness float32
    AvgRelevance    float32
    AvgPrecision    float32
    TotalDuration   time.Duration
    Results         []CaseResult
}
```

### 评估方法

| 方法 | 说明 |
|------|------|
| `NewRAGEvaluator(llm)` | 创建标准 RAG 评估器 |
| `NewCRAGEvaluator(llm)` | 创建 CRAG 评估器 |
| `NewRagasLLMJudge(llm)` | 创建 RAGAS Judge |
| `RunBenchmark(...)` | 运行批量评估 |
| `BenchmarkResult.Summary()` | 生成汇总报告 |

---

## 如何与项目集成

### 基本用法

```go
// 创建 RAGAS Judge
judge := evaluation.NewRagasLLMJudge(gpt4Client)

// 评估单个答案
faithScore, reason, err := judge.EvaluateFaithfulness(ctx, query, chunks, answer)
fmt.Printf("Faithfulness: %.2f - %s\n", faithScore, reason)
```

### 批量基准测试

```go
cases := []evaluation.TestCase{
    {Query: "法国的首都是什么？", GroundTruth: "巴黎"},
    {Query: "水的沸点是多少？", GroundTruth: "100°C"},
}

result, err := evaluation.RunBenchmark(ctx, retriever, judge, cases, 5)
fmt.Println(result.Summary())
// 输出:
// Benchmark completed in 2.5s
// Cases: 2
// Avg Faithfulness: 0.85
// Avg Relevance: 0.82
// Avg Precision: 0.78
```

### 在 Pipeline 中集成

```go
// Pipeline 评估中间件
p := pipeline.New[*core.GenerationContext]()

p.AddStep(evaluator.NewRAGEvaluator(llm))
p.AddStep(generator.Generate(...))
```

---

## 适用于哪些场景

### ✅ 适合使用

- **系统调优**：识别 RAG 流程中的瓶颈
- **模型选择**：比较不同 LLM 的表现
- **回归测试**：确保系统升级后质量不下降
- **生产监控**：实时监控答案质量

### ❌ 不适合使用

- **实时性要求高**：评估有额外延迟
- **成本敏感**：每次评估消耗 LLM token
- **简单场景**：规则判断足够时不需要 LLM

---

## API 参考

### `NewRagasLLMJudge`

```go
func NewRagasLLMJudge(judgeLLM chat.Client) *RagasLLMJudge
```

创建 RAGAS 风格的 LLM Judge。

### `EvaluateFaithfulness`

```go
func (j *RagasLLMJudge) EvaluateFaithfulness(
    ctx context.Context,
    query string,
    chunks []*core.Chunk,
    answer string,
) (float32, string, error)
```

评估答案是否忠实于给定的上下文（无幻觉）。

### `EvaluateAnswerRelevance`

```go
func (j *RagasLLMJudge) EvaluateAnswerRelevance(
    ctx context.Context,
    query string,
    answer string,
) (float32, string, error)
```

评估答案是否有效回答了用户的问题。

### `EvaluateContextPrecision`

```go
func (j *RagasLLMJudge) EvaluateContextPrecision(
    ctx context.Context,
    query string,
    chunks []*core.Chunk,
) (float32, string, error)
```

评估检索到的上下文的质量。

### `RunBenchmark`

```go
func RunBenchmark(
    ctx context.Context,
    retriever core.Retriever,
    judge LLMJudge,
    cases []TestCase,
    topK int,
) (*BenchmarkResult, error)
```

运行完整的评估基准测试套件。

### `BenchmarkResult.Summary`

```go
func (r *BenchmarkResult) Summary() string
```

生成人类可读的评估报告。

---

## 测试

```bash
go test ./pkg/generation/evaluator/... -v
```

**测试覆盖**：
- `TestRAGEvaluator_New` - 构造函数
- `TestRAGEvaluator_Evaluate` - 基本评估
- `TestRAGEvaluator_Evaluate_LLMError` - LLM 错误处理
- `TestCRAGEvaluator_New` - CRAG 构造函数
- `TestCRAGEvaluator_Evaluate` - CRAG 评估
- `TestCRAGEvaluator_Evaluate_LLMClientNotUsed` - LLM 未使用
- `TestBenchmarkResult_Summary` - 结果汇总
- `TestRunBenchmark` - 批量基准测试
- `TestRagasLLMJudge_New` - Judge 构造函数
- `TestRagasLLMJudge_EvaluateFaithfulness` - 忠诚度评估
- `TestRagasLLMJudge_EvaluateAnswerRelevance` - 相关性评估
- `TestRagasLLMJudge_EvaluateContextPrecision` - 上下文精确度
- `TestRagasLLMJudge_ParseEvalResponse` - 响应解析
- `TestRagasLLMJudge_ParseEvalResponse_InvalidScore` - 无效分数处理
- `TestRagasLLMJudge_ParseEvalResponse_NoReason` - 缺少原因处理
- `TestBuildContextText` - 上下文字本构建