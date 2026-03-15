package abstraction

import "time"

// Metrics 定义了可观测性指标收集接口，覆盖索引阶段与查询阶段。
type Metrics interface {
	// --- 索引阶段 ---

	// RecordIndexingDuration 记录文件索引耗时
	RecordIndexingDuration(file string, duration time.Duration)

	// RecordParsingErrors 记录解析错误
	RecordParsingErrors(file string, err error)

	// RecordEmbeddingCount 记录 embedding 数量
	RecordEmbeddingCount(count int)

	// RecordVectorStoreOperations 记录向量存储操作
	RecordVectorStoreOperations(op string, count int)

	// RecordGraphOperations 记录图存储操作
	RecordGraphOperations(op string, count int)

	// --- 查询（Search）阶段 ---

	// RecordSearchDuration 记录一次 Search 调用的端到端耗时。
	// searcher 为 Searcher 类型标识（如 "native"、"hybrid"）。
	RecordSearchDuration(searcher string, duration time.Duration)

	// RecordSearchError 记录一次 Search 调用失败。
	RecordSearchError(searcher string, err error)

	// RecordSearchResult 记录一次 Search 调用成功，并附带检索到的文档块数量。
	RecordSearchResult(searcher string, chunkCount int)

	// GetMetrics 获取所有指标数据
	GetMetrics() map[string]interface{}
}

// NoopMetrics is a no-op implementation of Metrics, suitable for testing and
// as the built-in default when no metrics collector is configured.
type NoopMetrics struct{}

func (NoopMetrics) RecordIndexingDuration(string, time.Duration) {}
func (NoopMetrics) RecordParsingErrors(string, error)            {}
func (NoopMetrics) RecordEmbeddingCount(int)                     {}
func (NoopMetrics) RecordVectorStoreOperations(string, int)      {}
func (NoopMetrics) RecordGraphOperations(string, int)            {}
func (NoopMetrics) RecordSearchDuration(string, time.Duration)   {}
func (NoopMetrics) RecordSearchError(string, error)              {}
func (NoopMetrics) RecordSearchResult(string, int)               {}
func (NoopMetrics) GetMetrics() map[string]interface{}           { return nil }
