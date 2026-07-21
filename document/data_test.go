package document

import (
	"encoding/json"
	"strings"
	"testing"
)

// ========================= ParseJSON =========================

func TestParseJSON_Object(t *testing.T) {
	input := `{"name":"gorag","version":2,"active":true}`
	doc, err := ParseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSON 失败: %v", err)
	}

	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}

	// 输出应为合法的紧凑 JSON 对象
	if !strings.HasPrefix(doc.Content(), "{") {
		t.Errorf("期望 JSON 对象前缀 '{', 实际: %s", doc.Content())
	}

	// 验证输出可被重新解析
	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if parsed["name"] != "gorag" {
		t.Errorf("期望 name 'gorag', 实际: %v", parsed["name"])
	}
	if parsed["version"] != float64(2) {
		t.Errorf("期望 version 2, 实际: %v", parsed["version"])
	}

	// 元数据应包含 json_type
	if doc.Meta()["json_type"] != "object" {
		t.Errorf("期望 json_type 'object', 实际: %v", doc.Meta()["json_type"])
	}
}

func TestParseJSON_Array(t *testing.T) {
	input := `[1,2,3]`
	doc, err := ParseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSON 失败: %v", err)
	}

	if !strings.HasPrefix(doc.Content(), "[") {
		t.Errorf("期望 JSON 数组前缀 '[', 实际: %s", doc.Content())
	}
	if doc.Meta()["json_type"] != "array" {
		t.Errorf("期望 json_type 'array', 实际: %v", doc.Meta()["json_type"])
	}
}

func TestParseJSON_Primitives(t *testing.T) {
	// JSON 标量类型测试
	tests := []struct {
		name     string
		input    string
		jsonType string
	}{
		{"字符串", `"hello"`, "string"},
		{"数字", `42`, "number"},
		{"布尔真", `true`, "boolean"},
		{"布尔假", `false`, "boolean"},
		{"null", `null`, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := ParseJSON(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("ParseJSON 失败: %v", err)
			}
			if doc.Meta()["json_type"] != tt.jsonType {
				t.Errorf("期望 json_type %q, 实际: %v", tt.jsonType, doc.Meta()["json_type"])
			}
		})
	}
}

func TestParseJSON_InvalidInput(t *testing.T) {
	_, err := ParseJSON(strings.NewReader(`{not json`))
	if err == nil {
		t.Fatal("非法 JSON 输入应返回错误")
	}
}

func TestParseJSON_EmptyInput(t *testing.T) {
	_, err := ParseJSON(strings.NewReader(""))
	if err == nil {
		t.Fatal("空 JSON 输入应返回错误")
	}
}

func TestParseJSON_CompactFormat(t *testing.T) {
	// 带缩进的 JSON 应被紧凑化
	input := `{
  "name": "test",
  "nested": {
    "key": "value"
  }
}`
	doc, err := ParseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSON 失败: %v", err)
	}
	// 紧凑 JSON 不应包含换行或缩进空格
	if strings.Contains(doc.Content(), "\n") {
		t.Errorf("紧凑 JSON 输出不应包含换行, 实际: %q", doc.Content())
	}
	if strings.Contains(doc.Content(), "  ") {
		t.Errorf("紧凑 JSON 输出不应包含缩进空格, 实际: %q", doc.Content())
	}
}

// ========================= ParseYAML =========================

func TestParseYAML_Object(t *testing.T) {
	input := "name: gorag\nversion: 2\nlist:\n  - a\n  - b\n"
	doc, err := ParseYAML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseYAML 失败: %v", err)
	}

	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}

	// 输出应为 JSON 对象（YAML 转 JSON）
	if !strings.HasPrefix(doc.Content(), "{") {
		t.Errorf("期望 JSON 对象前缀 '{', 实际: %s", doc.Content())
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if parsed["name"] != "gorag" {
		t.Errorf("期望 name 'gorag', 实际: %v", parsed["name"])
	}
	if parsed["version"] != float64(2) {
		t.Errorf("期望 version 2, 实际: %v", parsed["version"])
	}

	if doc.Meta()["source_format"] != "yaml" {
		t.Errorf("期望 source_format 'yaml', 实际: %v", doc.Meta()["source_format"])
	}
}

