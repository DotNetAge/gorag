package document

import (
	"io"
	"log/slog"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown"
)

// ParseHTML 将 HTML 内容转换为 Markdown 格式的 RawDocument。
// 使用 html-to-markdown 库处理标题、列表、链接、表格、图片、代码块等元素。
// 自动从 <title> 标签提取文档标题。
func ParseHTML(r io.Reader) (*RawDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	htmlContent := string(data)

	converter := md.NewConverter("", true, &md.Options{HeadingStyle: "atx"})

	markdown, err := converter.ConvertString(htmlContent)
	if err != nil {
		slog.Error("html to markdown conversion failed", "error", err)
		return nil, err
	}

	// 清理多余空行：连续三个以上换行缩减为两个
	markdown = collapseBlankLines(markdown)

	// 提取 <title> 标签内容作为文档标题
	title := extractTitle(htmlContent)

	rawDoc := NewRawDoc(strings.TrimSpace(markdown))
	if title != "" {
		rawDoc.SetValue("title", title)
	}

	return rawDoc, nil
}

// extractTitle 从 HTML 中提取 <title> 标签的文本内容
func extractTitle(html string) string {
	start := strings.Index(strings.ToLower(html), "<title")
	if start == -1 {
		return ""
	}
	// 找到 <title...> 的闭合 >
	gt := strings.Index(html[start:], ">")
	if gt == -1 {
		return ""
	}
	contentStart := start + gt + 1

	end := strings.Index(strings.ToLower(html[contentStart:]), "</title>")
	if end == -1 {
		return ""
	}

	title := strings.TrimSpace(html[contentStart : contentStart+end])
	// 清理 HTML 实体
	title = strings.ReplaceAll(title, "&amp;", "&")
	title = strings.ReplaceAll(title, "&lt;", "<")
	title = strings.ReplaceAll(title, "&gt;", ">")
	title = strings.ReplaceAll(title, "&quot;", "\"")

	return title
}

// collapseBlankLines 将连续三个及以上换行符缩减为两个
func collapseBlankLines(s string) string {
	var builder strings.Builder
	consecutiveNewlines := 0

	for _, ch := range s {
		if ch == '\n' {
			consecutiveNewlines++
			if consecutiveNewlines <= 2 {
				builder.WriteRune(ch)
			}
		} else {
			consecutiveNewlines = 0
			builder.WriteRune(ch)
		}
	}

	return builder.String()
}
