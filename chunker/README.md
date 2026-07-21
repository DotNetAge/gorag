# chunker 包设计说明

本包负责将 `document.RawDoc` 切分为 `core.Chunk`，并在切分过程中同时产出结构化的 `core.Node` 与 `core.Edge`。

V2 接口定义：

```go
type ChunkResult struct {
    Chunks []core.Chunk
    Nodes  []core.Node
    Edges  []core.Edge
}

type Chunker interface {
    Chunk(doc document.RawDoc) (ChunkResult, error)
}
```

---

## 1. MarkdownChunker

源码：`markdown.go`

### 1.1 分片逻辑

**依据**：用 tree-sitter-markdown 解析 Markdown 文档，识别 `atx_heading`（`#`/`##` 等）和 `setext_heading`（下划线标题），把 heading 作为语义单元的边界。

**过程**：

1. 解析 Markdown block 树。
2. 用 Query 捕获所有 heading 节点，记录起始字节位置、标题文本、层级。
3. 无 heading 时，整个文档作为一个 Chunk。
4. 有 heading 时：
   - 第一个 heading 之前的前言/元数据单独成一个 Chunk，标题取文件名。
   - 每个 heading 到下一个 heading 之间的内容成为一个 Chunk。
   - Chunk 的 `Title` = heading 文本，`Metadata["heading_level"]` = heading 层级。

**效果**：

- 章节语义完整，不会把某个 section 拦腰截断。
- heading 文本直接进入 `Title`，便于后续向量和图索引复用。
- 前置内容（YAML frontmatter、package 说明等）不会丢失。

### 1.2 实体与关系提取

**依据**：heading 层级本身就是文档结构，不需要 LLM。

**过程**：

1. 生成一个 `Document` 节点作为根。
2. 每个 heading 生成一个 `Section` 节点，`Properties` 中记录 `heading_level`。
3. 用栈维护当前祖先链：
   - level 1 heading 挂到 Document 下。
   - level N heading 挂到最近的 level < N 的 heading 下。
   - 遇到同级或更高级 heading，先弹出栈顶再重新找父。
4. 父子之间生成 `CONTAINS` 边。

**效果**：形成和 Markdown 目录完全一致的结构树。

例如 `# 标题一 / ## 子标题 / # 标题二`：

- Nodes：`Document`、`标题一`、`子标题`、`标题二`
- Edges：`Document→标题一`、`标题一→子标题`、`Document→标题二`

---

## 2. CodeChunker

源码：`code.go`

### 2.1 分片逻辑

**依据**：用 tree-sitter 解析代码 AST，按「定义节点」切分。不同语言注册不同的 Query 捕获函数、类、方法、接口、结构体、枚举、trait 等定义。

**支持语言**：Go、Python、JavaScript/TypeScript、Java、C/C++、Rust、C#、Ruby、Kotlin、Scala、Swift、PHP、Lua、Bash 等。

**过程**：

1. 根据文件扩展名拿到对应的 tree-sitter language 和 Query。
2. 解析代码得到 AST。
3. 用 Query 捕获 `@def`（定义节点）和 `@name`（名称节点）。
4. 按定义节点的字节偏移排序。
5. 第一个定义之前的 import/package/注释等作为 header Chunk。
6. 每个定义节点到下一个定义节点之间的内容成为一个 Chunk。
7. Chunk 的字段：
   - `Title` = 符号名
   - `Summary` = 符号前的注释、函数体内的 docstring，或内容第一句摘要（无注释时 fallback）
   - `Content` = 从符号定义开头到下一个符号定义开头之间的完整代码
   - `Metadata["symbol_type"]` = 规范化后的元素类型（去掉 tree-sitter 后缀，如 `function`、`method`、`class`、`struct`、`trait`、`interface`、`enum`、`impl`、`type`、`namespace`、`module` 等）

**效果**：

- 每个函数/类/方法成为语义完整的 Chunk：
  - `Title` = 符号名
  - `Summary` = 符号前的注释或函数体内的 docstring（按以下顺序提取：块注释 `/* */` → 行注释 `//` / `#` → 函数体内第一个三引号字符串）
  - `Content` = 从符号定义开头到下一个符号定义开头之间的完整代码（含签名、注释、实现）
- 不会像纯文本按字符切分那样把函数体拦腰截断。
- 标题即符号名，方便代码检索时直接命中函数名。

### 2.2 实体与关系提取

