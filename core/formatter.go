package core

import (
	"io"
	"strings"
)

// Formatter 定义搜索结果格式化接口
type Formatter interface {
	// Format 格式化单个 Hit
	Format(hit *Hit) string

	// FormatAll 格式化多个 Hit
	FormatAll(hits []Hit) string

	// Write 格式化并写入输出流
	Write(w io.Writer, hits []Hit) error
}

// BaseFormatter 提供通用格式化方法
type BaseFormatter struct{}

func (f *BaseFormatter) Format(hit *Hit) string {
	return hit.Content
}

func (f *BaseFormatter) FormatAll(hits []Hit) string {
	var sb strings.Builder
	for i, hit := range hits {
		sb.WriteString(f.Format(&hit))
		if i < len(hits)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (f *BaseFormatter) Write(w io.Writer, hits []Hit) error {
	_, err := w.Write([]byte(f.FormatAll(hits)))
	return err
}
