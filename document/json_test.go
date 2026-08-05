package document

import (
	"strings"
	"testing"
)

// TestParseJSON_Plain 验证纯 JSON（无注释）解析行为不变。
func TestParseJSON_Plain(t *testing.T) {
	doc, err := ParseJSON(strings.NewReader(`{"name": "gorag", "count": 3}`))
	if err != nil {
		t.Fatalf("ParseJSON 纯 JSON 失败: %v", err)
	}
	if doc.Type() != RawDocData {
		t.Fatalf("文档类型 = %v, 期望 RawDocData", doc.Type())
	}
	text := doc.Content()
	if !strings.Contains(text, `"name":"gorag"`) {
		t.Fatalf("内容未正确归一化为紧凑 JSON: %s", text)
	}
}

// TestParseJSON_LineComment 验证 // 行注释（JSONC）可正常解析。
func TestParseJSON_LineComment(t *testing.T) {
	input := "{\n  // 配置说明\n  \"name\": \"gorag\", // 行尾注释\n  \"count\": 3\n}\n"
	doc, err := ParseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSON 行注释失败: %v", err)
	}
	text := doc.Content()
	if !strings.Contains(text, `"name":"gorag"`) || !strings.Contains(text, `"count":3`) {
		t.Fatalf("内容解析不正确: %s", text)
	}
}

// TestParseJSON_BlockComment 验证 /* */ 块注释可正常解析。
func TestParseJSON_BlockComment(t *testing.T) {
	input := `{ /* 头部注释 */ "name": "gorag", "arr": [1, /* 中间注释 */ 2, 3] }`
	doc, err := ParseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSON 块注释失败: %v", err)
	}
	text := doc.Content()
	if !strings.Contains(text, `"arr":[1,2,3]`) {
		t.Fatalf("内容解析不正确: %s", text)
	}
}

// TestParseJSON_CommentInString 验证字符串内的 // 与 /* */ 不被误剥离。
func TestParseJSON_CommentInString(t *testing.T) {
	input := `{"url": "https://example.com/a//b", "desc": "注释 /* 原样 */ 保留"}`
	doc, err := ParseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSON 字符串内注释失败: %v", err)
	}
	text := doc.Content()
	if !strings.Contains(text, "https://example.com/a//b") {
		t.Fatalf("字符串内 // 被误剥离: %s", text)
	}
	if !strings.Contains(text, "注释 /* 原样 */ 保留") {
		t.Fatalf("字符串内 /* */ 被误剥离: %s", text)
	}
}

// TestParseJSON_CommentAfterEscapedQuote 验证转义引号后的注释符号不误判为字符串结束。
func TestParseJSON_CommentAfterEscapedQuote(t *testing.T) {
	input := `{"key": "a\"//b", "next": 1} // 尾部注释`
	doc, err := ParseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSON 转义引号场景失败: %v", err)
	}
	text := doc.Content()
	if !strings.Contains(text, `"key":"a\"//b"`) {
		t.Fatalf("内容解析不正确: %s", text)
	}
}
