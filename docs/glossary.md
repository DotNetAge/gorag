# 术语表

本文档定义GoRAG框架中使用的专业术语，帮助开发者快速理解相关概念。

## 核心概念

### RAG相关术语

| 术语 | 英文 | 解释 |
|------|------|------|
| RAG | Retrieval-Augmented Generation | 检索增强生成，一种结合信息检索和文本生成的AI技术，通过检索外部知识库增强LLM的生成能力 |
| VectorDB | Vector Database | 向量数据库，专门用于存储和检索高维向量的数据库系统 |
| GraphDB | Graph Database | 图数据库，用于存储实体和关系数据的数据库系统，支持图结构查询 |
| Embedding | Embedding | 嵌入，将文本、图片等数据转换为高维向量的过程 |
| Chunk | Chunk | 分块，将长文档切分为较小的语义单元，便于检索和处理 |
| Retrieval | Retrieval | 检索，根据查询从知识库中召回相关文档的过程 |
| Recall | Recall | 召回率，检索到的相关文档占所有相关文档的比例 |
| Precision | Precision | 精确率，检索到的相关文档占检索结果总数的比例 |

### GoRAG特有术语

| 术语 | 解释 |
|------|------|
| GoVector | GoRAG默认嵌入式VectorDB，纯Go实现，无需外部依赖 |
| GoGraph | GoRAG默认嵌入式GraphDB，纯Go实现，无需外部依赖 |
| nodeId | 统一ID，用于关联VectorDB和GraphDB中的数据，确保数据一致性 |
| 双管线架构 | 离线索引管线 + 检索管线的架构设计，实现索引构建与检索响应的解耦 |
| 离线索引管线 | 负责离线构建索引的管线，包括数据清洗、分块、向量化、实体提取等步骤 |
| 检索管线 | 负责在线响应用户查询的管线，包括查询预处理、检索召回、结果加工等步骤 |
| 函数式管道 | 将检索流程拆分为可插拔、可组合的函数，通过统一上下文Schema流转数据 |
| 上下文Schema | 固定的数据流转格式，所有函数通过Schema流转数据，结构固定，内容动态填充 |

### 高级RAG技术

| 术语 | 英文 | 解释 |
|------|------|------|
| GraphRAG | Graph-based RAG | 基于知识图谱的RAG，结合图检索和向量检索，支持多跳推理和复杂关系查询 |
| ParentDoc | Parent Document Retrieval | 父文档分层索引，将文档分为父块和子块，检索子块但返回父块，提供更完整的上下文 |
| Multi-modal RAG | Multi-modal RAG | 多模态RAG，支持文本、图片、音频等多种模态数据的检索和生成 |
| Agentic RAG | Agentic RAG | 智能体驱动的RAG，具备任务规划、工具调用能力的自主RAG系统 |
| Hybrid Retrieval | Hybrid Retrieval | 混合检索，结合向量检索、关键词检索、图检索等多种检索策略 |
| Reranking | Reranking | 重排序，对检索结果进行二次排序，提升结果相关性 |
| Query Rewriting | Query Rewriting | 查询改写，对用户查询进行预处理，提升检索效果 |
| Entity Linking | Entity Linking | 实体链接，将查询中的实体与知识库中的实体进行关联 |

## 数据结构术语

### 向量相关

| 术语 | 英文 | 解释 |
|------|------|------|
| Vector | Vector | 向量，高维数值数组，用于表示数据的语义特征 |
| Dimension | Dimension | 维度，向量的长度，如768维、1536维 |
| Similarity | Similarity | 相似度，衡量两个向量之间的相似程度 |
| Cosine Similarity | Cosine Similarity | 余弦相似度，计算两个向量夹角的余弦值，范围[-1, 1] |
| Euclidean Distance | Euclidean Distance | 欧氏距离，计算两个向量之间的直线距离 |
| Dot Product | Dot Product | 点积，两个向量对应位置元素的乘积之和 |
| ANN | Approximate Nearest Neighbor | 近似最近邻，一种高效的向量检索算法，牺牲少量精度换取速度 |
| HNSW | Hierarchical Navigable Small World | 分层导航小世界图，一种高效的ANN算法，支持快速向量检索 |
| IVF | Inverted File Index | 倒排文件索引，一种向量索引结构，通过聚类加速检索 |
| PQ | Product Quantization | 乘积量化，一种向量压缩技术，降低内存占用 |

### 图相关

| 术语 | 英文 | 解释 |
|------|------|------|
| Entity | Entity | 实体，知识图谱中的节点，如人物、地点、组织等 |
| Relation | Relation | 关系，知识图谱中的边，表示实体之间的关联 |
| Triple | Triple | 三元组，由(实体1, 关系, 实体2)组成的基本单元 |
| Subgraph | Subgraph | 子图，知识图谱的一个子集，包含部分节点和边 |
| Multi-hop Query | Multi-hop Query | 多跳查询，通过多个关系跳转查询相关实体 |
| Graph Traversal | Graph Traversal | 图遍历，按照一定规则访问图中的节点和边 |

