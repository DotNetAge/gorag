package core

import (
	"io"
	"strings"
)

// Formatter 定义搜索结果格式化接口。
//
// 适配说明：
//   - Hit 持有 Chunks/Nodes/Edges 三类集合的容器
//   - Search 返回单个 *Hit（不再返回 []Hit）
//   - Format/FormatAll/Write 统一接收 *Hit，内部遍历 hit.Chunks
type Formatter interface {
	// Format 格式化单个 Hit 容器（遍历其 Chunks 拼接 Content）
	Format(hit *Hit) string

	// FormatAll 格式化 Hit 容器中的全部 Chunks
	FormatAll(hit *Hit) string

	// Write 格式化并写入输出流
	Write(w io.Writer, hit *Hit) error
}

// BaseFormatter 提供通用格式化方法。
type BaseFormatter struct{}

// Format 遍历 hit.Chunks 拼接 Content，作为基础格式化输出。
// hit 为 nil 或不含 Chunks 时返回空字符串。
func (f *BaseFormatter) Format(hit *Hit) string {
	if hit == nil || len(hit.Chunks) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, ch := range hit.Chunks {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(ch.Content)
	}
	return sb.String()
}

// FormatAll 默认实现等价于 Format。
func (f *BaseFormatter) FormatAll(hit *Hit) string {
	return f.Format(hit)
}

// Write 格式化后写入输出流。
func (f *BaseFormatter) Write(w io.Writer, hit *Hit) error {
	_, err := w.Write([]byte(f.FormatAll(hit)))
	return err
}
