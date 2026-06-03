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

	// CleanText removes leading/trailing whitespace and normalizes text
	if node.Title == "  Test Title  " {
		t.Errorf("Expected Title to be cleaned (not '  Test Title  '), got '%s'", node.Title)
	}

	if node.Text == "  Test Text  " {
		t.Errorf("Expected Text to be cleaned (not '  Test Text  '), got '%s'", node.Text)
	}

	if node.Children[0].Text == "  Child Text  " {
		t.Errorf("Expected Child Text to be cleaned (not '  Child Text  '), got '%s'", node.Children[0].Text)
	}
}

// TestCleanText 测试CleanText函数
func TestCleanText(t *testing.T) {
	text := "Test Text"
	result := CleanText(text)

	if result == "" {
		t.Errorf("CleanText should not return empty string for '%s'", text)
	}
}

func TestCleanText_EmptyString(t *testing.T) {
	result := CleanText("")
	if result != "" {
		t.Errorf("CleanText('') should return empty string, got '%s'", result)
	}
}

func TestStructureNode_ID_EmptyText(t *testing.T) {
	node := &StructureNode{Text: ""}
	id := node.ID()
	if id != "" {
		t.Errorf("ID() should return empty string for node with no text, got '%s'", id)
	}
}

func TestStructureNode_ID_NonEmpty(t *testing.T) {
	node := &StructureNode{Text: "test content"}
	id := node.ID()
	if id == "" {
		t.Error("ID() should return non-empty string for node with text")
	}
}

func TestStructuredDocument_ID(t *testing.T) {
	doc := &mockDocument{id: "doc123"}
	sd := &StructuredDocument{RawDoc: doc}
	if sd.ID() != "doc123" {
		t.Errorf("expected ID 'doc123', got '%s'", sd.ID())
	}
}

func TestStructuredDocument_Meta(t *testing.T) {
	meta := map[string]any{"key": "value"}
	doc := &mockDocument{meta: meta}
	sd := &StructuredDocument{RawDoc: doc}

	result := sd.Meta()
	if result["key"] != "value" {
		t.Errorf("expected meta key='value', got %v", result["key"])
	}
}

func TestStructuredDocument_SetValue(t *testing.T) {
	doc := &mockDocument{meta: map[string]any{}}
	sd := &StructuredDocument{RawDoc: doc}

	result := sd.SetValue("newKey", "newValue")
	if result != sd {
		t.Error("SetValue should return the same StructuredDocument instance")
	}
	if sd.Meta()["newKey"] != "newValue" {
		t.Errorf("expected newKey='newValue', got %v", sd.Meta()["newKey"])
	}
}

type mockDocument struct {
	id   string
	meta map[string]any
}

func (m *mockDocument) GetID() string        { return m.id }
func (m *mockDocument) GetContent() string   { return "" }
func (m *mockDocument) GetMimeType() string  { return "" }
func (m *mockDocument) GetMeta() map[string]any { return m.meta }
func (m *mockDocument) GetImages() []Image    { return nil }
func (m *mockDocument) GetSource() string     { return "" }
func (m *mockDocument) GetExt() string        { return "" }
