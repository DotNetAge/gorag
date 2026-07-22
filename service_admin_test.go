package gorag

import (
	"testing"
)

// TestFoldReadmeIntoDirectories 验证 README.md 文件节点的摘要被折叠到父目录节点，且文件节点被移除。
func TestFoldReadmeIntoDirectories(t *testing.T) {
	root := &SourceTreeNode{Name: ".", IsDir: true}
	subDir := &SourceTreeNode{Name: "docs", IsDir: true}
	readmeFile := &SourceTreeNode{
		Name: "README.md",
		Chunks: []SourceChunkNode{
			{Title: "docs", Summary: "文档目录摘要。"},
			{Title: "intro", Summary: "简介摘要。"},
		},
	}
	otherFile := &SourceTreeNode{Name: "other.md"}

	subDir.Children = []*SourceTreeNode{readmeFile, otherFile}
	root.Children = []*SourceTreeNode{subDir}

	foldReadmeIntoDirectories(root)

	if subDir.Summary == "" {
		t.Fatalf("折叠后目录 Summary 不应为空")
	}
	if subDir.Summary != "文档目录摘要。；简介摘要。" {
		t.Errorf("目录 Summary 期望 %q，实际 %q", "文档目录摘要。；简介摘要。", subDir.Summary)
	}
	if len(subDir.Children) != 1 || subDir.Children[0].Name != "other.md" {
		t.Errorf("README.md 节点应被移除，剩余子节点: %v", subDir.Children)
	}
}

// TestCollectReadmeSummary 验证 README 摘要收集会去重并截断。
func TestCollectReadmeSummary(t *testing.T) {
	node := &SourceTreeNode{
		Name: "README.md",
		Chunks: []SourceChunkNode{
			{Summary: "摘要一。"},
			{Summary: "摘要一。"},
			{Summary: "摘要二。"},
		},
	}

	summary := collectReadmeSummary(node)
	if summary != "摘要一。；摘要二。" {
		t.Errorf("摘要去重后期望 %q，实际 %q", "摘要一。；摘要二。", summary)
	}
}