func TestParseYAML_Array(t *testing.T) {
	// YAML 顶层可以是数组
	input := "- a\n- b\n- c\n"
	doc, err := ParseYAML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseYAML 失败: %v", err)
	}

	if !strings.HasPrefix(doc.Content(), "[") {
		t.Errorf("期望 JSON 数组前缀 '[', 实际: %s", doc.Content())
	}

	var arr []any
	if err := json.Unmarshal([]byte(doc.Content()), &arr); err != nil {
		t.Fatalf("输出不是合法 JSON 数组: %v", err)
	}
	if len(arr) != 3 {
		t.Errorf("期望 3 个元素, 实际 %d", len(arr))
	}
}

func TestParseYAML_NestedStructures(t *testing.T) {
	// 测试嵌套对象和数组
	input := "parent:\n  child:\n    grandchild: deep\n  list:\n    - 1\n    - 2\n"
	doc, err := ParseYAML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseYAML 失败: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}

	parent, ok := parsed["parent"].(map[string]any)
	if !ok {
		t.Fatal("期望 parent 为对象")
	}
	child, ok := parent["child"].(map[string]any)
	if !ok {
		t.Fatal("期望 parent.child 为对象")
	}
	if child["grandchild"] != "deep" {
		t.Errorf("期望 grandchild 'deep', 实际: %v", child["grandchild"])
	}
}

func TestParseYAML_InvalidInput(t *testing.T) {
	// 未闭合的引号在 YAML 中是明确的语法错误
	_, err := ParseYAML(strings.NewReader("key: \"unclosed quote\n"))
	if err == nil {
		t.Fatal("非法 YAML 输入应返回错误")
	}
}

func TestParseYAML_EmptyInput(t *testing.T) {
	_, err := ParseYAML(strings.NewReader(""))
	if err == nil {
		t.Fatal("空 YAML 输入应返回错误")
	}
}

// ========================= ParseXML =========================

func TestParseXML_Simple(t *testing.T) {
	input := `<root><name>gorag</name><version>2</version></root>`
	doc, err := ParseXML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseXML 失败: %v", err)
	}

	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}

	// 输出应为 JSON 对象
	if !strings.HasPrefix(doc.Content(), "{") {
		t.Errorf("期望 JSON 对象前缀 '{', 实际: %s", doc.Content())
	}

	// 根元素被剥除，输出包含 name/version 键
	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if parsed["name"] == nil {
		t.Errorf("期望输出包含 'name' 键, 实际: %v", parsed)
	}

	if doc.Meta()["source_format"] != "xml" {
		t.Errorf("期望 source_format 'xml', 实际: %v", doc.Meta()["source_format"])
	}
}

func TestParseXML_WithAttributes(t *testing.T) {
	input := `<root id="123" type="doc"><item>value</item></root>`
	doc, err := ParseXML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseXML 失败: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}

	// 属性应使用 @ 前缀
	if parsed["@id"] != "123" {
		t.Errorf("期望 @id '123', 实际: %v", parsed["@id"])
	}
	if parsed["@type"] != "doc" {
		t.Errorf("期望 @type 'doc', 实际: %v", parsed["@type"])
	}
}

func TestParseXML_RepeatedElements(t *testing.T) {
	input := `<root><item>a</item><item>b</item></root>`
	doc, err := ParseXML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseXML 失败: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}

	// 同名子元素应合并为数组
	items, ok := parsed["item"].([]any)
	if !ok {
		t.Fatalf("期望 'item' 为数组, 实际类型: %T", parsed["item"])
	}
	if len(items) != 2 {
		t.Errorf("期望 2 个 item, 实际 %d", len(items))
	}
}

func TestParseXML_NestedElements(t *testing.T) {
	// 测试深层嵌套
	input := `<root><parent><child><grandchild>deep</grandchild></child></parent></root>`
	doc, err := ParseXML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseXML 失败: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}

	parent, ok := parsed["parent"].(map[string]any)
	if !ok {
		t.Fatal("期望 parent 为对象")
	}
	child, ok := parent["child"].(map[string]any)
	if !ok {
		t.Fatal("期望 parent.child 为对象")
	}
	grandchild, ok := child["grandchild"].(map[string]any)
	if !ok {
		t.Fatalf("期望 parent.child.grandchild 为对象, 实际: %T", child["grandchild"])
	}
	if grandchild["#text"] != "deep" {
		t.Errorf("期望 grandchild #text 'deep', 实际: %v", grandchild["#text"])
	}
}

