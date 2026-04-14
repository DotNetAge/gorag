package structurizer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	toml "github.com/BurntSushi/toml"
	"github.com/DotNetAge/gorag/core"
	"gopkg.in/yaml.v3"
)

// ConfigStructurizer 配置文件结构化分析器
// 支持 JSON、YAML、TOML 等配置文件格式的结构化解析
type ConfigStructurizer struct {
	// MaxDepth 最大嵌套深度限制，防止循环引用导致栈溢出
	MaxDepth int
	// ExtractTitleFields 用于提取标题的字段名列表（优先级从高到低）
	ExtractTitleFields []string
}

// NewConfigStructurizer 创建默认配置的结构化分析器
func NewConfigStructurizer() *ConfigStructurizer {
	return &ConfigStructurizer{
		MaxDepth:           100,
		ExtractTitleFields: []string{"name", "title", "id", "app", "project", "service"},
	}
}

// Parse 实现 Structurizer 接口
func (c *ConfigStructurizer) Parse(doc core.Document) (*core.StructuredDocument, error) {
	content := doc.GetContent()
	ext := strings.ToLower(doc.GetExt())

	// 去掉扩展名前面的点
	ft := strings.TrimPrefix(ext, ".")

	var root *core.StructureNode
	var err error

	switch ft {
	case "json":
		root, err = c.parseJSON(content)
	case "yaml", "yml":
		root, err = c.parseYAML(content)
	case "toml":
		root, err = c.parseTOML(content)
	default:
		return nil, fmt.Errorf("unsupported config type: %s (supported: json, yaml, yml, toml)", ft)
	}

	if err != nil {
		return nil, fmt.Errorf("parse %s failed: %w", ft, err)
	}

	// 设置文档根节点
	if root.NodeType == "" {
		root.NodeType = "document"
	}

	// 提取文档标题
	title := c.extractConfigTitle(root, doc.GetSource())

	// 统计节点数量
	nodeCount := countStructureNodes(root)

	root.Clean()

	sd := &core.StructuredDocument{
		RawDoc: doc,
		Title:  title,
		Root:   root,
	}

	return sd.SetValue("format", ft).
		SetValue("node_count", nodeCount), nil
}

// parseJSON 解析 JSON 内容
func (c *ConfigStructurizer) parseJSON(content string) (*core.StructureNode, error) {
	var v any
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	root := &core.StructureNode{
		NodeType: "document",
		Title:    "JSON Config",
		Children: []*core.StructureNode{},
	}

	c.build(root, "", v, 0)
	return root, nil
}

// parseYAML 解析 YAML 内容
func (c *ConfigStructurizer) parseYAML(content string) (*core.StructureNode, error) {
	var v any
	if err := yaml.Unmarshal([]byte(content), &v); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	root := &core.StructureNode{
		NodeType: "document",
		Title:    "YAML Config",
		Children: []*core.StructureNode{},
	}

	c.build(root, "", v, 0)
	return root, nil
}

// parseTOML 解析 TOML 内容
func (c *ConfigStructurizer) parseTOML(content string) (*core.StructureNode, error) {
	var v any
	meta, err := toml.Decode(content, &v)
	if err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}

	root := &core.StructureNode{
		NodeType: "document",
		Title:    "TOML Config",
		Children: []*core.StructureNode{},
	}

	// TOML 解码后通常是 map[string]any
	if m, ok := v.(map[string]any); ok {
		c.build(root, "", m, 0)
	} else {
		c.build(root, "", v, 0)
	}

	// 未解码的键（可选：记录警告）
	_ = meta.Undecoded

	return root, nil
}

