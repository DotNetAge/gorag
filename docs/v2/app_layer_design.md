# 应用层设计：.rag 文件模型与 LLM 上浮

> 状态：设计稿（待评审）
> 创建：2026-07-20
> 关联：[indexer_refactor.md](./indexer_refactor.md)、[README.md](./README.md)

---

## 1. 设计目标

V2 重构完成后，剩余两个根本性问题：

1. **LLM 实例化位置错误**：LLM 客户端是应用层依赖（像数据库连接池），却在 `indexer.New()` 内部创建，导致 `indexer.ModelConfig` 污染 indexer 包、测试无法注入 mock、一个 RAG 库绑定多个 LLM 困难。
2. **存储模型认知混乱**：当前 `dataDir` 是普通目录，用户面对 `gorag.Open("./my-rag")` 缺乏"这是一个完整库"的心智锚点。

本文档定义两个协同设计：

- **.rag 文件模型**：把 RAG 库从"目录"升级为"文件"心智
- **LLM 上浮到应用层**：把 LLM 客户端创建职责从 indexer 包上浮到 CLI 应用层

---

## 2. .rag 文件模型

### 2.1 心智模型优先

**核心契约**：`.rag` 后缀是用户心智中的"一个文件"——实现上是目录，但用户认知为不可分割的整体。

类比：

| 类比对象               | 用户认知            | 实现层         |
| ---------------------- | ------------------- | -------------- |
| `.docx` / `.xlsx`      | 一个文档            | zip 压缩的目录 |
| `.app`（macOS Bundle） | 一个应用            | 目录           |
| `.jar` / `.war`        | 一个归档            | zip            |
| **`.rag`**             | **一个 RAG 库文件** | **目录**       |

用户不需要、也不应该直接操作 `.rag` 内部的 `config.yml` 或 `vectors/`——一切通过 `grag` 命令交互。这就像用户不会解压 `.docx` 去编辑 `word/document.xml` 一样。

### 2.2 物理结构

```
myrag.rag/                    # Finder 中可显示为单一文件（macOS Bundle）
├── Info.plist                # macOS Bundle 声明（可选）
├── config.yml                # 配置（应用层读取，非敏感信息）
├── .api_key                  # API Key（敏感，grag config llm APIKey 写入，权限 600）
├── .ragignore                # git 忽略规则（grag init 自动生成）
├── .lock                     # 文件锁（flock，防止多进程同时写入）
├── meta.db                   # 元数据 SQLite 数据库（增量索引 + 灾难恢复）
├── vectors/                  # 向量数据库
│   └── myrag.db
├── graphs/                   # 图数据库
│   └── myrag.db
├── caches/                   # 缓存（LLM 响应缓存等）
└── logs/                     # 日志目录（grag logs 输出）
    └── gorag.log
```

**存储职责清晰**：
- `vectors/` 存语义数据（chunk 向量）
- `graphs/` 存关系数据（实体 + 边）
- `meta.db` 存元数据（文档状态、索引历史、ContentHash）
- `caches/` 存真正的缓存（非持久化数据）
- `logs/` 存运行日志（`grag logs` 命令读取）

**敏感文件隔离**：
- `.api_key` 独立存放，权限 600，不写入 config.yml，自动加入 `.ragignore`

### 2.3 为什么不是真 zip

- 向量库和图库需要**随机访问**（SQLite/BoltDB 文件），zip 每次读写需重打包，性能灾难
- 数据库文件频繁变更，zip 重组成本不可接受
- Office 用 zip 是因为内容相对静态，RAG 库是**热数据**
- 真 zip 适合**导出/分享**场景，不适合**运行时**场景

### 2.4 macOS Bundle 体验（可选增强）

加 `Info.plist` 后：
- Finder 默认显示为单一图标
- 双击可触发 `grag` 打开
- 右键"显示包内容"仍可查看内部（高级用户）

Linux/Windows 上显示为目录，但 `.rag` 后缀已是充分提示。

### 2.5 路径校验契约

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

**核心约定**：`.rag` 文件必须在当前工作目录，命令不接受 `.rag` 路径参数。

