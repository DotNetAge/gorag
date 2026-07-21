package document

import "io"

// ParseText 读取纯文本内容并原样返回为 textDoc。
// 元数据为空 map。
func ParseText(r io.Reader) (RawDoc, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return newParsedDoc(string(content), map[string]any{}, RawDocText), nil
}
