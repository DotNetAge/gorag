# Native RAG 示例 - 基础检索增强生成

## 概述

这个示例展示最简单的 RAG 流程，适合入门学习。

## Pipeline 组成

```
Query → [QueryRewriteStep] → VectorSearchStep → GenerationStep → Answer
```

### 涉及的 Steps

| Step | 包路径 | 作用 | 是否必需 |
|------|--------|------|---------|
| QueryRewriteStep | `infra/steps/pre_retrieval` | 优化查询表达 | 可选（推荐） |
| VectorSearchStep | `infra/steps/retrieval` | 稠密向量检索 | **必需** |
| GenerationStep | `infra/steps/post_retrieval` | LLM 生成答案 | **必需** |

## 使用方式

```go
package main

import (
	"context"
	"fmt"
	
	"github.com/DotNetAge/gorag/infra/searcher/native"
)

func main() {
	ctx := context.Background()
	
	// 创建 Searcher（自动组装 Steps）
	searcher := native.New(
		native.WithEmbedding(embedder),      // 注入嵌入模型
		native.WithVectorStore(store),       // 注入向量库
		native.WithGenerator(generator),     // 注入 LLM
		native.WithQueryRewriter(rewriter),  // 可选：查询改写
		native.WithTopK(10),                 // 配置检索数量
	)
	
	// 执行查询
	query := "GoRAG 的工作原理是什么？"
	answer, err := searcher.Search(ctx, query)
	if err != nil {
		panic(err)
	}
	
	fmt.Println(answer)
}
```

## Step 详细解析

### 1. QueryRewriteStep（预检索阶段）

**位置**: `infra/steps/pre_retrieval/query_rewrite_step.go`

**函数**: `prestep.NewQueryRewriteStep(rewriter retrieval.QueryRewriter)`

**作用**: 
- 将模糊查询转换为精确查询
- 扩展简短查询的上下文
- 示例："Go 语言" → "Go 编程语言的主要特性和应用场景"

**内部实现**:
```go
type QueryRewriteStep struct {
	rewriter retrieval.QueryRewriter
}

func (s *QueryRewriteStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	// 调用 Rewriter 改写查询
	newQuery, err := s.rewriter.Rewrite(ctx, state.Query)
	if err != nil {
		return err
	}
	state.Query.Text = newQuery
	return nil
}
```

### 2. VectorSearchStep（检索阶段）

**位置**: `infra/steps/retrieval/vector_search_step.go`

**函数**: `retrievalstep.NewVectorSearchStep(embedder, store, topK)`

**参数**:
- `embedder`: 嵌入模型提供者
- `store`: 向量存储接口
- `topK`: 返回最相似的 K 个结果

**工作流程**:
1. 读取 `state.Query.Text`
2. 使用 embedder 将查询转为向量
3. 在 store 中搜索 TopK 相似向量
4. 将结果存入 `state.RetrievedChunks`

**代码示例**:
```go
step := retrievalstep.NewVectorSearchStep(
	embedder,  // embedding.Provider
	store,     // abstraction.VectorStore
	10,        // topK int
)
```

### 3. GenerationStep（后检索阶段）

**位置**: `infra/steps/post_retrieval/generation_step.go`

**函数**: `poststep.NewGenerator(generator, logger)`

**输入**: 
- `state.Query.Text`: 用户查询
- `state.RetrievedChunks`: 检索到的文档块

**输出**: 
- `state.Answer`: 生成的自然语言答案

**Prompt 模板**:
```
系统：你是一个有帮助的助手。请基于以下检索到的内容回答问题。

检索到的内容:
[1] GoRAG 是一个模块化 RAG 框架...
[2] 它支持多种高级检索策略...

问题：{query}

答案：
```

## Pipeline 组装过程

Native Searcher 在 `buildPipeline()` 方法中组装 Steps：

```go
func (s *Searcher) buildPipeline() *pipeline.Pipeline[*entity.PipelineState] {
	p := pipeline.New[*entity.PipelineState]()
	
	// 可选：语义缓存检查
	if s.cacheService != nil {
		p.AddStep(prestep.NewSemanticCacheChecker(s.cacheService, s.logger))
	}
	
	// 可选：查询改写
	if s.queryRewriter != nil {
		p.AddStep(prestep.NewQueryRewriteStep(s.queryRewriter))
	}
	
	// 必需：向量检索
	p.AddStep(retrievalstep.NewVectorSearchStep(s.embedder, s.vectorStore, s.topK))
	
	// 必需：答案生成
	p.AddStep(poststep.NewGenerator(s.generator, s.logger))
	
	// 可选：缓存响应写入
	if s.cacheService != nil {
		p.AddStep(prestep.NewCacheResponseWriter(s.cacheService, s.logger))
	}
	
	return p
}
```

## State 数据流转

```
初始状态:
state.Query = "GoRAG 原理"
state.RetrievedChunks = []
state.Answer = ""

↓ QueryRewriteStep
state.Query = "GoRAG 框架的工作原理和架构设计"

↓ VectorSearchStep
state.RetrievedChunks = [
  [chunk1, chunk2, ..., chunk10]
]

↓ GenerationStep
state.Answer = "GoRAG 是一个基于 Go 语言的模块化 RAG 框架..."
```

## 性能调优

### 1. 调整 TopK

```go
// TopK 太小：可能漏掉关键信息
// TopK 太大：增加 LLM 上下文负担，降低准确性
searcher := native.New(
	native.WithTopK(15),  // 默认值：10
)
```

### 2. 启用语义缓存

```go
cacheService := service.NewSemanticCacheService(redisClient)
searcher := native.New(
	native.WithSemanticCache(cacheService),
)
```

**效果**: 
- 重复查询直接返回缓存答案
- 响应时间从秒级降至毫秒级

### 3. 自定义日志

```go
logger := logging.NewZapLogger(logging.InfoLevel)
searcher := native.New(
	native.WithLogger(logger),
)
```

## 典型应用场景

✅ **适用场景**:
- 文档问答系统
- 知识库查询
- 客服机器人
- 技术支持助手

❌ **不适用场景**:
- 需要多轮对话（考虑 Agentic RAG）
- 需要混合检索（考虑 Hybrid RAG）
- 需要图谱推理（考虑 Graph RAG）

## 下一步

掌握 Native RAG 后，可以学习：

1. **[Hybrid RAG](../02_hybrid_rag/)** - 结合稠密 + 稀疏检索
2. **[Agentic RAG](../03_agentic_rag/)** - Agent 自主决策多轮检索
3. **[Graph RAG](../04_graph_rag/)** - 知识图谱增强检索

## 参考资源

- [QueryRewriteStep 源码](../../infra/steps/pre_retrieval/query_rewrite_step.go)
- [VectorSearchStep 源码](../../infra/steps/retrieval/vector_search_step.go)
- [GenerationStep 源码](../../infra/steps/post_retrieval/generation_step.go)
- [Native Searcher 实现](../../infra/searcher/native/searcher.go)