```
grag init                     # 在当前目录创建 ./<basename>.rag
grag index [dest_path]        # 索引当前目录或指定目录（.rag 必须在当前目录）
grag query <text>             # 查询当前目录的 .rag 库
grag info                     # 查看当前 .rag 库信息
grag config <group> ...       # 配置管理（见 §3.3）
grag watch [dest_path]        # 监控目录变化同步到 .rag
grag doctor                   # 诊断配置完整性，引导补全缺失项
grag doctor --reindex         # 从向量库重建 meta.db（灾难恢复）
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

**`grag query <text>`**：执行混合检索（详见 §7 查询路径设计）。
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

**`grag logs`**：输出当前 .rag 库的日志（日志存储在 `.rag/logs/` 内）。

```bash
$ grag logs                    # 输出全部日志
$ grag logs --tail 50          # 输出最后 50 行
$ grag logs --follow           # 实时跟踪日志输出
```

### 3.3 `grag config` 分组配置

`grag config` 采用**分组式**设计，最常见的两类配置有专用语法：

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

**重要约束**：设置了 embedder 才能使用向量库。未设置 embedder 时：
- `grag index` 报错："未配置 embedder，请运行 `grag config embedder <path>`"
- `grag doctor` 会检测并提示

**通用配置**（其它键值对）

```bash
$ grag config set indexer.type graph
$ grag config get indexer.type
$ grag config list                  # 列出全部配置
```

### 3.4 全局参数

- `--json`：切换机器可读输出（对齐项目硬性约束"提供 --json 输出完整原始数据"）

---

## 4. LLM 上浮到应用层（基于 gochat）

### 4.1 设计原则

**核心约束**（[indexer_refactor.md](./indexer_refactor.md) §2.5、§7.1、§7.7）：
- `AddFile` 内置 LLM 全流程（读取→分块→提取→写入），调用方一行完成索引
- LLM 调用是 Indexer 的核心能力，**不剥离到应用层**
- GraphIndexer 职责单一化为"实体提取+图结构化"，**持有 Extractor 接口**（可选注入）
- SemanticIndexer 持有 Summarizer 接口（可选注入）
- HyperIndexer 编排双线：读文件→结构化→分块→分流到 SemanticIndexer + GraphIndexer

**LLM 客户端的正确注入路径**：
```
应用层创建 chat.Client
        ↓
注入到 extractor.LLMExtractor（extractor 包）
        ↓
Extractor 注入到 GraphIndexer（indexer 包）
        ↓
GraphIndexer.AddFile 内部调用 Extractor.Extract(ctx, doc)
```

### 4.2 直接引入 gochat（不自定义接口）

项目已使用 [gochat](file:///Users/ray/workspaces/ai-ecosystem/gorag/extractor/llm.go#L10-L11)（`github.com/DotNetAge/gochat`），直接复用其 `chat.Client` 类型，**不自定义 LLM 接口**（避免过度设计）。

gochat 已提供完整能力：Complete / Stream / FunctionCall / 错误分类，无需重新抽象。

### 4.3 应用层实例化与注入

```go
// cmd/main.go（应用层）
func runQuery(text string) error {
    // 1. 找到当前目录的 .rag 库
    ragDir, err := findRAGInCWD()
    if err != nil { return err }

    // 2. 加载配置
    cfg := loadConfig(ragDir)

    // 3. 应用层创建 gochat 客户端（LLM 是应用层依赖）
    apiKey := resolveAPIKey(ragDir)  // 四级回退：env > .api_key > file > keychain
    chatClient, err := openai.NewOpenAI(apiKey, cfg.LLM.BaseURL, cfg.LLM.Model)
    if err != nil { return fmt.Errorf("创建 LLM 客户端失败: %w", err) }

    // 4. 注入到 Open（gorag 包内部会构造 Extractor 并注入到 GraphIndexer）
    idx, err := gorag.Open(ragDir, gorag.WithLLM(chatClient))
    if err != nil { return err }
    defer idx.Close(context.Background())

    // 5. 执行查询...
    return nil
}
```

### 4.4 gorag.Open 内部构造 Extractor 并注入

```go
// gorag 包内部（不是应用层）
func Open(ragDir string, opts ...RAGOption) (indexer.Indexer, error) {
    // ... 加载配置 ...

    // 1. 应用层注入的 chat.Client（可选，仅 graph/hyper 类型需要）
    var chatClient chat.Client
    if opt.hasLLM {
        chatClient, err = openai.NewOpenAI(...)
        if err != nil { return nil, err }
    }

    // 2. 构造 Extractor（持有 chat.Client）
    var ext extractor.Extractor
    if chatClient != nil {
        ext, err = extractor.NewLLMExtractor(chatClient, schemas...)
        if err != nil { return nil, err }
    }

    // 3. 构造 GraphIndexer（注入 Extractor，不注入 chat.Client）
    graph, err := indexer.NewGraphIndexer(graphDB, ext, opts...)

    // 4. 构造 SemanticIndexer
    semantic, err := indexer.NewSemanticIndexer(vectorDB, embedder, opts...)

    // 5. 构造 HyperIndexer（编排双线）
    return indexer.NewHyperIndexer(chunker, semantic, graph)
}
```

### 4.5 GraphIndexer 不再持有 ModelConfig

```go
// 删除：indexer.ModelConfig 类型（待 graph.go 重构完成后）
// 删除：indexer.New(modelCfg ModelConfig, ...) 签名

