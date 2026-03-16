# Steps 组合选择指南

本指南帮助你根据具体需求选择合适的 Steps 组合策略。

## 快速决策树

```
你的需求是什么？
│
├─ 简单问答，快速响应
│  └─> Native RAG (01_native_rag)
│
├─ 大规模知识库，高召回率
│  └─> Hybrid RAG (02_hybrid_rag)
│
├─ 复杂问题，需要多轮检索
│  └─> Agentic RAG (03_agentic_rag)
│
├─ 实体关系推理
│  └─> Graph RAG (04_graph_rag)
│
└─ 高质量要求，多角度验证
   └─> Multi-Agent RAG (05_multiagent_rag)
```

## 详细对比表

| 维度 | Native | Hybrid | Agentic | Graph | Multi-Agent |
|------|--------|--------|---------|-------|-------------|
| **Steps 数量** | 3-4 | 6-8 | 7-9 | 4-6 | 5-7 |
| **检索轮数** | 1 | 1+ 融合 | 多轮 | 1+N-hop | 并行多路 |
| **延迟** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ |
| **准确率** | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **召回率** | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **成本** | $ | $$ | $$$ | $$ | $$$$ |
| **实现复杂度** | 简单 | 中等 | 复杂 | 中等 | 复杂 |

## 场景匹配矩阵

### 1. 客服机器人

**需求特点**:
- 响应时间 < 1 秒
- 常见问题为主
- 答案准确性要求中等

**推荐**: **Native RAG** + 语义缓存

```go
searcher := native.New(
	native.WithEmbedding(embedder),
	native.WithVectorStore(store),
	native.WithGenerator(llm),
	native.WithSemanticCache(cache),  // 关键：降低延迟
	native.WithTopK(5),
)
```

**预期效果**:
- P99 延迟：< 800ms
- 缓存命中率：60%+
- 单次查询成本：$0.002

---

### 2. 企业知识库搜索

**需求特点**:
- 文档量大（10 万 +）
- 查询多样化
- 需要高召回率

**推荐**: **Hybrid RAG**

```go
searcher := hybrid.New(
	hybrid.WithEmbedding(embedder),
	hybrid.WithVectorStore(store),
	hybrid.WithSparseStore(bm25),     // 关键：关键词匹配
	hybrid.WithFilterExtractor(extractor),
	hybrid.WithFusionEngine(fusion),
	hybrid.WithDenseTopK(20),
	hybrid.WithSparseTopK(20),
	hybrid.WithFusionTopK(15),
)
```

**预期效果**:
- 召回率提升：40% vs Native
- 关键词查询 F1: 0.85+
- 支持元数据过滤

---

### 3. 研究助手 / 论文问答

**需求特点**:
- 问题复杂，需要多跳推理
- 对准确性要求极高
- 可接受较长响应时间（5-10 秒）

**推荐**: **Agentic RAG**

```go
searcher := agentic.New(
	agentic.WithReasoner(reasoner),
	agentic.WithActionSelector(selector),
	agentic.WithGenerator(generator),
	agentic.WithRetriever(retriever),
	agentic.WithMaxIterations(7),      // 允许最多 7 轮
	agentic.WithSelfRAGJudge(judge, true),  // 严格验证
)
```

**预期效果**:
- 复杂问题准确率：85%+
- 平均迭代轮数：3-4 轮
- Self-RAG 通过率：90%+

---

### 4. 医疗/法律专业问答

**需求特点**:
- 容错率极低
- 需要多方验证
- 追溯信息来源

**推荐**: **Multi-Agent RAG**

```go
searcher := multiagent.New(
	multiagent.WithAgent(researcher),  // 信息收集
	multiagent.WithAgent(critic),      // 质量评估
	multiagent.WithAgent(writer),      // 答案组织
	multiagent.WithCoordinator(coord), // 任务分解
)
```

**工作流程**:
```
用户问题
  ↓
Coordinator 分解任务
  ↓
Researcher 检索 → Critic 评估 → Writer 整合
  ↓
最终答案（附引用来源）
```

---

### 5. 知识图谱增强问答

**需求特点**:
- 涉及实体关系
- 需要推理能力
- 结构化数据丰富

**推荐**: **Graph RAG (Local)**

```go
searcher := graphlocal.New(
	graphlocal.WithEntityExtractor(extractor),
	graphlocal.WithGraphSearcher(localSearcher),
	graphlocal.WithGenerator(generator),
	graphlocal.WithVectorSupplement(embedder, store, fusion),
	graphlocal.WithMaxHops(2),  // 2 跳邻居
	graphlocal.WithTopK(10),
)
```

**适用场景**:
- "A 和 B 之间有什么关系？"
- "找出所有与 X 相关的公司"
- "Y 技术的发明者还创造了什么？"

## 混合策略实战

### 策略 1: 分级降级

