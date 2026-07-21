package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ========================= New 工厂 =========================

func TestNew_ImageType(t *testing.T) {
	doc := New("base64-data", RawDocImage)
	if doc == nil {
		t.Fatal("New 不应返回 nil")
	}
	if doc.Type() != RawDocImage {
		t.Errorf("期望 docType %q, 实际 %q", RawDocImage, doc.Type())
	}
	if doc.Content() != "base64-data" {
		t.Errorf("期望 content 'base64-data', 实际: %q", doc.Content())
	}
	// New 场景 fileName 应为空
	if doc.FileName() != "" {
		t.Errorf("New 场景 fileName 应为空, 实际: %q", doc.FileName())
	}
	// size 应等于 content 长度
	if doc.Size() != int64(len("base64-data")) {
		t.Errorf("期望 size %d, 实际 %d", len("base64-data"), doc.Size())
	}
}

func TestNew_DocType(t *testing.T) {
	content := "# Markdown 标题"
	doc := New(content, RawDocDoc)
	if doc.Type() != RawDocDoc {
		t.Errorf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}
	if doc.Content() != content {
		t.Errorf("期望 content %q, 实际: %q", content, doc.Content())
	}
}

func TestNew_TextType(t *testing.T) {
	content := "纯文本内容"
	doc := New(content, RawDocText)
	if doc.Type() != RawDocText {
		t.Errorf("期望 docType %q, 实际 %q", RawDocText, doc.Type())
	}
	if doc.Content() != content {
		t.Errorf("期望 content %q, 实际: %q", content, doc.Content())
	}
}

func TestNew_DataType(t *testing.T) {
	content := `{"key":"value"}`
	doc := New(content, RawDocData)
	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}
	if doc.Content() != content {
		t.Errorf("期望 content %q, 实际: %q", content, doc.Content())
	}
}

func TestNew_UnknownTypeFallbackToText(t *testing.T) {
	// 未知 docType 应兜底为 RawDocText
	doc := New("内容", RawDocType("unknown"))
	if doc.Type() != RawDocText {
		t.Errorf("未知 docType 应兜底为 %q, 实际 %q", RawDocText, doc.Type())
	}
}

func TestNew_EmptyContent(t *testing.T) {
	doc := New("", RawDocText)
	if doc == nil {
		t.Fatal("New 处理空内容不应返回 nil")
	}
	if doc.Content() != "" {
		t.Errorf("期望空内容, 实际: %q", doc.Content())
	}
	if doc.Size() != 0 {
		t.Errorf("期望 size 0, 实际 %d", doc.Size())
	}
}

// ========================= ID 稳定性 =========================

func TestNew_IDBasedOnContent(t *testing.T) {
	// New 场景下 fileName 为空，ID 基于内容生成
	doc1 := New("相同内容", RawDocText)
	doc2 := New("相同内容", RawDocText)
	if doc1.ID() != doc2.ID() {
		t.Errorf("相同内容应生成相同 ID, doc1=%s, doc2=%s", doc1.ID(), doc2.ID())
	}
}

func TestNew_IDDifferentForDifferentContent(t *testing.T) {
	doc1 := New("内容A", RawDocText)
	doc2 := New("内容B", RawDocText)
	if doc1.ID() == doc2.ID() {
		t.Error("不同内容应生成不同 ID")
	}
}

func TestNew_IDStability(t *testing.T) {
	// ID 应为 64 字符的 SHA256 十六进制
	doc := New("稳定性测试", RawDocText)
	id := doc.ID()
	if len(id) != 64 {
		t.Errorf("期望 ID 长度 64, 实际 %d", len(id))
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("ID 应为十六进制字符, 实际包含: %c", c)
			break
		}
	}
}

func TestNew_ModTimeIsZero(t *testing.T) {
	// New 场景 modTime 应为零值
	doc := New("内容", RawDocText)
	if !doc.ModTime().IsZero() {
		t.Errorf("New 场景 modTime 应为零值, 实际: %v", doc.ModTime())
	}
}

func TestNew_MetaIsNotNil(t *testing.T) {
	// Meta 不应返回 nil（即使未设置）
	doc := New("内容", RawDocText)
	if doc.Meta() == nil {
		t.Fatal("Meta 不应返回 nil")
	}
}

// ========================= Open 工厂 =========================

