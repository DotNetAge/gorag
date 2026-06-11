# LLMIndexer 设计

> LLMIndexer 是 GoRAG 的第三个索引器实现，在 `semantic_indexer`（纯向量）和 `graph_indexer`（纯图谱）之外，
> 提供一种 **LLM 驱动的一次性索引方案**：文本输入 → LLM 分块+提取实体 → 同时写入 VectorStore + GraphStore。

---

## 设计原则

- **位于 GoRAG**：实现 `core.Indexer` 接口，与 `semantic_indexer`、`graph_indexer` 平级
- **一次调用**：单次 LLM 调用，不流式，不循环，不调工具
- **双写入**：LLM 解析完 JSON 后直接写入 VectorStore + GraphStore
- **不依赖 MindX**：不依赖 GoReact Runtime、Skill 加载、Daemon RPC
- **LLM 网关**：所有 LLM 调用通过 [GoChat](https://github.com/mindx/chat) 完成

---

## 接口

```go
// gorag/indexer/llm_indexer.go

// ModelConfig LLM 模型连接配置（GoRAG 自有定义，不依赖外部 config 包）
type ModelConfig struct {
    APIKey   string
    BaseURL  string
    Model    string
}

type LLMIndexer struct {
    model    ModelConfig
    embedder core.Embedder
    vectorDB core.VectorStore
    graphDB  core.GraphStore
}

func New(
    model ModelConfig,
    embedder core.Embedder,
    vectorDB core.VectorStore,
    graphDB core.GraphStore,
    opts ...Option,
) *LLMIndexer

// TokenUsage 单次 LLM 调用的 Token 消耗
// 调用方（如 MindX）可以根据此信息自行记录到 TokenUsageStore
type TokenUsage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

---

## 实现 core.Indexer

```go
// Add 对一段文本执行 LLM 索引
// 1. 调用 LLM 进行分块 + 提取实体和关系
// 2. 写入 vectorDB
// 3. 写入 graphDB
// 返回: 生成的 Chunk 列表 + 本次 LLM 调用的 Token 用量
func (idx *LLMIndexer) Add(ctx context.Context, content string) ([]*core.Chunk, *TokenUsage, error)

// AddFile 读取文件后调用 Add
func (idx *LLMIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, *TokenUsage, error)

// Search 委托给 vectorDB.Search + graphDB.GetMultiHopPaths 做混合检索
func (idx *LLMIndexer) Search(ctx context.Context, query core.Query) ([]core.Hit, error)

// 其他方法……
func (idx *LLMIndexer) Name() string                      { return "llm_indexer" }
func (idx *LLMIndexer) Type() string                      { return "llm" }
func (idx *LLMIndexer) Count(ctx context.Context) (int, error)
func (idx *LLMIndexer) Remove(ctx context.Context, chunkID string) error
func (idx *LLMIndexer) IndexChunk(ctx context.Context, chunk *core.Chunk) error
```

---

## Add 内部流程

```
LLMIndexer.Add(ctx, content)
  │
  ├── 1. 组装 System Prompt + User Prompt
  │       sys = [分块规则, 实体提取规则, JSON 输出格式说明]
  │       user = [content]
  │
  ├── 2. gochat.Client 同步调用
  │       client = gochat.NewClient(model.APIKey, model.BaseURL, model.Model)
  │       resp = client.Messages(sys..., user...).GetResponse()
  │
  ├── 3. 解析 JSON
  │       data = parseJSON[IndexData](resp.Content)
  │
  ├── 4. 写入 VectorStore
  │       for _, c := range data.Chunks {
  │           vec = embedder.CalcText(c.Content)
  │           vec.Metadata = { source, summary, ... }
  │           vectorDB.Upsert(ctx, []*Vector{vec})
  │       }
  │
  ├── 5. 写入 GraphStore
  │       graphDB.UpsertNodes(ctx, data.Entities)
  │       graphDB.UpsertEdges(ctx, data.Relations)
  │
  └── 6. 返回 (chunks, &TokenUsage, nil)
```

---

## LLM 输出 Schema

```json
{
  "chunks": [
    {
      "content": "func main() { ... }",
      "summary": "程序入口函数",
      "start_line": 10,
      "end_line": 30
    }
  ],
  "entities": [
    {
      "id": "func:main",
      "type": "Function",
      "name": "main",
      "description": "应用入口"
    }
  ],
  "relations": [
    {
      "source": "func:main",
      "target": "struct:Config",
      "type": "REFERENCES"
    }
  ]
}
```

---

## 文件结构

```
gorag/indexer/
├── llm_indexer.go        # LLMIndexer 主实现
├── llm_indexer_test.go   # 测试
└── prompt/               # 内置 Prompt 模板（可选）
    ├── chunk.txt         # 分块规则
    └── extract.txt       # 实体提取规则
```
