# Knowledge Graph Extractor - 知识图谱提取

## 什么是知识图谱？

知识图谱是一种用**图结构**表示知识的技术。它由**实体（Nodes）**和**关系（Edges）**组成，能够捕捉文本中概念之间的关联信息。

### 核心概念

```
实体（Node）:  表示一个具体的事物
  - 人物: Alice, Bob
  - 组织: Google, MIT
  - 概念: AI, 机器学习

关系（Edge）:  表示实体之间的关联
  - "Alice WORKS_AT Google"
  - "Bob KNOWS Alice"
  - "Google ACQUIRED DeepMind"
```

### 知识图谱结构

```
    [Alice] ──KNOWS──> [Bob]
      │                 │
      │                 │
   WORKS_AT          WORKS_AT
      │                 │
      ↓                 ↓
   [Google] <──ACQUIRED── [DeepMind]
```

---

## 有什么作用？

1. **结构化知识表示**：将非结构化文本转化为可推理的图结构
2. **关系推理**：支持查询实体间的多跳关系
3. **增强检索**：通过图结构发现语义相关但字面不同的内容
4. **可解释性**：清晰地展示知识之间的关联路径

---

## 怎么工作的？

### LLM 解析流程

```
输入文本 "Alice 在 Google 工作，她认识 Bob"
        ↓
    [GraphExtractor]
        ↓
    构建 Prompt（指导 LLM 提取实体和关系）
        ↓
    调用 LLM 解析
        ↓
    解析 JSON 响应
        ↓
    提取 Nodes: [{id:"Alice",type:"PERSON"}, {id:"Google",type:"ORG"}]
    提取 Edges: [{source:"Alice",target:"Google",type:"WORKS_AT"}]
        ↓
    附加元数据（source_chunk_id）
        ↓
    返回节点和边列表
```

### 提取结果示例

输入文本：
```
"Alice 在 Google 工作，她认识 Bob。Bob 是 MIT 的学生。"
```

LLM 返回的 JSON：
```json
{
  "nodes": [
    {"id": "Alice", "type": "PERSON"},
    {"id": "Google", "type": "ORGANIZATION"},
    {"id": "Bob", "type": "PERSON"},
    {"id": "MIT", "type": "ORGANIZATION"}
  ],
  "edges": [
    {"source": "Alice", "target": "Google", "type": "WORKS_AT"},
    {"source": "Alice", "target": "Bob", "type": "KNOWS"},
    {"source": "Bob", "target": "MIT", "type": "STUDIES_AT"}
  ]
}
```

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/graph/
├── extractor.go      # GraphExtractor 实现
└── graph_test.go     # 单元测试
```

### 核心类型

#### 1. Node（节点/实体）

```go
type Node struct {
    ID         string         // 实体唯一标识
    Type       string         // 实体类型：PERSON, ORGANIZATION, CONCEPT 等
    Properties map[string]any // 附加属性
}
```

#### 2. Edge（边/关系）

```go
type Edge struct {
    Source     string         // 源实体 ID
    Target     string         // 目标实体 ID
    Type       string         // 关系类型：KNOWS, WORKS_AT 等
    Properties map[string]any // 附加属性
}
```

#### 3. GraphExtractor（图提取器）

```go
type GraphExtractor struct {
    llm chat.Client  // LLM 客户端
}

// 创建提取器
extractor := graph.NewGraphExtractor(llmClient)

// 从文本块提取实体和关系
nodes, edges, err := extractor.Extract(ctx, chunk)
```

### 关键特性

- **自动 Markdown 清理**：能正确处理 LLM 返回的 ```json 代码块格式
- **元数据传播**：自动将 source_chunk_id 附加到所有节点和边，便于溯源
- **零外部依赖**：仅依赖 gochat 的 Chat 接口

---

## 如何与项目集成？

### 方式一：创建 GraphExtractor

