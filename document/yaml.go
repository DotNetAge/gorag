package document

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// ParseYAML 读取 YAML 文件并归一化为 dataDoc（内容为 JSON 字符串）。
//
// 归一化策略：
//   - 将 YAML 解析为通用数据结构，再序列化为紧凑 JSON 字符串
//   - 输出 JSON 字符串（紧凑格式，无缩进）
//   - 元数据包含 type 标记
func ParseYAML(r io.Reader) (RawDoc, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("YAML 输入为空")
	}

	// 解析 YAML 为通用数据结构
	var parsed any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("无效的 YAML: %w", err)
	}

	// 序列化为紧凑 JSON
	jsonBytes, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("YAML 转 JSON 失败: %w", err)
	}

	meta := map[string]any{
		"source_format": "yaml",
	}
	return newParsedDoc(string(jsonBytes), meta, RawDocData), nil
}