func TestOpen_EmptyPath(t *testing.T) {
	_, err := Open("")
	if err == nil {
		t.Fatal("空路径应返回错误")
	}
	if !strings.Contains(err.Error(), "文件路径为空") {
		t.Errorf("错误信息应包含 '文件路径为空', 实际: %v", err)
	}
}

func TestOpen_RelativePath(t *testing.T) {
	_, err := Open("relative/path.txt")
	if err == nil {
		t.Fatal("相对路径应返回错误")
	}
	if !strings.Contains(err.Error(), "绝对路径") {
		t.Errorf("错误信息应包含 '绝对路径', 实际: %v", err)
	}
}

func TestOpen_Directory(t *testing.T) {
	// 临时目录
	tmpDir := t.TempDir()
	_, err := Open(tmpDir)
	if err == nil {
		t.Fatal("目录应返回错误")
	}
	if !strings.Contains(err.Error(), "目录") {
		t.Errorf("错误信息应包含 '目录', 实际: %v", err)
	}
}

func TestOpen_NonExistentFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "不存在的文件.txt")
	_, err := Open(missingPath)
	if err == nil {
		t.Fatal("不存在的文件应返回错误")
	}
	if !strings.Contains(err.Error(), "文件信息失败") {
		t.Errorf("错误信息应包含 '文件信息失败', 实际: %v", err)
	}
}

func TestOpen_TextFile(t *testing.T) {
	// 创建临时文本文件
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	content := "Hello, Open 工厂测试！"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}

	doc, err := Open(path)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}

	// 应归一化为 RawDocText
	if doc.Type() != RawDocText {
		t.Errorf("期望 docType %q, 实际 %q", RawDocText, doc.Type())
	}

	// FileName 应为绝对路径
	if doc.FileName() != path {
		t.Errorf("期望 FileName %q, 实际 %q", path, doc.FileName())
	}
	if !filepath.IsAbs(doc.FileName()) {
		t.Errorf("FileName 应为绝对路径, 实际: %q", doc.FileName())
	}

	// 内容应原样返回
	if doc.Content() != content {
		t.Errorf("期望 content %q, 实际: %q", content, doc.Content())
	}

	// Size 应为文件大小
	if doc.Size() != int64(len(content)) {
		t.Errorf("期望 size %d, 实际 %d", len(content), doc.Size())
	}

	// ModTime 不应为零
	if doc.ModTime().IsZero() {
		t.Error("ModTime 不应为零值")
	}
}

