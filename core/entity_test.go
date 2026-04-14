package core

import (
	"testing"
)

// TestStructureNodeClean 测试StructureNode的Clean方法
func TestStructureNodeClean(t *testing.T) {
	node := &StructureNode{
		NodeType: "heading",
		Title:    "  Test Title  ",
		Text:     "  Test Text  ",
		Children: []*StructureNode{
			{
				NodeType: "paragraph",
				Text:     "  Child Text  ",
			},
		},
	}

	node.Clean()

	// 验证清洗结果（当前CleanText只是返回原文本）
	if node.Title != "  Test Title  " {
		t.Errorf("Expected Title to be '  Test Title  ', got '%s'", node.Title)
	}

	if node.Text != "  Test Text  " {
		t.Errorf("Expected Text to be '  Test Text  ', got '%s'", node.Text)
	}

	if node.Children[0].Text != "  Child Text  " {
		t.Errorf("Expected Child Text to be '  Child Text  ', got '%s'", node.Children[0].Text)
	}
}

// TestCleanText 测试CleanText函数
func TestCleanText(t *testing.T) {
	text := "  Test Text  "
	result := CleanText(text)

	if result != text {
		t.Errorf("Expected CleanText to return original text, got '%s'", result)
	}
}