**依据**：代码 AST 的字节范围。父定义节点的字节范围如果完全包含子定义节点，就认为父包含子。

**过程**：

1. 生成一个 `Document` 节点作为根。
2. 每个定义节点生成一个 Node：
   - `Name` = 符号名
   - `Labels` = 简化标签，由节点类型推导：
     - `function*` → `Function`
     - `method*` → `Method`
     - `class*` / `struct*` / `enum*` → `Class`
     - `interface*` / `trait*` / `protocol*` → `Interface`
     - 其它 → `Symbol` / `Type` / `Module` 等
   - `Properties["node_type"]` = 规范化后的元素类型（如 `struct`、`trait`、`impl`），去掉了 tree-sitter 中无意义的后缀（`_declaration`、`_definition`、`_item`、`_spec`、`_statement`）
3. 用栈维护当前祖先链：
   - 新符号如果在栈顶符号的字节范围内，则栈顶是其父。
   - 如果不在，则弹出栈顶继续找父，直到找到包含者或回到 Document。
4. 父子之间生成 `CONTAINS` 边，并绑定对应的 `SourceChunkIDs`。

**效果**：

- 对 Python/Java/C++/Rust 等类内定义方法的语言，能自动得到 `Class → Method` 的包含关系。
- 例如 Python 的 `class Animal:` 包含 `__init__` 和 `speak`：
  - Nodes：`Document`、`Animal`、`__init__`、`speak`
  - Edges：`Document→Animal`、`Animal→__init__`、`Animal→speak`

### 2.3 面向对象与调用关系

当前已从 AST 提取的关系：

| 关系         | 说明                            | 当前支持         |
| ------------ | ------------------------------- | ---------------- |
| `CONTAINS`   | 父符号包含子符号（按字节范围）  | 所有语言         |
| `BELONGS_TO` | Go 方法归属于某个 receiver 类型 | Go               |
| `CALLS`      | 函数/方法调用另一个函数/方法    | Go、Python       |
| `INHERITS`   | 类继承另一个类                  | Java、TypeScript |
| `IMPLEMENTS` | 类实现接口                      | Java、TypeScript |
| `USES`       | 使用某个类型 / 导入某个模块     | 后续补充         |

#### CALLS 提取策略

- **Go**：同时捕获普通函数调用 `add(1, 2)` 和选择器调用 `p.Greet()`。对选择器调用，通过变量声明/参数推断 receiver 类型，生成 `Person.Greet` 形式的目标名；推断失败时回退到方法名。外部符号（如 `fmt.Println`）不会生成桩节点。
- **Python**：捕获普通函数调用 `greet()` 和属性调用 `self.speak()`。对 `self.Method()`，解析为所属类的 `ClassName.Method()`；其他属性调用回退到方法名。类实例化（如 `Animal()`）不会被当作函数调用。

#### INHERITS / IMPLEMENTS 提取策略

- **Java**：查询 `class_declaration` 的 `superclass` 与 `super_interfaces` 子句。
- **TypeScript**：直接查询 `extends_clause` / `implements_clause`，再向上回溯到所属 `class_declaration`，避免不同 grammar 版本对 `class_heritage` 字段命名的差异。

#### 未提取的关系

- **Rust**：可通过 `impl_item` 增加 `IMPLEMENTS` 关系。
- **C#**：可通过 `extends` / `implements` 子句增加 `INHERITS` / `IMPLEMENTS` 关系。
- **USES**：导入、类型引用等跨模块关系需要更复杂的符号解析，后续补充。

---

## 3. DatumChunker

源码：`datum.go`

### 3.1 分片逻辑

**依据**：按数据结构的边界切分，而不是按字符数。支持 JSON、YAML、XML、CSV、TOML、LOG 等。

**过程**：

1. 根据扩展名判断数据类型（`JSON`/`YAML`/`XML`/`CSV`/`TOML`/`Log` 等）。
2. **Log 文件**：非结构化，直接按行切分。
3. **结构化数据**：
   - 使用 document 包归一化后的 JSON 内容。
   - 递归遍历对象/数组：
     - 对象属性：每个复杂属性（对象/数组）单独成一个 Chunk，标量属性合并到一个「其余字段」Chunk。
     - 数组：每个元素单独成一个 Chunk（如 `JSON.[0]`、`JSON.[1]`）。
   - 用 `datumMaxChunkSize` 控制大小，避免单个 Chunk 过大。
4. Chunk 的 `Title` = `数据类型.路径`（如 `JSON.users`、`JSON.config.debug`）。