```go
import "github.com/DotNetAge/gorag/pkg/retrieval/graph"

// 创建 LLM 客户端（使用 gochat）
llmClient := gochat.NewOpenAIChatClient(apiKey)

// 创建图提取器
extractor := graph.NewGraphExtractor(llmClient)

// 提取实体和关系
chunk := &core.Chunk{
    ID:      "chunk-001",
    Content: "Alice 在 Google 工作，她认识 Bob",
}

nodes, edges, err := extractor.Extract(ctx, chunk)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("提取到 %d 个实体, %d 个关系\n", len(nodes), len(edges))
```

### 方式二：配合 Graph Retriever 使用

```go
// 构建知识图谱存储
graphStore := graph.NewGraphStore()

// 将提取的实体和关系存入图谱
for _, node := range nodes {
    graphStore.AddNode(node)
}
for _, edge := range edges {
    graphStore.AddEdge(edge)
}

// 基于实体查询
results := graphStore.Search(ctx, "Alice")
```

### 方式三：在 Pipeline 中集成

```go
p := pipeline.New[*core.RetrievalContext]()

// 添加图谱提取步骤
p.AddStep(graph.Extract(myLLMClient, logger))

// 添加图谱存储步骤
p.AddStep(graph.Store(graphStore, logger))

// 添加生成步骤
p.AddStep(generate.New(llm, logger))
```

---

## 使用效果

### 提取能力示例

| 输入文本 | 提取的实体 | 提取的关系 |
|----------|-----------|-----------|
| "马斯克是特斯拉 CEO" | 马斯克(PERSON), 特斯拉(ORG) | 马斯克-CEO-特斯拉 |
| "苹果收购了 Beats" | 苹果(ORG), Beats(ORG) | 苹果-ACQUIRED-Beats |
| "GPT-4 是 OpenAI 开发的大语言模型" | GPT-4(CONCEPT), OpenAI(ORG), 大语言模型(CONCEPT) | GPT-4-DEVELOPED_BY-OpenAI, GPT-4-IS_A-大语言模型 |

### 元数据追踪

每个提取的节点和边都会自动附加 `source_chunk_id` 属性，便于追踪来源：

```go
nodes, _, _ := extractor.Extract(ctx, chunk)
// nodes[0].Properties["source_chunk_id"] == "chunk-001"
```

---

## 适用于哪些场景？

### ✅ 适合使用

- **文档分析**：从长文本中提取实体关系构建知识图谱
- **问答系统**：基于知识图谱进行多跳推理问答
- **关系挖掘**：发现文本中隐含的实体关联
- **知识管理**：将非结构化文档转化为可查询的图结构

### ❌ 不适合使用

- **简单检索**：仅需要向量相似度匹配
- **实时性要求高**：LLM 调用有延迟
- **资源受限**：LLM 调用成本较高
- **结构化数据**：已有明确定义的关系数据库

---

## 测试覆盖

| 测试用例 | 描述 |
|---------|------|
| `TestNewGraphExtractor` | 测试 NewGraphExtractor 创建 |
| `TestDefaultGraphExtractor` | 测试 DefaultGraphExtractor 创建 |
| `TestExtract_Success` | 测试成功提取实体和关系 |
| `TestExtract_EmptyChunk` | 测试空文本块处理 |
| `TestExtract_LLMError` | 测试 LLM 调用错误处理 |
| `TestExtract_InvalidJSON` | 测试无效 JSON 解析 |
| `TestExtract_WithMarkdown` | 测试 Markdown 格式清理 |
| `TestExtract_ChunkMetadataPropagation` | 测试节点元数据传播 |
| `TestExtractResult_Structure` | 测试提取结果结构 |
| `TestExtract_MultipleNodes` | 测试多实体多关系提取 |
| `TestExtract_EdgeMetadataPropagation` | 测试边元数据传播 |
| `TestExtract_NilPropertiesInitialized` | 测试 nil 属性初始化 |

**12 个测试全部通过**