func TestParseXML_TextAndAttributes(t *testing.T) {
	// 同时有属性和文本的元素
	input := `<root><item id="1">value</item></root>`
	doc, err := ParseXML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseXML 失败: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}

	item, ok := parsed["item"].(map[string]any)
	if !ok {
		t.Fatalf("期望 item 为对象, 实际类型: %T", parsed["item"])
	}
	if item["@id"] != "1" {
		t.Errorf("期望 @id '1', 实际: %v", item["@id"])
	}
	if item["#text"] != "value" {
		t.Errorf("期望 #text 'value', 实际: %v", item["#text"])
	}
}

func TestParseXML_EmptyInput(t *testing.T) {
	_, err := ParseXML(strings.NewReader(""))
	if err == nil {
		t.Fatal("空 XML 输入应返回错误")
	}
}

func TestParseXML_InvalidInput(t *testing.T) {
	_, err := ParseXML(strings.NewReader(`<root><unterminated>`))
	if err == nil {
		t.Fatal("非法 XML 输入应返回错误")
	}
}

// ========================= ParseTOML =========================

func TestParseTOML_Object(t *testing.T) {
	input := `title = "gorag"
version = 2

[author]
name = "test"
`
	doc, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML 失败: %v", err)
	}

	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}

	// 输出应为 JSON 对象
	if !strings.HasPrefix(doc.Content(), "{") {
		t.Errorf("期望 JSON 对象前缀 '{', 实际: %s", doc.Content())
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if parsed["title"] != "gorag" {
		t.Errorf("期望 title 'gorag', 实际: %v", parsed["title"])
	}
	if parsed["version"] != float64(2) {
		t.Errorf("期望 version 2, 实际: %v", parsed["version"])
	}

	if doc.Meta()["source_format"] != "toml" {
		t.Errorf("期望 source_format 'toml', 实际: %v", doc.Meta()["source_format"])
	}
}

func TestParseTOML_ArrayOfTables(t *testing.T) {
	// TOML 数组表
	input := `[[items]]
name = "a"

[[items]]
name = "b"
`
	doc, err := ParseTOML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTOML 失败: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &parsed); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}

	items, ok := parsed["items"].([]any)
	if !ok {
		t.Fatalf("期望 items 为数组, 实际类型: %T", parsed["items"])
	}
	if len(items) != 2 {
		t.Errorf("期望 2 个 items, 实际 %d", len(items))
	}
}

func TestParseTOML_EmptyInput(t *testing.T) {
	_, err := ParseTOML(strings.NewReader(""))
	if err == nil {
		t.Fatal("空 TOML 输入应返回错误")
	}
}

func TestParseTOML_InvalidInput(t *testing.T) {
	// TOML 中键值必须同行 = 号
	_, err := ParseTOML(strings.NewReader("invalid syntax = = =\n"))
	if err == nil {
		t.Fatal("非法 TOML 输入应返回错误")
	}
}

// ========================= ParseLog =========================

func TestParseLog_Basic(t *testing.T) {
	input := "2024-01-01 INFO start\n2024-01-01 ERROR something failed\n\n2024-01-01 INFO done\n"
	doc, err := ParseLog(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseLog 失败: %v", err)
	}

	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}

	// 输出应为 JSON 数组
	if !strings.HasPrefix(doc.Content(), "[") {
		t.Errorf("期望 JSON 数组前缀 '[', 实际: %s", doc.Content())
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &entries); err != nil {
		t.Fatalf("输出不是合法 JSON 数组: %v", err)
	}
	// 3 个非空行
	if len(entries) != 3 {
		t.Errorf("期望 3 个非空条目, 实际 %d", len(entries))
	}

	// 验证行号和文本
	if entries[0]["line"] != float64(1) {
		t.Errorf("期望首条 line=1, 实际: %v", entries[0]["line"])
	}
	if !strings.Contains(entries[0]["text"].(string), "INFO start") {
		t.Errorf("期望首条 text 包含 'INFO start', 实际: %v", entries[0]["text"])
	}

	// 元数据包含 lines 和 nonblank_lines
	meta := doc.Meta()
	if meta["lines"] != 4 {
		t.Errorf("期望 lines=4（含空行）, 实际: %v", meta["lines"])
	}
	if meta["nonblank_lines"] != 3 {
		t.Errorf("期望 nonblank_lines=3, 实际: %v", meta["nonblank_lines"])
	}
}

