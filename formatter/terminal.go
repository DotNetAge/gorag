package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
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

// Format 格式化单个 Hit
func (f *TerminalFormatter) Format(hit *core.Hit) string {
	var sb strings.Builder

	// 分数
	if f.config.ShowScore {
		sb.WriteString(f.config.ScoreColor)
		fmt.Fprintf(&sb, "[%.4f]", hit.Score)
		sb.WriteString(Reset)
		sb.WriteString(" ")
	}

	// 元数据
	var meta []string
	if f.config.ShowDocID && hit.DocID != "" {
		meta = append(meta, fmt.Sprintf("doc:%s", hit.ID))
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
	content := hit.Content
	if f.config.ContentMax > 0 && len(content) > f.config.ContentMax {
		content = content[:f.config.ContentMax] + "..."
	}
	sb.WriteString(content)
	sb.WriteString(Reset)

	return sb.String()
}

// FormatAll 格式化多个 Hit
func (f *TerminalFormatter) FormatAll(hits []core.Hit) string {
	var sb strings.Builder

	sb.WriteString(f.config.TitleColor)
	fmt.Fprintf(&sb, "Found %d results:", len(hits))
	sb.WriteString(Reset)
	sb.WriteString("\n\n")

	for i, hit := range hits {
		if f.config.ShowIndex {
			sb.WriteString(f.config.TitleColor)
			fmt.Fprintf(&sb, "%d. ", i+1)
			sb.WriteString(Reset)
		}
		sb.WriteString(f.Format(&hit))
		if i < len(hits)-1 {
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// Write 格式化并写入输出流
func (f *TerminalFormatter) Write(w io.Writer, hits []core.Hit) error {
	_, err := fmt.Fprint(w, f.FormatAll(hits))
	return err
}

// Print 便捷方法：直接打印到标准输出
func (f *TerminalFormatter) Print(hits []core.Hit) {
	fmt.Print(f.FormatAll(hits))
}

// Println 便捷方法：打印并换行
func (f *TerminalFormatter) Println(hits []core.Hit) {
	fmt.Println(f.FormatAll(hits))
}
