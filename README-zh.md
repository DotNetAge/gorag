<div align="center">
  <h1>🦖 GoRAG</h1>
  <p><b>专为 Go 语言打造的生产级、高性能、模块化 RAG 框架</b></p>
  
  [![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
  [![codecov](https://codecov.io/gh/DotNetAge/gorag/graph/badge.svg?token=placeholder)](https://codecov.io/gh/DotNetAge/gorag)
  [![Go Version](https://img.shields.io/badge/go-1.24%2B-blue.svg)](https://golang.org)
  
  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

**GoRAG** 是一个完全使用 Go 语言编写的生产级、高性能 RAG（检索增强生成）框架。告别 Python 的依赖地狱和缓慢的异步循环吧！GoRAG 将 **Go 的高并发性能、极低内存占用和静态类型安全** 引入 AI 工程世界。

无论你是构建简单的文档问答机器人，还是需要多跳推理的复杂 Agentic RAG 系统，GoRAG 都能为你提供坚实的底层积木。

## ✨ 为什么选择 GoRAG？

- 🚀 **极速处理与低内存占用**: 内置 10+ 并发 Worker 协程池，结合 `O(1)` 内存消耗的流式解析器 (Streaming Parser)，轻松应对 GB 级海量知识库。
- 🧩 **纯粹的整洁架构**: 遵循 Clean Architecture 设计。高度模块化，只需修改一行代码，即可无缝切换底层大模型、向量数据库和文档解析器。
- 🧠 **开箱即用的高阶 RAG 模式**: 内置 HyDE (假设性文档嵌入)、RAG-Fusion (多路召回融合)、Semantic Chunking (语义分块)、Cross-Encoder (交叉重排序) 以及 Context Pruning (上下文剪枝) 等前沿技术。
- ☁️ **生产级云原生**: 编译产出单体无依赖二进制文件。框架原生内置熔断器 (Circuit Breakers)、限流器、优雅降级策略以及完整的可观测性监控指标。
- 📦 **零外部依赖极速启动**: 深度集成自研的纯 Go 向量数据库 `govector` 和统一大模型 SDK `gochat`。无需部署笨重的独立数据库，拔掉网线也能实现 100% 本地隐私 RAG 开发。

## 🧰 生态与无缝集成

### 🤖 大语言模型支持 (基于 [`gochat`](https://github.com/DotNetAge/gochat))
- **国际主流**: OpenAI (GPT-4o), Anthropic (Claude 3), Azure OpenAI
- **开源/本地**: Ollama (Llama 3, Qwen, Mistral)
- **国产大模型**: Kimi (月之暗面), DeepSeek (深度求索), GLM-4 (智谱), Minimax, 通义千问, 百川等

### 🗄️ 向量与图数据库支持
- **govector** 🌟 (原生纯 Go 嵌入式向量库 - 零成本起步！)
- **Milvus / Zilliz** (生产环境企业标准)
- **Qdrant** (高性能 Rust 向量库)
- **Weaviate** (领先的语义搜索引擎)
- **Neo4j / ArangoDB** (用于 Graph RAG 和知识图谱)

### 📄 全能解析器生态
原生提供针对 **16+ 种文档格式** 的流式解析支持，包括：文本、PDF、DOCX、Markdown、HTML、CSV、JSON，甚至原生支持主流编程语言的源码解析 (Go, Python, Java, TS/JS 等)。

---

## 🚀 快速开始

### 安装

```bash
go get github.com/DotNetAge/gorag
```

### 10 行代码构建专属私有知识库
通过 `Ollama` 结合我们的内置 `govector` 引擎，实现无需任何 API Key，也无需部署外部数据库的纯本地 RAG：

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/DotNetAge/gochat/pkg/client/base"
    "github.com/DotNetAge/gochat/pkg/client/ollama"
    "github.com/DotNetAge/gorag/rag"
    "github.com/DotNetAge/gorag/vectorstore/govector"
)

func main() {
    ctx := context.Background()

    // 1. 初始化大模型客户端 (基于 gochat SDK)
    llmClient, _ := ollama.New(ollama.Config{
        Config: base.Config{Model: "qwen3.5:0.8b"},
    })

    // 2. 初始化纯 Go 原生向量数据库 (零外部依赖)
    vectorStore, _ := govector.NewStore(govector.Config{
        Dimension:  1536,
        Collection: "my_knowledge",
    })

    // 3. 构建 RAG 引擎
    engine, _ := rag.New(
        rag.WithLLM(llmClient),
        rag.WithVectorStore(vectorStore),
    )

    // 4. 将私有文档灌库（自动分块、向量化）
    engine.Index(ctx, rag.Source{
        Type:    "text",
        Content: "GoRAG 是一个使用纯 Go 语言开发的本地检索引擎框架。",
    })

    // 5. 进行知识问答
    resp, _ := engine.Query(ctx, "GoRAG 是什么？", rag.QueryOptions{TopK: 3})
    fmt.Println("回答:", resp.Answer)
}
```

### 高并发企业级目录吞吐
需要处理代码仓库、公司规章制度或几十 GB 的研究资料包？GoRAG 的并发池为你自动提效：

```go
// 🚀 一键索引整个目录！自动识别 .pdf, .go, .md, .docx 等后缀并分配解析器
err := engine.IndexDirectory(ctx, "./my-company-docs")

// 采用流式打字机效果回复，极致提升前端用户体验
ch, _ := engine.QueryStream(ctx, "总结一下这份目录里的 Q3 财报核心信息", rag.QueryOptions{
    Stream: true,
})

for resp := range ch {
    fmt.Print(resp.Chunk)
}
```

---

## ⚡ 深入探索高级架构

GoRAG 绝不仅仅是一个粘合剂框架，我们在底层为你实现了当代最前沿的检索引擎技术：

- **智能体 RAG (CRAG & Self-RAG)**：内置评估器 (Evaluator)，模型自动根据意图对检索质量进行评分与反思，触发重写查询或补充搜索。
- **混合多路召回与融合 (RAG-Fusion)**：将单次查询裂变为多个不同视角的 Query 并行召回，再通过 RRF 算法进行结果融合，大幅提升召回率。
- **跨编码器与上下文剪枝 (Cross-Encoder & Pruning)**：先海量召回，后精准重排。通过 LLM 或小型交叉模型对大文本块进行高精度裁剪，只将最关键的句子送入上下文，不仅大幅降低 Token 消耗，更有效缓解模型幻觉。
- **图检索增强 (Graph RAG)**：原生适配 Neo4j 等图数据库，支持跨越文档边界的实体多跳逻辑关系网络。

---

## 🛠 命令行运维工具
GoRAG 随附强大的 CLI 命令行工具，专为快速验证与运维排障设计：

```bash
# 安装命令行
go install github.com/DotNetAge/gorag/cmd/gorag@latest

# 直接在终端将指定文件向量化并存入库中
gorag index --api-key $OPENAI_API_KEY --file ./docs/architecture.md

# 命令行直连知识库查询
gorag query --api-key $OPENAI_API_KEY "你们的架构有什么优势？"
```

## 📈 演进路线图
- [x] 核心接口规范、存储抽象与 DI 容器设计
- [x] RAG-Fusion, HyDE, Context Pruning 高阶检索增强节点
- [x] Graph RAG 原生集成 (支持 Neo4j 等主流图库)
- [ ] 多模态 RAG 解析器支持 (图像、视频时序抽取)
- [ ] 企业级控制台仪表盘与开箱即用的 API Server 

## 🤝 参与贡献
我们致力于在 Go 语言生态构建最强的企业级 AI 底层基础设施。无论你是提交 Issue、改进文档，还是提交 PR 增加新的向量库驱动，我们都非常欢迎！
- 请参考我们的 [贡献指南](CONTRIBUTING.md)

如果这个项目对你的业务或学习有启发，欢迎随手点个 **⭐️ Star**，这会对我们继续迭代开源非常有帮助！

## 📄 许可协议
本项目基于 [MIT License](LICENSE) 协议开源。