// build 递归构建结构树
func (c *ConfigStructurizer) build(parent *core.StructureNode, key string, val any, depth int) {
	// 防止无限递归
	if depth > c.MaxDepth {
		node := &core.StructureNode{
			NodeType: "error",
			Title:    key,
			Text:     "max depth exceeded",
		}
		parent.Children = append(parent.Children, node)
		return
	}

	switch v := val.(type) {
	case map[string]any:
		c.buildObject(parent, key, v, depth)

	case []any:
		c.buildArray(parent, key, v, depth)

	case string:
		c.buildValue(parent, key, v, "string")

	case float64:
		// JSON 数字默认解析为 float64
		c.buildValue(parent, key, formatNumber(v), "number")

	case int, int64, int32, float32:
		c.buildValue(parent, key, fmt.Sprintf("%v", v), "number")

	case bool:
		c.buildValue(parent, key, fmt.Sprintf("%v", v), "boolean")

	case nil:
		c.buildValue(parent, key, "null", "null")

	default:
		// 处理其他类型（如 YAML 的特定类型）
		c.buildValue(parent, key, fmt.Sprintf("%v", v), "unknown")
	}
}

// buildObject 构建对象节点
func (c *ConfigStructurizer) buildObject(parent *core.StructureNode, key string, obj map[string]any, depth int) {
	node := &core.StructureNode{
		NodeType: "object",
		Title:    key,
		Level:    depth,
		Children: []*core.StructureNode{},
	}

	// 按键排序遍历（保证输出稳定性）
	for _, k := range sortedKeys(obj) {
		c.build(node, k, obj[k], depth+1)
	}

	parent.Children = append(parent.Children, node)
}

// buildArray 构建数组节点
func (c *ConfigStructurizer) buildArray(parent *core.StructureNode, key string, arr []any, depth int) {
	node := &core.StructureNode{
		NodeType: "array",
		Title:    key,
		Level:    depth,
		Children: []*core.StructureNode{},
	}

	for i, item := range arr {
		itemKey := fmt.Sprintf("[%d]", i)
		c.build(node, itemKey, item, depth+1)
	}

	parent.Children = append(parent.Children, node)
}

// buildValue 构建键值对节点
func (c *ConfigStructurizer) buildValue(parent *core.StructureNode, key, val, valType string) {
	text := val
	if key != "" {
		text = fmt.Sprintf("%s: %s", key, val)
	}

	node := &core.StructureNode{
		NodeType: "key_value",
		Title:    key,
		Text:     text,
	}

	// 在 Title 中嵌入类型信息（通过后缀）
	if valType != "" && key != "" {
		node.Title = fmt.Sprintf("%s (%s)", key, valType)
	}

	parent.Children = append(parent.Children, node)
}

// extractConfigTitle 从配置中提取文档标题
func (c *ConfigStructurizer) extractConfigTitle(root *core.StructureNode, source string) string {
	// 1. 尝试从第一层子节点中查找标题字段
	for _, field := range c.ExtractTitleFields {
		for _, child := range root.Children {
			if child.Title == field && child.NodeType == "key_value" {
				// 提取值部分
				parts := strings.SplitN(child.Text, ":", 2)
				if len(parts) == 2 {
					title := strings.TrimSpace(parts[1])
					if title != "" && title != "null" {
						return title
					}
				}
			}
		}
	}

	// 2. 如果第一个子节点是对象，尝试从其 title/name 字段提取
	if len(root.Children) > 0 && root.Children[0].NodeType == "object" {
		for _, child := range root.Children[0].Children {
			for _, field := range c.ExtractTitleFields {
				if child.Title == field || strings.HasPrefix(child.Title, field+" ") {
					parts := strings.SplitN(child.Text, ":", 2)
					if len(parts) == 2 {
						title := strings.TrimSpace(parts[1])
						if title != "" && title != "null" {
							return title
						}
					}
				}
			}
		}
	}

	// 3. 默认使用文件名
	return source
}

// countStructureNodes 统计节点数量
func countStructureNodes(node *core.StructureNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += countStructureNodes(child)
	}
	return count
}

// sortedKeys 返回排序后的键列表
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 简单排序（可以优化为自然排序）
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// formatNumber 格式化数字（处理整数/浮点数显示）
func formatNumber(f float64) string {
	// 如果是整数，显示为整数
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	// 否则显示浮点数，去除尾部多余的零
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return s
}