func TestOpen_DataFile(t *testing.T) {
	// 创建临时 JSON 文件
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.json")
	content := `{"key":"value","num":42}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}

	doc, err := Open(path)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}

	// JSON 应归一化为 RawDocData
	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}

	// FileName 应为绝对路径
	if doc.FileName() != path {
		t.Errorf("期望 FileName %q, 实际 %q", path, doc.FileName())
	}
}

// ========================= newParsedDoc 内部构造 =========================

func TestNewParsedDoc_NilMeta(t *testing.T) {
	// 传入 nil meta 不应 panic，且 Meta() 返回非 nil
	doc := newParsedDoc("内容", nil, RawDocText)
	if doc == nil {
		t.Fatal("newParsedDoc 不应返回 nil")
	}
	if doc.Meta() == nil {
		t.Fatal("Meta 不应返回 nil")
	}
}

func TestNewParsedDoc_PreservedMeta(t *testing.T) {
	meta := map[string]any{"custom": "value"}
	doc := newParsedDoc("内容", meta, RawDocData)
	if doc.Meta()["custom"] != "value" {
		t.Errorf("期望 custom 'value', 实际: %v", doc.Meta()["custom"])
	}
}

func TestNewParsedDoc_ImageType(t *testing.T) {
	doc := newParsedDoc("base64", nil, RawDocImage)
	if doc.Type() != RawDocImage {
		t.Errorf("期望 docType %q, 实际 %q", RawDocImage, doc.Type())
	}
}

func TestNewParsedDoc_DocType(t *testing.T) {
	doc := newParsedDoc("# MD", nil, RawDocDoc)
	if doc.Type() != RawDocDoc {
		t.Errorf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}
}

func TestNewParsedDoc_DataType(t *testing.T) {
	doc := newParsedDoc(`{"k":"v"}`, nil, RawDocData)
	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}
}

func TestNewParsedDoc_UnknownFallback(t *testing.T) {
	// 未知 docType 应兜底为 RawDocText
	doc := newParsedDoc("内容", nil, RawDocType("unknown"))
	if doc.Type() != RawDocText {
		t.Errorf("未知 docType 应兜底为 %q, 实际 %q", RawDocText, doc.Type())
	}
}

// ========================= withFileMeta 回填 =========================

func TestWithFileMeta_Backfill(t *testing.T) {
	// 通过 newParsedDoc 创建 doc（size 为 content 长度）
	doc := newParsedDoc("内容", nil, RawDocText)
	originalSize := doc.Size()

	// 回填文件级元数据
	modTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	withFileMeta(doc, "/abs/path/file.txt", 1024, modTime)

	if doc.FileName() != "/abs/path/file.txt" {
		t.Errorf("期望 FileName '/abs/path/file.txt', 实际: %q", doc.FileName())
	}
	// size 已被 newParsedDoc 设置（非 0），不应被覆盖
	if doc.Size() != originalSize {
		t.Errorf("size 不应被回填覆盖, 期望 %d, 实际 %d", originalSize, doc.Size())
	}
	if !doc.ModTime().Equal(modTime) {
		t.Errorf("期望 ModTime %v, 实际 %v", modTime, doc.ModTime())
	}
}

func TestWithFileMeta_ZeroSizeBackfill(t *testing.T) {
	// 通过 baseRawDoc 直接构造 size=0 的 doc，回填时应使用文件 size
	base := baseRawDoc{
		docType:  RawDocText,
		content:  "内容",
		size:     0, // 未设置
		meta:     nil,
	}
	doc := &textDoc{base}

	modTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	withFileMeta(doc, "/abs/path/file.txt", 2048, modTime)

	if doc.Size() != 2048 {
		t.Errorf("size=0 时应回填为文件大小 2048, 实际 %d", doc.Size())
	}
}

// ========================= baseRawDoc.Meta nil 兜底 =========================

func TestBaseRawDoc_MetaNilReturnsEmptyMap(t *testing.T) {
	base := baseRawDoc{
		docType: RawDocText,
		content: "内容",
		meta:    nil,
	}
	doc := &textDoc{base}

	meta := doc.Meta()
	if meta == nil {
		t.Fatal("Meta 不应返回 nil")
	}
	if len(meta) != 0 {
		t.Errorf("期望空 map, 实际长度 %d", len(meta))
	}
}

// ========================= RawDocType 字符串值 =========================

func TestRawDocType_StringValues(t *testing.T) {
	// 验证 RawDocType 的字符串值与设计一致
	tests := []struct {
		typ    RawDocType
		expect string
	}{
		{RawDocImage, "image"},
		{RawDocDoc, "document"},
		{RawDocText, "text"},
		{RawDocData, "data"},
	}
	for _, tt := range tests {
		if string(tt.typ) != tt.expect {
			t.Errorf("期望 %q, 实际 %q", tt.expect, string(tt.typ))
		}
	}
}

// ========================= getParserByExt 兜底 =========================

func TestGetParserByExt_UnknownReturnsParseText(t *testing.T) {
	// 未知扩展名应返回 ParseText
	pf := getParserByExt(".unknownext")
	if pf == nil {
		t.Fatal("getParserByExt 不应返回 nil")
	}

	// 调用 ParseText 验证：原样返回输入内容
	doc, err := pf(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("兜底 ParseText 调用失败: %v", err)
	}
	if doc.Content() != "hello" {
		t.Errorf("期望 content 'hello', 实际: %q", doc.Content())
	}
	if doc.Type() != RawDocText {
		t.Errorf("期望 docType %q, 实际 %q", RawDocText, doc.Type())
	}
}

func TestGetParserByExt_EmptyString(t *testing.T) {
	// 空扩展名应返回 ParseText 兜底
	pf := getParserByExt("")
	if pf == nil {
		t.Fatal("getParserByExt 不应返回 nil")
	}
}

func TestGetParserByExt_CaseSensitive(t *testing.T) {
	// funcDict 中的 key 是小写，传入大写应走兜底
	// 注意：Open() 中已对 ext 做了 ToLower，这里直接测 getParserByExt
	pf := getParserByExt(".TXT")
	if pf == nil {
		t.Fatal("getParserByExt 不应返回 nil")
	}
	// 大写未命中应走兜底（ParseText），结果仍是文本解析
	doc, err := pf(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("大写扩展名兜底失败: %v", err)
	}
	if doc.Type() != RawDocText {
		t.Errorf("大写扩展名应兜底为 %q, 实际 %q", RawDocText, doc.Type())
	}
}
