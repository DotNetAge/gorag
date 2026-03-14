# goRAG 整洁架构设计

## 目录结构

```
gorag/
├── pkg/                         # 公共包 (外部可导入)
│   ├── domain/                  # 领域模型 (Entities)
│   │   ├── entity/              # 核心实体
│   │   ├── valueobject/         # 值对象
│   │   └── repository/          # 仓储接口
│   ├── usecase/                 # 业务用例 (Use Cases)
│   │   ├── dataprep/            # 数据准备用例
│   │   ├── retrieval/           # 检索用例
│   │   └── evaluation/          # 评估用例
│   ├── interface/               # 接口适配器 (Interface Adapters)
│   │   ├── controller/          # 控制器
│   │   ├── gateway/             # 网关
│   │   └── presenter/           # 呈现器
│   ├── adapter/                 # 适配器
│   ├── di/                      # 依赖注入
│   └── utils/                   # 工具函数
├── infra/                       # 框架与驱动 (Frameworks & Drivers)
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

## 模块设计

### 1. 领域模型 (Entities)

#### 核心实体
- `Document`: 文档实体
- `Chunk`: 文档分块
- `Vector`: 向量表示
- `Query`: 查询实体
- `RetrievalResult`: 检索结果

#### 仓储接口
- `DocumentRepository`: 文档仓储接口
- `VectorRepository`: 向量仓储接口
- `GraphRepository`: 图仓储接口

### 2. 业务用例 (Use Cases)

#### 数据准备用例
- `DocumentParserUseCase`: 文档解析用例
- `ChunkerUseCase`: 文档分块用例
- `MiddlewarePipelineUseCase`: 中间件流水线用例

#### 检索用例
- `HyDEUseCase`: 假设性文档增强用例
- `RAGFusionUseCase`: RAG融合用例
- `ContextPruningUseCase`: 上下文剪枝用例

#### 评估用例
- `EvaluationUseCase`: 评估用例

### 3. 接口适配器 (Interface Adapters)

#### 控制器
- `DocumentController`: 文档控制器
- `QueryController`: 查询控制器
- `EvaluationController`: 评估控制器

#### 网关
- `VectorStoreGateway`: 向量存储网关
- `GraphStoreGateway`: 图存储网关
- `LLMGateway`: LLM网关

#### 呈现器
- `DocumentPresenter`: 文档呈现器
- `QueryPresenter`: 查询呈现器
- `EvaluationPresenter`: 评估呈现器

### 4. 框架与驱动 (Frameworks & Drivers)

#### 解析器实现
- `PDFParser`: PDF解析器
- `MarkdownParser`: Markdown解析器
- `JSONParser`: JSON解析器
- `TextParser`: 文本解析器
- `CodeParser`: 代码解析器

#### 向量存储实现
- `GovectorStore`: 本地向量存储
- `MilvusStore`: Milvus向量存储
- `QdrantStore`: Qdrant向量存储
- `PineconeStore`: Pinecone向量存储
- `WeaviateStore`: Weaviate向量存储

#### 图存储实现
- `MemoryGraphStore`: 内存图存储
- `Neo4jStore`: Neo4j图存储

#### 中间件实现
- `DesensitizationMiddleware`: 脱敏中间件
- `CleaningMiddleware`: 清洗中间件
- `ValidationMiddleware`: 验证中间件
