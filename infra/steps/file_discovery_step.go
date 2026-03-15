package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag/infra/indexing"
)

// FileDiscoveryStep 文件发现与验证步骤
type FileDiscoveryStep struct{}

// NewFileDiscoveryStep 创建文件发现步骤
func NewFileDiscoveryStep() *FileDiscoveryStep {
	return &FileDiscoveryStep{}
}

// Name 返回步骤名称
func (s *FileDiscoveryStep) Name() string {
	return "FileDiscovery"
}

// Execute 执行文件发现步骤（实现 gochat/pkg/pipeline.Step 接口）
func (s *FileDiscoveryStep) Execute(ctx context.Context, state *indexing.State) error {
	// 检查文件是否存在
	info, err := os.Stat(state.FilePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", state.FilePath)
	}

	// 更新状态中的元数据
	state.Metadata = indexing.Metadata{
		Source:   state.FilePath,
		FileName: filepath.Base(state.FilePath),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
	}

	return nil
}
