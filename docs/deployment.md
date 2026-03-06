# GoRAG 生产部署指南

## 1. 部署准备

### 1.1 系统要求
- **硬件要求**：
  - 至少 4GB RAM（推荐 8GB+）
  - 至少 2 CPU 核心（推荐 4 核心+）
  - 至少 50GB 磁盘空间
- **软件要求**：
  - Go 1.20+ 
  - 可选：Docker 20.10+ 
  - 可选：Kubernetes 1.24+ 

### 1.2 依赖服务
- **向量数据库**：根据选择的存储后端（Milvus、Qdrant、Weaviate、Pinecone）
- **LLM 服务**：OpenAI API、Anthropic API 或本地 Ollama 服务
- **监控系统**：Prometheus、Grafana（可选）

## 2. 环境配置

### 2.1 配置文件
创建 `config.yaml` 文件：

```yaml
# 基本配置
server:
  port: 8080
  host: 0.0.0.0

# RAG 引擎配置
rag:
  topK: 5
  chunkSize: 1000
  chunkOverlap: 100

# 嵌入模型配置
embedding:
  provider: "openai"  # 可选: openai, ollama
  openai:
    apiKey: "your-api-key"
    model: "text-embedding-ada-002"
  ollama:
    model: "qllama/bge-small-zh-v1.5:latest"

# LLM 配置
llm:
  provider: "openai"  # 可选: openai, anthropic, ollama
  openai:
    apiKey: "your-api-key"
    model: "gpt-4"
  anthropic:
    apiKey: "your-api-key"
    model: "claude-3-opus-20240229"
  ollama:
    model: "qwen3:7b"

# 向量存储配置
vectorstore:
  type: "milvus"  # 可选: memory, milvus, qdrant, weaviate, pinecone
  milvus:
    host: "localhost"
    port: 19530
  qdrant:
    url: "http://localhost:6333"
  weaviate:
    url: "http://localhost:8080"
  pinecone:
    apiKey: "your-api-key"
    environment: "gcp-starter"

# 日志配置
logging:
  level: "info"
  format: "json"

# 监控配置
metrics:
  enabled: true
  port: 9090
```

### 2.2 环境变量

| 环境变量 | 描述 | 默认值 |
|---------|------|--------|
| `GORAG_SERVER_PORT` | 服务器端口 | 8080 |
| `GORAG_RAG_TOPK` | 检索文档数量 | 5 |
| `GORAG_EMBEDDING_PROVIDER` | 嵌入模型提供商 | openai |
| `GORAG_LLM_PROVIDER` | LLM 提供商 | openai |
| `GORAG_VECTORSTORE_TYPE` | 向量存储类型 | memory |
| `GORAG_OPENAI_API_KEY` | OpenAI API 密钥 | - |
| `GORAG_ANTHROPIC_API_KEY` | Anthropic API 密钥 | - |
| `GORAG_PINECONE_API_KEY` | Pinecone API 密钥 | - |

## 3. 容器化部署

### 3.1 Docker 构建

创建 `Dockerfile`：

```dockerfile
FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gorag-server ./examples/web

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/gorag-server .
COPY config.yaml .

EXPOSE 8080
EXPOSE 9090

CMD ["./gorag-server"]
```

### 3.2 Docker Compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  gorag:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      - GORAG_OPENAI_API_KEY=${OPENAI_API_KEY}
      - GORAG_EMBEDDING_PROVIDER=openai
      - GORAG_LLM_PROVIDER=openai
      - GORAG_VECTORSTORE_TYPE=qdrant
    depends_on:
      - qdrant

  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"
    volumes:
      - qdrant_data:/qdrant/storage

volumes:
  qdrant_data:
```

### 3.3 运行容器

```bash
docker-compose up -d
```

## 4. 云服务部署

### 4.1 Kubernetes 部署

创建 `k8s/deployment.yaml`：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gorag-deployment
  labels:
    app: gorag
spec:
  replicas: 2
  selector:
    matchLabels:
      app: gorag
  template:
    metadata:
      labels:
        app: gorag
    spec:
      containers:
      - name: gorag
        image: your-registry/gorag:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: GORAG_OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: api-keys
              key: openai-api-key
        - name: GORAG_EMBEDDING_PROVIDER
          value: "openai"
        - name: GORAG_LLM_PROVIDER
          value: "openai"
        - name: GORAG_VECTORSTORE_TYPE
          value: "pinecone"
```

创建 `k8s/service.yaml`：

```yaml
apiVersion: v1
kind: Service
metadata:
  name: gorag-service
spec:
  selector:
    app: gorag
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

### 4.2 云函数部署

对于无服务器部署，可以使用 AWS Lambda 或 Google Cloud Functions：

```go
// main.go
package main

import (
	"context"
	"net/http"

	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/vectorstore/memory"
	"github.com/DotNetAge/gorag/embedding/openai"
	"github.com/DotNetAge/gorag/llm/openai"
)

var engine *rag.Engine

