# Logging (日志系统)

`pkg/logging` 提供了 GoRAG 框架的统一日志接口及其工业级实现。

## 设计哲学

在工业级 RAG 系统中，日志不仅是调试工具，更是审计和排障的关键依据。本包采用了**平权化接口设计**：
1. **接口隔离**：框架核心只依赖 `Logger` 接口，不绑定任何具体实现。
2. **渐进式升级**：提供从极简控制台打印到高性能滚动日志的全量支持。

## 核心接口

```go
type Logger interface {
    Info(msg string, fields ...map[string]any)
    Error(msg string, err error, fields ...map[string]any)
    Debug(msg string, fields ...map[string]any)
    Warn(msg string, fields ...map[string]any)
}
```

## 我们提供的实现

### 1. ConsoleLogger (控制台日志)
- **用途**：本地开发、单元测试。
- **特点**：直接输出到 `stdout`，带标准时间戳。
- **初始化**：`logging.DefaultConsoleLogger()`

### 2. ZapLogger (工业级高性能日志)
- **用途**：生产环境、高并发场景。
- **特点**：
    - 基于 `uber-go/zap`，极低的内存分配和极高的吞吐量。
    - **自动滚动 (Rotation)**：集成 `lumberjack`，支持基于文件大小、保留天数自动切割日志，防止磁盘爆满。
    - **日志压缩**：旧日志自动 gzip 压缩。
    - **多路输出 (Tee)**：支持同时输出 JSON 格式（用于日志采集系统如 ELK）和控制台高亮格式。
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

### 3. NoopLogger (静默日志)
- **用途**：不需要任何日志输出的极端性能场景或基准测试。

## 在 GoRAG 中快速集成

建议通过 `indexer` 的 Option 直接注入：

```go
// 生产环境推荐配置
idx, _ := indexer.NewBuilder().
    WithZapLogger("./data/logs/app.log", 500, 7, 5, false).
    Build()
```
