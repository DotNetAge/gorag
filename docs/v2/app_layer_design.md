# 应用层设计：.rag 文件模型与 LLM 上浮

> 状态：设计定稿（基于当前代码验证）
> 创建：2026-07-20
> 更新：2026-07-21（匹配 hyper_indexer 重构后的代码状态）
> 关联：[indexer_refactor.md](./indexer_refactor.md)、[README.md](./README.md)、[HyperIndexer的定位.md](./HyperIndexer的定位.md)

---

## 1. 设计目标

V2 重构完成后，剩余两个根本性问题：

1. **LLM 实例化位置错误**：LLM 客户端是应用层依赖（像数据库连接池），不应在 indexer 包内部创建，否则导致配置污染、测试无法注入 mock、一个 RAG 库绑定多个 LLM 困难。LLM 组件通过 `llm.Summarizer` / `llm.Refiller` 接口在应用层创建并注入 HyperIndexer。
2. **存储模型认知混乱**：当前 `dataDir` 是普通目录，用户面对 `gorag.Open("./my-rag")` 缺乏"这是一个完整库"的心智锚点。

本文档定义两个协同设计：

- **.rag 文件模型**：把 RAG 库从"目录"升级为"文件"心智
- **LLM 上浮到应用层**：LLM 客户端创建职责由应用层（CLI）完成

---

## 2. .rag 文件模型

**核心契约**：`.rag` 后缀是用户心智中的"一个文件"——实现上是目录，但用户认知为不可分割的整体。用户通过 `grag` 命令交互，不直接操作内部文件。

### 2.1 物理结构

```
myrag.rag/
├── config.yml                # 配置（应用层读取，非敏感信息）
├── .api_key                  # API Key
├── .ragignore                # git 忽略规则（grag init 自动生成）
├── .lock                     # 文件锁（flock，防止多进程同时写入）
├── meta.db                   # 元数据 SQLite 数据库（增量索引 + 灾难恢复）
├── vectors/                  # 向量数据库
│   └── myrag.db
├── graphs/                   # 图数据库
│   └── myrag.db
└── logs/                     # 日志目录
    └── gorag.log
```

`.api_key` 独立存放，权限 600，不写入 config.yml，自动加入 `.ragignore`。

### 2.2 路径校验契约

**CLI 层约定**：`.rag` 文件必须在当前工作目录，命令不需要也无法指定 `.rag` 路径。

```bash
$ cd ~/projects/mydocs
$ grag init                    # 在当前目录创建 ./mydocs.rag/
$ grag index                   # 索引当前目录（自动排除 .rag 目录）
$ grag index ./subdir/         # 索引当前目录下的指定子目录
$ grag query "搜索内容"         # 查询当前目录的 .rag 库
```

**API 层契约**：`gorag.Open(path)` 仍然校验 `.rag` 后缀（供库形式调用）：

```go
func Open(path string, opts ...RAGOption) (indexer.Indexer, error) {
    if !strings.HasSuffix(path, ".rag") {
        return nil, fmt.Errorf("路径必须以 .rag 结尾（RAG 库文件）: %s", path)
    }
    // ...
}
```

**CLI 自动检测规则**：
- 当前工作目录下查找唯一的 `*.rag` 子目录
- 找不到则报错："当前目录下未找到 .rag 库，请先运行 `grag init`"
- 找到多个则报错："当前目录有多个 .rag 库，请进入具体目录运行"
- **不向上查找**（简化心智模型，避免歧义）

**索引排除规则**：`grag index` 默认排除 `.rag` 后缀目录，避免索引库索引自己。

---

## 3. CLI 命令设计

### 3.1 命令清单

```
grag init                     # 在当前目录创建 ./<basename>.rag
grag index [dest_path]        # 索引当前目录或指定目录（.rag 必须在当前目录）
grag query <text>             # 查询当前目录的 .rag 库
grag info                     # 查看当前 .rag 库信息
grag config <group> ...       # 配置管理（见 §3.3）
grag watch [dest_path]        # 监控目录变化同步到 .rag
grag doctor                   # 诊断配置完整性，引导补全缺失项
grag doctor --reindex         # 从向量库重建 meta.db（灾难恢复）
grag update <path>            # 多文件实体关系发现（跨文件合并实体边）
grag logs                     # 输出当前 .rag 库的日志
```

