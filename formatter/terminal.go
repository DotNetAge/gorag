package formatter

import (
	"fmt"
	"io"
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

// formatChunk 格式化单个 ChunkHit，直接使用其 Score/DocID/ID/Content。
// hit.Chunks 为空时由 FormatAll 处理，此处不处理空场景。
func (f *TerminalFormatter) formatChunk(idx int, ch core.ChunkHit) string {
	if ch.Chunk == nil {
		return ""
	}

	var sb strings.Builder

	// 序号
	if f.config.ShowIndex {
		sb.WriteString(f.config.TitleColor)
		fmt.Fprintf(&sb, "%d. ", idx+1)
		sb.WriteString(Reset)
	}

	// 分数
	if f.config.ShowScore {
		sb.WriteString(f.config.ScoreColor)
		fmt.Fprintf(&sb, "[%.4f]", ch.Score)
		sb.WriteString(Reset)
		sb.WriteString(" ")
	}

	// 元数据
	var meta []string
	if f.config.ShowDocID && ch.DocID != "" {
		meta = append(meta, fmt.Sprintf("doc:%s", ch.ID))
	}
	if len(meta) > 0 {
		sb.WriteString(f.config.MetaColor)
		sb.WriteString("(")
		sb.WriteString(strings.Join(meta, ", "))
		sb.WriteString(")")
		sb.WriteString(Reset)
		sb.WriteString("\n")
	}

	// 内容
	sb.WriteString(f.config.ContentColor)
	content := ch.Content
	if f.config.ContentMax > 0 && len(content) > f.config.ContentMax {
		content = content[:f.config.ContentMax] + "..."
	}
	sb.WriteString(content)
	sb.WriteString(Reset)

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
