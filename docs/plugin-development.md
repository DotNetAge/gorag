# GoRAG 插件开发指南

本指南将帮助您了解如何为 GoRAG 开发自定义插件，以扩展框架的功能。

## 目录

- [插件系统概述](#插件系统概述)
- [插件类型](#插件类型)
- [开发文档解析器插件](#开发文档解析器插件)
- [开发向量存储插件](#开发向量存储插件)
- [开发嵌入模型插件](#开发嵌入模型插件)
- [开发 LLM 客户端插件](#开发-llm-客户端插件)
- [插件注册与加载](#插件注册与加载)
- [插件打包与分发](#插件打包与分发)
- [最佳实践](#最佳实践)
- [示例项目](#示例项目)

## 插件系统概述

GoRAG 的插件系统基于 Go 的标准 `plugin` 包，允许您在运行时动态加载扩展功能。插件系统提供了以下优势：

- **可扩展性**：无需修改核心代码即可添加新功能
- **模块化**：插件独立开发、测试和部署
- **灵活性**：根据需求动态加载所需功能
- **解耦**：核心系统与扩展功能分离

### 核心接口

所有插件都必须实现 `Plugin` 接口：

```go
type Plugin interface {
    Name() string
    Type() PluginType
    Init(config map[string]interface{}) error
}
```

- `Name()` - 返回插件的唯一标识名称
- `Type()` - 返回插件类型
- `Init()` - 使用配置初始化插件

## 插件类型

GoRAG 支持四种插件类型：

### 1. 文档解析器插件 (Parser Plugin)
用于解析各种格式的文档，提取文本内容。

### 2. 向量存储插件 (Vector Store Plugin)
用于存储和检索向量数据。

### 3. 嵌入模型插件 (Embedder Plugin)
用于将文本转换为向量表示。

### 4. LLM 客户端插件 (LLM Plugin)
用于与大语言模型交互。

## 开发文档解析器插件

### 接口定义

```go
type ParserPlugin interface {
    Plugin
    Parser() parser.Parser
}
```

### 实现示例

以下是一个完整的 Markdown 解析器插件示例：

```go
package main

import (
    "context"
    "fmt"
    "io"
    "strings"
    
    "github.com/DotNetAge/gorag/parser"
    "github.com/DotNetAge/gorag/plugins"
)

type MarkdownParserPlugin struct {
    config map[string]interface{}
}

func (p *MarkdownParserPlugin) Name() string {
    return "markdown-parser"
}

func (p *MarkdownParserPlugin) Type() plugins.PluginType {
    return plugins.PluginTypeParser
}

func (p *MarkdownParserPlugin) Init(config map[string]interface{}) error {
    p.config = config
    return nil
}

func (p *MarkdownParserPlugin) Parser() parser.Parser {
    return &MarkdownParser{config: p.config}
}

type MarkdownParser struct {
    config map[string]interface{}
}

func (p *MarkdownParser) Parse(ctx context.Context, r io.Reader) ([]parser.Chunk, error) {
    data, err := io.ReadAll(r)
    if err != nil {
        return nil, fmt.Errorf("failed to read content: %w", err)
    }
    
    content := string(data)
    chunks := p.splitByHeadings(content)
    
    return chunks, nil
}

func (p *MarkdownParser) SupportedFormats() []string {
    return []string{".md", ".markdown"}
}

func (p *MarkdownParser) splitByHeadings(content string) []parser.Chunk {
    var chunks []parser.Chunk
    sections := strings.Split(content, "\n#")
    
    for i, section := range sections {
        if strings.TrimSpace(section) == "" {
            continue
        }
        
        chunk := parser.Chunk{
            ID:      fmt.Sprintf("chunk-%d", i),
            Content: strings.TrimSpace(section),
            Metadata: map[string]interface{}{
                "format": "markdown",
                "index":  i,
            },
        }
        chunks = append(chunks, chunk)
    }
    
    return chunks
}

func NewPlugin() plugins.Plugin {
    return &MarkdownParserPlugin{}
}
```

### 关键要点

1. **实现 `Parser` 接口**：必须实现 `Parse()` 和 `SupportedFormats()` 方法
2. **分块策略**：根据文档特性选择合适的分块策略
3. **元数据**：为每个块添加有用的元数据信息
4. **错误处理**：妥善处理解析过程中的错误

## 开发向量存储插件

### 接口定义

```go
type VectorStorePlugin interface {
    Plugin
    VectorStore(ctx context.Context) (vectorstore.Store, error)
}
```

### 实现示例

以下是一个 Redis 向量存储插件示例：

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/DotNetAge/gorag/plugins"
    "github.com/DotNetAge/gorag/vectorstore"
    "github.com/redis/go-redis/v9"
)

type RedisVectorStorePlugin struct {
    client *redis.Client
    config map[string]interface{}
}

func (p *RedisVectorStorePlugin) Name() string {
    return "redis-vectorstore"
}

func (p *RedisVectorStorePlugin) Type() plugins.PluginType {
    return plugins.PluginTypeVectorStore
}

func (p *RedisVectorStorePlugin) Init(config map[string]interface{}) error {
    p.config = config
    
    addr, _ := config["address"].(string)
    if addr == "" {
        addr = "localhost:6379"
    }
    
    password, _ := config["password"].(string)
    db, _ := config["db"].(int)
    
    p.client = redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,
    })
    
    return nil
}

func (p *RedisVectorStorePlugin) VectorStore(ctx context.Context) (vectorstore.Store, error) {
    if p.client == nil {
        return nil, fmt.Errorf("plugin not initialized")
    }
    
    return &RedisStore{client: p.client}, nil
}

type RedisStore struct {
    client *redis.Client
}

func (s *RedisStore) Add(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
    pipe := s.client.Pipeline()
    
    for i, chunk := range chunks {
        key := fmt.Sprintf("chunk:%s", chunk.ID)
        
        pipe.HSet(ctx, key, map[string]interface{}{
            "content":   chunk.Content,
            "embedding": embeddings[i],
            "metadata":  chunk.Metadata,
        })
    }
    
    _, err := pipe.Exec(ctx)
    return err
}

func (s *RedisStore) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
    // 实现向量搜索逻辑
    // 可以使用 Redis 的向量搜索功能或自定义实现
    return []vectorstore.Result{}, nil
}

func (s *RedisStore) Delete(ctx context.Context, ids []string) error {
    pipe := s.client.Pipeline()
    
    for _, id := range ids {
        key := fmt.Sprintf("chunk:%s", id)
        pipe.Del(ctx, key)
    }
    
    _, err := pipe.Exec(ctx)
    return err
}

func NewPlugin() plugins.Plugin {
    return &RedisVectorStorePlugin{}
}
```

### 关键要点

1. **连接管理**：妥善管理数据库连接
2. **批量操作**：使用管道提高性能
3. **索引优化**：为向量搜索创建合适的索引
4. **错误恢复**：实现重试和错误处理机制

## 开发嵌入模型插件

### 接口定义

```go
type EmbedderPlugin interface {
    Plugin
    Embedder() embedding.Provider
}
```

### 实现示例

以下是一个自定义嵌入模型插件示例：

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/DotNetAge/gorag/embedding"
    "github.com/DotNetAge/gorag/plugins"
)

type CustomEmbedderPlugin struct {
    embedder *CustomEmbedder
    config   map[string]interface{}
}

func (p *CustomEmbedderPlugin) Name() string {
    return "custom-embedder"
}

func (p *CustomEmbedderPlugin) Type() plugins.PluginType {
    return plugins.PluginTypeEmbedder
}

func (p *CustomEmbedderPlugin) Init(config map[string]interface{}) error {
    p.config = config
    
    modelPath, _ := config["model_path"].(string)
    if modelPath == "" {
        return fmt.Errorf("model_path is required")
    }
    
    embedder, err := NewCustomEmbedder(modelPath)
    if err != nil {
        return fmt.Errorf("failed to initialize embedder: %w", err)
    }
    
    p.embedder = embedder
    return nil
}

func (p *CustomEmbedderPlugin) Embedder() embedding.Provider {
    return p.embedder
}

type CustomEmbedder struct {
    modelPath string
    dimension int
}

func NewCustomEmbedder(modelPath string) (*CustomEmbedder, error) {
    return &CustomEmbedder{
        modelPath: modelPath,
        dimension: 768,
    }, nil
}

func (e *CustomEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    embeddings := make([][]float32, len(texts))
    
    for i, text := range texts {
        embedding, err := e.embedSingle(ctx, text)
        if err != nil {
            return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
        }
        embeddings[i] = embedding
    }
    
    return embeddings, nil
}

func (e *CustomEmbedder) embedSingle(ctx context.Context, text string) ([]float32, error) {
    // 实现实际的嵌入逻辑
    // 这里可以调用本地模型或远程 API
    return make([]float32, e.dimension), nil
}

func (e *CustomEmbedder) Dimension() int {
    return e.dimension
}

func NewPlugin() plugins.Plugin {
    return &CustomEmbedderPlugin{}
}
```

### 关键要点

1. **模型加载**：支持从本地或远程加载模型
2. **批量处理**：优化批量嵌入性能
3. **维度一致性**：确保返回的向量维度一致
4. **资源管理**：合理管理 GPU/CPU 资源

## 开发 LLM 客户端插件

### 接口定义

```go
type LLMPlugin interface {
    Plugin
    LLM() llm.Client
}
```

### 实现示例

以下是一个自定义 LLM 客户端插件示例：

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/DotNetAge/gorag/llm"
    "github.com/DotNetAge/gorag/plugins"
)

type CustomLLMPlugin struct {
    client *CustomLLMClient
    config map[string]interface{}
}

func (p *CustomLLMPlugin) Name() string {
    return "custom-llm"
}

func (p *CustomLLMPlugin) Type() plugins.PluginType {
    return plugins.PluginTypeLLM
}

func (p *CustomLLMPlugin) Init(config map[string]interface{}) error {
    p.config = config
    
    endpoint, _ := config["endpoint"].(string)
    apiKey, _ := config["api_key"].(string)
    model, _ := config["model"].(string)
    
    if endpoint == "" {
        return fmt.Errorf("endpoint is required")
    }
    
    p.client = &CustomLLMClient{
        endpoint: endpoint,
        apiKey:   apiKey,
        model:    model,
    }
    
    return nil
}

func (p *CustomLLMPlugin) LLM() llm.Client {
    return p.client
}

type CustomLLMClient struct {
    endpoint string
    apiKey   string
    model    string
}

func (c *CustomLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
    // 实现同步完成逻辑
    // 调用 LLM API 并返回结果
    return "", nil
}

func (c *CustomLLMClient) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
    ch := make(chan string)
    
    go func() {
        defer close(ch)
        
        // 实现流式完成逻辑
        // 将响应分块发送到通道
        ch <- "Response chunk 1"
        ch <- "Response chunk 2"
        ch <- "Response chunk 3"
    }()
    
    return ch, nil
}

func NewPlugin() plugins.Plugin {
    return &CustomLLMPlugin{}
}
```

### 关键要点

1. **API 集成**：正确处理 API 认证和请求
2. **流式响应**：支持流式输出提升用户体验
3. **错误处理**：处理网络错误和 API 限制
4. **超时控制**：使用 context 管理请求超时

## 插件注册与加载

### 在代码中注册插件

```go
package main

import (
    "github.com/DotNetAge/gorag/plugins"
    myplugin "github.com/yourorg/gorag-my-plugin"
)

func main() {
    registry := plugins.NewRegistry()
    
    // 注册插件
    err := registry.Register(myplugin.NewPlugin())
    if err != nil {
        panic(err)
    }
    
    // 获取插件
    plugin := registry.Get("my-plugin")
    
    // 按类型获取插件
    parserPlugins := registry.GetByType(plugins.PluginTypeParser)
}
```

### 动态加载插件

```go
package main

import (
    "github.com/DotNetAge/gorag/plugins"
)

func main() {
    registry := plugins.NewRegistry()
    
    // 从 .so 文件加载插件
    err := registry.Load("./plugins/my-plugin.so")
    if err != nil {
        panic(err)
    }
    
    // 列出所有插件
    allPlugins := registry.List()
    for _, p := range allPlugins {
        fmt.Printf("Plugin: %s (Type: %s)\n", p.Name(), p.Type())
    }
}
```

## 插件打包与分发

### 构建插件

```bash
# 构建为共享库
go build -buildmode=plugin -o my-plugin.so my-plugin.go
```

### 项目结构

```
gorag-my-plugin/
├── go.mod
├── go.sum
├── my-plugin.go      # 插件主文件
├── README.md         # 插件文档
├── config.yaml       # 配置示例
└── examples/         # 使用示例
    └── example.go
```

### go.mod 文件

```go
module github.com/yourorg/gorag-my-plugin

go 1.20

require (
    github.com/DotNetAge/gorag v0.5.0
)
```

### README 模板

```markdown
# GoRAG My Plugin

Brief description of your plugin.

## Installation

```bash
go get github.com/yourorg/gorag-my-plugin
```

## Usage

```go
import (
    "github.com/DotNetAge/gorag/plugins"
    myplugin "github.com/yourorg/gorag-my-plugin"
)

func main() {
    registry := plugins.NewRegistry()
    registry.Register(myplugin.NewPlugin())
}
```

## Configuration

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| param1 | string | Yes | - | Description |
| param2 | int | No | 100 | Description |

## License

MIT License
```

## 最佳实践

### 1. 命名规范

- 使用小写字母和连字符：`my-parser-plugin`
- 包含插件类型信息：`redis-vectorstore`
- 避免使用保留名称

### 2. 配置管理

```go
type Config struct {
    Required string `yaml:"required"`
    Optional string `yaml:"optional"`
    Port     int    `yaml:"port"`
}

func (p *MyPlugin) Init(config map[string]interface{}) error {
    cfg := &Config{}
    
    if err := mapstructure.Decode(config, cfg); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }
    
    if cfg.Required == "" {
        return fmt.Errorf("required field is missing")
    }
    
    if cfg.Port == 0 {
        cfg.Port = 8080 // 默认值
    }
    
    p.config = cfg
    return nil
}
```

### 3. 错误处理

```go
func (p *MyPlugin) Init(config map[string]interface{}) error {
    if p.initialized {
        return fmt.Errorf("plugin already initialized")
    }
    
    if err := p.validateConfig(config); err != nil {
        return fmt.Errorf("config validation failed: %w", err)
    }
    
    if err := p.setupResources(); err != nil {
        return fmt.Errorf("resource setup failed: %w", err)
    }
    
    p.initialized = true
    return nil
}
```

### 4. 资源清理

```go
type MyPlugin struct {
    client *Client
    closer func() error
}

func (p *MyPlugin) Close() error {
    if p.closer != nil {
        return p.closer()
    }
    return nil
}
```

### 5. 日志记录

```go
import "log/slog"

type MyPlugin struct {
    logger *slog.Logger
}

func (p *MyPlugin) Init(config map[string]interface{}) error {
    p.logger = slog.Default().With("plugin", p.Name())
    
    p.logger.Info("Initializing plugin", "config", config)
    
    // 初始化逻辑
    
    p.logger.Info("Plugin initialized successfully")
    return nil
}
```

### 6. 测试

```go
package main

import (
    "context"
    "testing"
    
    "github.com/DotNetAge/gorag/plugins"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyPlugin_Init(t *testing.T) {
    plugin := NewPlugin()
    
    config := map[string]interface{}{
        "required": "value",
        "port":     8080,
    }
    
    err := plugin.Init(config)
    require.NoError(t, err)
    assert.Equal(t, "my-plugin", plugin.Name())
    assert.Equal(t, plugins.PluginTypeParser, plugin.Type())
}

func TestMyPlugin_InvalidConfig(t *testing.T) {
    plugin := NewPlugin()
    
    config := map[string]interface{}{
        // 缺少必需字段
    }
    
    err := plugin.Init(config)
    assert.Error(t, err)
}
```

## 示例项目

查看以下示例项目了解更多实现细节：

- [PDF Parser Plugin](../parser/pdf/) - PDF 文档解析器
- [Milvus Vector Store Plugin](../vectorstore/milvus/) - Milvus 向量存储
- [OpenAI Embedder Plugin](../embedding/openai/) - OpenAI 嵌入模型
- [OpenAI LLM Plugin](../llm/openai/) - OpenAI LLM 客户端

## 常见问题

### Q: 插件可以依赖其他插件吗？

A: 可以，但需要在 `Init()` 方法中通过插件注册表获取依赖插件。

### Q: 如何处理插件版本兼容性？

A: 在 `Init()` 方法中检查版本兼容性，并在文档中明确说明兼容的 GoRAG 版本。

### Q: 插件可以修改核心功能吗？

A: 不建议。插件应该扩展功能而不是修改核心行为。

### Q: 如何调试插件？

A: 使用日志记录和单元测试。可以为插件添加调试模式配置。

## 获取帮助

- [GitHub Issues](https://github.com/DotNetAge/gorag/issues)
- [文档](./)
- [示例代码](../examples/)

## 贡献

欢迎贡献插件！请查看 [贡献指南](../CONTRIBUTING.md) 了解详情。