// 新签名：接收 Extractor 接口（可选，nil 表示纯层级树模式）
func NewGraphIndexer(
    graphDB core.GraphStore,
    extractor extractor.Extractor,  // 可选注入
    opts ...GraphOption,
) (Indexer, error)
```

**收益**：
- GraphIndexer 不持有 LLM 客户端，符合 §7.7 职责单一化
- LLM 调用由 Extractor 承载，符合 §2.5 LLM 内置不剥离
- AddFile 一行完成索引（LLM 调用封装在 Extractor 内部）
- **只支持单个 LLM**（一个 .rag 库绑定一个 LLM，多 LLM 视为过度设计）

---

## 5. 配置分层与安全

### 5.1 config.yml 分层结构

```yaml
version: 1                   # 配置版本号（未来格式变更时用于兼容性检查）

# 存储配置（由 init 创建，一般不变）
storage:
  vectors_dir: vectors
  graphs_dir: graphs
  caches_dir: caches
  logs_dir: logs
  meta_db: meta.db

# 向量模型配置（未设置则不能使用向量库）
embedding:
  model_file: ~/.embeddings/Xenova/bge-base-zh-v1.5/onnx/model.onnx
  dimension: 512

# LLM 配置（应用层注入，只支持单个 LLM）
llm:
  base_url: https://api.openai.com/v1
  model: gpt-4o-mini
  language: Chinese
  max_tokens: 128000
  context_length: 128000
  thinking_budget: 0
  # 不存 api_key（敏感信息，存于 .rag/.api_key）

# 索引器配置
indexer:
  type: graph  # semantic | graph | hyper

# 查询配置
query:
  semantic_weight: 0.8       # 语义检索权重
  graph_weight: 0.2          # 图检索权重
```

### 5.2 API Key 存储（不进 config.yml）

**`grag config llm APIKey <key>` 写入位置**：`.rag/.api_key` 文件

**关键安全约束**：
- 文件权限强制 `chmod 600`（仅 owner 可读写）
- 自动加入 `.ragignore`，不被 git 提交
- 启动时检查权限，过宽则警告

**API Key 四级回退**：

```go
func resolveAPIKey(ragDir string) (string, error) {
    // 1. 环境变量 GORAG_API_KEY（最高优先级，CI/CD 友好）
    if k := os.Getenv("GORAG_API_KEY"); k != "" {
        return k, nil
    }
    // 2. .rag/.api_key 文件（grag config llm APIKey 写入位置）
    apiKeyFile := filepath.Join(ragDir, ".api_key")
    if data, err := os.ReadFile(apiKeyFile); err == nil {
        return strings.TrimSpace(string(data)), nil
    }
    // 3. 外部文件引用（cfg.LLM.APIKeyFile，自定义路径）
    if cfg.LLM.APIKeyFile != "" {
        return os.ReadFile(cfg.LLM.APIKeyFile)
    }
    // 4. 系统 keychain（macOS，可选）
    return keychain.Get("gorag-api-key")
}
```

**`.ragignore` 默认内容**（`grag init` 自动生成）：

```
# 敏感信息
.api_key

