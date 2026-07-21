package document

import (
	"io"
)

// ParseMarkdown 解析 Markdown 文件，返回 RawDocDoc 类型。
// Markdown 文件内容已经是 Markdown 格式，无需转换。
func ParseMarkdown(r io.Reader) (RawDoc, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return newParsedDoc(string(content), map[string]any{}, RawDocDoc), nil
}