func init() {
	// 初始化 RAG 引擎
	var err error
	engine, err = rag.New(
		rag.WithParser(text.NewParser()),
		rag.WithVectorStore(memory.NewStore()),
		rag.WithEmbedder(openai.NewEmbedder(os.Getenv("OPENAI_API_KEY"))),
		rag.WithLLM(openai.NewClient(os.Getenv("OPENAI_API_KEY"))),
	)
	if err != nil {
		panic(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	// 处理请求
	// ...
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
```

## 5. 监控和维护

### 5.1 监控指标

GoRAG 提供以下 Prometheus 指标：

- `gorag_index_duration_seconds` - 索引操作耗时
- `gorag_query_duration_seconds` - 查询操作耗时
- `gorag_index_count` - 索引操作计数
- `gorag_query_count` - 查询操作计数
- `gorag_error_count` - 错误计数

### 5.2 日志管理

推荐使用 ELK Stack 或 Loki 进行日志收集和分析：

- **Elasticsearch**：存储和索引日志
- **Logstash**：处理和转换日志
- **Kibana**：可视化日志
- **Loki**：轻量级日志聚合系统

### 5.3 健康检查

实现健康检查端点：

```go
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

http.HandleFunc("/health", healthCheck)
```

## 6. 性能优化

### 6.1 索引优化

- **批量处理**：使用批量索引 API 减少网络往返
- **并行处理**：使用 goroutine 并行处理文档
- **缓存策略**：缓存嵌入向量避免重复计算

### 6.2 查询优化

- **TopK 调整**：根据文档库大小调整 TopK 值
- **查询缓存**：缓存常见查询的结果
- **异步处理**：使用流式响应提高用户体验

### 6.3 存储优化

- **向量压缩**：使用量化技术减少存储需求
- **索引优化**：根据查询模式优化向量索引
- **分区策略**：对大型文档库使用分区

## 7. 安全最佳实践

### 7.1 认证与授权

- **API 密钥管理**：使用环境变量或密钥管理服务存储 API 密钥
- **请求认证**：实现 API 密钥或 OAuth2 认证
- **权限控制**：基于角色的访问控制

### 7.2 数据安全

- **数据加密**：加密敏感数据
- **数据脱敏**：对个人身份信息进行脱敏
- **访问控制**：限制对向量数据库的访问

### 7.3 网络安全

- **HTTPS**：使用 TLS 加密传输
- **防火墙**：配置适当的防火墙规则
- **CORS**：正确配置跨域资源共享

### 7.4 依赖安全

- **依赖扫描**：定期扫描依赖漏洞
- **版本管理**：使用固定版本依赖
- **安全更新**：及时更新安全补丁

## 8. 故障排查

### 8.1 常见问题

| 问题 | 可能原因 | 解决方案 |
|------|---------|----------|
| 索引失败 | 嵌入模型错误 | 检查 API 密钥和网络连接 |
| 查询超时 | LLM 响应慢 | 增加超时设置，使用流式响应 |
| 内存不足 | 文档过大 | 调整 chunk 大小，增加内存 |
| 向量存储连接失败 | 网络问题或配置错误 | 检查连接字符串和网络设置 |

### 8.2 日志分析

使用结构化日志进行故障排查：

```json
{
  "level": "error",
  "time": "2024-01-01T12:00:00Z",
  "component": "rag",
  "operation": "index",
  "error": "failed to embed document",
  "details": {
    "document_id": "123",
    "error_code": "API_ERROR"
  }
}
```

## 9. 扩展策略

### 9.1 水平扩展

- **多实例部署**：使用负载均衡器分发请求
- **无状态设计**：确保服务无状态，支持水平扩展
- **自动缩放**：基于流量自动调整实例数量

### 9.2 垂直扩展

- **增加资源**：增加 CPU、内存和存储
- **优化配置**：调整 JVM 内存和连接池设置

### 9.3 服务拆分

- **微服务架构**：将 RAG 引擎拆分为多个微服务
- **功能分离**：将索引和查询服务分离
- **专用服务**：为不同的向量存储后端创建专用服务

## 10. 最佳实践总结

1. **从小规模开始**：先在测试环境验证，再逐步扩展
2. **监控先行**：部署前设置好监控和告警
3. **安全优先**：实施多层次安全措施
4. **持续优化**：根据实际使用情况调整配置
5. **备份策略**：定期备份向量数据库和配置
6. **灾备方案**：制定详细的灾难恢复计划

## 11. 部署清单

- [ ] 环境配置完成
- [ ] 依赖服务就绪
- [ ] 安全措施到位
- [ ] 监控系统配置
- [ ] 测试环境验证
- [ ] 部署计划制定
- [ ] 回滚方案准备
- [ ] 生产环境部署
- [ ] 性能监控启动
- [ ] 文档更新完成

## 12. 相关资源

- [API 参考](api.md)
- [高级检索策略](advanced-retrieval.md)
- [核心概念](core-concepts.md)
- [示例代码](../examples/)
- [GitHub 仓库](https://github.com/DotNetAge/gorag)