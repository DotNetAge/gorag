package indexing

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// State 索引管线的状态类型（用于 gochat/pkg/pipeline 的泛型参数）
type State struct {
	Ctx         context.Context
	FilePath    string
	Documents   <-chan *entity.Document
	Chunks      <-chan *entity.Chunk
	Vectors     []*entity.Vector
	Metadata    Metadata
	TotalChunks int
}

// Metadata 文件元数据
type Metadata struct {
	Source   string
	FileName string
	Size     int64
	ModTime  interface{}
}

// DefaultState 创建默认的索引管线状态
func DefaultState(ctx context.Context, filePath string) *State {
	return &State{
		Ctx:      ctx,
		FilePath: filePath,
		Metadata: Metadata{
			Source:   filePath,
			FileName: filePath,
		},
	}
}
