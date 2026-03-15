package abstraction

import "time"

// Metrics 定义了可观测性指标收集接口
type Metrics interface {
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

	// GetMetrics 获取所有指标数据
	GetMetrics() map[string]interface{}
}
