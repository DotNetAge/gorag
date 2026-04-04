# NativeRAG 检索器

NativeRAG 是 GoRAG 的核心检索器实现，支持从基础 RAG 到高级 RAG 的各种场景。

## 核心设计理念

**用户无需了解顺序，Retriever 自动按三阶段组装 Pipeline**

```go
// 用户可以随意组合 options - 无需关心顺序
retriever := NewRetriever(vectorStore, embedder, llm, 10,
    WithQueryRewrite(),  // 查询前
    WithHyDE(),          // 查询前
    WithFusion(5),       // 查询前 + 查询后
    WithParentDoc(nil),  // 查询后
    WithRerank(),        // 查询后
)

// Retriever 内部自动按三阶段锁死顺序组装：
// [Pre-Retrieval]  QueryRewrite → HyDE → Decompose
// [Retrieval]      Vector Search
// [Post-Retrieval] RRF → ParentDoc → Rerank → Generate
```

## 三阶段 Pipeline 架构

### PHASE 0: Cache Check（可选）
- `SemanticCache Check` - 检查语义缓存命中

### PHASE 1: Pre-Retrieval（查询增强）
查询前的优化步骤，按以下顺序执行：
1. `QueryRewrite` - 重写/澄清查询
2. `HyDE` - 生成假设性文档
3. `StepBack` - 生成后退一步的抽象问题
4. `Decompose` - 分解复杂查询（Fusion 需要）

### PHASE 2: Retrieval（向量检索）
核心检索步骤：
- `Vector Search` - 向量相似度搜索

### PHASE 3: Post-Retrieval（结果增强）
检索后的优化步骤，按以下顺序执行：
1. `RRF` - 倒数排名融合（Fusion 需要）
2. `ParentDoc` - 父文档扩展
3. `SentenceWindow` - 句子窗口扩展
4. `Prune` - 上下文裁剪
5. `Rerank` - 重排序
6. `Generate` - 生成答案

### PHASE 4: Cache Store（可选）
- `SemanticCache Store` - 缓存查询结果

## 使用示例

### 基础 RAG
```go
import nativeretriever "github.com/DotNetAge/gorag/pkg/retriever/native"

retriever := nativeretriever.NewRetriever(
    vectorStore,
    embedder,
    llm,
    5, // topK
)
```

### 查询重写 + Fusion + 父文档扩展
```go
retriever := nativeretriever.NewRetriever(
    vectorStore, embedder, llm, 10,
    nativeretriever.WithQueryRewrite(),
    nativeretriever.WithFusion(5),
    nativeretriever.WithParentDoc(nil),  // 自动使用 DocStore 创建
)

// 自动生成的 Pipeline：
// [Pre] Rewrite → Decompose
// [Retrieval] MultiSearch
// [Post] RRF → ParentDoc → Generate
```

### 完整高级 RAG
```go
retriever := nativeretriever.NewRetriever(
    vectorStore, embedder, llm, 10,
    nativeretriever.WithQueryRewrite(),
    nativeretriever.WithHyDE(),
    nativeretriever.WithFusion(5),
    nativeretriever.WithParentDoc(nil),
    nativeretriever.WithSentenceWindow(nil),
    nativeretriever.WithContextPrune(nil),
    nativeretriever.WithRerank(),
)

// 自动生成的 Pipeline：
// [Pre] QueryRewrite → HyDE → Decompose
// [Retrieval] MultiSearch
// [Post] RRF → ParentDoc → SentenceWindow → Prune → Rerank → Generate
```

### 带缓存的高级 RAG
```go
cache := memory.NewSemanticCache(embedder, 0.95)

retriever := nativeretriever.NewRetriever(
    vectorStore, embedder, llm, 10,
    nativeretriever.WithCache(cache),
    nativeretriever.WithQueryRewrite(),
    nativeretriever.WithFusion(5),
)

// 自动生成的 Pipeline：
// [Cache] Check
// [Pre] Rewrite → Decompose
// [Retrieval] MultiSearch
// [Post] RRF → Generate
// [Cache] Store
```

### 通过 pattern 包使用（推荐）
```go
import "github.com/DotNetAge/gorag/pkg/pattern"

// 基础 RAG
rag, _ := pattern.NativeRAG("myapp", pattern.WithBGE("bge-small-zh-v1.5"))

// 高级 RAG - 随意组合 options
rag, _ := pattern.NativeRAG("myapp",
    pattern.WithBGE("bge-small-zh-v1.5"),
    pattern.WithLLM(myLLMClient),
    pattern.WithQueryRewrite(),
    pattern.WithFusion(5),
    pattern.WithParentDoc(),
    pattern.WithSentenceWindow(),
    pattern.WithContextPrune(),
)
```

## 可用选项

### 基础配置
- `WithName(name string)` - 设置实例名称
- `WithTopK(k int)` - 设置检索结果数量
- `WithVectorStore(s core.VectorStore)` - 设置向量存储
- `WithDocStore(s core.DocStore)` - 设置文档存储
- `WithEmbedder(e embedding.Provider)` - 设置嵌入模型
- `WithLLM(l chat.Client)` - 设置 LLM 客户端
- `WithLogger(l logging.Logger)` - 设置日志器
- `WithTracer(t observability.Tracer)` - 设置追踪器

### Pre-Retrieval 增强
- `WithQueryRewrite()` - 重写/澄清查询
- `WithHyDE()` - 生成假设性文档
- `WithStepBack()` - 生成后退一步问题
- `WithFusion(count int)` - 多查询融合（同时也涉及 Post-Retrieval）

### Post-Retrieval 增强
- `WithParentDoc(expander core.ResultEnhancer)` - 父文档扩展（需要 DocStore）
- `WithSentenceWindow(expander core.ResultEnhancer)` - 句子窗口扩展
- `WithContextPrune(pruner core.ResultEnhancer)` - 上下文裁剪（需要 LLM）
- `WithRerank()` - 重排序结果

### Cache
- `WithCache(cache core.SemanticCache)` - 语义缓存

## 组件依赖关系

| 选项 | 依赖 | 自动创建 |
|------|------|----------|
| `WithParentDoc()` | `DocStore` | ✅ 如果提供了 DocStore |
| `WithSentenceWindow()` | 无 | ✅ 总是自动创建 |
| `WithContextPrune()` | `LLM` | ✅ 如果提供了 LLM |
| `WithCache()` | `SemanticCache` | ❌ 需要用户提供 |

## 设计原则

1. **用户友好** - 用户随意组合 options，无需了解执行顺序
2. **自动装配** - Retriever 内部按三阶段锁死顺序组装
3. **组合灵活** - 支持任意选项组合
4. **渐进增强** - 从基础模式开始，按需添加增强功能
5. **智能依赖** - 自动检测依赖并创建必要组件
