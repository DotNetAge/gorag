package meta

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStore_Basic(t *testing.T) {
	dir, err := os.MkdirTemp("", "meta_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore 失败: %v", err)
	}
	defer store.Close()

	// 测试 Save 和 Get
	now := time.Now()
	doc := &Document{
		AbsolutePath: "/test/file.go",
		FileName:     "file.go",
		Extension:    ".go",
		SizeBytes:    1024,
		ModifiedAt:   now,
		ContentHash:  "abc123",
		Status:       "indexed",
		ChunkIDs:     []string{"chunk1", "chunk2"},
		IndexedAt:    &now,
	}
	if err := store.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument 失败: %v", err)
	}

	got, err := store.GetDocumentByPath("/test/file.go")
	if err != nil {
		t.Fatalf("GetDocumentByPath 失败: %v", err)
	}
	if got == nil {
		t.Fatal("GetDocumentByPath 返回 nil")
	}
	if got.ContentHash != "abc123" {
		t.Errorf("ContentHash = %q, 期望 %q", got.ContentHash, "abc123")
	}
	if len(got.ChunkIDs) != 2 {
		t.Errorf("ChunkIDs 长度 = %d, 期望 2", len(got.ChunkIDs))
	}
	if got.Status != "indexed" {
		t.Errorf("Status = %q, 期望 %q", got.Status, "indexed")
	}
}

func TestSQLiteStore_Update(t *testing.T) {
	dir, err := os.MkdirTemp("", "meta_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore 失败: %v", err)
	}
	defer store.Close()

	// 插入
	doc := &Document{
		AbsolutePath: "/test/update.go",
		FileName:     "update.go",
		Status:       "failed",
		ErrorMessage: "第一次失败",
	}
	if err := store.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument 失败: %v", err)
	}

	// 更新
	doc.Status = "indexed"
	doc.ErrorMessage = ""
	doc.ContentHash = "def456"
	if err := store.SaveDocument(doc); err != nil {
		t.Fatalf("SaveDocument 更新失败: %v", err)
	}

	got, err := store.GetDocumentByPath("/test/update.go")
	if err != nil {
		t.Fatalf("GetDocumentByPath 失败: %v", err)
	}
	if got.Status != "indexed" {
		t.Errorf("Status = %q, 期望 %q", got.Status, "indexed")
	}
	if got.ContentHash != "def456" {
		t.Errorf("ContentHash = %q, 期望 %q", got.ContentHash, "def456")
	}
}

func TestSQLiteStore_ListAndDelete(t *testing.T) {
	dir, err := os.MkdirTemp("", "meta_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore 失败: %v", err)
	}
	defer store.Close()

	docs := []*Document{
		{AbsolutePath: "/a.go", FileName: "a.go", Status: "indexed"},
		{AbsolutePath: "/b.go", FileName: "b.go", Status: "failed", ErrorMessage: "err"},
		{AbsolutePath: "/c.go", FileName: "c.go", Status: "indexed"},
	}
	for _, d := range docs {
		if err := store.SaveDocument(d); err != nil {
			t.Fatalf("SaveDocument %s 失败: %v", d.AbsolutePath, err)
		}
	}

	// 按状态列出
	failed, err := store.ListDocuments("failed")
	if err != nil {
		t.Fatalf("ListDocuments failed 失败: %v", err)
	}
	if len(failed) != 1 {
		t.Errorf("failed 记录数 = %d, 期望 1", len(failed))
	}

	all, err := store.ListDocuments("")
	if err != nil {
		t.Fatalf("ListDocuments 全部 失败: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("全部记录数 = %d, 期望 3", len(all))
	}

	// 删除
	if err := store.DeleteDocument("/a.go"); err != nil {
		t.Fatalf("DeleteDocument 失败: %v", err)
	}

	got, err := store.GetDocumentByPath("/a.go")
	if err != nil {
		t.Fatalf("GetDocumentByPath 失败: %v", err)
	}
	if got != nil {
		t.Error("删除后记录应返回 nil")
	}
}

func TestSQLiteStore_EmptyPathError(t *testing.T) {
	dir, err := os.MkdirTemp("", "meta_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore 失败: %v", err)
	}
	defer store.Close()

	// 空路径应返回错误
	if err := store.SaveDocument(&Document{AbsolutePath: ""}); err == nil {
		t.Error("SaveDocument 空路径应返回错误")
	}

	if _, err := store.GetDocumentByPath(""); err == nil {
		t.Error("GetDocumentByPath 空路径应返回错误")
	}

	if err := store.DeleteDocument(""); err == nil {
		t.Error("DeleteDocument 空路径应返回错误")
	}
}

func TestSQLiteStore_UsageCRUD(t *testing.T) {
	dir, err := os.MkdirTemp("", "meta_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore 失败: %v", err)
	}
	defer store.Close()

	now := time.Now()

	// 插入两条记录
	usages := []*Usage{
		{
			Model:            "gpt-4o",
			Label:            "Summarizer(单条)",
			PromptTokens:     150,
			CompletionTokens: 50,
			TotalTokens:      200,
			CachedTokens:     10,
			ReasoningTokens:  5,
			CreatedAt:        now,
		},
		{
			Model:            "gpt-4o",
			Label:            "Refiller",
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
			CreatedAt:        now.Add(-time.Hour),
		},
	}

	for i, u := range usages {
		if err := store.SaveUsage(u); err != nil {
			t.Fatalf("SaveUsage[%d] 失败: %v", i, err)
		}
	}

	// 查询全部
	all, err := store.QueryUsages(0)
	if err != nil {
		t.Fatalf("QueryUsages 失败: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("记录数 = %d, 期望 2", len(all))
	}

	// 查询限制 1 条（按时间倒序，应返回第 1 条）
	limited, err := store.QueryUsages(1)
	if err != nil {
		t.Fatalf("QueryUsages(1) 失败: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limited 记录数 = %d, 期望 1", len(limited))
	}
	if limited[0].Label != "Summarizer(单条)" {
		t.Errorf("最新记录 Label = %q, 期望 %q", limited[0].Label, "Summarizer(单条)")
	}
	if limited[0].TotalTokens != 200 {
		t.Errorf("TotalTokens = %d, 期望 200", limited[0].TotalTokens)
	}

	// 空表查询
	emptyStore, err := NewSQLiteStore(filepath.Join(dir, "empty.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore 空表失败: %v", err)
	}
	defer emptyStore.Close()

	empty, err := emptyStore.QueryUsages(0)
	if err != nil {
		t.Fatalf("QueryUsages 空表失败: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("空表应返回空切片，得到 %v", empty)
	}
}
