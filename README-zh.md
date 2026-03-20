<div align="center">
  <h1>🦖 GoRAG</h1>
  <p><b>工业级、高性能、模块化 Go 语言 RAG 专家框架</b></p>
  
  [![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
  [![Go Version](https://img.shields.io/badge/go-1.24%2B-blue.svg)](https://golang.org)
  
  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

**GoRAG** 是一个专为大规模 AI 工程设计的生产级 RAG（检索增强生成）框架。不同于复杂的“黑盒”框架，GoRAG 提供了一个 **透明、基于流水线 (Pipeline) 的架构**，将 Go 的原生并发优势与前沿的 RAG 模式深度结合。

从具备自动三元组提取能力的 **GraphRAG**，到具备自我修正能力的 **Agentic RAG**，GoRAG 致力于帮助开发者将 AI 应用从“实验脚本”快速推进到“生产服务”。

## ✨ 为什么选择 GoRAG？

- 🚀 **性能至上**: 内置高性能并发 Worker 和 `O(1)` 内存效率的流式解析器。轻松应对 TB 级知识库索引。
- 🏗️ **透明流水线架构**: 基于 `gochat/pkg/pipeline`。每个检索步骤都清晰、可追踪且可插拔，告别深层继承与黑盒逻辑。
- 🧠 **智能意图路由**: 根据用户意图，自动在向量检索 (Vector)、图检索 (Graph) 或全局汇总 (Global) 策略间进行最优调度。
- 🕸️ **进阶 GraphRAG**: 原生支持 **Neo4j**、**SQLite (Zero-CGO)** 和 **BoltDB**。内置 LLM 驱动的知识图谱自动构建引擎。
- 🔭 **全链路可观测性**: 针对所有核心 Retriever 和 Step 的分布式追踪。精准掌握每一毫秒时间与每一个 Token 的去向。
- 📊 **企业级评测协议**: 内置标准化的评测指标计算 (RAGAS 风格)，包括 **忠实度 (Faithfulness)**、**答案相关性** 和 **上下文精准度**。

---

## 🧰 RAG “专家”生态

GoRAG 不仅仅提供工具，更将**最佳实践**固化为标准组件：

| 检索策略 | 适用场景 | 核心特性 |
|----------|-------------|--------------|
| **Native RAG** | 标准语义搜索 | 纯向量、高速、低成本 |
| **Graph RAG** | 复杂关系推理 | 实体、三元组、多跳推理 |
| **Self-RAG** | 高精度要求场景 | 自我反思、幻觉检测 |
| **CRAG** | 处理模糊查询 | 质量评估、自动回退至 Web 搜索 |
| **Fusion RAG**| 多维度复杂问题 | 查询重写、RRF 排序融合 |
| **Smart Router**| 动态工作负载 | 基于意图的自动分流调度 |

---

## 🚀 快速开始

### 安装

```bash
go get github.com/DotNetAge/gorag
```

### 1. 智能路由：基于意图的自动调度
针对领域事实使用向量检索，针对关系推理自动切换至图检索：

```go
package main

import (
    "context"
    "github.com/DotNetAge/gorag/pkg/retriever/agentic"
    "github.com/DotNetAge/gorag/pkg/retriever/graph"
    "github.com/DotNetAge/gorag/pkg/retriever/native"
)

func main() {
    // 1. 初始化不同的检索器
    vectorRet := native.NewRetriever(vectorStore, embedder, llm)
    graphRet := graph.NewRetriever(vectorStore, graphStore, embedder, llm)

    // 2. 创建智能路由器
    router := agentic.NewSmartRouter(
        classifier, 
        map[core.IntentType]core.Retriever{
            core.IntentRelational: graphRet,  // 关系类问题使用 GraphRAG
            core.IntentDomain:     vectorRet, // 事实类问题使用向量检索
        },
        vectorRet, // 默认回退
        logger,
    )

    // 3. 直接提问，路由器会处理“如何检索”
    results, _ := router.Retrieve(ctx, []string{"项目 X 与人员 Y 之间有什么关系？"}, 5)
    fmt.Println(results[0].Answer)
}
```

### 2. 自动化知识图谱索引
一键将非结构化文本转化为可查询的知识图谱：

```go
// 初始化三元组提取索引步骤
triplesStep := indexing.NewTriplesStep(llm, graphStore)

// 处理文档 - GoRAG 自动提取 (主体, 谓语, 客体) 并写入图数据库
err := indexer.IndexDirectory(ctx, "./docs", true)
```

### 3. 生产级观测与评测
利用内置工具监控每一步并量化检索质量：

```go
// 对检索器运行基准测试
report, _ := evaluation.RunBenchmark(ctx, retriever, judge, testCases, 5)
fmt.Println(report.Summary())
// 输出: 平均忠实度: 0.92, 答案相关性: 0.88, 上下文精准度: 0.85
```

---

## ⚡ 技术规范与标准

- **Go 1.24+**: 拥抱最新的 Go 语言特性。
- **Zero-CGO SQLite**: 采用 `modernc.org/sqlite`，实现无痛苦的跨平台交叉编译。
- **整洁架构**: 严格分离接口 (`pkg/core`) 与具体实现。
- **模块化 Step**: 所有核心步骤（如 `hyde`, `rerank`, `fuse`, `prune`）均可在自定义 Pipeline 中复用。

## 🤝 参与贡献
我们致力于构建 Go 生态最强大的 AI 基础设施。无论是增加新的 `VectorStore` 驱动，还是改进 `Parser`，我们都欢迎您的 PR！
- 请参考 [贡献指南](CONTRIBUTING.md)。

## 📄 许可协议
GoRAG 采用 [MIT License](LICENSE) 开源协议。