```go
func Search(ctx context.Context, query string) (string, error) {
	// 第 1 层：尝试 Native RAG（最快）
	if isSimpleQuery(query) {
		return nativeSearcher.Search(ctx, query)
	}
	
	// 第 2 层：Hybrid RAG（平衡）
	if hasKeywords(query) {
		return hybridSearcher.Search(ctx, query)
	}
	
	// 第 3 层：Agentic RAG（最准确）
	return agenticSearcher.Search(ctx, query)
}
```

### 策略 2: 智能路由

```go
func routeQuery(query string) string {
	// 使用 LLM 分类查询复杂度
	classification := llm.Classify(query, []string{
		"simple",    // Native
		"complex",   // Hybrid
		"multi-hop", // Agentic
	})
	
	switch classification {
	case "simple":
		return "native"
	case "complex":
		return "hybrid"
	default:
		return "agentic"
	}
}
```

### 策略 3: A/B 测试框架

```go
func ABTestSearch(ctx context.Context, query string) (string, error) {
	// 同时运行两种策略
	nativeAnswer, _ := nativeSearcher.Search(ctx, query)
	hybridAnswer, _ := hybridSearcher.Search(ctx, query)
	
	// 使用评估器选择更好的答案
	scores := evaluator.Evaluate(query, []string{nativeAnswer, hybridAnswer})
	
	if scores[0] > scores[1] {
		return nativeAnswer, nil
	}
	return hybridAnswer, nil
}
```

## 成本估算公式

### Native RAG
```
单次查询成本 = Embedding($0.0001) + LLM($0.002) = $0.0021
```

### Hybrid RAG
```
单次查询成本 = Embedding($0.0001) + BM25($0.0001) 
             + Fusion(计算免费) + LLM($0.003) = $0.0032
```

### Agentic RAG
```
单次查询成本 = (Embedding + LLM) × 平均迭代次数
             = ($0.0001 + $0.002) × 3.5 = $0.0074
```

## 性能基准测试

### 测试条件
- 数据集：10 万文档
- 查询集：1000 个问题
- 硬件：8 核 CPU, 32GB RAM

### 结果对比

| 模式 | P50 延迟 | P99 延迟 | 吞吐量 (QPS) |
|------|---------|---------|-------------|
| Native | 320ms | 680ms | 45 |
| Hybrid | 580ms | 1.2s | 28 |
| Agentic | 2.1s | 4.5s | 8 |
| Graph | 750ms | 1.5s | 22 |

## 调试和监控

### 1. 指标收集

```go
metrics := observability.NewPrometheusMetrics()

searcher := native.New(
	native.WithMetrics(metrics),
)

// 暴露 /metrics 端点
http.Handle("/metrics", promhttp.Handler())
```

**关键指标**:
- `rag_search_duration_seconds`
- `rag_retrieved_chunks_count`
- `rag_cache_hit_total`
- `rag_selfrag_score`

### 2. 链路追踪

```go
tracer := otel.Tracer("gorag")

ctx, span := tracer.Start(ctx, "RAG.Search")
defer span.End()

answer, err := searcher.Search(ctx, query)
```

### 3. 日志记录

```go
logger := logging.NewJSONLogger(os.Stdout)
logger = logger.With("request_id", requestID)

searcher := agentic.New(
	agentic.WithLogger(logger),
)
```

## 最佳实践清单

### ✅ 必做项

- [ ] 启用语义缓存（降低成本 50%+）
- [ ] 设置合理的 TopK（避免上下文爆炸）
- [ ] 添加超时控制（防止无限循环）
- [ ] 实现降级策略（服务不可用时 fallback）
- [ ] 监控关键指标（延迟、准确率、成本）

### ❌ 避免事项

- [ ] 不要盲目堆叠 Steps（增加复杂度无收益）
- [ ] 不要忽略错误处理（网络异常、LLM 失败）
- [ ] 不要硬编码参数（使用配置中心）
- [ ] 不要跳过评估（上线前必须 A/B 测试）
- [ ] 不要忽视用户体验（延迟感知）

## 学习路径建议

```
Week 1-2: Native RAG
  └─ 理解基本 Pipeline 组装
  └─ 掌握 VectorSearch 和 Generation
  
Week 3-4: Hybrid RAG
  └─ 学习 RRF 融合算法
  └─ 实践 BM25 索引构建
  
Week 5-6: Agentic RAG
  └─ 深入 ReAct 决策循环
  └─ 调试多轮推理过程
  
Week 7-8: 生产优化
  └─ 性能调优和成本控制
  └─ 监控和告警系统搭建
```

## 参考资源

- [Native RAG 示例](./01_native_rag/)
- [Hybrid RAG 示例](./02_hybrid_rag/)
- [Agentic RAG 示例](./03_agentic_rag/)
- [RAG 评估指南](../../docs/evaluation.md)
- [性能优化手册](../../docs/performance.md)
