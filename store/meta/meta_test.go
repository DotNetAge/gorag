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
