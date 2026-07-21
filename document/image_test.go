package document

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// ========================= detectImageMimeType =========================

func TestDetectImageMimeType_PNG(t *testing.T) {
	// PNG 魔术字节: 89 50 4E 47 0D 0A 1A 0A
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if got := detectImageMimeType(data); got != "image/png" {
		t.Errorf("期望 image/png, 实际: %q", got)
	}
}

func TestDetectImageMimeType_JPEG(t *testing.T) {
	// JPEG 魔术字节: FF D8 FF
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	if got := detectImageMimeType(data); got != "image/jpeg" {
		t.Errorf("期望 image/jpeg, 实际: %q", got)
	}
}

func TestDetectImageMimeType_GIF87a(t *testing.T) {
	data := []byte("GIF87a")
	if got := detectImageMimeType(data); got != "image/gif" {
		t.Errorf("期望 image/gif, 实际: %q", got)
	}
}

func TestDetectImageMimeType_GIF89a(t *testing.T) {
	data := []byte("GIF89a")
	if got := detectImageMimeType(data); got != "image/gif" {
		t.Errorf("期望 image/gif, 实际: %q", got)
	}
}

func TestDetectImageMimeType_WebP(t *testing.T) {
	// WebP: RIFF....WEBP
	data := []byte("RIFF\x00\x00\x00\x00WEBP")
	if got := detectImageMimeType(data); got != "image/webp" {
		t.Errorf("期望 image/webp, 实际: %q", got)
	}
}

func TestDetectImageMimeType_WebP_TooShort(t *testing.T) {
	// 不足 12 字节的 RIFF 不应识别为 WebP
	data := []byte("RIFF\x00\x00\x00")
	if got := detectImageMimeType(data); got == "image/webp" {
		t.Errorf("不足 12 字节的 RIFF 不应识别为 image/webp, 实际: %q", got)
	}
}

func TestDetectImageMimeType_BMP(t *testing.T) {
	data := []byte("BM\x00\x00")
	if got := detectImageMimeType(data); got != "image/bmp" {
		t.Errorf("期望 image/bmp, 实际: %q", got)
	}
}

func TestDetectImageMimeType_TIFF_LittleEndian(t *testing.T) {
	// TIFF 小端: II 2A 00
	data := []byte{'I', 'I', 0x2A, 0x00}
	if got := detectImageMimeType(data); got != "image/tiff" {
		t.Errorf("期望 image/tiff, 实际: %q", got)
	}
}

func TestDetectImageMimeType_TIFF_BigEndian(t *testing.T) {
	// TIFF 大端: MM 00 2A
	data := []byte{'M', 'M', 0x00, 0x2A}
	if got := detectImageMimeType(data); got != "image/tiff" {
		t.Errorf("期望 image/tiff, 实际: %q", got)
	}
}

func TestDetectImageMimeType_UnknownFallback(t *testing.T) {
	// 未知格式应走 http.DetectContentType 兜底
	data := []byte("plain text data")
	got := detectImageMimeType(data)
	if got == "" {
		t.Fatal("未知格式兜底不应返回空字符串")
	}
	if !strings.HasPrefix(got, "text/plain") {
		t.Errorf("期望兜底返回 text/plain 前缀, 实际: %q", got)
	}
}

func TestDetectImageMimeType_Empty(t *testing.T) {
	// 空数据应走兜底逻辑（不 panic）
	got := detectImageMimeType(nil)
	if got == "" {
		t.Fatal("空数据兜底不应返回空字符串")
	}
}

// ========================= resizeImage =========================

func TestResizeImage_Square(t *testing.T) {
	// 创建 100x100 的纯色图片
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	fillImage(src, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	dst := resizeImage(src, thumbnailSize)
	if dst == nil {
		t.Fatal("resizeImage 不应返回 nil")
	}

	bounds := dst.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("缩放后尺寸应大于 0, 实际: %dx%d", bounds.Dx(), bounds.Dy())
	}

	// 最大边应不超过 thumbnailSize
	if bounds.Dx() > thumbnailSize || bounds.Dy() > thumbnailSize {
		t.Errorf("缩放后最大边应不超过 %d, 实际: %dx%d", thumbnailSize, bounds.Dx(), bounds.Dy())
	}

	// 正方形图片缩放后应仍为正方形
	if bounds.Dx() != bounds.Dy() {
		t.Errorf("正方形图片缩放后应保持正方形, 实际: %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestResizeImage_Landscape(t *testing.T) {
	// 横向图片 200x100
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	fillImage(src, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	dst := resizeImage(src, thumbnailSize)
	bounds := dst.Bounds()

	// 宽应大于高
	if bounds.Dx() <= bounds.Dy() {
		t.Errorf("横向图片缩放后宽应大于高, 实际: %dx%d", bounds.Dx(), bounds.Dy())
	}
	// 最大边（宽）应等于 thumbnailSize
	if bounds.Dx() != thumbnailSize {
		t.Errorf("横向图片宽应等于 %d, 实际: %d", thumbnailSize, bounds.Dx())
	}
}

func TestResizeImage_Portrait(t *testing.T) {
	// 纵向图片 100x200
	src := image.NewRGBA(image.Rect(0, 0, 100, 200))
	fillImage(src, color.RGBA{R: 0, G: 0, B: 255, A: 255})

	dst := resizeImage(src, thumbnailSize)
	bounds := dst.Bounds()

	// 高应大于宽
	if bounds.Dy() <= bounds.Dx() {
		t.Errorf("纵向图片缩放后高应大于宽, 实际: %dx%d", bounds.Dx(), bounds.Dy())
	}
	// 最大边（高）应等于 thumbnailSize
	if bounds.Dy() != thumbnailSize {
		t.Errorf("纵向图片高应等于 %d, 实际: %d", thumbnailSize, bounds.Dy())
	}
}

func TestResizeImage_SmallerThanThumbnail(t *testing.T) {
	// 比缩略图小的图片：仍会按 thumbnailSize 放大（max 边等于 thumbnailSize）
	src := image.NewRGBA(image.Rect(0, 0, 50, 50))
	fillImage(src, color.RGBA{R: 128, G: 128, B: 128, A: 255})

	dst := resizeImage(src, thumbnailSize)
	bounds := dst.Bounds()
	if bounds.Dx() != thumbnailSize || bounds.Dy() != thumbnailSize {
		t.Errorf("小图缩放后应为 %dx%d, 实际: %dx%d", thumbnailSize, thumbnailSize, bounds.Dx(), bounds.Dy())
	}
}

func TestResizeImage_ZeroSize(t *testing.T) {
	// 0 尺寸图片应返回 thumbnailSize x thumbnailSize 的空图
	src := image.NewRGBA(image.Rect(0, 0, 0, 0))
	dst := resizeImage(src, thumbnailSize)
	bounds := dst.Bounds()
	if bounds.Dx() != thumbnailSize || bounds.Dy() != thumbnailSize {
		t.Errorf("0 尺寸图应返回 %dx%d 空图, 实际: %dx%d", thumbnailSize, thumbnailSize, bounds.Dx(), bounds.Dy())
	}
}

// fillImage 用指定颜色填充图片
func fillImage(img *image.RGBA, c color.RGBA) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
