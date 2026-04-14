package structurizer

import (
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/document"
)

func TestConfigStructurizer_Parse_JSON(t *testing.T) {
	jsonContent := `{
		"name": "my-app",
		"version": "1.0.0",
		"debug": true,
		"port": 8080,
		"database": {
			"host": "localhost",
			"port": 5432,
			"name": "mydb"
		},
		"servers": [
			{"name": "server1", "url": "http://localhost:3000"},
			{"name": "server2", "url": "http://localhost:3001"}
		]
	}`

	cs := NewConfigStructurizer()
	doc := document.New(jsonContent, "application/json")

	result, err := cs.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证标题提取
	if result.Title != "my-app" {
		t.Errorf("Title = %s, want my-app", result.Title)
	}

	// 验证根节点
	if result.Root.NodeType != "document" {
		t.Errorf("Root.NodeType = %s, want document", result.Root.NodeType)
	}

	// 验证子节点数量
	if len(result.Root.Children) == 0 {
		t.Error("Root has no children")
	}

	// 验证 database 对象存在
	var foundDB bool
	for _, child := range result.Root.Children {
		if child.Title == "database" && child.NodeType == "object" {
			foundDB = true
			break
		}
	}
	if !foundDB {
		t.Error("database object not found")
	}

	// 验证 metadata
	if result.RawDoc.GetMeta()["format"] != "json" {
		t.Errorf("format = %v, want json", result.RawDoc.GetMeta()["format"])
	}
}

func TestConfigStructurizer_Parse_YAML(t *testing.T) {
	yamlContent := `
name: my-service
version: 2.0.0
debug: false
features:
  - name: auth
    enabled: true
  - name: logging
    enabled: false
config:
  timeout: 30
  retries: 3
`

	cs := NewConfigStructurizer()
	doc := document.New(yamlContent, "application/x-yaml")

	result, err := cs.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证标题提取
	if result.Title != "my-service" {
		t.Errorf("Title = %s, want my-service", result.Title)
	}

	// 验证数组解析
	var foundFeatures bool
	for _, child := range result.Root.Children {
		if child.Title == "features" && child.NodeType == "array" {
			foundFeatures = true
			if len(child.Children) != 2 {
				t.Errorf("features array has %d items, want 2", len(child.Children))
			}
			break
		}
	}
	if !foundFeatures {
		t.Error("features array not found")
	}
}

func TestConfigStructurizer_Parse_TOML(t *testing.T) {
	tomlContent := `
name = "my-toml-app"
version = "1.0.0"
debug = true

[server]
host = "localhost"
port = 8080

[[features]]
name = "auth"
enabled = true

[[features]]
name = "logging"
enabled = false
`

	cs := NewConfigStructurizer()
	doc := document.New(tomlContent, "application/toml")

	result, err := cs.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证标题提取
	if result.Title != "my-toml-app" {
		t.Errorf("Title = %s, want my-toml-app", result.Title)
	}

	// 验证 metadata
	if result.RawDoc.GetMeta()["format"] != "toml" {
		t.Errorf("format = %v, want toml", result.RawDoc.GetMeta()["format"])
	}

	// 验证 server 对象存在
	var foundServer bool
	for _, child := range result.Root.Children {
		if child.Title == "server" && child.NodeType == "object" {
			foundServer = true
			break
		}
	}
	if !foundServer {
		t.Error("server object not found")
	}

	// 验证 features 数组存在
	var foundFeatures bool
	for _, child := range result.Root.Children {
		if child.Title == "features" && child.NodeType == "array" {
			foundFeatures = true
			if len(child.Children) != 2 {
				t.Errorf("features array has %d items, want 2", len(child.Children))
			}
			break
		}
	}
	if !foundFeatures {
		t.Error("features array not found")
	}
}

func TestConfigStructurizer_Parse_Unsupported(t *testing.T) {
	cs := NewConfigStructurizer()
	doc := document.New("<config></config>", "text/xml")

	_, err := cs.Parse(doc)
	if err == nil {
		t.Error("Parse() should return error for unsupported format")
	}
}