### 3.2 命令语义

**`grag init`**：无参数，在当前目录创建 `./<basename>.rag`，basename 取当前目录名。

**`grag index [dest_path]`**：
- 无参数：索引当前目录的所有文件（自动排除 `.rag` 子目录）
- 有参数：索引指定的文件或目录（相对于当前目录或绝对路径）
- `.rag` 文件必须在当前目录（不在则报错）

```bash
$ grag index                   # 索引当前目录
$ grag index ./docs/           # 索引当前目录下的 docs/ 子目录
$ grag index /abs/path/file.go # 索引指定绝对路径文件
$ grag index ./docs ./extras   # 索引多个目录
```

**`grag query <text>`**：执行混合检索。
- 先查语义（向量库，权重 0.8）
- 再查节点（图库，权重 0.2）
- 融合返回结果

**`grag doctor`**：诊断当前 .rag 库的配置完整性，引导用户补全缺失项。

```bash
$ grag doctor
# ✓ config.yml 已存在
# ✗ embedding.model_file 未设置 → 运行：grag config embedder <path>
# ✗ llm.model 未设置 → 运行：grag config llm Model <name>
# ✓ .api_key 已设置
# ✓ vectors/ 目录存在
# ✓ graphs/ 目录存在
```

**`grag doctor --reindex`**：从向量库重建 meta.db（灾难恢复）。
- 场景：meta.db 损坏或丢失，但 vectors/ 和 graphs/ 仍然完好
- 流程：扫描向量库中的所有 chunk → 从 `chunk.DocID` 反推文档列表 → 重建 documents 表
- 注意：ContentHash 字段无法恢复（需要重新计算），重建后所有文档视为"未变更"，下次 `grag index` 会重新计算 hash

**`grag logs`**：输出当前 .rag 库的日志。

**`grag update <path>`**：对已索引的目录或文件触发多文件实体关系发现。调用 HyperIndexer.Update()，读取已有 Chunks，通过 Refiller 跨文件合并实体节点和关系边。用户手动触发，不自动执行。

```bash
$ grag logs                    # 输出全部日志
$ grag logs --tail 50          # 输出最后 50 行
$ grag logs --follow           # 实时跟踪日志输出
```

### 3.3 `grag config` 分组配置

**`grag config` 采用分组式设计**，最常见的两类配置有专用语法：

**LLM 配置**（BaseURL / APIKey / Model）

```bash
# 单字段设置
$ grag config llm BaseURL https://api.openai.com/v1
$ grag config llm Model gpt-4o-mini

# APIKey 单独处理（不写入 config.yml，见 §5.2）
$ grag config llm APIKey sk-xxx
# → 写入 .rag/.api_key 文件（权限 600，自动加入 .ragignore）

# 查看当前 LLM 配置
$ grag config llm
# BaseURL: https://api.openai.com/v1
# Model:   gpt-4o-mini
# APIKey:  ******（已设置，存储于 .rag/.api_key）
```

**Embedder 配置**（模型路径）

```bash
# 设置 embedder 模型路径
$ grag config embedder ~/.embeddings/Xenova/bge-base-zh-v1.5/onnx/model.onnx

# 查看 embedder 配置
$ grag config embedder
# ModelFile: ~/.embeddings/Xenova/bge-base-zh-v1.5/onnx/model.onnx
# Dimension: 512
```

未设置 embedder 时 `grag index` 报错，`grag doctor` 会检测。

**通用配置**（其它键值对）

```bash
$ grag config set indexer.type hyper   # 已废弃，仅为兼容保留
$ grag config get indexer.type
$ grag config list                  # 列出全部配置
```

### 3.4 全局参数

- `--json`：切换机器可读输出

---

## 4. LLM 上浮到应用层（基于 gochat）

### 4.1 设计原则

