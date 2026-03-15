package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/infra/indexing"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
)

// ParseStep 流式解析步骤（支持多解析器）
type ParseStep struct {
	parsers []dataprep.Parser
}

// NewParseStep 创建解析步骤（支持多个解析器）
func NewParseStep(parsers ...dataprep.Parser) *ParseStep {
	return &ParseStep{
		parsers: parsers,
	}
}

// Name 返回步骤名称
func (s *ParseStep) Name() string {
	return "Parse"
}

// selectParser 根据文件扩展名选择合适的解析器
func (s *ParseStep) selectParser(filePath string) (dataprep.Parser, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	for _, parser := range s.parsers {
		supportedTypes := parser.GetSupportedTypes()
		for _, supportedType := range supportedTypes {
			if strings.ToLower(supportedType) == ext {
				return parser, nil
			}
		}
	}

	return nil, fmt.Errorf("no parser found for file extension: %s", ext)
}

// Execute 执行解析步骤（实现 gochat/pkg/pipeline.Step 接口）
func (s *ParseStep) Execute(ctx context.Context, state *indexing.State) error {
	if len(s.parsers) == 0 {
		return fmt.Errorf("no parsers configured")
	}

	// 根据文件类型选择合适的解析器
	parser, err := s.selectParser(state.FilePath)
	if err != nil {
		return err
	}

	file, err := os.Open(state.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	metadataMap := map[string]any{
		"source":   state.Metadata.Source,
		"filename": state.Metadata.FileName,
		"size":     state.Metadata.Size,
		"mod_time": state.Metadata.ModTime,
	}

	docChan, err := parser.ParseStream(ctx, file, metadataMap)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// 将解析结果传递到状态
	state.Documents = docChan

	return nil
}