func TestConfigStructurizer_Parse_InvalidJSON(t *testing.T) {
	cs := NewConfigStructurizer()
	doc := document.New("{invalid json}", "application/json")

	_, err := cs.Parse(doc)
	if err == nil {
		t.Error("Parse() should return error for invalid JSON")
	}
}

func TestConfigStructurizer_Parse_InvalidYAML(t *testing.T) {
	cs := NewConfigStructurizer()
	doc := document.New(":\n  invalid", "application/x-yaml")

	_, err := cs.Parse(doc)
	if err == nil {
		t.Error("Parse() should return error for invalid YAML")
	}
}

func TestConfigStructurizer_Parse_InvalidTOML(t *testing.T) {
	cs := NewConfigStructurizer()
	doc := document.New("[invalid\n", "application/toml")

	_, err := cs.Parse(doc)
	if err == nil {
		t.Error("Parse() should return error for invalid TOML")
	}
}

func TestConfigStructurizer_Types(t *testing.T) {
	jsonContent := `{
		"string_val": "hello",
		"number_int": 42,
		"number_float": 3.14,
		"bool_true": true,
		"bool_false": false,
		"null_val": null,
		"empty_array": [],
		"empty_object": {}
	}`

	cs := NewConfigStructurizer()
	doc := document.New(jsonContent, "application/json")

	result, err := cs.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证各类型节点
	typeChecks := map[string]string{
		"string_val":   "string",
		"number_int":   "number",
		"number_float": "number",
		"bool_true":    "boolean",
		"bool_false":   "boolean",
		"null_val":     "null",
		"empty_array":  "array",
		"empty_object": "object",
	}

	for _, child := range result.Root.Children {
		expectedType, ok := typeChecks[child.Title]
		if !ok {
			continue
		}
		if child.NodeType == "key_value" {
			// 检查类型标注
			if !containsType(child.Title, expectedType) {
				t.Errorf("Field %s should have type %s, got %s", child.Title, expectedType, child.Title)
			}
		} else if child.NodeType != expectedType && child.NodeType != "array" && child.NodeType != "object" {
			t.Errorf("Field %s NodeType = %s, want %s", child.Title, child.NodeType, expectedType)
		}
	}
}

func TestConfigStructurizer_MaxDepth(t *testing.T) {
	// 创建深度嵌套的 JSON
	depth := 50
	jsonContent := `{"a": `
	for i := 0; i < depth; i++ {
		jsonContent += `{"b": `
	}
	jsonContent += `"value"`
	for i := 0; i < depth; i++ {
		jsonContent += `}`
	}
	jsonContent += `}`

	cs := &ConfigStructurizer{
		MaxDepth:           10,
		ExtractTitleFields: []string{"name", "title"},
	}

	doc := document.New(jsonContent, "application/json")

	result, err := cs.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证深度限制生效
	if !hasErrorNode(result.Root) {
		t.Error("Expected error node due to max depth exceeded")
	}
}

func TestConfigStructurizer_CustomTitleFields(t *testing.T) {
	jsonContent := `{"app_name": "custom-app", "version": "1.0"}`

	cs := &ConfigStructurizer{
		MaxDepth:           100,
		ExtractTitleFields: []string{"app_name", "service_name"},
	}

	doc := document.New(jsonContent, "application/json")

	result, err := cs.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Title != "custom-app" {
		t.Errorf("Title = %s, want custom-app", result.Title)
	}
}

func TestConfigStructurizer_DefaultTitle(t *testing.T) {
	jsonContent := `{"foo": "bar"}`

	cs := NewConfigStructurizer()
	doc := document.New(jsonContent, "application/json")

	result, err := cs.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 没有匹配的标题字段，应使用文件名
	if result.Title != "config.json" {
		t.Errorf("Title = %s, want config.json", result.Title)
	}
}

// 辅助函数
func containsType(title, expectedType string) bool {
	return strings.Contains(title, "("+expectedType+")")
}

func hasErrorNode(node *core.StructureNode) bool {
	if node.NodeType == "error" {
		return true
	}
	for _, child := range node.Children {
		if hasErrorNode(child) {
			return true
		}
	}
	return false
}
