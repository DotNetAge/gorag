package parser

import (
	"context"
	"io"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
)

// Parser 解析器实现接口
type Parser interface {
	dataprep.Parser
	
	// Initialize 初始化解析器
	Initialize(options map[string]interface{}) error
	
	// Close 关闭解析器
	Close() error
}
