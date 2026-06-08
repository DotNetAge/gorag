package minirag

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// --- Mock Embedder for testing ---

type mockEmbedder struct {
	dim int
}

func (m *mockEmbedder) EmbedText(text string) ([]byte, error) {
	vec := make([]byte, m.dim*4)
	for i := 0; i < m.dim; i++ {
		val := float32(i) / float32(m.dim)
		binary.LittleEndian.PutUint32(vec[i*4:i*4+4], math.Float32bits(val))
	}
	return vec, nil
}

type failingEmbedder struct {
	err error
}

func (f *failingEmbedder) EmbedText(text string) ([]byte, error) {
	return nil, f.err
}

// --- Helper to create temp directory ---

func tmpDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// --- AddText Tests ---

func TestAddText_SingleParagraph(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	result, err := rag.AddText("Hello world, this is a single paragraph.")
	if err != nil {
		t.Fatalf("AddText() error: %v", err)
	}

	var chunks []struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	if err := unmarshalJSON(result, &chunks); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content != "Hello world, this is a single paragraph." {
		t.Errorf("unexpected content: %q", chunks[0].Content)
	}
	if chunks[0].ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestAddText_MultipleParagraphs(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	content := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	result, err := rag.AddText(content)
	if err != nil {
		t.Fatalf("AddText() error: %v", err)
	}

	var chunks []struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	if err := unmarshalJSON(result, &chunks); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}
	expectedContents := []string{"First paragraph.", "Second paragraph.", "Third paragraph."}
	for i, chunk := range chunks {
		if chunk.Content != expectedContents[i] {
			t.Errorf("chunk[%d] content = %q, want %q", i, chunk.Content, expectedContents[i])
		}
	}
}

func TestAddText_EmptyString(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	result, err := rag.AddText("")
	if err != nil {
		t.Fatalf("AddText() error: %v", err)
	}

	if string(result) != "[]" {
		t.Errorf("expected empty array JSON, got %s", string(result))
	}
}

func TestAddText_WhitespaceOnly(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	testCases := []string{"   ", "\t", "\n", "  \n\t  "}
	for _, tc := range testCases {
		result, err := rag.AddText(tc)
		if err != nil {
			t.Fatalf("AddText(%q) error: %v", tc, err)
		}
		if string(result) != "[]" {
			t.Errorf("AddText(%q): expected [], got %s", tc, string(result))
		}
	}
}

func TestAddText_EmbedError(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &failingEmbedder{err: os.ErrClosed})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	_, err = rag.AddText("test content")
	if err == nil {
		t.Fatal("expected error from embedder failure")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestAddText_SpecialCharacters(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	content := "Hello 世界 🌍 Emoji & <special> \"chars\""
	result, err := rag.AddText(content)
	if err != nil {
		t.Fatalf("AddText() error: %v", err)
	}

	var chunks []struct {
		Content string `json:"content"`
	}
	if err := unmarshalJSON(result, &chunks); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(chunks) != 1 || chunks[0].Content != content {
		t.Errorf("special chars not preserved")
	}
}

func TestAddText_LongContent(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	longContent := make([]rune, 10000)
	for i := range longContent {
		longContent[i] = rune('A' + (i % 26))
	}
	content := string(longContent)

	result, err := rag.AddText(content)
	if err != nil {
		t.Fatalf("AddText() error: %v", err)
	}

	var chunks []struct {
		Content string `json:"content"`
	}
	if err := unmarshalJSON(result, &chunks); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	totalLen := 0
	for _, c := range chunks {
		totalLen += len(c.Content)
	}
	if totalLen == 0 {
		t.Error("total content length should be > 0")
	}
}

func TestAddText_ConsistentIDs(t *testing.T) {
	// Verify that contentID produces consistent hashes for same input
	content := "Same content"
	id1 := contentID(content)
	id2 := contentID(content)

	if id1 != id2 {
		t.Errorf("same content should produce same ID: %q vs %q", id1, id2)
	}

	// Different content should produce different IDs
	differentContent := "Different content"
	id3 := contentID(differentContent)
	if id1 == id3 {
		t.Error("different content should produce different IDs")
	}
}

// --- AddFile Tests ---

func TestAddFile_ValidTxtFile(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	// Create temp file
	filePath := filepath.Join(dir, "test.txt")
	content := "First line\n\nSecond line"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := rag.AddFile(filePath)
	if err != nil {
		t.Fatalf("AddFile() error: %v", err)
	}

	var chunks []struct {
		ID       string `json:"id"`
		Content  string `json:"content"`
		Filename string `json:"filename"`
		Filepath string `json:"filepath"`
	}
	if err := unmarshalJSON(result, &chunks); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if chunk.Filename != "test.txt" {
			t.Errorf("filename = %q, want %q", chunk.Filename, "test.txt")
		}
		if chunk.Filepath == "" {
			t.Error("filepath should not be empty")
		}
	}
}

func TestAddFile_EmptyPath(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	_, err = rag.AddFile("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !contains(err.Error(), "empty") {
		t.Errorf("error should mention 'empty': %v", err)
	}
}

