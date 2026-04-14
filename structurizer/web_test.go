package structurizer

import (
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/document"
)

func TestWebStructurizer_Parse_HTML(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
</head>
<body>
	<h1>Main Title</h1>
	<p>This is a paragraph.</p>
	<ul>
		<li>Item 1</li>
		<li>Item 2</li>
	</ul>
	<table>
		<tr><th>Name</th><th>Value</th></tr>
		<tr><td>foo</td><td>bar</td></tr>
	</table>
	<a href="https://example.com">Link</a>
	<img src="image.png" alt="An image">
</body>
</html>`

	ws := NewWebStructurizer()
	doc := document.New(htmlContent, "text/html")

	result, err := ws.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证标题提取
	if result.Title != "Test Page" {
		t.Errorf("Title = %s, want 'Test Page'", result.Title)
	}

	// 验证格式
	if result.RawDoc.GetMeta()["format"] != "html" {
		t.Errorf("format = %v, want html", result.RawDoc.GetMeta()["format"])
	}

	// 验证根节点
	if result.Root.NodeType != "document" {
		t.Errorf("Root.NodeType = %s, want document", result.Root.NodeType)
	}

	// 验证存在 heading 节点
	var foundH1 bool
	for _, child := range result.Root.Children {
		if child.NodeType == "heading" && child.Level == 1 {
			foundH1 = true
			if child.Title != "Main Title" {
				t.Errorf("H1 title = %s, want 'Main Title'", child.Title)
			}
			break
		}
	}
	if !foundH1 {
		t.Error("H1 heading not found")
	}
}

func TestWebStructurizer_Parse_HTML_SkipScript(t *testing.T) {
	htmlContent := `<html>
<body>
	<p>Content</p>
	<script>alert('test');</script>
	<style>body { color: red; }</style>
	<p>More content</p>
</body>
</html>`

	ws := NewWebStructurizer()
	doc := document.New(htmlContent, "text/html")

	result, err := ws.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证 script 和 style 被跳过
	var hasScript, hasStyle bool
	var check func(node *core.StructureNode)
	check = func(node *core.StructureNode) {
		if node.Title == "script" || node.NodeType == "script" {
			hasScript = true
		}
		if node.Title == "style" || node.NodeType == "style" {
			hasStyle = true
		}
		for _, child := range node.Children {
			check(child)
		}
	}
	check(result.Root)

	if hasScript {
		t.Error("Script tags should be skipped")
	}
	if hasStyle {
		t.Error("Style tags should be skipped")
	}
}

func TestWebStructurizer_Parse_XML(t *testing.T) {
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<root>
	<name>Test XML</name>
	<items>
		<item id="1">First</item>
		<item id="2">Second</item>
	</items>
</root>`

	ws := NewWebStructurizer()
	doc := document.New(xmlContent, "text/xml")

	result, err := ws.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证格式
	if result.RawDoc.GetMeta()["format"] != "xml" {
		t.Errorf("format = %v, want xml", result.RawDoc.GetMeta()["format"])
	}

	// 验证根节点
	if result.Root.NodeType != "document" {
		t.Errorf("Root.NodeType = %s, want document", result.Root.NodeType)
	}

	// 验证存在 item 元素
	var itemCount int
	var countItems func(node *core.StructureNode)
	countItems = func(node *core.StructureNode) {
		if node.Title == "item" {
			itemCount++
		}
		for _, child := range node.Children {
			countItems(child)
		}
	}
	countItems(result.Root)

	if itemCount != 2 {
		t.Errorf("item count = %d, want 2", itemCount)
	}
}