**核心约束**（基于 [HyperIndexer的定位.md](./HyperIndexer的定位.md) 最终设计）：
- `AddFile` 内置全流程（读取→分块→语义向量化→关系结构化），调用方一行完成索引
- HyperIndexer 是双线编排枢纽，持有 Summarizer / Refiller / Schema 注册表 / 事件 Hook

**LLM 注入路径**：
```
应用层创建 gochat 客户端
        ↓
注入到 llm.NewSummarizer / llm.NewRefiller（llm 包）
        ↓
Summarizer / Refiller 注入到 HyperIndexer（indexer 包）
        ↓
HyperIndexer.AddFile 内部按需调用 Summarizer 和 Refiller
```

### 4.2 llm 包内的 Summarizer 与 Refiller

**Summarizer**（[summarizer.go](file:///Users/ray/workspaces/ai-ecosystem/gorag/llm/summarizer.go)）：
- 对文档类分片的 Title 和 Summary 进行 LLM 增强
- 支持逐分片模式（`Summarize`）和批量模式（`SummarizeBatch`）
- 批量模式将分片数组序列化为 JSON 数组字符串，一次 LLM 请求完成所有分片的摘要生成
- 仅当 LLM 返回合法 title/summary 时才覆盖原值，失败时保留原分片

```go
type Summarizer interface {
    Summarize(ctx context.Context, chunks []core.Chunk) ([]core.Chunk, error)
}
```

**Refiller**（[refiller.go](file:///Users/ray/workspaces/ai-ecosystem/gorag/llm/refiller.go)）：
- 基于预分块文本，结合 EntitySchema 让 LLM 提取实体和关系
- 将 Chunk 序列化为 JSON 数组，输出实体和关系后回填为 Node/Edge
- 不删除或修改已有的 Nodes/Edges，仅做追加

```go
type Refiller interface {
    Refill(ctx context.Context, result chunker.ChunkResult, schemas []EntitySchema) (chunker.ChunkResult, error)
}
```

两个组件均基于 gochat 的 `chat.Client` 实现。

### 4.3 应用层实例化与注入

```go
// cmd/main.go（应用层）
func runQuery(text string) error {
    // 1. 找到当前目录的 .rag 库
    ragDir, err := findRAGInCWD()
    if err != nil { return err }

    // 2. 加载配置
    cfg := loadConfig(ragDir)

    // 3. 应用层创建 gochat 客户端并注入 Summarizer
    apiKey := resolveAPIKey(ragDir)
    chatClient, err := openai.NewOpenAI(apiKey, cfg.LLM.BaseURL, cfg.LLM.Model)
    if err != nil { return fmt.Errorf("创建 LLM 客户端失败: %w", err) }

    // 4. 创建 Summarizer 并注入 HyperIndexer
    summarizer, _ := llm.NewSummarizer(llm.Config{
        BaseURL:  cfg.LLM.BaseURL,
        Model:    cfg.LLM.Model,
        Language: cfg.LLM.Language,
    }, logger)

    idx, err := gorag.Open(ragDir,
        gorag.WithHyperSummarizer(summarizer),
    )
    if err != nil { return err }
    defer idx.Close(context.Background())

    // 5. 执行查询...
    return nil
}
```

### 4.4 gorag.Open 内部构造

`gorag.Open` 不涉及 LLM 客户端的实例化。LLM 组件（Summarizer / Refiller / Schema）通过 HyperIndexer 的选项在应用层注入：

```go
// gorag 包内部简化（main.go createIndexer）
func createIndexer(ragDir string, cfg *Config) (indexer.Indexer, error) {
    // 1. 创建 embedder 和向量库（必选）
    clip, err := embedder.NewChineseClipEmbedder(...)
    vectorStore, err := createVectorDB(ragDir, clip)

    // 2. 创建 SemanticIndexer（不含 Summarizer）
    semantic, _ := indexer.NewSemanticIndexer(vectorStore, clip)

    // 3. 创建 GraphIndexer（仅 GraphStore，不含 Extractor）
    graphStore, _ := createGraphDB(ragDir)
    graph, _ := indexer.New(graphStore)

    // 4. 组合为 HyperIndexer（semantic 必传，graph 可选）
    return indexer.NewHyperIndexer(semantic, graph)
}
```

HyperIndexer 是应用层配置的入口点，通过 `WithHyperSummarizer`、`WithHyperRefiller`、`WithHooks`、`AddSchemas` 注入 LLM 组件。

### 4.5 GraphIndexer 职责：仅图存储

```go
// 当前 GraphIndexer 签名（indexer.New）
func New(graphStore core.GraphStore, opts ...GraphOption) (Indexer, error)
```

GraphIndexer 的职责：

- 关系检索（GraphSearcher 接口）

### 4.6 Schema 注册机制

HyperIndexer 维护一个 `schemasByPath` 注册表，用于关联外部实体 Schema 定义与索引路径。

**Schema 定义**（[schema.go](file:///Users/ray/workspaces/ai-ecosystem/gorag/llm/schema.go)）：

```go
type EntitySchema struct {
    Type       string   // 实体类型（作为 core.Node.Labels[0]）
    Prompt     string   // LLM 提示词中的实体说明
    JSONSchema string   // 可选的 JSON Schema 约束（压缩为单行）
}
```

**Schema 加载**（[schema_loader.go](file:///Users/ray/workspaces/ai-ecosystem/gorag/llm/schema_loader.go)）：
- `LoadEntitySchema(path)` — 从单个 JSON Schema 文件加载
- `LoadEntitySchemasFromDir(dir)` — 扫描目录下所有 .json 文件批量加载
- 内置默认 Schema 按文档类型分类：[CodeEntitySchemas](file:///Users/ray/workspaces/ai-ecosystem/gorag/llm/schema.go#L22-L29)、[DocumentEntitySchemas](file:///Users/ray/workspaces/ai-ecosystem/gorag/llm/schema.go#L31-L39)、[DataEntitySchemas](file:///Users/ray/workspaces/ai-ecosystem/gorag/llm/schema.go#L41-L47)

**注入方式**（[hyper.go](file:///Users/ray/workspaces/ai-ecosystem/gorag/indexer/hyper.go#L651-L660)）：

```go
// 注册外部 Schema，path 是源目录路径
hyper.AddSchemas("schemas/general", schemas)
```

**消费链路**：Schema → Refiller.Refill() → LLM 提取实体和关系 → 回填 Nodes/Edges → GraphStore

**失败处理**：Refiller 返回 error 时阻塞管线，调用方需处理。

### 4.7 事件扩展点（Event Hooks）

HyperIndexer 的索引管线在关键步骤设置事件扩展点（[hooks.go](file:///Users/ray/workspaces/ai-ecosystem/gorag/indexer/hooks.go)），分为**修改型**和**通知型**两类：

| Hook                       | 类型   | 触发时机                            | 典型用途                 |
| -------------------------- | ------ | ----------------------------------- | ------------------------ |
| `OnFileOpenedHook`         | 修改型 | `document.Open` 之后、Chunker 之前  | 文件类型白名单、前置过滤 |
| `OnChunkHook`              | 修改型 | Chunker 产出每个 Chunk 后           | 敏感词过滤、补充标签     |
| `OnBeforeSemanticSaveHook` | 修改型 | Summarizer 之后、semantic.Save 之前 | 批量审核、外部 API 增强  |
| `OnIndexCompleteHook`      | 通知型 | AddFile 所有步骤完成后              | 通知下游、审计日志       |

**注册方式**（[hyper.go](file:///Users/ray/workspaces/ai-ecosystem/gorag/indexer/hyper.go#L108-L114)）：

```go
hyper, _ := indexer.NewHyperIndexer(semantic, graph,
    indexer.WithHooks(
        myFileFilterHook,    // 实现 OnFileOpenedHook
        mySensitiveFilter,   // 实现 OnChunkHook
        myAuditHook,         // 实现 OnIndexCompleteHook
    ),
)
```

---

## 5. 配置分层与安全

### 5.1 config.yml 分层结构

```yaml
version: 1

storage:
  vectors_dir: vectors
  graphs_dir: graphs
  logs_dir: logs
  meta_db: meta.db

embedding:
  model_file: ~/.embeddings/Xenova/bge-base-zh-v1.5/onnx/model.onnx
  dimension: 512

llm:
  base_url: https://api.openai.com/v1
  model: gpt-4o-mini
  language: Chinese
  max_tokens: 128000
  context_length: 128000
  thinking_budget: 0

indexer:
  type: hyper  # 保留仅用于兼容，CLI 固定使用 hyper

query:
  semantic_weight: 0.8
  graph_weight: 0.2
```

### 5.2 API Key 存储（不进 config.yml）

**`grag config llm APIKey <key>` 写入位置**：`.rag/.api_key` 文件

**关键安全约束**：
- 文件权限强制 `chmod 600`
- 自动加入 `.ragignore`
- 启动时检查权限，过宽则警告

**API Key 四级回退**：

```go
func resolveAPIKey(ragDir string) (string, error) {
    // 1. 环境变量 GORAG_API_KEY
    if k := os.Getenv("GORAG_API_KEY"); k != "" {
        return k, nil
    }
    // 2. .rag/.api_key 文件
    apiKeyFile := filepath.Join(ragDir, ".api_key")
    if data, err := os.ReadFile(apiKeyFile); err == nil {
        return strings.TrimSpace(string(data)), nil
    }
    // 3. 外部文件引用（cfg.LLM.APIKeyFile）
    if cfg.LLM.APIKeyFile != "" {
        return os.ReadFile(cfg.LLM.APIKeyFile)
    }
    // 4. 系统 keychain（macOS）
    return keychain.Get("gorag-api-key")
}
```

**`.ragignore` 默认内容**（`grag init` 自动生成）：

```
.api_key
.lock
vectors/
graphs/
meta.db
meta.db-wal
meta.db-shm
logs/
```

---

## 6. 架构层次

```
┌─────────────────────────────────────────────────┐
│  应用层（cmd/main.go）                           │
│  - 读取 myrag.rag/config.yml                    │
│  - 实例化 gochat 客户端                           │
│  - 解析 API Key（env > file > keychain）         │
│  - 创建 llm.Summarizer / llm.Refiller           │
│  - 调用 gorag.Open + 注入 HyperIndexer 选项      │
└──────────────────┬──────────────────────────────┘
                   │ 注入 Summarizer / Refiller / Hooks / Schemas
┌──────────────────▼──────────────────────────────┐
│  RAG 库层（gorag 包）                            │
│  - .rag 文件 = 存储容器（强制后缀校验）           │
│  - 构造 SemanticIndexer（无 Summarizer）          │
│  - 构造 GraphIndexer（无 Extractor）             │
│  - 构造 HyperIndexer 编排双线                    │
│  - 返回 Indexer 接口                             │
└──────────────────┬──────────────────────────────┘
                   │ 语义线 Save / 关系线 Save
┌──────────────────▼──────────────────────────────┐
│  Indexer 层（indexer 包）                        │
│  - HyperIndexer 持有 Summarizer/Refiller/Schemas │
│  - HyperIndexer.AddFile 完整管线：               │
│    读文件→Hook→分块→Hook→摘要→Hook→语义Save→    │
│    实体兜底→关系线Save→Hook                     │
│  - SemanticIndexer：分块+多维度向量化            │
│  - GraphIndexer：实体+CONTAINS 边+图存储         │
│  - LLM 调用封装在 Summarizer/Refiller 内部       │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│  元数据层（store/meta 包）                       │
│  - SQLite 存储文档元数据 + ContentHash           │
│  - 增量索引：跳过未变更文件                       │
│  - 失败状态持久化：记录错误信息                   │
│  - IndexingService 持有 MetaStore                │
└─────────────────────────────────────────────────┘
```

---

## 7. 元数据 SQLite 改造

### 7.1 现状问题

当前 `indexedFiles` + `history/doc_index.json` 存在以下缺陷：
- 只存 Chunks ID 列表 + 时间戳，无 ContentHash，无法判断文件是否变更
- 无法记录失败状态和错误信息
- JSON 文件并发写需锁，性能差
- 无法按路径/状态/时间查询

### 7.2 SQLite 表设计

```sql
-- 文档元数据表（唯一核心表）
CREATE TABLE documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    absolute_path TEXT UNIQUE NOT NULL,
    file_name TEXT NOT NULL,
    extension TEXT,
    size_bytes INTEGER,
    modified_at TIMESTAMP,
    content_hash TEXT,
    status TEXT NOT NULL,
    chunk_ids JSON,
    indexed_at TIMESTAMP,
    error_message TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_documents_hash ON documents(content_hash);
```

**设计要点**：
- `chunk_ids` 用 JSON 数组存储
- 删除文档时遍历 `chunk_ids` 调用 `indexer.Remove(chunkID)`
- 失败状态记录 `error_message`
- WAL 模式支持并发读 + 单写

### 7.3 MetaStore 接口

新建 `gorag/store/meta/` 包：

```go
package meta

type Store interface {
    SaveDocument(doc *Document) error
    GetDocumentByPath(absPath string) (*Document, error)
    ListDocuments(status string) ([]*Document, error)
    DeleteDocument(absPath string) error
    Close() error
}

type Document struct {
    ID            int64
    AbsolutePath  string
    FileName      string
    Extension     string
    SizeBytes     int64
    ModifiedAt    time.Time
    ContentHash   string
    Status        string
    ChunkIDs      []string
    IndexedAt     *time.Time
    ErrorMessage  string
    UpdatedAt     time.Time
}
```

### 7.4 整合到 IndexingService

```go
type IndexingService struct {
    dataDir      string
    watchs       []string
    metaStore    meta.Store
    indexer      indexer.Indexer
    logger       logging.Logger
    workerCount  int
    ctx          context.Context
    cancel       context.CancelFunc
    wg           sync.WaitGroup
}

func (s *IndexingService) processFile(ctx context.Context, absPath string) error {
    info, err := os.Stat(absPath)
    if err != nil { return err }

    // 1. 两级预检：mtime + size 快速判断，不匹配才计算 hash
    existing, _ := s.metaStore.GetDocumentByPath(absPath)
    if existing != nil && existing.Status == "indexed" {
        if existing.ModifiedAt.Equal(info.ModTime()) && existing.SizeBytes == info.Size() {
            s.logger.Info("文件未变更（mtime+size 命中），跳过", "path", absPath)
            return nil
        }
    }

    // 2. 计算内容 hash（仅 mtime/size 变化时计算）
    contentHash, err := computeFileHash(absPath)
    if err != nil { return err }
    if existing != nil && existing.ContentHash == contentHash && existing.Status == "indexed" {
        s.logger.Info("文件未变更（hash 命中），跳过", "path", absPath)
        return nil
    }

    // 3. 先清理旧数据
    if existing != nil && len(existing.ChunkIDs) > 0 {
        if err := s.removeFileIndex(ctx, absPath); err != nil {
            s.logger.Warn("清理旧索引失败，继续写入新数据", "path", absPath, "err", err)
        }
    }

    // 4. 调用 indexer.AddFile 写入新数据
    chunks, err := s.indexer.AddFile(ctx, absPath)
    if err != nil {
        _ = s.metaStore.SaveDocument(&meta.Document{
            AbsolutePath: absPath,
            ContentHash:  contentHash,
            Status:       "failed",
            ErrorMessage: err.Error(),
        })
        return err
    }

    // 5. 成功状态写入 meta.db
    chunkIDs := make([]string, len(chunks))
    for i, c := range chunks { chunkIDs[i] = c.ID }
    return s.metaStore.SaveDocument(&meta.Document{
        AbsolutePath: absPath,
        ContentHash:  contentHash,
        Status:       "indexed",
        ChunkIDs:     chunkIDs,
        IndexedAt:    time.Now(),
    })
}
```

### 7.5 删除文件清理逻辑（原子性保障）

```go
func (s *IndexingService) removeFileIndex(ctx context.Context, absPath string) error {
    doc, err := s.metaStore.GetDocumentByPath(absPath)
    if err != nil || doc == nil { return err }

    var failedChunks []string
    if admin, ok := s.indexer.(indexer.IndexerAdmin); ok {
        for _, chunkID := range doc.ChunkIDs {
            if err := admin.Remove(ctx, chunkID); err != nil {
                s.logger.Error("删除 chunk 失败", err, "chunk_id", chunkID)
                failedChunks = append(failedChunks, chunkID)
            }
        }
    }

    if len(failedChunks) > 0 {
        return s.metaStore.SaveDocument(&meta.Document{
            AbsolutePath: doc.AbsolutePath,
            ContentHash:  doc.ContentHash,
            Status:       "partial_deleted",
            ChunkIDs:     failedChunks,
            ErrorMessage: fmt.Sprintf("部分 chunk 删除失败：%d/%d", len(failedChunks), len(doc.ChunkIDs)),
        })
    }

    return s.metaStore.DeleteDocument(absPath)
}
```

---

## 8. 查询路径设计

### 8.1 `grag query` 执行流程

```
grag query "搜索内容"
       │
       ▼
┌─────────────────────────────────────────┐
│  1. 解析查询（core.Query 接口）          │
│     - 识别查询类型（semantic/graph/hybrid）│
│     - 计算查询向量（需 embedder）         │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  2. 语义检索（权重 0.8）                 │
│     SemanticIndexer.Search() → *Hit     │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  3. 图检索（权重 0.2）                   │
│     GraphIndexer.SearchGraph() → *Hit   │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  4. RRF 融合（语义 0.8 + 图 0.2）       │
│     - 对 Chunks/Nodes/Edges 分类融合    │
│     - 返回融合后的 *Hit                 │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  5. 输出结果                             │
│     - 默认：terminal 格式化              │
│     - --json：机器可读 JSON              │
└─────────────────────────────────────────┘
```

### 8.2 关键约束

**embedder 前置检查**：
- 未配置 embedder 时语义检索不可用，`grag query` 降级为纯图检索（权重 0:1）
- `grag doctor` 会检测并提示配置 embedder

**权重可配置**：config.yml 中 `query.semantic_weight` 和 `query.graph_weight`，默认 0.8:0.2

---

## 9. 实施计划

### 阶段 1：.rag 文件模型落地
- `gorag.Open` 强制 `.rag` 后缀校验
- `grag init` 无参数，创建 `./<basename>.rag`
- CLI 自动检测当前目录的 .rag 子目录（不向上查找）
- `grag index` 默认排除 `.rag` 子目录
- 并发控制：`.rag/.lock` 文件锁
- **不兼容旧 dataDir**

### 阶段 2：元数据 SQLite 改造
- 新建 `gorag/store/meta/` 包，定义 `Store` 接口
- 实现 SQLite 后端（单表 `documents` + WAL 模式）
- `IndexingService` 整合 `MetaStore`，替代 `indexedFiles map`
- 两级预检：mtime+size 快速判断，不匹配才计算 hash
- 先清理旧数据再写入新数据
- 删除原子性：部分失败标记 `partial_deleted`，供 `grag doctor` 重试
- 失败文档持久化：记录 error_message
- **不提供旧 doc_index.json 迁移**

### 阶段 3：多文件实体关系发现
- `HyperIndexer.Update()` 读取已索引 Chunks，通过 Refiller 跨文件合并实体节点和关系边
- `grag update <path>` CLI 命令触发，用户手动执行

### 阶段 4：清理与辅助命令
- 实现 `grag doctor`（检测配置完整性 + 重试 partial_deleted）
- 实现 `grag doctor --reindex`（从向量库重建 meta.db）
- 实现 `grag logs`（输出 `.rag/logs/` 日志）

### 验收标准
- `go build ./...` 通过
- `go vet ./...` 通过
- `go test ./... -timeout 60s` 通过
- `grag init` 在当前目录创建 `./<basename>.rag`
- `grag query "text"` 在当前目录可工作
- `grag index ./docs/` 第二次运行跳过未变更文件
- LLM 组件在应用层创建，通过 HyperIndexer 选项注入
- meta.db 中的 status 字段可正确记录 indexed / failed / partial_deleted
- `grag doctor` 能检测出缺失 embedder 配置
- `grag logs` 能输出 `.rag/logs/gorag.log`
