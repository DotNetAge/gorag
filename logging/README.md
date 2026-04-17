# Logging (日志系统)

`pkg/logging` 提供了 GoRAG 框架的统一日志接口及其工业级实现。

## 设计哲学

在工业级 RAG 系统中，日志不仅是调试工具，更是审计和排障的关键依据。本包采用了**平权化接口设计**，API 风格完全仿照 uber-go/zap 的调用约定：

1. **接口隔离**：框架核心只依赖 `Logger` 接口，不绑定任何具体实现。
2. **渐进式升级**：提供从极简控制台打印到高性能滚动日志的全量支持。
3. **零学习成本**：用法与 zap 完全一致，key-value 交替传入。

## 核心接口

```go
type Logger interface {
    Info(msg string, keyvals ...any)
    Error(msg string, err error, keyvals ...any)
    Debug(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
}
```

### 使用示例

```go
logger.Info("server started", "port", 8080, "host", "localhost")
logger.Warn("slow request", "duration", 2.5*time.Second, "path", "/api/search")
logger.Error("connection failed", err, "addr", "127.0.0.1:3306")
logger.Debug("cache hit", "key", userID)
// 不带字段也可以
logger.Info("heartbeat")
```

## 实现列表

### 1. ConsoleLogger (控制台日志)
- **用途**：本地开发、单元测试。
- **特点**：直接输出到 `stdout`。
- **初始化**：`logging.DefaultConsoleLogger()`

### 2. FileLogger (文件日志)
- **用途**：轻量级文件输出场景。
- **特点**：写入指定文件，支持级别过滤。
- **初始化**：
  ```go
  logger, err := logging.DefaultFileLogger("app.log", logging.WithLevel(logging.DEBUG))
  defer logger.(*logging.defaultLogger).Close()
  ```

### 3. ZapLogger (工业级高性能日志)
- **用途**：生产环境、高并发场景。
- **特点**：
    - 基于 `uber-go/zap`，极低的内存分配和极高的吞吐量。
    - **自动滚动 (Rotation)**：集成 `lumberjack`，支持基于文件大小、保留天数自动切割日志。
    - **日志压缩**：旧日志自动 gzip 压缩。
    - **多路输出 (Tee)**：支持同时输出 JSON 格式和控制台高亮格式。
- **初始化**：
  ```go
  logger := logging.DefaultZapLogger(logging.ZapConfig{
      Filename:   "logs/gorag.log",
      MaxSize:    100,  // MB
      MaxAge:     30,   // Days
      MaxBackups: 7,    // 备份数
      Compress:   true,
      Console:    true, // 同时打印到控制台
  })
  ```

### 4. NoopLogger (静默日志)
- **用途**：不需要任何日志输出的场景或基准测试。
- **初始化**：`logging.DefaultNoopLogger()`

## 在 GoRAG 中快速集成

```go
// 生产环境推荐配置
idx, _ := gorag.NewHybridIndexer(
    logging.DefaultZapLogger(logging.ZapConfig{
        Filename:   "./data/logs/app.log",
        MaxSize:    500,
        MaxBackups: 7,
        MaxAge:     5,
        Console:    true,
    }),
    vectorStore, graphStore, docStore, llm, embedder,
)
```
