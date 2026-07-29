package formatter

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
)

// ANSI 颜色码
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
)

// TerminalConfig 终端格式化配置
type TerminalConfig struct {
	ShowScore    bool   // 是否显示分数
	ShowDocID    bool   // 是否显示文档ID
	ShowIndex    bool   // 是否显示序号
	ContentMax   int    // 内容最大长度，0 表示不限制
	ScoreColor   string // 分数颜色
	ContentColor string // 内容颜色
	MetaColor    string // 元数据颜色
	TitleColor   string // 标题颜色
}

// DefaultTerminalConfig 默认终端配置
func DefaultTerminalConfig() *TerminalConfig {
	return &TerminalConfig{
		ShowScore:    true,
		ShowDocID:    true,
		ShowIndex:    true,
		ContentMax:   500,
		ScoreColor:   Green,
		ContentColor: White,
		MetaColor:    Dim + Cyan,
		TitleColor:   Bold + Yellow,
	}
}

// TerminalFormatter 终端彩色输出格式化器
type TerminalFormatter struct {
	core.BaseFormatter
	config *TerminalConfig
}

// NewTerminalFormatter 创建终端格式化器
func NewTerminalFormatter(opts ...func(*TerminalConfig)) *TerminalFormatter {
	cfg := DefaultTerminalConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &TerminalFormatter{config: cfg}
}

// WithShowScore 设置是否显示分数
func WithShowScore(show bool) func(*TerminalConfig) {
	return func(c *TerminalConfig) {
		c.ShowScore = show
	}
}

// WithShowDocID 设置是否显示文档ID
func WithShowDocID(show bool) func(*TerminalConfig) {
	return func(c *TerminalConfig) {
		c.ShowDocID = show
	}
}

// WithShowIndex 设置是否显示序号
func WithShowIndex(show bool) func(*TerminalConfig) {
	return func(c *TerminalConfig) {
		c.ShowIndex = show
	}
}

// WithContentMax 设置内容最大长度
func WithContentMax(max int) func(*TerminalConfig) {
	return func(c *TerminalConfig) {
		c.ContentMax = max
	}
}

// WithColors 设置颜色方案
func WithColors(score, content, meta, title string) func(*TerminalConfig) {
	return func(c *TerminalConfig) {
		if score != "" {
			c.ScoreColor = score
		}
		if content != "" {
			c.ContentColor = content
		}
		if meta != "" {
			c.MetaColor = meta
		}
		if title != "" {
			c.TitleColor = title
		}
	}
}

// formatChunk 格式化单个 ChunkHit。
// 格式：
//
//	N. title
//	summary
//	位置:[PXX-PXXX] 路径:[] 标签:[tag1, tag2, tag3]
func (f *TerminalFormatter) formatChunk(idx int, ch core.ChunkHit) string {
	if ch.Chunk == nil {
		return ""
	}
	fullPath := filepath.Join(ch.Chunk.Dir, ch.Chunk.FileName)

	// 位置信息
	pos := ""
	if ch.Chunk.StartPos > 0 || ch.Chunk.EndPos > 0 {
		if ch.Chunk.EndPos > ch.Chunk.StartPos {
			pos = fmt.Sprintf("P%d-P%d", ch.Chunk.StartPos, ch.Chunk.EndPos)
		} else {
			pos = fmt.Sprintf("P%d", ch.Chunk.StartPos)
		}
	}

	// 标签
	tags := ""
	if len(ch.Chunk.Tags) > 0 {
		tags = strings.Join(ch.Chunk.Tags, ", ")
	}

	var sb strings.Builder

	// 序号. title
	fmt.Fprintf(&sb, "%s%d. %s%s\n", f.config.TitleColor, idx+1, ch.Chunk.Title, Reset)

	// summary
	fmt.Fprintf(&sb, "%s%s%s\n", f.config.ContentColor, ch.Chunk.Summary, Reset)

	// 元数据行：路径、位置、标签
	fmt.Fprintf(&sb, "%s 路径:%s 位置:[%s]", f.config.MetaColor, fullPath, pos)
	if tags != "" {
		fmt.Fprintf(&sb, " 标签:[%s]", tags)
	}
	fmt.Fprint(&sb, Reset)

	return sb.String()
}

// Format 格式化 Hit 容器为终端输出。
// 遍历 hit.Chunks 输出每个 Chunk。
// hit 为 nil 或 Chunks 为空时返回空字符串。
func (f *TerminalFormatter) Format(hit *core.Hit) string {
	if hit == nil || len(hit.Chunks) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, ch := range hit.Chunks {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(f.formatChunk(i, ch))
	}
	return sb.String()
}

// FormatAll 格式化 Hit 容器，含标题和全部 Chunks。
// 标题显示 Chunks 数量。
func (f *TerminalFormatter) FormatAll(hit *core.Hit) string {
	if hit == nil || len(hit.Chunks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(f.config.TitleColor)
	fmt.Fprintf(&sb, "Found %d results:", len(hit.Chunks))
	sb.WriteString(Reset)
	sb.WriteString("\n\n")

	sb.WriteString(f.Format(hit))

	return sb.String()
}

// Write 格式化并写入输出流
func (f *TerminalFormatter) Write(w io.Writer, hit *core.Hit) error {
	_, err := fmt.Fprint(w, f.FormatAll(hit))
	return err
}

// Print 便捷方法：直接打印到标准输出
func (f *TerminalFormatter) Print(hit *core.Hit) {
	fmt.Print(f.FormatAll(hit))
}

// Println 便捷方法：打印并换行
func (f *TerminalFormatter) Println(hit *core.Hit) {
	fmt.Println(f.FormatAll(hit))
}
