# Reciprocal Rank Fusion - 多路召回融合

## 什么是 RRF？

RRF（Reciprocal Rank Fusion，倒数排名融合）是一种将多个检索结果集合并为单一排名列表的算法。它通过计算每个文档在不同结果集中的排名得分来融合结果。

### 核心原理

```
查询 "Go 语言教程"
        ↓
    [多路召回]
        ↓
    ┌────────────┬────────────┐
    │  Sparse    │  Dense      │
    │  检索结果  │  检索结果   │
    └────────────┴────────────┘
            ↓
    [RRF 融合引擎]
            ↓
    根据排名计算融合分数
            ↓
    输出最终排序结果
```

### RRF 公式

```
RRF_Score = 1 / (k + rank)
```

其中：
- `k`: 平滑常数（默认 60）
- `rank`: 文档在结果集中的排名位置（从 1 开始）

---

## 有什么作用？

1. **多模态融合**：合并 Sparse（关键词）+ Dense（向量）等多种检索结果
2. **提升召回率**：不同检索方式互补，减少单一检索的遗漏
3. **统一排序**：将不同来源的结果按统一标准排序输出

### 与简单融合的区别

| 特性 | 简单合并 | RRF 融合 |
|------|----------|----------|
| 排序方式 | 保持原顺序 | 按 RRF 分数重新排序 |
| 结果质量 | 可能有重复和顺序偏差 | 去重 + 全局最优排序 |
| 排名考虑 | 不考虑 | 综合多路排名 |

---

## 怎么工作的？

### RRF 分数计算流程

```
示例：两个检索结果集

Sparse 检索结果:    [Doc_A, Doc_B, Doc_C]
Dense 检索结果:     [Doc_B, Doc_D, Doc_A]

Step 1: 计算每个文档的 RRF 分数
┌─────────┬──────────────┬──────────────┬─────────────┐
│  文档   │  Sparse 排名 │  Dense 排名  │  RRF 分数  │
├─────────┼──────────────┼──────────────┼─────────────┤
│ Doc_A   │  1 (k=60)    │  3           │  0.0164     │
│ Doc_B   │  2           │  1           │  0.0162     │
│ Doc_C   │  3           │  -           │  0.0159     │
│ Doc_D   │  -           │  2           │  0.0161     │
└─────────┴──────────────┴──────────────┴─────────────┘

Step 2: 按 RRF 分数降序排序
Doc_A (0.0164) > Doc_B (0.0162) > Doc_D (0.0161) > Doc_C (0.0159)
```

### 融合流程图

```
多个检索结果集
        ↓
    遍历每个结果集
        ↓
    遍历每个文档
        ↓
    根据排名计算 RRF 分数
        ↓
    累加同一文档的多路分数
        ↓
    按总分排序输出
        ↓
    返回 TopK 结果
```

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/fusion/
├── fusion.go       # FusionEngine 接口定义
├── rrf_fusion.go   # RRFFusionEngine 实现
└── rrf_fusion_test.go  # 测试
```

### 1. 统一接口（fusion.go）

```go
type FusionEngine interface {
    ReciprocalRankFusion(ctx context.Context, resultSets [][]*core.Chunk, topK int) ([]*core.Chunk, error)
}
```

### 2. RRF 实现（RRFFusionEngine）

```go
engine := fusion.NewRRFFusionEngine()

resultSets := [][]*core.Chunk{
    sparseResults,  // Sparse 检索结果
    denseResults,   // Dense 检索结果
}

fused, err := engine.ReciprocalRankFusion(ctx, resultSets, 10)
```

**特性**：
- **平滑常数 k=60**：遵循 RRF 原始论文的默认值
- **自动去重**：相同文档在不同结果集中只出现一次
- **分数累加**：文档在多路检索中均有出现时，分数叠加提升排名

### 3. 核心算法

```go
func (e *RRFFusionEngine) ReciprocalRankFusion(ctx context.Context, resultSets [][]*core.Chunk, topK int) ([]*core.Chunk, error) {
    scoreMap := make(map[string]float32)
    chunkMap := make(map[string]*core.Chunk)

    for _, resultSet := range resultSets {
        for rank, chunk := range resultSet {
            score := 1.0 / (e.k + float32(rank+1))
            scoreMap[chunk.ID] += score
            chunkMap[chunk.ID] = chunk
        }
    }

    var fusedChunks []*core.Chunk
    for _, chunk := range chunkMap {
        fusedChunks = append(fusedChunks, chunk)
    }

    sort.SliceStable(fusedChunks, func(i, j int) bool {
        return scoreMap[fusedChunks[i].ID] > scoreMap[fusedChunks[j].ID]
    })

    return e.limit(fusedChunks, topK), nil
}
```

---

## 如何与项目集成？

### 方式一：Pipeline 集成（推荐）

在 RAG Pipeline 中添加融合步骤：

```go
p := pipeline.New[*core.RetrievalContext]()

p.AddStep(multivector.Search(store, embedder, opts))
p.AddStep(fusion.Merge(fusionEngine, logger))
p.AddStep(generate.New(llm, logger))
```

### 方式二：直接调用

```go
engine := fusion.NewRRFFusionEngine()

sparseResults, _ := sparseRetriever.Retrieve(ctx, query)
denseResults, _ := denseRetriever.Retrieve(ctx, query)

resultSets := [][]*core.Chunk{sparseResults, denseResults}
fused, err := engine.ReciprocalRankFusion(ctx, resultSets, 10)
```

### 方式三：RAG 入口配置

```go
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
    gorag.WithFusion(true),  // 启用多路召回融合
)
```

---

## 适用于哪些场景？

### ✅ 适合使用

- **多模态检索**：同时使用 Sparse（BM25）+ Dense（向量）检索
- **混合搜索**：向量搜索 + 关键词搜索 + 图搜索
- **跨模态场景**：文本 + 图像 + 结构化数据的多路召回
- **搜索质量优化**：单一检索方式召回不足

### ❌ 不适合使用

- **单一路检索**：只有一种检索方式时无需融合
- **实时性要求极高**：融合增加额外计算开销
- **结果集差异大**：不同检索结果完全无关时融合效果有限

---

## 配置推荐

| 场景 | 推荐配置 |
|------|----------|
| 标准混合检索 | k=60（默认） |
| 强调整合性 | k=100（平滑更强） |
| 强调差异性 | k=30（放大排名差异） |

```go
// 自定义 k 值
engine := &fusion.RRFFusionEngine{k: 100}
```