func TestParseLog_EmptyInput(t *testing.T) {
	doc, err := ParseLog(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseLog 处理空输入失败: %v", err)
	}
	if doc.Content() != "[]" {
		t.Errorf("期望空 JSON 数组 '[]', 实际: %q", doc.Content())
	}
}

func TestParseLog_AllBlankLines(t *testing.T) {
	doc, err := ParseLog(strings.NewReader("\n\n\n"))
	if err != nil {
		t.Fatalf("ParseLog 处理全空行失败: %v", err)
	}
	if doc.Content() != "[]" {
		t.Errorf("期望空 JSON 数组 '[]', 实际: %q", doc.Content())
	}
}

func TestParseLog_LongLine(t *testing.T) {
	// 测试长行（超过默认 64KB 缓冲区）
	longLine := strings.Repeat("x", 100_000)
	input := "short line\n" + longLine + "\n"
	doc, err := ParseLog(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseLog 处理长行失败: %v", err)
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &entries); err != nil {
		t.Fatalf("输出不是合法 JSON 数组: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("期望 2 个条目, 实际 %d", len(entries))
	}
	if len(entries[1]["text"].(string)) != 100_000 {
		t.Errorf("期望长行长度 100000, 实际 %d", len(entries[1]["text"].(string)))
	}
}

func TestParseLog_CRLFLineEndings(t *testing.T) {
	// Windows 风格 CRLF 行尾
	input := "line1\r\nline2\r\n"
	doc, err := ParseLog(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseLog 处理 CRLF 失败: %v", err)
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(doc.Content()), &entries); err != nil {
		t.Fatalf("输出不是合法 JSON 数组: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("期望 2 个条目, 实际 %d", len(entries))
	}
	// 行尾 \r 应被 TrimRight
	if strings.HasSuffix(entries[0]["text"].(string), "\r") {
		t.Errorf("CRLF 行尾的 \\r 应被剥离, 实际: %q", entries[0]["text"])
	}
}

// ========================= stripHTMLTags（共享辅助函数） =========================

func TestStripHTMLTags_Simple(t *testing.T) {
	input := "<p>Hello <strong>world</strong></p>"
	got := stripHTMLTags(input)
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Errorf("剥离后应保留文本内容, 实际: %q", got)
	}
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Errorf("剥离后不应包含标签, 实际: %q", got)
	}
}

func TestStripHTMLTags_Nested(t *testing.T) {
	input := "<div><p>Nested <span>content</span></p></div>"
	got := stripHTMLTags(input)
	if !strings.Contains(got, "Nested") || !strings.Contains(got, "content") {
		t.Errorf("剥离后应保留嵌套文本, 实际: %q", got)
	}
}

func TestStripHTMLTags_Empty(t *testing.T) {
	if got := stripHTMLTags(""); got != "" {
		t.Errorf("空输入应返回空字符串, 实际: %q", got)
	}
}

func TestStripHTMLTags_NoTags(t *testing.T) {
	input := "plain text without tags"
	if got := stripHTMLTags(input); got != input {
		t.Errorf("无标签文本应原样返回, 实际: %q", got)
	}
}

func TestStripHTMLTags_CollapsesWhitespace(t *testing.T) {
	input := "<p>  multiple   spaces  </p>"
	got := stripHTMLTags(input)
	// 多余空白应被折叠
	if strings.Contains(got, "  ") {
		t.Errorf("多余空白应被折叠, 实际: %q", got)
	}
}

func TestStripHTMLTags_SelfClosingTags(t *testing.T) {
	input := "before<br/>after"
	got := stripHTMLTags(input)
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("自闭合标签应被剥离且保留文本, 实际: %q", got)
	}
}
