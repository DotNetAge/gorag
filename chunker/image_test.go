package chunker

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/v2/document"
)

// TestImageChunker_Metadata 验证 ImageChunker 回填 document 包提取的图片元数据。
func TestImageChunker_Metadata(t *testing.T) {
	path := writeTempImageFile(t, "test.jpg")
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewImageChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("ImageChunker 返回错误: %v", err)
	}

	if len(result.Chunks) != 1 {
		t.Fatalf("期望 1 个 chunk，实际 %d", len(result.Chunks))
	}

	chunk := result.Chunks[0]
	if chunk.Metadata["mime_type"] != "image/jpeg" {
		t.Errorf("mime_type 期望 image/jpeg，实际 %v", chunk.Metadata["mime_type"])
	}
	if size, ok := chunk.Metadata["thumbnail_size"].(int); !ok || size <= 0 {
		t.Errorf("thumbnail_size 应为正整数，实际 %v", chunk.Metadata["thumbnail_size"])
	}
}

// writeTempImageFile 创建临时 JPEG 图片文件并返回绝对路径。
func writeTempImageFile(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建临时图片失败: %v", err)
	}
	defer f.Close()

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatalf("编码 JPEG 失败: %v", err)
	}
	return path
}
