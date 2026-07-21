package document

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ParseXML 读取 XML 文件并归一化为 dataDoc（内容为 JSON 字符串）。
//
// 归一化策略：
//   - 将 XML 解析为通用数据结构，再序列化为紧凑 JSON 字符串
//   - 属性使用 "@name" 前缀；文本节点使用 "#text" 字段
//   - 重复子元素自动合并为数组
//   - 输出 JSON 字符串（紧凑格式，无缩进）
//   - 元数据包含 source_format 标记
func ParseXML(r io.Reader) (RawDoc, error) {
	decoder := xml.NewDecoder(r)
	root, err := parseXMLElement(decoder)
	if err != nil {
		return nil, fmt.Errorf("无效的 XML: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("XML 输入为空")
	}

	jsonBytes, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("XML 转 JSON 失败: %w", err)
	}

	meta := map[string]any{
		"source_format": "xml",
	}
	return newParsedDoc(string(jsonBytes), meta, RawDocData), nil
}

// parseXMLElement 递归解析 XML 元素为 map[string]any。
//
// 转换规则：
//   - 元素 → map[name]any，包含属性（@前缀）、子元素、文本
//   - 元素的属性使用 "@attr" 作为 key
//   - 元素的文本内容使用 "#text" 作为 key
//   - 同名子元素自动合并为数组
//
// 实现说明：为简化逻辑，外层调用只返回第一个根元素的内容（去掉根元素名）。
func parseXMLElement(decoder *xml.Decoder) (any, error) {
	rootBuilt := false
	var root any

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if !rootBuilt {
				return nil, nil
			}
			return root, nil
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// 解析此元素（含子元素）为对象
			child, err := parseXMLChild(decoder, t)
			if err != nil {
				return nil, err
			}
			if !rootBuilt {
				// 根元素：返回其内容（不包含根元素名）
				root = child
				rootBuilt = true
			}
		case xml.EndElement:
			// 上层应已消费 EndElement，这里兜底跳过
		}
	}
}

// parseXMLChild 解析一个 StartElement...EndElement 块，返回 map[string]any。
func parseXMLChild(decoder *xml.Decoder, start xml.StartElement) (map[string]any, error) {
	obj := make(map[string]any)

	// 属性 → @key
	for _, attr := range start.Attr {
		obj["@"+attr.Name.Local] = attr.Value
	}

	// 遍历子 token
	var textParts []string
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// 递归解析子元素
			child, err := parseXMLChild(decoder, t)
			if err != nil {
				return nil, err
			}
			// 同名子元素合并为数组
			name := t.Name.Local
			if existing, exists := obj[name]; exists {
				switch arr := existing.(type) {
				case []any:
					obj[name] = append(arr, child)
				default:
					obj[name] = []any{existing, child}
				}
			} else {
				obj[name] = child
			}
		case xml.EndElement:
			// 元素结束：拼装文本
			if len(textParts) > 0 {
				text := strings.TrimSpace(strings.Join(textParts, ""))
				if text != "" {
					// 仅在无子元素时存为 #text；有子元素时也存为 #text
					obj["#text"] = text
				}
			}
			return obj, nil
		case xml.CharData:
			// 累积文本节点
			textParts = append(textParts, string(t))
		case xml.Comment:
			// 忽略注释
		}
	}
}
