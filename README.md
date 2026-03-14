# goRAG - 高性能模块化 RAG 开发框架

## 概述

goRAG 是一款专为 Go 语言生态打造的 **模块化 RAG 应用开发框架**。它封装了 RAG 领域中极其复杂且难以优化的底层逻辑，并为关键节点提供高性能的默认实现。

## 核心特性

- **模块化设计**：基于整洁架构，提供清晰的接口定义和模块划分
- **高性能**：支持零拷贝数据流、并发索引、流式解析等高级特性
- **多模态支持**：支持图文同构混合检索
- **高级检索策略**：内置 HyDE、RAG-Fusion、上下文剪枝等高级检索技术
- **可扩展性**：提供丰富的插件接口，支持自定义解析器、存储后端等
- **与 goChat 深度集成**：利用 goChat 提供的 LLM/Embedding 接口与通用 Pipeline 开发框架

## 目录结构

```
gorag/
├── pkg/                         # 公共包 (外部可导入)
│   ├── domain/                  # 领域模型
│   │   ├── entity/              # 核心实体
│   │   ├── valueobject/         # 值对象
│   │   ├── repository/          # 仓储接口
│   │   └── abstraction/         # 存储抽象层
│   ├── usecase/                 # 业务用例
│   │   ├── dataprep/            # 数据准备用例
│   │   ├── retrieval/           # 检索用例
│   │   └── evaluation/          # 评估用例
│   ├── interface/               # 接口适配器
│   │   ├── controller/          # 控制器
│   │   ├── gateway/             # 网关
│   │   └── presenter/           # 呈现器
│   ├── adapter/                 # 适配器
│   ├── di/                      # 依赖注入
│   └── utils/                   # 工具函数
├── infra/                       # 框架与驱动
│   ├── parser/                  # 解析器实现
│   ├── vectorstore/             # 向量存储实现
│   ├── graphstore/              # 图存储实现
│   └── middleware/              # 中间件实现
├── examples/                    # 示例代码
├── cmd/                         # 命令行工具
├── config/                      # 配置
├── go.mod                       # Go 模块文件
└── go.sum                       # Go 依赖校验文件
```

## 快速开始

### 安装

```bash
go get github.com/DotNetAge/gorag
```

### 基本使用

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
	"github.com/DotNetAge/gorag/pkg/interface/gateway"
)

func main() {
	ctx := context.Background()
	
	// 1. 初始化解析器
	parser := initParser()
	
	// 2. 初始化分块器
	chunker := initChunker()
	
	// 3. 初始化中间件流水线
	pipeline := initMiddlewarePipeline()
	
	// 4. 初始化向量存储网关
	vectorStoreGateway := initVectorStoreGateway()
	
	// 5. 初始化LLM网关
	llmGateway := initLLMGateway()
	
	// 6. 初始化HyDE
	hyde := initHyDE(llmGateway)
	
	// 7. 初始化RAG-Fusion
	fusion := initRAGFusion()
	
	// 8. 初始化上下文剪枝
	contextPruning := initContextPruning()
	
	// 示例1: 解析文档
	document, err := parser.Parse(ctx, []byte("这是一个测试文档，包含一些示例内容。"), map[string]interface{}{
		"title": "测试文档",
		"author": "goRAG",
	})
	if err != nil {
		log.Fatalf("解析文档失败: %v", err)
	}
	fmt.Printf("解析文档成功: %s\n", document.ID)
	
	// 示例2: 分块文档
	chunks, err := chunker.Chunk(ctx, document, map[string]interface{}{
		"strategy": "semantic",
		"chunkSize": 100,
		"overlap": 20,
	})
	if err != nil {
		log.Fatalf("分块文档失败: %v", err)
	}
	fmt.Printf("分块成功，得到 %d 个分块\n", len(chunks))
	
	// 示例3: 处理分块
	processedChunks, err := pipeline.ProcessBatch(ctx, chunks)
	if err != nil {
		log.Fatalf("处理分块失败: %v", err)
	}
	fmt.Printf("处理分块成功，得到 %d 个处理后的分块\n", len(processedChunks))
	
	// 示例4: 向量化并存储
	for _, chunk := range processedChunks {
		vector, err := llmGateway.Embed(ctx, chunk.Content)
		if err != nil {
			log.Printf("向量化失败: %v", err)
			continue
		}
		
		vectorEntity := entity.NewVector(fmt.Sprintf("vec_%s", chunk.ID), vector, chunk.ID, chunk.Metadata)
		err = vectorStoreGateway.AddVector(ctx, vectorEntity)
		if err != nil {
			log.Printf("存储向量失败: %v", err)
			continue
		}
	}
	fmt.Println("向量化并存储成功")
	
	// 示例5: 查询
	query := entity.NewQuery("q1", "测试文档的内容是什么？", map[string]interface{}{})
	enhancedQuery, err := hyde.Enhance(ctx, query)
	if err != nil {
		log.Fatalf("增强查询失败: %v", err)
	}
	
	// 示例6: 检索
	queryVector, err := llmGateway.Embed(ctx, enhancedQuery.Text)
	if err != nil {
		log.Fatalf("向量化查询失败: %v", err)
	}
	
	vectors, scores, err := vectorStoreGateway.SearchVectors(ctx, queryVector, 5, nil)
	if err != nil {
		log.Fatalf("搜索向量失败: %v", err)
	}
	fmt.Printf("搜索到 %d 个结果\n", len(vectors))
	
	// 示例7: 上下文剪枝
	retrievalResult := entity.NewRetrievalResult("rr1", query.ID, []*entity.Chunk{}, scores, map[string]interface{}{})
	prunedResult, err := contextPruning.Prune(ctx, retrievalResult, 1000)
	if err != nil {
		log.Fatalf("剪枝上下文失败: %v", err)
	}
	fmt.Println("上下文剪枝成功")
	
	fmt.Println("goRAG 基本示例执行完成！")
}