**效果**：

- JSON 里的 `users` 数组和 `config` 对象会被拆成独立的 Chunk，检索时能精准命中具体字段。
- 标量字段不会零散成无数小 Chunk，而是合并到「其余字段」中。
- 深层嵌套（如 `level1.level2.level3`）也能按路径递归切分。

### 3.2 实体与关系提取

**依据**：数据路径的层级前缀。父路径天然包含子路径。

**过程**：

1. 生成一个 `Document` 节点，名称是数据类型（如 `JSON`/`YAML`）。
2. 对每个非「其余字段」的 Chunk 生成一个 Node：
   - 路径包含 `[N]`（数组元素）→ `node_type=record`，label=`Collection`
   - 路径还有子路径 → `node_type=object`，label=`Collection`
   - 叶子路径 → `node_type=field`，label=`Data`
3. 根据标题推导父标题：
   - `JSON.users[0].name` → 父 `JSON.users[0]`
   - `JSON.users[0]` → 父 `JSON.users`
   - `JSON.config.debug` → 父 `JSON.config`
   - `JSON.users` → 父 `JSON`
4. 父子之间生成 `CONTAINS` 边。

**效果**：形成和数据结构完全一致的树。

例如：

```json
{
  "users": [{"id": 1}],
  "config": {"debug": true}
}
```

- Nodes：`JSON`、`JSON.users`、`JSON.users[0]`、`JSON.config`
- Edges：`JSON→JSON.users`、`JSON.users→JSON.users[0]`、`JSON→JSON.config`

### 3.3 Log 文件

**分片逻辑**：Log 是半结构化文本，按行切分，每行作为一个 Chunk。`Chunk.Title` 取前若干个非空白字符（如时间戳前缀），便于快速浏览。

**元数据增强**：对常见日志格式（如 `2024-01-01 10:00:00 [ERROR] serviceA - message`），用轻量正则解析出以下字段写入 `Chunk.Metadata`：

| 字段        | 类型   | 说明                           |
| ----------- | ------ | ------------------------------ |
| `timestamp` | string | 日志时间戳（ISO 或常见格式）   |
| `level`     | string | 日志级别（ERROR/WARN/INFO 等） |
| `logger`    | string | logger / 服务名                |
| `message`   | string | 日志正文                       |

**边界**：Log 文件不在 Chunker 阶段生成 Node/Edge。更深层的实体提取（如「这段日志涉及哪个服务」「出现了什么错误码」）交给上层 `Extractor` 用 LLM 处理。

### 3.4 EML / MSG 邮件文件

`document` 包已经把 `.eml` 和 `.msg` 归一化为统一的 JSON 结构：

```json
{
  "from": "张三 <zhangsan@example.com>",
  "to": "李四 <lisi@example.com>",
  "cc": "王五 <wangwu@example.com>",
  "subject": "会议通知",
  "date": "2024-01-01T10:00:00Z",
  "body": "..."
}
```

DatumChunker 检测到 `dataKind == "Email"` 时，除了按 JSON 结构切分外，还可以生成邮件领域的 Node/Edge：

| 节点         | 标签     | 来源字段              |
| ------------ | -------- | --------------------- |
| 邮件本身     | `Email`  | subject               |
| 发件人       | `Person` | from                  |
| 收件人       | `Person` | to                    |
| 抄送人       | `Person` | cc                    |
| 附件（如有） | `File`   | document 包提取的附件 |

| 关系             | 说明                |
| ---------------- | ------------------- |
| `SENT_BY`        | 邮件 → 发件人       |
| `SENT_TO`        | 邮件 → 收件人       |
| `HAS_CC`         | 邮件 → 抄送人       |
| `HAS_ATTACHMENT` | 邮件 → 附件（如有） |
| `CONTAINS`       | 邮件 → body Chunk   |

这样可以在图检索中直接回答「张三发送过哪些邮件」「这封邮件发给了谁」等关系型查询。

---

## 4. ImageChunker

源码：`image.go`

### 4.1 分片逻辑

**依据**：图片文件无法像文本那样按语义边界切分，因此整张图片作为一个文档级 Chunk。

**过程**：

1. `document.ParseImage` 将图片等比缩放，短边固定为 `thumbnail_size`（默认 224 像素），再编码为 Base64 缩略图。
2. `ImageChunker` 把整张图片作为一个 Chunk。
3. `Chunk.Title` 取文件名（去掉扩展名）。
4. `Chunk.Content` 为 Base64 缩略图字符串。