# 运行时锁文件
.lock

# 数据库（体积大，不入版本控制）
vectors/
graphs/
caches/
meta.db
meta.db-wal
meta.db-shm

# 日志
logs/
```

---

## 6. 架构层次

```
┌─────────────────────────────────────────────────┐
│  应用层（cmd/main.go）                           │
│  - 读取 myrag.rag/config.yml                    │
│  - 实例化 gochat 客户端（应用层依赖）             │
│  - 解析 API Key（env > file > keychain）         │
│  - 调用 gorag.Open(path, WithLLM(client))       │
└──────────────────┬──────────────────────────────┘
                   │ 注入 chat.Client
┌──────────────────▼──────────────────────────────┐
│  RAG 库层（gorag 包）                            │
│  - .rag 文件 = 存储容器（强制后缀校验）           │
│  - 构造 extractor.LLMExtractor（持有 chat.Client）│
│  - 构造 SemanticIndexer + GraphIndexer          │
│  - 构造 HyperIndexer 编排双线                    │
│  - 返回 Indexer 接口                             │
└──────────────────┬──────────────────────────────┘
                   │ 注入 Extractor / Summarizer
┌──────────────────▼──────────────────────────────┐
│  Indexer 层（indexer 包）                        │
│  - GraphIndexer 持有 Extractor（不持有 LLM）      │
│  - SemanticIndexer 持有 Summarizer（不持有 LLM）  │
│  - HyperIndexer.AddFile 一行完成索引             │
│  - LLM 调用封装在 Extractor/Summarizer 内部      │
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

**.rag 文件的物理边界 = RAG 库的逻辑边界**：库内一切自洽，外部通过 CLI 操作。这与 Office 文件模型完全一致。

---

## 7. 元数据 SQLite 改造

### 7.1 现状问题

