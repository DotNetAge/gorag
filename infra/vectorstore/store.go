package vectorstore

import (
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
)

// VectorStore 向量存储实现接口
type VectorStore interface {
	abstraction.VectorStore
	
	// Initialize 初始化存储
	Initialize(options map[string]interface{}) error
}
