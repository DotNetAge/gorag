package document

import (
	"encoding/json"
	"fmt"
	"io"
)

// ParseJSON 读取 JSON 文件并归一化为 dataDoc（内容为 JSON 字符串）。
//
// 归一化策略：
//   - 校验输入是合法 JSON 后原样返回（紧凑格式化）
//   - 兼容 JSONC：解析前剥离 // 行注释与 /* */ 块注释
//   - 输出 JSON 字符串（紧凑格式，无缩进）
//   - 元数据包含 type 标记
func ParseJSON(r io.Reader) (RawDoc, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("JSON 输入为空")
	}

	// 剥离 JSONC 注释（如 tsconfig.json 常带 // 注释），保证合法 JSON 解析
	data = stripJSONComments(data)

	// 解析为 any 以校验合法性
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("无效的 JSON: %w", err)
	}

	// 重新序列化为紧凑 JSON
	jsonBytes, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("JSON 重新序列化失败: %w", err)
	}

	meta := map[string]any{
		"json_type": jsonTypeName(parsed),
	}
	return newParsedDoc(string(jsonBytes), meta, RawDocData), nil
}

// stripJSONComments 剥离 JSON 文本中的 // 行注释与 /* */ 块注释。
// 仅处理字符串之外的注释；字符串内的 "/" 与注释符号原样保留（如 "https://..."）。
// 行注释剥离时保留换行符，避免注释后的内容与前文拼接导致语法错误。
func stripJSONComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if c == '\\' && i+1 < len(data) {
				// 转义序列整体保留（如 \" 与 \\），跳过下一个字符
				i++
				out = append(out, data[i])
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			// 行注释：跳到行尾，保留换行符
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			// 块注释：跳到 */ 之后
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++ // 越过 '*'，循环末尾的 i++ 再越过 '/'
		default:
			out = append(out, c)
		}
	}
	return out
}

// jsonTypeName 返回 JSON 顶层值的类型名称（用于元数据）。
func jsonTypeName(v any) string {
	switch v.(type) {
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}
