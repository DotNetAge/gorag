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