**效果**：

- 每个图片文件对应一个 Chunk，方便后续多模态向量嵌入或视觉模型处理。
- 不强行拆分图片，避免破坏视觉语义。

### 4.2 元数据回填

`document.ParseImage` 已提取的元数据会写入 `Chunk.Metadata`：

| 字段             | 类型   | 说明                     |
| ---------------- | ------ | ------------------------ |
| `mime_type`      | string | 图片 MIME 类型           |
| `thumbnail_size` | int    | 缩略图短边像素（如 224） |

### 4.3 实体与关系

当前 `ImageChunker` 不生成 Node/Edge。原因：

- 图片的语义信息主要来自像素内容，无法像文本/代码那样用确定性规则提取结构化实体。
- 如果需要图片级别的图节点（如把图片作为 `Image` 节点加入图谱），可通过上层 `Extractor` 或后续扩展实现。

---

## 5. AST 可提取信息与元数据设计

为了从分片阶段就建立高密度的检索链路，Chunker 会尽可能把 AST/结构信息下沉到 `Chunk.Metadata` 和 `Node.Properties` 中。

### 5.1 信息分层原则

| 位置                | 适合放什么                               | 为什么                                    |
| ------------------- | ---------------------------------------- | ----------------------------------------- |
| **Chunk.Metadata**  | 分片级别的位置、边界、类型、显示辅助信息 | 用于结果高亮、排序、过滤、快速预览        |
| **Node.Properties** | 实体的语义属性、签名、可见性、关系线索   | 用于图查询、Cypher/向量混合检索、实体消歧 |

### 5.2 通用元数据（所有 Chunker）

每个 Chunk 都会自动填充：

| 字段         | 类型   | 说明                                                                       |
| ------------ | ------ | -------------------------------------------------------------------------- |
| `source`     | string | 来源文件名                                                                 |
| `start_line` | int    | 在源文件中的起始行号（从 1 开始）                                          |
| `end_line`   | int    | 在源文件中的结束行号（包含）                                               |
| `language`   | string | 文件语言（代码扩展名，如 `go`/`python`；文档扩展名，如 `markdown`/`json`） |

此外，**Chunk.ID 生成策略**为 `GenerateID(docID + ":" + title + ":" + content)`。将 `title`（heading 路径、符号名、数据路径等）纳入盐值，可避免数据文件中相同内容的记录（如数组里多个相同对象）产生重复 ID，同时保证同一内容在不同文档或不同位置下 ID 稳定且唯一。

### 5.2.1 Chunk.ParentID 分块树

每个 Chunk 通过 `ParentID` 字段指向其直接父 Chunk，形成一棵与文档结构一致的分块树：

- **MarkdownChunker**：按 heading 层级确定父子关系（level 2 的 Chunk 父级是最近的 level 1 Chunk）。
- **CodeChunker**：按符号字节范围确定父子关系（class/method 的父级是包含它的 class/namespace）。
- **DatumChunker**：按数据路径前缀确定父子关系（`users[0].name` 的父级是 `users[0]`）。
- **ImageChunker**：单张图片作为一个文档级 Chunk，`ParentID` 为空；`Chunk.Metadata` 回填 `mime_type` 与 `thumbnail_size`。

`ParentID` 为空表示该 Chunk 是文档级根节点。通过 `ParentID` 可以在向量检索命中子 Chunk 时快速向上追溯到父级上下文，也可以在摘要/引用时按层级聚合。

### 5.3 CodeChunker 扩展信息

| 字段          | 放哪里                                        | 说明                                                                   |
| ------------- | --------------------------------------------- | ---------------------------------------------------------------------- |
| `symbol_type` | Chunk.Metadata + Node.Properties["node_type"] | 规范化后的元素类型（去掉 `_declaration`/`_definition`/`_item` 等后缀） |
| `visibility`  | Chunk.Metadata + Node.Properties              | 可见性（Go 大写导出、`public`/`private`/`protected` 等）               |
| `signature`   | Chunk.Metadata + Node.Properties              | 函数/方法/类型的签名行（如 `func (p Person) Greet() string`）          |
| `receiver`    | Chunk.Metadata + Node.Properties              | Go 方法 receiver 类型（如 `Person`）                                   |
| `package`     | Chunk.Metadata + 文档根 Node.Properties       | Go / Java / Kotlin 的 package 名                                       |

