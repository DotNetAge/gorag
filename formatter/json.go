package formatter

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
)

// JSONFormatter JSON 格式化器
type JSONFormatter struct {
	core.BaseFormatter
}

// NewJSONFormatter 创建 JSON 格式化器
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// FormatAll 格式化为 JSON。
func (f *JSONFormatter) FormatAll(hit *core.Hit) string {
	if hit == nil || len(hit.Chunks) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[\n")
	for i, ch := range hit.Chunks {
		if ch.Chunk == nil {
			continue
		}
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString("  {\n")
		fmt.Fprintf(&sb, "    \"id\": \"%s\",\n", ch.ID)
		fmt.Fprintf(&sb, "    \"score\": %.4f,\n", ch.Score)
		fmt.Fprintf(&sb, "    \"doc_id\": \"%s\",\n", ch.DocID)
		// 转义内容中的特殊字符
		content := strings.ReplaceAll(ch.Content, "\n", "\\n")
		content = strings.ReplaceAll(content, "\"", "\\\"")
		fmt.Fprintf(&sb, "    \"content\": \"%s\"\n", content)
		sb.WriteString("  }")
	}
	sb.WriteString("\n]")
	return sb.String()
}
