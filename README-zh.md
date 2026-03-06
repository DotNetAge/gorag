# GoRAG

[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen.svg)](https://github.com/DotNetAge/gorag)
[![Go Version](https://img.shields.io/badge/go-1.20%2B-blue.svg)](https://golang.org)

**GoRAG** - 生产级 Go 语言 RAG（检索增强生成）框架

[English](README.md) | **[中文文档](README-zh.md)**

## 特性

- **高性能** - 为生产环境构建，低延迟高吞吐
- **模块化设计** - 可插拔的解析器、向量存储和 LLM 提供商
- **云原生** - 支持 Kubernetes，单二进制部署
- **类型安全** - 利用 Go 强类型系统实现完整类型安全
- **生产就绪** - 内置可观测性、指标和错误处理
- **混合检索** - 结合向量搜索和关键词搜索获得更好结果
- **重排序** - 基于 LLM 的结果重排序提升相关性
- **流式响应** - 实时流式输出提升用户体验
- **插件系统** - 可扩展架构支持自定义功能
- **命令行工具** - 提供易用的命令行界面
- **全面测试** - 85%+ 测试覆盖率，使用 Testcontainers 进行集成测试
- **多模态支持** - 处理图像和其他媒体类型
- **配置管理** - 灵活的 YAML 和环境变量配置
- **自定义提示模板** - 支持带占位符的自定义提示格式
- **性能基准测试** - 内置性能基准测试

### 🎯 开箱即用支持

#### 文档解析器（9 种类型）
- **Text** - 纯文本和 Markdown 文件
- **PDF** - PDF 文档
- **DOCX** - Microsoft Word 文档
- **HTML** - HTML 网页
- **JSON** - JSON 数据文件
- **YAML** - YAML 配置文件
- **Excel** - Microsoft Excel 电子表格（.xlsx）
- **PPT** - Microsoft PowerPoint 演示文稿（.pptx）
- **Image** - 图像（支持 OCR）

#### 嵌入模型提供商（2 个提供商）
- **OpenAI** - OpenAI 嵌入模型（text-embedding-ada-002、text-embedding-3-small、text-embedding-3-large）
- **Ollama** - 本地嵌入模型（bge-small-zh-v1.5、nomic-embed-text 等）

#### LLM 客户端（5 个客户端）
- **OpenAI** - GPT-3.5、GPT-4、GPT-4 Turbo、GPT-4o
- **Anthropic** - Claude 3（Opus、Sonnet、Haiku）
- **Azure OpenAI** - Azure OpenAI 服务
- **Ollama** - 本地 LLM（Llama 3、Qwen、Mistral 等）
- **Compatible** - OpenAI API 兼容服务（支持国产大模型：通义千问、智谱、百川、月之暗面、深度求索等）

#### 向量存储（5 个后端）
- **Memory** - 内存存储，用于开发和测试
- **Milvus** - 生产级向量数据库
- **Qdrant** - 高性能向量搜索引擎
- **Pinecone** - 全托管向量数据库
- **Weaviate** - 支持 GraphQL API 的语义搜索引擎

## 快速开始

```go
package main

import (
    "context"
    "log"
    "os"
    
    embedder "github.com/DotNetAge/gorag/embedding/openai"
    llm "github.com/DotNetAge/gorag/llm/openai"
    "github.com/DotNetAge/gorag/parser/text"
    "github.com/DotNetAge/gorag/rag"
    "github.com/DotNetAge/gorag/vectorstore/memory"
)

func main() {
    ctx := context.Background()
    apiKey := os.Getenv("OPENAI_API_KEY")
    
    // 创建 RAG 引擎
    embedderInstance, _ := embedder.New(embedder.Config{APIKey: apiKey})
    llmInstance, _ := llm.New(llm.Config{APIKey: apiKey})
    
    engine, err := rag.New(
        rag.WithParser(text.NewParser()),
        rag.WithVectorStore(memory.NewStore()),
        rag.WithEmbedder(embedderInstance),
        rag.WithLLM(llmInstance),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 索引文档
    err = engine.Index(ctx, rag.Source{
        Type: "text",
        Content: "Go 是一门开源编程语言...",
    })
    
    // 使用自定义提示模板查询
    resp, err := engine.Query(ctx, "什么是 Go？", rag.QueryOptions{
        TopK: 5,
        PromptTemplate: "你是一个有用的助手。根据以下上下文：\n\n{context}\n\n回答问题：{question}",
    })
    
    log.Println(resp.Answer)
}
```

## 安装

```bash
go get github.com/DotNetAge/gorag
```

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                    GoRAG                                 │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  Document   │  │   Vector    │  │    LLM      │     │
│  │   Parser    │  │   Store     │  │   Client    │     │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │
│         └─────────────────┼─────────────────┘           │
│                           ▼                           │
│                  ┌─────────────────┐                    │
│                  │   RAG Engine    │                    │
│                  └─────────────────┘                    │
└─────────────────────────────────────────────────────────┘
```

## 模块

- **parser** - 文档解析器（9 种类型：Text、PDF、DOCX、HTML、JSON、YAML、Excel、PPT、Image）
- **embedding** - 嵌入模型提供器（OpenAI、Ollama）
- **llm** - LLM 客户端（OpenAI、Anthropic、Azure OpenAI、Ollama、Compatible API）
- **vectorstore** - 向量存储后端（Memory、Milvus、Qdrant、Pinecone、Weaviate）
- **rag** - RAG 引擎和编排
- **plugins** - 插件系统，用于扩展功能
- **config** - 配置管理系统

## 命令行工具

GoRAG 提供命令行界面，方便使用：

```bash
# 安装
go install github.com/DotNetAge/gorag/cmd/gorag@latest

# 索引文档
gorag index --api-key $OPENAI_API_KEY "Go 是一门开源编程语言..."

# 从文件索引
gorag index --api-key $OPENAI_API_KEY --file README.md

# 查询引擎
gorag query --api-key $OPENAI_API_KEY "什么是 Go？"

# 流式响应
gorag query --api-key $OPENAI_API_KEY --stream "Go 的主要特性有哪些？"

# 使用自定义提示模板
gorag query --api-key $OPENAI_API_KEY --prompt "你是一个有用的助手。回答问题：{question}"

# 导出已索引的文档
gorag export --api-key $OPENAI_API_KEY --file export.json

# 导入文档
gorag import --api-key $OPENAI_API_KEY --file export.json
```

## 配置

GoRAG 支持通过 YAML 文件和环境变量进行灵活配置：

### YAML 配置

创建 `config.yaml` 文件：

```yaml
rag:
  topK: 5
  chunkSize: 1000
  chunkOverlap: 100

embedding:
  provider: "openai"
  openai:
    apiKey: "your-api-key"
    model: "text-embedding-ada-002"

llm:
  provider: "openai"
  openai:
    apiKey: "your-api-key"
    model: "gpt-4"

vectorstore:
  type: "milvus"
  milvus:
    host: "localhost"
    port: 19530

logging:
  level: "info"
  format: "json"
```

### 环境变量

```bash
export GORAG_RAG_TOPK=5
export GORAG_EMBEDDING_PROVIDER=openai
export GORAG_LLM_PROVIDER=openai
export GORAG_VECTORSTORE_TYPE=memory
export GORAG_OPENAI_API_KEY=your-api-key
export GORAG_ANTHROPIC_API_KEY=your-api-key
export GORAG_PINECONE_API_KEY=your-api-key
```

## 示例

- **Basic** - 简单的 RAG 使用示例
- **Advanced** - 高级功能，包括流式响应和混合检索
- **Web** - HTTP API 服务器示例

## 测试

GoRAG 拥有全面的测试覆盖，包括单元测试和集成测试：

### 测试覆盖

- **整体覆盖率**：所有模块 85%+ 覆盖率
- **单元测试**：所有核心模块都有全面的单元测试
- **集成测试**：使用 Testcontainers 与真实向量数据库进行测试
- **性能基准测试**：内置 Index 和 Query 操作的基准测试

### 运行测试

```bash
# 运行所有单元测试
go test ./...

# 运行集成测试（需要 Docker）
go test -v ./integration_test/...

# 运行测试并查看覆盖率
go test -cover ./...

# 运行基准测试
go test -bench=. ./rag/
```

### 集成测试

集成测试使用 [Testcontainers](https://testcontainers.com/) 启动真实实例：
- **Milvus** - 生产级向量数据库
- **Qdrant** - 高性能向量搜索引擎
- **Weaviate** - 支持 GraphQL API 的语义搜索引擎

这确保了 GoRAG 在生产环境中与真实向量数据库正确工作。

## 文档

- [入门指南](docs/getting-started.md)
- [API 参考](docs/api.md)
- [生产部署指南](docs/deployment.md)
- [插件开发指南](docs/plugin-development.md)
- [示例代码](examples/)
- [贡献指南](CONTRIBUTING.md)

## 路线图

### 已完成（v0.5.0）
- [x] 文档解析器（9 种类型）
- [x] 向量存储（5 个后端）
- [x] 嵌入模型提供商（2 个提供商）
- [x] LLM 客户端（5 个客户端）
- [x] 混合检索和重排序
- [x] 流式响应
- [x] 多模态支持
- [x] 插件系统
- [x] 命令行工具
- [x] 全面测试覆盖（85%+）
- [x] 使用 Testcontainers 的集成测试
- [x] 配置管理
- [x] 自定义提示模板
- [x] 性能基准测试
- [x] 生产部署指南
- [x] 插件开发指南

### 计划中（v0.6.0 - 质量改进）
- [ ] 提高测试覆盖率（config: 0%、Azure OpenAI: 0%、Excel: 13.5%、Milvus: 18.8%、Qdrant: 13.0%、Weaviate: 14.1%、RAG engine: 40.6%）
- [ ] 实现正确的 LLM 响应解析以获取重排序分数
- [ ] 添加边缘情况的错误处理
- [ ] 改进代码文档

### 计划中（v0.7.0 - 性能与可靠性）
- [ ] 优化嵌入批处理
- [ ] 为向量存储添加连接池
- [ ] 实现查询结果缓存
- [ ] 添加重试逻辑和熔断器模式

### 计划中（v0.8.0 - 文档与示例）
- [ ] 添加架构决策记录（ADR）
- [ ] 创建真实世界用例示例
- [ ] 设置 GitHub Actions CI/CD
- [ ] 创建故障排除指南

### 未来
- [ ] 评估 Graph RAG 可行性
- [ ] 插件市场

## 贡献

欢迎贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详情。

## 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE)。

## 更新日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解版本历史和变更。
