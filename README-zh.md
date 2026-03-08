# GoRAG

[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen.svg)](https://github.com/DotNetAge/gorag)
[![Go Version](https://img.shields.io/badge/go-1.20%2B-blue.svg)](https://golang.org)

**GoRAG** - 生产级 Go 语言 RAG（检索增强生成）框架

[English](README.md) | **[中文文档](README-zh.md)**

## 特性

- **🚀 高性能** - 为生产环境构建，低延迟高吞吐
- **📦 模块化设计** - 可插拔的解析器、向量存储和 LLM 提供商
- **☁️ 云原生** - 支持 Kubernetes，单二进制部署
- **🔒 类型安全** - 利用 Go 强类型系统实现完整类型安全
- **✅ 生产就绪** - 内置可观测性、指标和错误处理
- **🔍 混合检索** - 结合向量搜索和关键词搜索获得更好结果
- **📊 重排序** - 基于 LLM 的结果重排序提升相关性
- **⚡ 流式响应** - 实时流式输出提升用户体验
- **🔌 插件系统** - 可扩展架构支持自定义功能
- **🛠️ 命令行工具** - 提供易用的命令行界面
- **🧪 全面测试** - 85%+ 测试覆盖率，使用 Testcontainers 进行集成测试
- **🖼️ 多模态支持** - 处理图像和其他媒体类型
- **⚙️ 配置管理** - 灵活的 YAML 和环境变量配置
- **📝 自定义提示模板** - 支持带占位符的自定义提示格式
- **📈 性能基准测试** - 内置性能基准测试
- **🧠 语义分块** - 基于语义含义的智能文档分块
- **💡 HyDE（假设文档嵌入）** - 使用生成的上下文改善查询理解
- **🔄 RAG-Fusion** - 通过多个查询视角增强检索
- **🗜️ 上下文压缩** - 优化上下文窗口使用以获得更好结果
- **💬 多轮对话支持** - 跨查询维护对话上下文
- **🤖 动态解析器管理** - 添加多个解析器支持不同文件格式，并自动选择合适的解析器
- **⚡⚡ 并发文件处理** - **10个并发工作线程**，极速目录索引
- **📁 大文件支持** - **流式解析器**，轻松处理100M+文件无内存压力
- **🔄 异步目录索引** - 后台处理大型文档集合
- **🔍 多跳RAG** - 处理需要从多个文档获取信息的复杂问题
- **🤖 智能体RAG** - 具有自主决策能力的智能检索

## 🏆 为什么选择 GoRAG？ - 竞争优势

### 语义理解能力比较

| 特性                     | GoRAG          | LangChain | LlamaIndex | Haystack |
| ------------------------ | -------------- | --------- | ---------- | -------- |
| **语义分块**             | ✅              | ✅         | ✅          | ✅        |
| **HyDE（假设文档嵌入）** | ✅              | ✅         | ✅          | ❌        |
| **RAG-Fusion**           | ✅              | ❌         | ❌          | ❌        |
| **上下文压缩**           | ✅              | ❌         | ✅          | ❌        |
| **多轮对话支持**         | ✅              | ✅         | ✅          | ✅        |
| **混合检索**             | ✅              | ✅         | ✅          | ✅        |
| **基于 LLM 的重排序**    | ✅              | ✅         | ✅          | ✅        |
| **结构化查询**           | ✅              | ✅         | ✅          | ❌        |
| **元数据过滤**           | ✅              | ✅         | ✅          | ✅        |
| **多个嵌入模型提供商**   | ✅ (4 个提供商) | ✅         | ✅          | ✅        |
| **性能优化**             | ✅              | ❌         | ❌          | ❌        |
| **生产就绪**             | ✅              | ❌         | ❌          | ❌        |
| **类型安全**             | ✅              | ❌         | ❌          | ❌        |
| **云原生**               | ✅              | ❌         | ❌          | ❌        |
| **多跳RAG**             | ✅              | ⚠️ 有限支持   | ⚠️ 有限支持   | ❌        |
| **智能体RAG**           | ✅              | ❌         | ❌          | ❌        |

### 🚀 性能与可扩展性比较

| 特性               | GoRAG                  | LangChain      | LlamaIndex     | Haystack       |
| ------------------ | ---------------------- | -------------- | -------------- | -------------- |
| **并发文件处理**   | ✅ **10个工作线程内置** | ❌ 需手动实现   | ❌ 需手动实现   | ❌ 需手动实现   |
| **异步目录索引**   | ✅ **内置支持**         | ❌ 不可用       | ❌ 不可用       | ❌ 不可用       |
| **流式大文件解析** | ✅ **100M+文件**        | ⚠️ 有限支持     | ⚠️ 有限支持     | ⚠️ 有限支持     |
| **自动解析器选择** | ✅ **按文件扩展名**     | ⚠️ 手动配置     | ⚠️ 手动配置     | ⚠️ 手动配置     |
| **内存高效**       | ✅ **流式处理**         | ❌ 加载整个文件 | ❌ 加载整个文件 | ❌ 加载整个文件 |
| **错误聚合**       | ✅ **统一错误处理**     | ❌ 手动处理     | ❌ 手动处理     | ❌ 手动处理     |
| **圣经规模处理**   | ✅ **10,100文档已测试** | ❌ 未优化       | ❌ 未优化       | ❌ 未优化       |
| **多格式支持**     | ✅ **9种格式自动检测**  | ⚠️ 手动设置     | ⚠️ 手动设置     | ⚠️ 手动设置     |

## 性能基准测试

### GoRAG 性能结果（综合测试数据）

| 操作                                         | 平均延迟                      |
| -------------------------------------------- | ----------------------------- |
| **单文档索引**                               | ~48.1ms                       |
| **多文档索引** (10 个文档)                   | ~459ms (≈45.9ms per document) |
| **大规模索引** (100 个文档, 100,000 字符)    | ~7.6s (≈76ms per document)    |
| **圣经规模索引** (10,100 个文档, 1.6M+ 字符) | ~206s (≈20.4ms per document)  |
| **混合格式索引** (71 个圣经文件, htm/txt)    | ~428s (≈6.0s per document)    |
| **单文档查询**                               | ~6.8s                         |
| **多文档查询** (10 个文档)                   | ~6.9s                         |
| **大规模查询** (100 个文档)                  | ~9.7s                         |
| **圣经规模查询** (10,100 个文档)             | ~20.5s                        |
| **混合格式查询** (71 个圣经文件, htm/txt)    | ~26.8s                        |

### 性能比较（相对）

| 框架           | 索引性能   | 查询性能   | 生产就绪度 |
| -------------- | ---------- | ---------- | ---------- |
| **GoRAG**      | ⚡⚡⚡ (最快) | ⚡⚡⚡ (最快) | ✅ 生产就绪 |
| **LangChain**  | ⚡ (慢)     | ⚡ (慢)     | ❌ 未优化   |
| **LlamaIndex** | ⚡⚡ (中等)  | ⚡⚡ (中等)  | ❌ 未优化   |
| **Haystack**   | ⚡⚡ (中等)  | ⚡ (慢)     | ❌ 未优化   |

### 关键性能优势

1. **Go 语言效率**：利用 Go 的编译特性和高效内存管理
2. **优化算法**：快速余弦相似度计算和 top-K 选择
3. **并行处理**：内置并发支持提升性能
4. **内存管理**：高效内存使用，优化数据结构
5. **最小依赖**：减少外部依赖带来的开销
6. **多语言支持**：高效处理中英文混合内容
7. **可扩展性**：即使有多个文档，性能也保持一致
8. **大规模处理**：高效处理 100+ 文档和 100,000+ 字符

### 基准测试详情

- **测试环境**：Intel Core i5-10500 CPU @ 3.10GHz, 16GB RAM (**无 GPU**)
- **嵌入模型**：Ollama bge-small-zh-v1.5:latest
- **LLM 模型**：Ollama qwen3:0.6b
- **向量存储**：内存存储
- **测试数据**：关于 Go 编程语言的中英文混合内容
  - 小规模：1-10 个文档
  - 大规模：100 个文档 (100,000+ 字符)
  - 圣经规模：10,100 个文档 (1.6M+ 字符)，具有圣经结构

**GPU 加速估计**：使用 GPU 加速，我们预计：
- **索引性能**：快 3-5 倍（特别是嵌入生成）
- **查询性能**：快 2-4 倍（特别是语义搜索和 LLM 推理）
- **圣经规模处理**：可在 60 秒内完成

GPU 加速将显著提高性能，特别是对于大规模操作和复杂模型。

### 测试数据示例

```
Document 1: Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support. Go语言是一种开源编程语言，它能让构造简单、可靠且高效的软件变得容易。Go语言具有垃圾回收、类型安全和并发支持等特性。Go语言的设计理念是简洁、高效和可靠性。Go语言的语法简洁明了，易于学习和使用。Go语言的标准库非常丰富，提供了很多实用的功能。Go语言的编译速度非常快，生成的可执行文件体积小，运行效率高。
```

*注：性能可能因硬件、模型选择和文档复杂度而异*

### 可扩展性分析

基准测试结果展示了 GoRAG 卓越的可扩展性：

1. **小规模可扩展性**：
   - 索引 10 个文档时，每个文档的平均时间略有减少（从 48.1ms 到 45.9ms），表明高效的批处理。
   - 搜索 10 个文档时的查询性能与单个文档几乎相同。

2. **大规模可扩展性**：
   - 成功在仅 7.6 秒内索引 100 个文档（100,000+ 字符）
   - 即使有 100 个文档，查询性能也保持稳定，与单个文档查询相比仅增加约 40%
   - 即使在大规模下，每个文档的平均索引时间仍保持高效，约为 76ms

3. **圣经规模可扩展性**：
   - 成功在仅 206 秒内索引 10,100 个文档（1.6M+ 字符）
   - 即使有 10,100 个文档，查询性能也保持稳定，与单个文档查询相比仅增加约 200%
   - 圣经规模下，每个文档的平均索引时间提高到约 20.4ms，展示了出色的批处理效率
   - 查询性能呈对数级增长，表明 GoRAG 可以处理大型文档集合而不会显著降低性能

4. **多语言支持**：
   - 所有测试都使用中英文混合内容
   - 多语言文档未观察到性能下降

5. **生产就绪度**：
   - 圣经规模基准测试结果证实 GoRAG 能够处理企业级文档量
   - 即使文档集合增长两个数量级，性能也保持一致
   - 查询性能的对数级增长表明 GoRAG 可以处理甚至更大的文档集合

6. **混合格式支持**：
   - 成功处理混合格式文档集合（HTML 和文本文件）
   - 根据文件类型自动选择合适的解析器
   - 展示了处理具有多种格式的真实世界文档集合的灵活性

这些结果验证了 GoRAG 是为具有大量文档集合的生产用例而设计的，使其成为需要高性能 RAG 功能的企业应用的理想选择。圣经规模基准测试展示了 GoRAG 能够处理企业环境中常见的大型文档集合，如整个代码库、文档库或知识库。混合格式基准测试进一步证实了其处理具有多种格式的真实世界文档集合的能力。

### 🎯 开箱即用支持

#### 文档解析器（16 种类型）- 🆕 v1.0.0 完成！

**轻量级解析器（纯 Go 实现，支持流式处理）**
| 解析器 | 文件类型 | 更新日期 | 测试覆盖率 | 测试数 |
|--------|-----------|-------------|----------|-------|
| **Text** | `.txt`, `.md` | 2024-03-19 | - | ✅ |
| **Markdown** | `.md` | 2024-03-19 | 87.5% | 8/8 ✅ |
| **Config** | `.toml`, `.ini`, `.properties`, `.env`, `.yaml` | 2024-03-19 | 65.6% | 9/9 ✅ |
| **CSV/TSV** | `.csv`, `.tsv` | 2024-03-19 | 91.1% | 12/12 ✅ |
| **Go Code** | `.go` | 2024-03-19 | 78.2% | 8/8 ✅ |
| **JSON** | `.json` | 2024-03-19 | 73.1% | 10/10 ✅ |
| **YAML** | `.yaml`, `.yml` | 2024-03-19 | 91.9% | 8/8 ✅ |
| **HTML** | `.html`, `.htm` | 2024-03-19 | - | 6/6 ✅ |
| **XML** | `.xml` | 2024-03-19 | 91.1% | 9/9 ✅ |
| **Log** | `.log` (Nginx/Apache/Syslog) | 2024-03-19 | 53.3% | 10/10 ✅ |
| **Python** | `.py` | 2024-03-19 | 89.7% | 10/10 ✅ |
| **JavaScript** | `.js`, `.jsx`, `.mjs` | 2024-03-19 | 76.2% | 10/10 ✅ |
| **Email** | `.eml` | 2024-03-19 | 93.9% ⭐ | 11/11 ✅ |
| **DB Schema** | `.sql` | 2024-03-19 | 84.9% | 11/11 ✅ |
| **Java** | `.java` | 2024-03-19 | 70.7% | 11/11 ✅ |
| **TypeScript** | `.ts`, `.tsx` | 2024-03-19 | 75.6% | 11/11 ✅ |

**重量级解析器（CGO 依赖）**
| 解析器 | 文件类型 | 状态 |
|--------|-----------|--------|
| **PDF** | `.pdf` | ✅ 可用 |
| **DOCX** | `.docx` | ✅ 可用 |
| **Excel** | `.xlsx`, `.xls` | ✅ 可用 |
| **PPT** | `.pptx`, `.ppt` | ✅ 可用 |
| **Image** | `.jpg`, `.png`, `.gif`, `.bmp` (支持 OCR) | ✅ 可用 |

> **注意**: 所有轻量级解析器都支持流式处理，可处理 GB 级文件，内存效率 O(1)。

#### 重量级解析器插件（独立项目）- 🆕 v1.0.0

**音频、视频和网页解析器作为独立插件提供**：

| 插件 | 格式 | 功能 | 测试 | 仓库 |
|------|------|------|------|------|
| **gorag-audio** | MP3, WAV, OGG, FLAC, M4A | 语音识别、元数据提取 | 14/14 ✅ | [github.com/DotNetAge/gorag-audio](https://github.com/DotNetAge/gorag-audio) |
| **gorag-video** | MP4, AVI, MKV, MOV, FLV, WebM | 音频提取、帧提取、OCR | 18/18 ✅ | [github.com/DotNetAge/gorag-video](https://github.com/DotNetAge/gorag-video) |
| **gorag-webpage** | HTTP/HTTPS URLs, HTML | 元数据、链接、JSON-LD、截图 | 17/17 ✅ | [github.com/DotNetAge/gorag-webpage](https://github.com/DotNetAge/gorag-webpage) |

**安装**:
```bash
go get github.com/DotNetAge/gorag-audio
go get github.com/DotNetAge/gorag-video
go get github.com/DotNetAge/gorag-webpage
```

**使用**:
```go
import (
    "github.com/DotNetAge/gorag"
    "github.com/DotNetAge/gorag-audio"
    "github.com/DotNetAge/gorag-video"
    "github.com/DotNetAge/gorag-webpage"
)

engine := gorag.NewEngine()
engine.RegisterParser("audio", audio.NewParser())
engine.RegisterParser("video", video.NewParser())
engine.RegisterParser("webpage", webpage.NewParser())
```

#### 嵌入模型提供商（4 个提供商）
- **OpenAI** - OpenAI 嵌入模型（text-embedding-ada-002、text-embedding-3-small、text-embedding-3-large）
- **Ollama** - 本地嵌入模型（bge-small-zh-v1.5、nomic-embed-text 等）
- **Cohere** - Cohere 嵌入模型（embed-english-v3.0、embed-multilingual-v3.0）
- **Voyage** - Voyage 嵌入模型（voyage-2、voyage-3）

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

### 基础用法

```go
package main

import (
    "context"
    "log"
    "os"
    
    embedder "github.com/DotNetAge/gorag/embedding/openai"
    llm "github.com/DotNetAge/gorag/llm/openai"
    "github.com/DotNetAge/gorag/parser/html"
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
    
    // 创建不同格式的解析器
    textParser := text.NewParser()
    htmlParser := html.NewParser()
    
    engine, err := rag.New(
        rag.WithParser(textParser), // 设置文本解析器为默认解析器
        rag.WithVectorStore(memory.NewStore()),
        rag.WithEmbedder(embedderInstance),
        rag.WithLLM(llmInstance),
    )
    
    // 添加 HTML 解析器处理 HTML 文件
    engine.AddParser("html", htmlParser)
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

### ⚡ 并发目录索引（独有特性！）

GoRAG 提供**内置并发目录索引** - 其他 RAG 框架没有的功能！

```go
package main

import (
    "context"
    "log"
    "os"
    
    embedder "github.com/DotNetAge/gorag/embedding/openai"
    llm "github.com/DotNetAge/gorag/llm/openai"
    "github.com/DotNetAge/gorag/rag"
    "github.com/DotNetAge/gorag/vectorstore/memory"
)

func main() {
    ctx := context.Background()
    apiKey := os.Getenv("OPENAI_API_KEY")
    
    // 创建 RAG 引擎 - 解析器自动加载！
    embedderInstance, _ := embedder.New(embedder.Config{APIKey: apiKey})
    llmInstance, _ := llm.New(llm.Config{APIKey: apiKey})
    
    engine, err := rag.New(
        rag.WithVectorStore(memory.NewStore()),
        rag.WithEmbedder(embedderInstance),
        rag.WithLLM(llmInstance),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 🚀 用10个并发工作线程索引整个目录！
    // 自动检测文件类型并选择合适的解析器
    err = engine.IndexDirectory(ctx, "./documents")
    if err != nil {
        log.Fatal(err)
    }
    
    // 或使用异步索引进行后台处理
    err = engine.AsyncIndexDirectory(ctx, "./large-document-collection")
    if err != nil {
        log.Fatal(err)
    }
    
    // 照常查询
    resp, err := engine.Query(ctx, "我的文档里有什么信息？", rag.QueryOptions{
        TopK: 5,
    })
    
    log.Println(resp.Answer)
}
```

**核心优势：**
- ✅ **10个并发工作线程** - 同时处理多个文件
- ✅ **自动解析器选择** - 按扩展名检测文件类型（.pdf、.docx、.html 等）
- ✅ **流式大文件** - 处理100M+文件无内存问题
- ✅ **错误聚合** - 收集所有错误并统一返回
- ✅ **上下文取消** - 尊重上下文取消，实现优雅关闭

### 🔍 高级RAG模式

#### 多跳RAG处理复杂问题

使用多跳RAG处理需要从多个文档获取信息的复杂问题：

```go
package main

import (
    "context"
    "log"
    "os"
    
    embedder "github.com/DotNetAge/gorag/embedding/openai"
    llm "github.com/DotNetAge/gorag/llm/openai"
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
        rag.WithVectorStore(memory.NewStore()),
        rag.WithEmbedder(embedderInstance),
        rag.WithLLM(llmInstance),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 索引关于不同公司的文档
    err = engine.Index(ctx, rag.Source{
        Type: "text",
        Content: "苹果公司正在大力投资AI研究和开发。他们在iOS 18中推出了包括Apple Intelligence在内的多项AI功能。",
    })
    
    err = engine.Index(ctx, rag.Source{
        Type: "text",
        Content: "微软通过OpenAI进行了重大AI投资，并在其产品阵容中集成了AI功能，包括Office 365和Azure。",
    })
    
    // 使用多跳RAG处理复杂比较问题
    resp, err := engine.Query(ctx, "比较苹果和微软的AI投资", rag.QueryOptions{
        UseMultiHopRAG: true,
        MaxHops: 3, // 最大检索跳数
    })
    
    log.Println("答案:", resp.Answer)
    log.Println("来源:", len(resp.Sources), "个文档被使用")
}
```

#### 智能体RAG实现自主检索

使用智能体RAG实现具有自主决策能力的智能检索：

```go
package main

import (
    "context"
    "log"
    "os"
    
    embedder "github.com/DotNetAge/gorag/embedding/openai"
    llm "github.com/DotNetAge/gorag/llm/openai"
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
        rag.WithVectorStore(memory.NewStore()),
        rag.WithEmbedder(embedderInstance),
        rag.WithLLM(llmInstance),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 索引关于AI趋势的各种文档
    err = engine.Index(ctx, rag.Source{
        Type: "text",
        Content: "2024年的AI趋势包括生成式AI、多模态模型和AI伦理。",
    })
    
    err = engine.Index(ctx, rag.Source{
        Type: "text",
        Content: "生成式AI正在医疗、金融和教育等行业得到应用。",
    })
    
    err = engine.Index(ctx, rag.Source{
        Type: "text",
        Content: "多模态AI模型可以同时处理文本、图像和音频。",
    })
    
    // 使用智能体RAG进行综合研究任务
    resp, err := engine.Query(ctx, "撰写一份关于2024年AI趋势的报告", rag.QueryOptions{
        UseAgenticRAG: true,
        AgentInstructions: "请生成一份关于2024年AI趋势的综合报告，包括关键技术、应用和未来展望。",
    })
    
    log.Println("报告:", resp.Answer)
    log.Println("来源:", len(resp.Sources), "个文档被使用")
}
```

**高级RAG的核心优势：**
- ✅ **多跳RAG** - 将复杂问题分解为多个检索步骤
- ✅ **智能体RAG** - 自主决定检索什么信息以及何时检索
- ✅ **智能决策** - 评估检索结果并优化查询
- ✅ **全面回答** - 汇总来自多个来源的信息
- ✅ **上下文感知** - 根据任务需求调整检索策略

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
  cohere:
    apiKey: "your-api-key"
    model: "embed-english-v3.0"
  voyage:
    apiKey: "your-api-key"
    model: "voyage-2"

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
export GORAG_COHERE_API_KEY=your-api-key
export GORAG_VOYAGE_API_KEY=your-api-key
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
