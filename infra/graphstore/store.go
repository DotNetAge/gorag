package graphstore

import (
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
)

// GraphStore 图存储实现接口
type GraphStore interface {
	abstraction.GraphStore

	// Initialize 初始化存储
	Initialize(options map[string]interface{}) error
}
