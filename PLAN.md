# GoRAG 代码质量改善计划

## 📋 计划概述

根据代码走查结果，制定以下改善计划，重点解决测试失败问题和代码质量隐患。

---

## 🔴 P0 - 必须修复 (测试失败)

### 1. 修复 Unicode 处理问题

**文件**: `pkg/indexing/chunker/character_chunker.go`

**问题**: `TestCharacterChunker_chunkText/unicode_text` 失败，中文字符按字节而非字符分割

**解决方案**:
- 使用 `[]rune(str)` 而非 `[]byte(str)` 进行字符计数
- 验证 `chunkText` 方法的 Unicode 边界处理

---

### 2. 修复 ParserRegistry 测试失败

**文件**: `pkg/indexing/parser/config/types/registry.go`

**问题**:
- `TestParserRegistry` 返回错误数量
- `TestDefaultParser` 找不到 parser

**解决方案**:
- 检查 `EnsureInitialized` 中 `once.Do` 与 `lock` 的交互问题
- 确保 `GetAllFactories` 的去重逻辑正确
- 验证 `DefaultParser` 的 ParserType 映射

---

### 3. 修复 Milvus 向量维度不匹配

**文件**: `pkg/indexing/vectorstore/milvus/store_test.go`

**问题**: 测试使用维度 8 而非预期的 1536

**解决方案**:
- 检查测试向量生成逻辑
- 确保使用正确的 embedding dimension

---

## 🟡 P1 - 高优先级 (架构问题)

### 4. 增强错误处理一致性

**文件**: 多处

**问题**: 关键路径错误被静默处理

**解决方案**:
- 在 `graph/retriever.go` 的 `entityExtractionStep` 中增加错误传播选项
- 在 `indexer/builder.go` 中确保 goroutine 正确传递 context 取消信号
- 添加错误处理策略配置（fail-fast vs continue）

---

### 6. 改进 DI 容器全局状态

**文件**: `pkg/di/container.go`

**问题**: `ResetForTesting` 重置 `sync.Once` 的做法不安全

**解决方案**:
- 移除 `ResetForTesting` 对 `once` 的直接重置
- 使用测试辅助函数在测试前清理容器内容
- 添加 `Container.Clear()` 调用确保清洁状态

---

## 🟢 P2 - 中优先级 (代码质量)

### 7. 改进日志实现

**文件**: `pkg/logging/logger.go`

**解决方案**:
- 添加日志级别过滤
- 支持结构化日志字段
- 可选的 JSON 格式输出
- 考虑集成 Zap 作为可选实现

---

### 8. 清理未使用参数

**文件**: `pkg/retriever/graph/retriever.go`

**问题**: `DefaultGraphRetriever` 中 options 参数未完全使用

**解决方案**:
- 审查并清理 options 结构
- 移除未使用的字段或实现相应功能

---

### 9. 添加缺失的导出函数文档

**文件**: `gorag.go`

**问题**: `buildRAG` 函数缺少详细注释

**解决方案**:
- 添加函数文档注释
- 说明参数含义和返回值
- 添加使用示例

---

## 📊 任务分解

| 任务 | 优先级 | 估计工时 | 依赖 |
|------|--------|----------|------|
| 修复 Unicode chunker | P0 | 1h | 无 |
| 修复 ParserRegistry | P0 | 2h | 无 |
| 修复 Milvus 测试 | P0 | 1h | 无 |
| 统一 Parser 接口 | P1 | 3h | 任务1,2 |
| 增强错误处理 | P1 | 2h | 无 |
| 改进 DI 容器 | P1 | 1h | 无 |
| 改进日志实现 | P2 | 2h | 无 |
| 清理未使用参数 | P2 | 0.5h | 无 |
| 添加文档注释 | P2 | 1h | 无 |

---

## ✅ 验收标准

1. 所有测试通过 (`go test ./...`)
2. 无新增编译警告
3. Parser 接口统一
4. 错误处理行为一致可预测
5. 代码注释覆盖率提升

---

## 📅 执行顺序

```
第一阶段 (P0 - 立即执行):
├── 1. 修复 Unicode chunker
├── 2. 修复 ParserRegistry
└── 3. 修复 Milvus 测试

第二阶段 (P1 - 尽快执行):
├── 4. 统一 Parser 接口
├── 5. 增强错误处理
└── 6. 改进 DI 容器

第三阶段 (P2 - 计划执行):
├── 7. 改进日志实现
├── 8. 清理未使用参数
└── 9. 添加文档注释
```