当前 [svc.go:512](../../svc.go#L512-L552) 的 `indexedFiles` + `history/doc_index.json` 实现：

```go
type indexedDoc struct {
    Chunks    []string `json:"chunks"`    // 文件产生的所有 Chunk ID
    Timestamp string   `json:"timestamp"` // 索引时间
}
// 持久化为 JSON 文件：.rag/history/doc_index.json
```

问题：
- 只存 `Chunks ID 列表 + 时间戳`，无 ContentHash，无法判断文件是否变更
- 每次索引都要全量重做 LLM 调用，浪费成本
- 无法记录失败状态、错误信息
- JSON 文件并发写需要锁，性能差
- 无法按路径/状态/时间查询

### 7.2 借鉴 mindx-indexer 的核心优点

参考 [mindx-indexer/pkg/indexer/indexer.go](file:///Users/ray/workspaces/ai-ecosystem/mindx-indexer/pkg/indexer/indexer.go)，吸收以下核心设计：

**1. ContentHash 增量索引**（核心价值，节省 LLM 成本）

```go
existing, _ := store.GetDocumentByPath(absPath)
if existing != nil && existing.ContentHash == contentHash && existing.Status == "indexed" {
    logger.Info("文件未变更，跳过", "path", absPath)
    return nil // 跳过 LLM 调用
}
```

**2. 失败文档持久化**（基础需求，便于排查）

```go
// 失败状态写入 SQLite，记录错误信息
store.SaveDocument(&Document{
    AbsolutePath: absPath,
    ContentHash:  contentHash,
    Status:       "failed",
    ErrorMessage: err.Error(),
})
```

**3. 三阶段索引流程**（并发安全，已在 gorag 部分实现）

```
阶段 1：worker 并发处理文件 → 仅收集内存结果
阶段 2：统一持久化到 SQLite（避免并发写锁）
阶段 3：收集处理错误
```

**不吸收的部分**（避免过度设计）：
- ❌ `chunks` 元数据表 — 向量库已有 chunk 数据，不重复存储
- ❌ `entities` 元数据表 — 图库已有实体数据，不重复存储
- ❌ `index_history` 历史表 — 阶段 1 不实现，后续按需追加
- ❌ `loggingClient` 包装器 — gorag 已有 logging 接口

### 7.3 SQLite 表设计（极简版）

只保留一张表，满足核心需求：

```sql
-- 文档元数据表（唯一核心表）
CREATE TABLE documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    absolute_path TEXT UNIQUE NOT NULL,    -- 绝对路径（硬性约束）
    file_name TEXT NOT NULL,
    extension TEXT,
    size_bytes INTEGER,
    modified_at TIMESTAMP,                 -- 文件 mtime
    content_hash TEXT,                     -- SHA256，增量索引的关键
    status TEXT NOT NULL,                  -- indexed / failed
    chunk_ids JSON,                        -- 该文档产生的所有 chunk ID（删除时清理向量库）
    indexed_at TIMESTAMP,
    error_message TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_documents_hash ON documents(content_hash);
```

**设计要点**：
- `chunk_ids` 用 JSON 数组存储（对齐当前 `indexedDoc.Chunks` 字段）
- 删除文档时遍历 `chunk_ids` 调用 `indexer.Remove(chunkID)`
- 失败状态记录 `error_message`，便于排查
- WAL 模式支持并发读 + 单写

### 7.4 MetaStore 接口

新建 `gorag/store/meta/` 包：

```go
package meta

// Store 元数据存储抽象（接口形式，便于测试 mock）
type Store interface {
    // SaveDocument 保存或更新文档元数据（按 absolute_path UPSERT）
    SaveDocument(doc *Document) error

    // GetDocumentByPath 按绝对路径查询文档
    GetDocumentByPath(absPath string) (*Document, error)

    // ListDocuments 按状态过滤文档列表
    ListDocuments(status string) ([]*Document, error)

    // DeleteDocument 删除文档元数据
    DeleteDocument(absPath string) error

    // Close 关闭数据库
    Close() error
}

// Document 文档元数据
type Document struct {
    ID            int64
    AbsolutePath  string
    FileName      string
    Extension     string
    SizeBytes     int64
    ModifiedAt    time.Time
    ContentHash   string
    Status        string  // indexed / failed
    ChunkIDs      []string
    IndexedAt     *time.Time
    ErrorMessage  string
    UpdatedAt     time.Time
}
```

### 7.5 整合到 IndexingService

```go
// svc.go 改造
type IndexingService struct {
    dataDir    string
    watchs     []string
    metaStore  meta.Store          // 新增：替代 indexedFiles map
    indexer    indexer.Indexer
    logger     logging.Logger
    workerCount int
    // 删除：indexedFiles map[string]*indexedDoc
    // 删除：indexFile string
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
}

// 增量索引逻辑（借鉴 mindx-indexer，加入两级预检和先清理后写入）
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

    // 3. 先清理旧数据（关键！避免孤儿 chunks）
    if existing != nil && len(existing.ChunkIDs) > 0 {
        if err := s.removeFileIndex(ctx, absPath); err != nil {
            s.logger.Warn("清理旧索引失败，继续写入新数据", "path", absPath, "err", err)
        }
    }

    // 4. 调用 indexer.AddFile 写入新数据
    chunks, err := s.indexer.AddFile(ctx, absPath)
    if err != nil {
        // 失败状态写入 meta.db
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

### 7.6 删除文件清理逻辑（原子性保障）

```go
// removeFileIndex 删除文件的索引记录并清理向量库
// 全部 Remove 成功才删除 meta.db 记录，部分失败标记 partial_deleted
func (s *IndexingService) removeFileIndex(ctx context.Context, absPath string) error {
    doc, err := s.metaStore.GetDocumentByPath(absPath)
    if err != nil || doc == nil { return err }

    // 调用 indexer.Remove 清理向量库（type-assert IndexerAdmin）
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
        // 部分失败：标记 partial_deleted，保留未删除的 chunk_ids 供 grag doctor 重试
        return s.metaStore.SaveDocument(&meta.Document{
            AbsolutePath: doc.AbsolutePath,
            ContentHash:  doc.ContentHash,
            Status:       "partial_deleted",
            ChunkIDs:     failedChunks,
            ErrorMessage: fmt.Sprintf("部分 chunk 删除失败：%d/%d", len(failedChunks), len(doc.ChunkIDs)),
        })
    }

    // 全部成功：删除 meta.db 中的文档记录
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
│  2. 双线并行检索                         │
│  ┌─────────────────┐ ┌────────────────┐ │
│  │ 语义检索（0.8）  │ │ 图检索（0.2）   │ │
│  │ SemanticIndexer │ │ GraphIndexer   │ │
│  │ .Search()       │ │ .SearchGraph() │ │
│  │ → *Hit          │ │ → *Hit         │ │
│  └─────────────────┘ └────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  3. RRF 融合（按权重 0.8:0.2）           │
│     - 对 Chunks/Nodes/Edges 分类融合     │
│     - 返回融合后的 *Hit                  │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  4. 输出结果                             │
│     - 默认：terminal 格式化              │
│     - --json：机器可读 JSON              │
└─────────────────────────────────────────┘
```

### 8.2 关键约束

**embedder 前置检查**：
- 未配置 embedder 时，语义检索不可用
- `grag query` 降级为纯图检索（权重自动调整为 0:1）
- `grag doctor` 会检测并提示配置 embedder

**权重可配置**：
- config.yml 中 `query.semantic_weight` 和 `query.graph_weight`
- 默认 0.8:0.2（语义优先）
- 纯图谱场景可调整为 0:1

**.rag 库间不共享**：
- 每个 .rag 库独立，不跨库查询
- 需要共享索引时合并到同一个 .rag 库

---

## 9. 潜在问题与对策

| 问题                           | 对策                                                                                                 |
| ------------------------------ | ---------------------------------------------------------------------------------------------------- |
| 用户直接编辑 `.rag/config.yml` | 允许（高级用户），但推荐用 `grag config set`                                                         |
| 用户直接删除 `.rag/vectors/`   | 类似用户删除 `.docx/word/document.xml`——文档损坏，可从 meta.db 重建索引                              |
| 跨平台 `.rag` 显示不一致       | macOS 用 Bundle 显示为单文件；Linux/Windows 显示为目录但后缀明确（接受差异，不强求一致）             |
| 版本控制（git 提交 .rag）      | 提供 `.ragignore` 默认忽略 `vectors/`、`graphs/`、`caches/`、`meta.db`、`logs/`，只提交 `config.yml` |
| 备份与分享                     | 直接 `cp -r myrag.rag backup.rag`；或打包为 zip                                                      |
| SQLite 与向量库事务一致性      | 采用"先写向量/图库，后写 meta.db"策略；meta.db 失败可重放（向量化是幂等的）                          |
| meta.db 文件损坏               | 启用 SQLite WAL 模式 + 定期 checkpoint；可从向量库重建元数据                                         |
| 跨机器迁移 absolute_path 失效  | 文档明确说明：跨机器迁移后需 `grag doctor --reindex` 重建索引                                        |
| 并发写冲突                     | `.rag/.lock` 文件锁（flock）防止多进程同时写入；`IndexingService` 内部 worker pool 已有并发控制      |
| 多进程同时操作同一 .rag        | 阶段 1 实现文件锁：进入命令时获取 flock，退出时释放；获取失败则报错"另一个 grag 进程正在运行"        |

---

## 10. 实施计划

采用"先注入后删除"的渐进策略，避免 V2 重构成果被打乱。

### 阶段 1：.rag 文件模型落地
- `gorag.Open` 强制 `.rag` 后缀校验
- `grag init` 无参数，创建 `./<basename>.rag`
- CLI 自动检测当前目录的 .rag 子目录（不向上查找）
- `grag index` 默认排除 `.rag` 子目录
- 并发控制：`.rag/.lock` 文件锁（防止多个 grag 进程同时写入）
- **不兼容旧 dataDir**（旧文件让用户重建）

### 阶段 2：LLM 上浮到应用层（基于 gochat）
- `gorag.Open` 增加 `WithLLM(chatClient)` 选项
- `chatClient` 类型为 gochat 的 `chat.Client`
- `cmd/main.go` 实例化 gochat 客户端并注入
- 只支持单个 LLM（多 LLM 视为过度设计）

### 阶段 3：元数据 SQLite 改造
- 新建 `gorag/store/meta/` 包，定义 `Store` 接口
- 实现 SQLite 后端（单表 `documents` + WAL 模式）
- `IndexingService` 整合 `MetaStore`，替代 `indexedFiles map`
- 两级预检：mtime+size 快速判断，不匹配才计算 hash
- 先清理旧数据再写入新数据（避免孤儿 chunks）
- 删除原子性：部分失败标记 `partial_deleted`，供 `grag doctor` 重试
- 失败文档持久化：记录 error_message
- **不提供旧 doc_index.json 迁移**（用户重建索引）

### 阶段 4：清理与辅助命令
- 删除 `indexer.ModelConfig` 类型
- `indexer.New` 签名改为接收 `chat.Client`
- `indexer/graph_test.go` 改为注入 mock
- 删除 `main.go` 的 `createGraphIndexer`
- 实现 `grag doctor` 命令（检测配置完整性 + 重试 partial_deleted）
- 实现 `grag doctor --reindex` 子命令（从向量库重建 meta.db）
- 实现 `grag logs` 命令（输出 `.rag/logs/` 日志）
- macOS Bundle 的 `Info.plist` 支持（延后到后续版本）

### 验收标准
- `go build ./...` 通过
- `go vet ./...` 通过
- `go test ./... -timeout 60s` 通过
- `grag init` 在当前目录创建 `./<basename>.rag`
- `grag query "text"` 在当前目录可工作（.rag 在当前目录）
- `grag index ./docs/` 第二次运行跳过未变更文件（验证 mtime+size 预检）
- LLM 客户端在 cmd 层创建，indexer 包无 LLM SDK 依赖
- meta.db 中的 status 字段可正确记录 indexed / failed / partial_deleted
- `grag doctor` 能检测出缺失 embedder 配置并提示
- `grag logs` 能输出 `.rag/logs/gorag.log`

---

## 11. 设计要点总结

1. **`.rag` 后缀是心智模型契约**，不是技术选择——告诉用户"这是一个完整的 RAG 库文件"
2. **`grag init` 无参数**，在当前目录创建 `./<basename>.rag`，用户无需指定路径
3. **实现上是目录，认知上是文件**（类 Office 文档、macOS Bundle）
4. **`.rag` 必须在当前工作目录**，命令不接受 .rag 路径参数（简化心智模型）
5. **`grag config` 分组式设计**：`grag config llm BaseURL|APIKey|Model` 与 `grag config embedder <path>` 专用语法优先，通用 set/get 兜底
6. **LLM 在应用层实例化**，直接复用 gochat 的 `chat.Client`（不自定义接口，避免过度设计）
7. **只支持单个 LLM**（多 LLM 视为过度设计）
8. **API Key 不进 config.yml**，独立存于 `.rag/.api_key` 文件（权限 600），四级回退
9. **SQLite 元数据库**：两级预检（mtime+size → hash）+ 失败状态持久化 + 删除原子性
10. **存储职责清晰**：vectors（语义）+ graphs（关系）+ meta.db（元数据）+ logs（日志）
11. **查询路径**：语义检索（权重 0.8）+ 图检索（权重 0.2）→ RRF 融合
12. **embedder 前置约束**：未配置 embedder 不能使用向量库，`grag doctor` 检测并提示
13. **辅助命令**：`grag doctor`（诊断配置 + 重试失败）+ `grag doctor --reindex`（重建 meta.db）+ `grag logs`（输出日志）
14. **并发控制**：`.rag/.lock` 文件锁（flock）防止多进程同时写入，阶段 1 实现
15. **不兼容旧版本**：旧 dataDir 用户需重建索引（简化设计，避免迁移复杂度）
16. **macOS Bundle 的 `Info.plist` 延后**到后续版本（不阻塞阶段 1-4）

这是把 gorag 从"开发者库"升级为"终端用户产品"的关键一步。