其中 `signature`、`visibility`、`receiver` 已随分片过程自动填充。`package` 通过正则从文件头提取，并回填到所有 Chunk.Metadata 中。

### 5.4 MarkdownChunker 扩展信息

| 字段             | 放哪里                                               | 说明                             |
| ---------------- | ---------------------------------------------------- | -------------------------------- |
| `heading_level`  | Chunk.Metadata + Section.Properties["heading_level"] | heading 层级 1~6                 |
| `is_frontmatter` | Chunk.Metadata                                       | 该 Chunk 是否为 YAML frontmatter |

### 5.5 DatumChunker 扩展信息

| 字段               | 放哪里          | 说明                                   |
| ------------------ | --------------- | -------------------------------------- |
| `data_type`        | Node.Properties | 数据类型（JSON/YAML/XML/CSV/TOML/Log） |
| `path`             | Node.Properties | 数据路径（如 `users[0].name`）         |
| `is_array_element` | Node.Properties | 是否为数组元素                         |

### 5.6 信息密度与检索链路

这些字段的作用不是简单堆砌，而是为了把「代码图」和「文档图」在同一套索引里打通：

- 向量检索命中某个 Chunk 后，可通过 `symbol_type` + `signature` 快速判断这是不是目标函数。
- 图检索命中某个 Node 后，可通过 `source_chunk_ids` 回到原始 Chunk，再通过 `start_line` 定位到具体代码行。
- 代码调用链、类继承链已通过 `CALLS` / `INHERITS` / `IMPLEMENTS` 边与现有 `CONTAINS` / `BELONGS_TO` 边形成完整链路。

### 5.7 SourceChunkIDs 为空的语义

Chunker 生成的 Node 分为两种：

1. **文档级根节点**（`Document` 节点）：代表整个文件/数据/文档，不绑定到任何单个 Chunk，因此 `SourceChunkIDs` 为空。
2. **结构节点**（`Section` / `Function` / `Data` 等）：与具体 Chunk 一一对应，`SourceChunkIDs` 包含对应 Chunk.ID。

**设计决策**：明确 `SourceChunkIDs` 为空是合法状态，表示该 Node 是纯图实体。上层代码（如 Indexer、Extractor、Formatter）不得假设每个 Node 都有 Chunk。这样 LLMExtractor 生成的文档级实体、未来的知识图谱补全节点都可以自然接入，而无需伪造 Chunk 绑定。

### 5.8 统一的 Summary 策略

为了提升向量检索效果，每个 textual Chunk（Markdown / Code / Datum）都会填充 `Summary`：

- **提取策略**：从 `Content` 中取前 `maxSentences` 个句子作为 Summary，支持中文句号 `。` 和英文句号 `. ` 切分。
- **各 Chunker 取值**：
  - `MarkdownChunker`：每个 heading 分块取内容前 2 个句子。
  - `CodeChunker`：优先取符号前的注释 / 函数体内 docstring；无注释时 fallback 到内容第一句摘要。
  - `DatumChunker`：每个数据分块取内容前 2 个句子；Log 按行切分后同样适用。
- **ImageChunker**：`Summary` 留空，因为 Base64 缩略图内容不适合句子摘要，后续可由 Extractor 补充。

统一 Summary 的入口为 `chunker.go` 中的 `deriveSummary(content string, maxSentences int) string`。

---

## 6. 四者对比

| 维度         | MarkdownChunker | CodeChunker              | DatumChunker           | ImageChunker |
| ------------ | --------------- | ------------------------ | ---------------------- | ------------ |
| 分片依据     | heading 层级    | AST 定义节点             | 数据结构路径           | 整张图片     |
| 标题来源     | heading 文本    | 符号名                   | `类型.路径`            | 文件名       |
| 实体来源     | Section 节点    | Function/Class/Method 等 | Data/Collection 节点   | 暂不生成     |
| 关系依据     | heading 层级    | 字节范围包含             | 路径前缀包含           | 无           |
| 是否需要 LLM | 否              | 否                       | 否                     | 否           |
| 最适合场景   | 文档/笔记       | 代码                     | JSON/YAML/CSV/Log/邮件 | 图片/截图    |

这四个 Chunker 产出的是**确定性结构**，速度快、不依赖 LLM。更复杂的语义实体（如「这段代码调用了哪个外部 API」「这个文档提到了哪个人名」「图片里有什么内容」）仍然交给上层 `Extractor` 用 LLM 补充。