func TestWebStructurizer_Parse_HTML_Nested(t *testing.T) {
	htmlContent := `<html>
<body>
	<div id="container">
		<section class="main">
			<h2>Section Title</h2>
			<p>Section content</p>
		</section>
	</div>
</body>
</html>`

	ws := NewWebStructurizer()
	doc := document.New(htmlContent, "text/html")

	result, err := ws.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证嵌套结构
	var foundDiv, foundSection bool
	var check func(node *core.StructureNode)
	check = func(node *core.StructureNode) {
		if node.NodeType == "container" && strings.Contains(node.Title, "#container") {
			foundDiv = true
		}
		if node.NodeType == "section" && strings.Contains(node.Title, ".main") {
			foundSection = true
		}
		for _, child := range node.Children {
			check(child)
		}
	}
	check(result.Root)

	if !foundDiv {
		t.Error("Div with id 'container' not found")
	}
	if !foundSection {
		t.Error("Section with class 'main' not found")
	}
}

func TestWebStructurizer_Parse_Table(t *testing.T) {
	htmlContent := `<html>
<body>
	<table>
		<thead>
			<tr><th>A</th><th>B</th></tr>
		</thead>
		<tbody>
			<tr><td>1</td><td>2</td></tr>
		</tbody>
	</table>
</body>
</html>`

	ws := NewWebStructurizer()
	doc := document.New(htmlContent, "text/html")

	result, err := ws.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证表格结构
	var foundTable bool
	var check func(node *core.StructureNode)
	check = func(node *core.StructureNode) {
		if node.NodeType == "table" {
			foundTable = true
			// 验证表格子节点
			var hasRow bool
			for _, child := range node.Children {
				if child.NodeType == "table_row" || child.NodeType == "table_section" {
					hasRow = true
					break
				}
			}
			if !hasRow {
				t.Error("Table should have rows")
			}
		}
		for _, child := range node.Children {
			check(child)
		}
	}
	check(result.Root)

	if !foundTable {
		t.Error("Table not found")
	}
}

func TestWebStructurizer_Parse_Form(t *testing.T) {
	htmlContent := `<html>
<body>
	<form id="login">
		<input type="text" name="username" value="">
		<input type="password" name="password">
		<button type="submit">Submit</button>
	</form>
</body>
</html>`

	ws := NewWebStructurizer()
	doc := document.New(htmlContent, "text/html")

	result, err := ws.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证表单结构
	var foundForm bool
	var check func(node *core.StructureNode)
	check = func(node *core.StructureNode) {
		if node.NodeType == "form" {
			foundForm = true
			// 验证输入框
			var inputCount int
			for _, child := range node.Children {
				if child.NodeType == "input" {
					inputCount++
				}
			}
			if inputCount < 2 {
				t.Errorf("Form should have at least 2 inputs, got %d", inputCount)
			}
		}
		for _, child := range node.Children {
			check(child)
		}
	}
	check(result.Root)

	if !foundForm {
		t.Error("Form not found")
	}
}

func TestWebStructurizer_Parse_Empty(t *testing.T) {
	ws := NewWebStructurizer()
	doc := document.New("", "text/html")

	_, err := ws.Parse(doc)
	if err == nil {
		t.Error("Parse() should return error for empty content")
	}
}

func TestWebStructurizer_Parse_InvalidHTML(t *testing.T) {
	ws := NewWebStructurizer()
	doc := document.New("<html><unclosed", "text/html")

	// HTML 解析器是宽容的，即使不完整也会尝试解析
	result, err := ws.Parse(doc)
	// 可能不会报错，因为 golang.org/x/net/html 是宽容解析器
	if err == nil && result == nil {
		t.Error("Parse() should return result or error")
	}
}

func TestWebStructurizer_CustomConfig(t *testing.T) {
	ws := &WebStructurizer{
		SkipTags: map[string]bool{
			"script": true,
		},
		InlineTags: map[string]bool{
			"span": true,
		},
		ExtractAttributes: []string{"id", "data-value"},
	}

	htmlContent := `<html>
<body>
	<div id="test" data-value="123">Content</div>
	<script>skip</script>
	<span>inline</span>
</body>
</html>`

	doc := document.New(htmlContent, "text/html")

	result, err := ws.Parse(doc)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result == nil {
		t.Error("Parse() should return result")
	}
}