func TestAddFile_FileNotFound(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	_, err = rag.AddFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

func TestAddFile_DirectoryInsteadOfFile(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	// dir itself is a directory
	_, err = rag.AddFile(dir)
	if err == nil {
		t.Fatal("expected error for directory path")
	}
	if !contains(err.Error(), "directory") {
		t.Errorf("error should mention 'directory': %v", err)
	}
}

func TestAddFile_EmptyFile(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	filePath := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := rag.AddFile(filePath)
	if err != nil {
		t.Fatalf("AddFile() error: %v", err)
	}

	if string(result) != "[]" {
		t.Errorf("expected [] for empty file, got %s", string(result))
	}
}

func TestAddFile_WhitespaceOnlyFile(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	filePath := filepath.Join(dir, "whitespace.txt")
	if err := os.WriteFile(filePath, []byte("   \n\t\n   "), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := rag.AddFile(filePath)
	if err != nil {
		t.Fatalf("AddFile() error: %v", err)
	}

	if string(result) != "[]" {
		t.Errorf("expected [] for whitespace-only file, got %s", string(result))
	}
}

func TestAddFile_LargeFileRejected(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	filePath := filepath.Join(dir, "large.bin")
	// Create file larger than 10MB limit
	largeData := make([]byte, 11<<20) // 11MB
	if err := os.WriteFile(filePath, largeData, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err = rag.AddFile(filePath)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
	if !contains(err.Error(), "too large") {
		t.Errorf("error should mention 'too large': %v", err)
	}
}

func TestAddFile_AtMaxSizeAccepted(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	filePath := filepath.Join(dir, "maxsize.txt")
	// Create file exactly at 10MB limit
	data := make([]byte, 10<<20) // 10MB - at the boundary
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Should NOT reject - it's exactly at the limit
	_, err = rag.AddFile(filePath)
	if err != nil && contains(err.Error(), "too large") {
		t.Errorf("file at max size (10MB) should be accepted, got: %v", err)
	}
	// Note: may fail for other reasons (embed), but not for size
}

func TestAddFile_RelativePathResolved(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	// Create file and use relative path
	filePath := filepath.Join(dir, "relative_test.txt")
	content := "Relative path content"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Change to dir and use relative path
	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	result, err := rag.AddFile("relative_test.txt")
	os.Chdir(originalDir)

	if err != nil {
		t.Fatalf("AddFile() with relative path error: %v", err)
	}

	var chunks []struct {
		Filepath string `json:"filepath"`
	}
	unmarshalJSON(result, &chunks)
	if len(chunks) > 0 && !filepath.IsAbs(chunks[0].Filepath) {
		t.Errorf("filepath should be absolute, got: %s", chunks[0].Filepath)
	}
}

func TestAddFile_FilenameExtraction(t *testing.T) {
	testCases := []struct {
		filename     string
		expectedBase string
		content      string
	}{
		{"simple.txt", "simple.txt", "content A"},
		{"document.pdf", "document.pdf", "content B"},
		{"file_with_underscores.go", "file_with_underscores.go", "content C"},
		{"报告文档.md", "报告文档.md", "content D"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			dir := tmpDir(t)
			rag, err := New(dir, 16, &mockEmbedder{dim: 16})
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			defer rag.Close()

			filePath := filepath.Join(dir, tc.filename)
			if err := os.WriteFile(filePath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("write %s: %v", tc.filename, err)
			}

			result, err := rag.AddFile(filePath)
			if err != nil {
				t.Fatalf("AddFile(%s) error: %v", tc.filename, err)
			}

			var chunks []struct {
				Filename string `json:"filename"`
			}
			unmarshalJSON(result, &chunks)
			if len(chunks) == 0 {
				t.Fatalf("%s: expected at least one chunk", tc.filename)
			}
			if chunks[0].Filename != tc.expectedBase {
				t.Errorf("filename = %q, want %q", chunks[0].Filename, tc.expectedBase)
			}
		})
	}
}

func TestAddFile_MultiParagraphFile(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	content := "Paragraph one.\n\nParagraph two.\n\nParagraph three."
	filePath := filepath.Join(dir, "multi.txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := rag.AddFile(filePath)
	if err != nil {
		t.Fatalf("AddFile() error: %v", err)
	}

	var chunks []struct {
		Content  string `json:"content"`
		Filename string `json:"filename"`
		Filepath string `json:"filepath"`
	}
	unmarshalJSON(result, &chunks)

	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}

	// All chunks should share filename/filepath
	for _, chunk := range chunks {
		if chunk.Filename != "multi.txt" {
			t.Errorf("filename mismatch: %q", chunk.Filename)
		}
		if !contains(chunk.Filepath, "multi.txt") {
			t.Errorf("filepath should contain filename: %s", chunk.Filepath)
		}
	}
}

func TestAddFile_SpecialCharactersInContent(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &mockEmbedder{dim: 16})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	specialContent := "Unicode: 中文 🎉\nSpecial: <>&\"'\nCode: func main() {}"
	filePath := filepath.Join(dir, "special.txt")
	if err := os.WriteFile(filePath, []byte(specialContent), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := rag.AddFile(filePath)
	if err != nil {
		t.Fatalf("AddFile() error: %v", err)
	}

	var chunks []struct {
		Content string `json:"content"`
	}
	unmarshalJSON(result, &chunks)

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	// Verify special characters are preserved in first chunk
	if !contains(chunks[0].Content, "中文") || !contains(chunks[0].Content, "🎉") {
		t.Errorf("special characters not preserved: %q", chunks[0].Content)
	}
}

func TestAddFile_EMBEDError(t *testing.T) {
	dir := tmpDir(t)
	rag, err := New(dir, 16, &failingEmbedder{err: os.ErrPermission})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer rag.Close()

	filePath := filepath.Join(dir, "fail.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err = rag.AddFile(filePath)
	if err == nil {
		t.Fatal("expected error from embedder failure")
	}
}

// --- Utility functions ---

func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
