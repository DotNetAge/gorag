# 🦖 GoRAG

[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
[![License:MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen.svg)](https://github.com/DotNetAge/gorag)
[![Go Version](https://img.shields.io/badge/go-1.22%2B-blue.svg)](https://golang.org)
[![codecov](https://codecov.io/gh/DotNetAge/gorag/graph/badge.svg?token=placeholder)](https://codecov.io/gh/DotNetAge/gorag)
[![Powered by gochat](https://img.shields.io/badge/Powered%20by-gochat-ff69b4.svg)](https://github.com/DotNetAge/gochat)
[![Pure Go Vector](https://img.shields.io/badge/Vector%20Store-govector-success.svg)](https://github.com/DotNetAge/govector)

**GoRAG** 是一个完全使用 Go 语言编写的生产级、高性能 RAG（检索增强生成）框架。专为企业级可扩展性设计，让你零 Python 依赖即可将内部私有数据与最强大的大语言模型（LLMs）无缝连接。

[English](README.md) | **[中文文档](README-zh.md)**

---

## 🔥 为什么选择 GoRAG？

告别 Python 的依赖地狱和缓慢的异步循环吧！GoRAG 将 **Go 的高并发性能和静态类型安全** 引入 AI 世界。

- **🚀 极速处理**: 内置 10 个并发 Worker 协程，轻松应对 100M+ 巨量文件解析。
- **🛡️ 生产级可靠**: 原生内置熔断器（Circuit Breakers）、优雅降级策略以及完整的可观测性监控指标。
- **🧩 乐高式模块化**: 修改一行代码，即可无缝切换底层大模型、向量数据库和文档解析器。
- **🧠 高阶检索能力**: 开箱即用支持 Agentic RAG（智能体 RAG）、多跳检索（Multi-hop）、语义分块（Semantic Chunking）、HyDE 和 RAG-Fusion。
- **☁️ 纯粹云原生**: 编译产出单体二进制文件，无需庞大的运行时环境，完美适配 Docker 与 Kubernetes。
- **📦 零外部依赖部署**: 内置原生纯 Go 向量数据库 `govector`，无需额外部署独立数据库服务即可完成本地开发与测试，同时无缝支持 Milvus、Qdrant 等企业级集群库。

## ✨ 核心更新 (v1.0.0)

- **全面拥抱 gochat 统一模型 SDK**: 框架底层已完全升级为 [`gochat`](https://github.com/DotNetAge/gochat)。一次编写代码，即可任意切换 OpenAI、Anthropic、Ollama、Azure，以及各类国产大语言模型（Kimi、通义千问、智谱、深度求索、Minimax 等）。
- **原生向量引擎融合**: 深度集成 [`govector`](https://github.com/DotNetAge/govector)，带来零依赖的纯 Go 嵌入式向量搜索体验。
- **繁荣的文档解析生态**: 原生支持多达 16 种常用文档格式解析（文本、PDF、DOCX、代码、HTML、Email等），并提供独立的音视频和网页提取插件引擎。

---

## 🛠️ 开箱即用的生态支持

### 🤖 大语言模型支持 (基于 `gochat`)
- **OpenAI**: GPT-4o, GPT-4 Turbo, GPT-3.5
- **Anthropic**: Claude 3 (Opus, Sonnet, Haiku)
- **开源/本地模型**: Ollama (Llama 3, Qwen, Mistral)
- **企业云服务**: Azure OpenAI
- **兼容国产模型 API**: Kimi（月之暗面）, DeepSeek（深度求索）, GLM-4（智谱）, Minimax, 百川等。

### 🗄️ 向量数据库支持
- **govector** 🌟 (原生纯 Go 嵌入式向量库 - 零成本起步！)
- **Milvus** (生产环境企业标准)
- **Qdrant** (高性能 Rust 向量库)
- **Weaviate** (领先的语义搜索引擎)
- **Pinecone** (全托管云端数据库)
- **Memory** (开发测试内存库)

---

## 🚀 快速开始

### 安装

```bash
go get github.com/DotNetAge/gorag
```

### 1. 10行代码搞定专属知识库

通过 `Ollama` 结合我们的内置 `govector` 引擎，实现无需任何 API Key，也无需部署外部数据库的 100% 纯本地隐私 RAG：

```go
package main

import (
	"context"
	"log"

	"github.com/DotNetAge/gochat/pkg/client/base"
	"github.com/DotNetAge/gochat/pkg/client/ollama"
	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/vectorstore/govector"
)

func main() {
	ctx := context.Background()

	// 1. 初始化大模型客户端 (基于 gochat SDK)
	llmClient, _ := ollama.New(ollama.Config{
		Config: base.Config{Model: "qwen:0.5b"},
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
	log.Println("回答:", resp.Answer)
}
```

### 2. 高并发企业级目录吞吐

需要处理代码仓库、公司规章制度或几十 GB 资料包？GoRAG 的 **多协程 Worker 池** 为你自动并行处理。

```go
// ... (初始化 Engine 同上)

// 🚀 一键索引整个目录！自动识别 .pdf, .go, .md, .docx 等后缀选择对应解析器
err := engine.IndexDirectory(ctx, "./my-company-docs")
if err != nil {
    log.Fatal(err)
}

// 采用流式打字机效果回复，提升前端用户体验
ch, _ := engine.QueryStream(ctx, "总结一下这份目录里的 Q3 财报核心信息", rag.QueryOptions{
    Stream: true,
})

for resp := range ch {
    fmt.Print(resp.Chunk)
}
```

---

## 📊 性能基准测试

与传统的 Python 框架相比，GoRAG 展现了真正的工业级性能。以下数据在标准 Intel Core i5（**无 GPU**）环境下测得：

| 场景操作                            | GoRAG 耗时    | Python竞品平均耗时    |
| ----------------------------------- | ------------- | --------------------- |
| 解析并索引单个文档                  | **~48ms**     | ~200ms                |
| 解析并索引 100 个文档               | **~7.6s**     | ~25s+                 |
| **《圣经》体量文档** (10,100+ 文档) | **~3.4 分钟** | 经常 OOM / 需重度调优 |

*注：如果通过 Milvus / GPU 版 Ollama 开启显卡加速，性能将再获得 3~5 倍的暴增！*

## 🌟 深入探索高级 RAG 模式

GoRAG 不仅仅是个胶水框架，它原生在底层实现了前沿检索增强技术：

- **智能体 RAG (Agentic RAG)**：模型根据意图自动决策，按需调用检索工具还是直接作答。
- **多跳逻辑检索 (Multi-hop)**：专门针对涉及跨文件推理的长逻辑复杂问题。
- **语义分块与幻觉缓解 (HyDE & Semantic Chunking)**：提供比暴力文本截断更智能的切词与召回算法，解决漏召和召回噪音问题。

---

## 🛠 命令行快速上手

GoRAG 自带强大的命令行交互工具，方便运维及快速验证：

```bash
# 安装命令行
go install github.com/DotNetAge/gorag/cmd/gorag@latest

# 直接在终端索引指定文件
gorag index --api-key $OPENAI_API_KEY --file ./docs/architecture.md

# 命令行直连知识库查询
gorag query --api-key $OPENAI_API_KEY "你们的熔断器是如何工作的？"
```

## 🤝 参与贡献与社区

我们致力于在 Go 语言生态构建最强企业级 AI 底层基础设施。无论你是提 Issue 还是提交 PR，我们都非常欢迎！
- 详见我们的 [贡献指南](CONTRIBUTING.md)
- 如果这个项目对你的业务有启发或帮助，欢迎随手点个 **⭐️ Star**，这会对我们继续迭代开源非常有帮助！

## 📄 许可协议

本项目基于 MIT 协议开源。详细信息请参阅 [LICENSE](LICENSE) 文件。