### 索引相关

| 术语 | 英文 | 解释 |
|------|------|------|
| Index | Index | 索引，用于加速数据检索的数据结构 |
| Inverted Index | Inverted Index | 倒排索引，从内容到文档的映射，用于关键词检索 |
| Forward Index | Forward Index | 正排索引，从文档到内容的映射 |
| Index Building | Index Building | 索引构建，创建索引的过程 |
| Incremental Index | Incremental Index | 增量索引，仅更新新增或修改的数据，避免全量重建 |

## 性能指标术语

### 延迟指标

| 术语 | 英文 | 解释 |
|------|------|------|
| Latency | Latency | 延迟，从发起请求到收到响应的时间 |
| P50 | Percentile 50 | 50分位数，50%的请求延迟低于此值 |
| P90 | Percentile 90 | 90分位数，90%的请求延迟低于此值 |
| P99 | Percentile 99 | 99分位数，99%的请求延迟低于此值 |
| P99.9 | Percentile 99.9 | 99.9分位数，99.9%的请求延迟低于此值 |
| QPS | Queries Per Second | 每秒查询数，衡量系统吞吐量的指标 |
| Throughput | Throughput | 吞吐量，单位时间内处理的请求数量 |

### 质量指标

| 术语 | 英文 | 解释 |
|------|------|------|
| Recall@K | Recall at K | 前K个结果中的召回率 |
| Precision@K | Precision at K | 前K个结果中的精确率 |
| F1 Score | F1 Score | 精确率和召回率的调和平均 |
| MRR | Mean Reciprocal Rank | 平均倒数排名，衡量第一个相关文档的排名 |
| NDCG | Normalized Discounted Cumulative Gain | 归一化折损累积增益，衡量排序质量 |
| MAP | Mean Average Precision | 平均精确率均值，衡量整体检索质量 |
| Faithfulness | Faithfulness | 忠实度，生成的答案与检索内容的一致性 |
| Relevance | Relevance | 相关性，生成的答案与查询的相关程度 |

## 架构术语

### 设计模式

| 术语 | 英文 | 解释 |
|------|------|------|
| Pipeline | Pipeline | 管道，将复杂流程拆分为多个阶段，依次执行 |
| Driver | Driver | 驱动，用于对接外部系统的适配器 |
| Plugin | Plugin | 插件，可插拔的功能模块 |
| Middleware | Middleware | 中间件，在请求处理过程中插入的处理逻辑 |
| Cache | Cache | 缓存，临时存储频繁访问的数据，提升性能 |
| Circuit Breaker | Circuit Breaker | 熔断器，防止故障扩散的保护机制 |
| Rate Limiter | Rate Limiter | 限流器，控制请求速率，防止系统过载 |

### 部署术语

| 术语 | 英文 | 解释 |
|------|------|------|
| Microservices | Microservices | 微服务，将应用拆分为多个独立部署的服务 |
| Container | Container | 容器，轻量级的虚拟化技术，如Docker |
| Kubernetes | Kubernetes | 容器编排平台，用于管理容器化应用 |
| Load Balancer | Load Balancer | 负载均衡器，将请求分发到多个实例 |
| High Availability | High Availability | 高可用，系统持续提供服务的能力 |
| Failover | Failover | 故障转移，主节点故障时切换到备用节点 |
| Scaling | Scaling | 扩展，增加系统资源以处理更多负载 |
| Horizontal Scaling | Horizontal Scaling | 水平扩展，增加实例数量 |
| Vertical Scaling | Vertical Scaling | 垂直扩展，增加单实例资源 |

## 算法术语

### 检索算法

| 术语 | 英文 | 解释 |
|------|------|------|
| BM25 | BM25 | 一种关键词检索算法，基于词频和逆文档频率 |
| TF-IDF | Term Frequency-Inverse Document Frequency | 词频-逆文档频率，衡量词语重要性的指标 |
| Semantic Search | Semantic Search | 语义搜索，基于向量相似度的检索 |
| Dense Retrieval | Dense Retrieval | 稠密检索，使用向量进行检索 |
| Sparse Retrieval | Sparse Retrieval | 稀疏检索，使用关键词进行检索 |
| Hybrid Search | Hybrid Search | 混合搜索，结合稠密检索和稀疏检索 |

### 融合算法

| 术语 | 英文 | 解释 |
|------|------|------|
| RRF | Reciprocal Rank Fusion | 倒数排名融合，一种多路召回结果融合算法 |
| Weighted Fusion | Weighted Fusion | 加权融合，根据权重融合多路召回结果 |
| Borda Count | Borda Count | 波达计数，一种排名融合算法 |

### 优化算法