// 初始化函数实现...
```

## 核心模块

### 1. 数据接入与解析 (Data Prep)

- **Pluggable Parser**：支持流式/异构解析
- **Chunker Engine**：支持语义/AST/图分块
- **Middleware Pipeline**：支持动态脱敏/清洗

### 2. 检索与增强核心积木

- **HyDE**：假设性文档增强
- **RAG-Fusion**：多路召回融合
- **Context Pruning**：上下文剪枝/压缩

### 3. 存储抽象层

- **VectorStore Interface**：向量存储适配
- **GraphStore Interface**：知识图谱适配

### 4. 框架与驱动

- **解析器实现**：PDF、Markdown、JSON、Text、Code等
- **向量存储实现**：govector、Milvus、Qdrant、Pinecone、Weaviate等
- **图存储实现**：内存图存储、Neo4j等
- **中间件实现**：脱敏、清洗、验证等

## 示例场景

- **QuickStart**：10 行代码的极简开箱
- **More Content**：50GB 维基百科的 O(1) 流式解析
- **Dynamic Data**：实时资讯流的无缝接入
- **Compliance**：企业内网数据的动态脱敏
- **Privacy-First**：拔掉网线的单二进制部署
- **Code Agent**：架构级源码重构助手
- **E-commerce**：意图优先的极长商品描述发现
- **Cross-Modal Search**：图文同构混合检索
- **Medical Hologram**：医疗全息档案
- **Bio-Scientist**：海量文献的全局隐性知识挖掘
- **Support Expert**：极低意图的口语化匹配
- **Hybrid Master**：专有名词的防漏搜引擎
- **Legal Pro**：超长法条的精准切片与降噪
- **Semantic Cache**：突发热点的高并发防穿透
- **Quant Causal Engine**：金融量化与因果推演
- **Auditor**：跨版本合同冲突审计
- **Support Center**：从情绪安抚到精准报修的自主客服
- **SRE Admin**：毫秒级故障根因溯源
- **Auto-Evaluator**：知识库质量自动化体检

## 发展路径

1. **Phase 1 (Infrastructure)**：完成核心接口规范与存储抽象，实现与 goChat 的深度融合
2. **Phase 2 (Standard Nodes)**：交付所有默认节点实现（如 HyDE, Joint Embedding, Agentic Router 等）
3. **Phase 3 (Cookbook)**：持续丰富核心示例，深耕医疗、金融、研发等垂直行业的“跨模态与异构数据”抄作业模板

## 贡献指南

欢迎通过以下方式贡献：

1. 提交 Issue 报告 bug 或建议新功能
2. 提交 Pull Request 改进代码
3. 参与讨论和文档完善

## 许可证

goRAG 采用 MIT 许可证。
