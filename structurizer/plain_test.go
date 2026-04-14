package structurizer

import (
	"testing"

	"github.com/DotNetAge/gorag/document"
)

func TestPlainTextStructurizer_Parse(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantBlocks int
		wantTypes  []string
	}{
		{
			name:       "empty document",
			content:    "",
			wantBlocks: 0,
			wantTypes:  nil,
		},
		{
			name: "simple heading",
			content: `第一章 概述

这是正文内容。`,
			wantBlocks: 2,
			wantTypes:  []string{"heading", "paragraph"},
		},
		{
			name: "numbered heading",
			content: `1.1 简介

这是介绍内容。

1.2 详细说明

这是详细内容。`,
			wantBlocks: 4,
			wantTypes:  []string{"heading", "paragraph", "heading", "paragraph"},
		},
		{
			name: "unordered list",
			content: `- 项目一
- 项目二
- 项目三`,
			wantBlocks: 1,
			wantTypes:  []string{"list"},
		},
		{
			name: "ordered list",
			content: `1. 第一步
2. 第二步
3. 第三步`,
			wantBlocks: 1,
			wantTypes:  []string{"list"},
		},
		{
			name: "task list",
			content: `- [ ] 待办事项一
- [x] 已完成事项
- [ ] 待办事项二`,
			wantBlocks: 1,
			wantTypes:  []string{"list"},
		},
		{
			name: "quote block",
			content: `> 这是一段引用
> 引用可以多行`,
			wantBlocks: 1,
			wantTypes:  []string{"quote"},
		},
		{
			name: "code block with indent",
			content: `    func main() {
        fmt.Println("Hello")
    }`,
			wantBlocks: 1,
			wantTypes:  []string{"code_block"},
		},
		{
			name: "table",
			content: `| Name | Age |
|------|-----|
| John | 30  |
| Jane | 25  |`,
			wantBlocks: 1,
			wantTypes:  []string{"table"},
		},
		{
			name: "mixed content",
			content: `第一章 开始

这是一段介绍文字。

- 列表项一
- 列表项二

    code here
    more code

> 引用内容`,
			wantBlocks: 5,
			wantTypes:  []string{"heading", "paragraph", "list", "code_block", "quote"},
		},
		{
			name: "all caps heading",
			content: `MAIN TITLE

This is content under the title.`,
			wantBlocks: 2,
			wantTypes:  []string{"heading", "paragraph"},
		},
		{
			name: "title case heading",
			content: `Introduction to the System

This is the introduction.`,
			wantBlocks: 2,
			wantTypes:  []string{"heading", "paragraph"},
		},
	}

	ps := NewPlainTextStructurizer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := document.New(tt.content, "text/plain")

			doc, err := ps.Parse(raw)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if doc == nil {
				t.Fatal("Parse() returned nil document")
			}

			if doc.Root == nil {
				t.Fatal("Parse() returned nil root")
			}

			// Check block count
			if len(doc.Root.Children) != tt.wantBlocks {
				t.Errorf("Parse() got %d blocks, want %d", len(doc.Root.Children), tt.wantBlocks)
			}

			// Check block types
			for i, wantType := range tt.wantTypes {
				if i >= len(doc.Root.Children) {
					break
				}
				if doc.Root.Children[i].NodeType != wantType {
					t.Errorf("Parse() block[%d] type = %s, want %s",
						i, doc.Root.Children[i].NodeType, wantType)
				}
			}
		})
	}
}

func TestPlainTextStructurizer_HeadingLevel(t *testing.T) {
	tests := []struct {
		content   string
		wantLevel int
	}{
		{"第一章 概述", 1},
		{"1.1 简介", 2},
		{"1.1.1 详细说明", 3},
		{"一、主要功能", 2},
		{"第二章 系统设计", 1},
		{"第十一条 用户权利", 3},
	}

	ps := NewPlainTextStructurizer()

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			raw := document.New(tt.content, "text/plain")

			doc, err := ps.Parse(raw)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if len(doc.Root.Children) == 0 {
				t.Fatal("No children found")
			}

			if doc.Root.Children[0].NodeType != "heading" {
				t.Fatalf("Expected heading, got %s", doc.Root.Children[0].NodeType)
			}

			if doc.Root.Children[0].Level != tt.wantLevel {
				t.Errorf("Level = %d, want %d", doc.Root.Children[0].Level, tt.wantLevel)
			}
		})
	}
}

func TestPlainTextStructurizer_TableParsing(t *testing.T) {
	content := `| Name | Age | City |
|------|-----|------|
| John | 30  | NYC  |
| Jane | 25  | LA   |`

	ps := NewPlainTextStructurizer()
	raw := document.New(content, "text/plain")

	doc, err := ps.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(doc.Root.Children) == 0 {
		t.Fatal("No children found")
	}

	if doc.Root.Children[0].NodeType != "table" {
		t.Fatalf("Expected table, got %s", doc.Root.Children[0].NodeType)
	}
}

func TestPlainTextStructurizer_TaskList(t *testing.T) {
	content := `- [ ] Task 1
- [x] Task 2
- [ ] Task 3`

	ps := NewPlainTextStructurizer()
	raw := document.New(content, "text/plain")

	doc, err := ps.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(doc.Root.Children) == 0 {
		t.Fatal("No children found")
	}

	if doc.Root.Children[0].NodeType != "list" {
		t.Fatalf("Expected list, got %s", doc.Root.Children[0].NodeType)
	}
}

func TestPlainTextStructurizer_NestedHeadings(t *testing.T) {
	content := `第一章 顶层

1.1 子节一

1.1.1 子节一之一

内容一

1.1.2 子节一之二

内容二

1.2 子节二

内容三`

	ps := NewPlainTextStructurizer()
	raw := document.New(content, "text/plain")

	doc, err := ps.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// 验证标题层级结构
	root := doc.Root
	if len(root.Children) == 0 {
		t.Fatal("No children found")
	}

	// 第一个子节点应该是 H1
	if root.Children[0].Level != 1 {
		t.Errorf("First child level = %d, want 1", root.Children[0].Level)
	}
}

func TestPlainTextStructurizer_CustomConfig(t *testing.T) {
	config := &ClassificationConfig{
		HeadingMinLength:    5,
		HeadingMaxLength:    50,
		CodeMinLines:        3,
		CodeIndentThreshold: 0.6,
	}

	ps := NewPlainTextStructurizerWithConfig(config)

	content := `ABCD

    code
    here`

	raw := document.New(content, "text/plain")

	doc, err := ps.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// ABCD 长度 < 5，不应被识别为标题
	if len(doc.Root.Children) > 0 && doc.Root.Children[0].NodeType == "heading" {
		t.Error("ABCD should not be classified as heading with custom config")
	}
}

func BenchmarkPlainTextStructurizer_Parse(b *testing.B) {
	content := `第一章 概述

这是正文内容，包含一些文字。

- 列表项一
- 列表项二
- 列表项三

    func main() {
        fmt.Println("Hello")
    }

1.1 详细说明

更多内容在这里。

| Name | Age |
|------|-----|
| John | 30  |`

	ps := NewPlainTextStructurizer()
	raw := document.New(content, "text/plain")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Parse(raw)
	}
}