| 术语 | 英文 | 解释 |
|------|------|------|
| Quantization | Quantization | 量化，将高精度数值转换为低精度数值，降低存储和计算成本 |
| Pruning | Pruning | 剪枝，删除不重要的数据或参数，降低模型复杂度 |
| Distillation | Distillation | 蒸馏，将大模型的知识迁移到小模型 |
| Fine-tuning | Fine-tuning | 微调，在预训练模型基础上进行训练，适配特定任务 |

## 工具与框架术语

### 数据处理工具

| 术语 | 英文 | 解释 |
|------|------|------|
| ETL | Extract, Transform, Load | 数据抽取、转换、加载，数据处理的标准流程 |
| OCR | Optical Character Recognition | 光学字符识别，将图片中的文字转换为文本 |
| NER | Named Entity Recognition | 命名实体识别，从文本中识别实体 |
| POS Tagging | Part-of-Speech Tagging | 词性标注，为词语标注词性 |

### 评估工具

| 术语 | 英文 | 解释 |
|------|------|------|
| RAGAS | RAGAS | 一个开源的RAG评估框架，提供多种评估指标 |
| HiCBench | HiCBench | 一个高性能基准测试工具 |
| A/B Testing | A/B Testing | A/B测试，对比两个版本的效果 |
| Ground Truth | Ground Truth | 标准答案，用于评估的基准数据 |

### 监控工具

| 术语 | 英文 | 解释 |
|------|------|------|
| Prometheus | Prometheus | 一个开源的监控和告警系统 |
| Grafana | Grafana | 一个开源的可视化平台，常与Prometheus配合使用 |
| ELK Stack | Elasticsearch, Logstash, Kibana | 日志聚合和分析平台 |
| Tracing | Tracing | 追踪，记录请求的完整调用链路 |

## 安全术语

| 术语 | 英文 | 解释 |
|------|------|------|
| Authentication | Authentication | 认证，验证用户身份 |
| Authorization | Authorization | 授权，验证用户权限 |
| RBAC | Role-Based Access Control | 基于角色的访问控制 |
| API Key | API Key | API密钥，用于认证的密钥 |
| JWT | JSON Web Token | 一种用于认证的令牌标准 |
| TLS | Transport Layer Security | 传输层安全协议，用于加密通信 |
| Encryption | Encryption | 加密，将明文转换为密文 |
| Decryption | Decryption | 解密，将密文转换为明文 |
| Audit Log | Audit Log | 审计日志，记录用户操作 |
| Data Masking | Data Masking | 数据脱敏，隐藏敏感信息 |

## 其他术语

| 术语 | 英文 | 解释 |
|------|------|------|
| LLM | Large Language Model | 大语言模型，如GPT、Claude等 |
| Prompt | Prompt | 提示词，输入给LLM的文本 |
| Context Window | Context Window | 上下文窗口，LLM一次能处理的最大文本长度 |
| Token | Token | 词元，LLM处理文本的基本单位 |
| Temperature | Temperature | 温度参数，控制LLM生成的随机性 |
| Top-K Sampling | Top-K Sampling | Top-K采样，从概率最高的K个词中选择 |
| Top-P Sampling | Top-P Sampling | Top-P采样，从累积概率达到P的词中选择 |
| Hallucination | Hallucination | 幻觉，LLM生成的不符合事实的内容 |
| Grounding | Grounding | 基于事实，确保生成内容有据可查 |
| Context | Context | 上下文，提供给LLM的背景信息 |

## 缩写对照表

| 缩写 | 全称 | 中文 |
|------|------|------|
| RAG | Retrieval-Augmented Generation | 检索增强生成 |
| ANN | Approximate Nearest Neighbor | 近似最近邻 |
| HNSW | Hierarchical Navigable Small World | 分层导航小世界 |
| IVF | Inverted File Index | 倒排文件索引 |
| PQ | Product Quantization | 乘积量化 |
| BM25 | Best Matching 25 | 最佳匹配25 |
| TF-IDF | Term Frequency-Inverse Document Frequency | 词频-逆文档频率 |
| RRF | Reciprocal Rank Fusion | 倒数排名融合 |
| MRR | Mean Reciprocal Rank | 平均倒数排名 |
| NDCG | Normalized Discounted Cumulative Gain | 归一化折损累积增益 |
| MAP | Mean Average Precision | 平均精确率均值 |
| QPS | Queries Per Second | 每秒查询数 |
| API | Application Programming Interface | 应用程序接口 |
| REST | Representational State Transfer | 表述性状态转移 |
| gRPC | Google Remote Procedure Call | Google远程过程调用 |
| JSON | JavaScript Object Notation | JavaScript对象表示法 |
| YAML | YAML Ain't Markup Language | YAML不是标记语言 |
| TLS | Transport Layer Security | 传输层安全 |
| JWT | JSON Web Token | JSON网络令牌 |
| RBAC | Role-Based Access Control | 基于角色的访问控制 |
| LLM | Large Language Model | 大语言模型 |
| NER | Named Entity Recognition | 命名实体识别 |
| OCR | Optical Character Recognition | 光学字符识别 |
| ETL | Extract, Transform, Load | 抽取、转换、加载 |
