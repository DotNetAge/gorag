package document

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"io"
	"net/http"

	"golang.org/x/image/draw"
)

const thumbnailSize = 224

// ParseImage 读取图片文件，返回 RawDocument
// 优化：只加载缩略图（224x224）以节省内存，并检测图片真实 MIME 类型
func ParseImage(r io.Reader) (*RawDocument, error) {
	// 读取图片二进制内容
	var contentBytes []byte
	var err error
	if contentBytes, err = io.ReadAll(r); err != nil {
		return nil, err
	}

	// 检测图片真实 MIME 类型
	mimeType := detectImageMimeType(contentBytes)

	// 解码图片
	img, _, err := image.Decode(bytes.NewReader(contentBytes))
	if err != nil {
		return nil, err
	}

	// 缩放到 224x224 thumbnail 以节省内存
	thumbnail := resizeImage(img, thumbnailSize)

	// 将缩略图编码为 JPEG 并转为 base64
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	content := base64.StdEncoding.EncodeToString(buf.Bytes())

	// 创建 RawDocument，设置 MIME 类型
	doc := NewRawDoc(content)
	doc.SetValue("mime_type", mimeType)
	doc.SetValue("thumbnail_size", thumbnailSize)

	return doc, nil
}

// detectImageMimeType 检测图片的真实 MIME 类型
// 通过检查文件头魔术字节来判断
func detectImageMimeType(data []byte) string {
	if len(data) < 4 {
		return http.DetectContentType(data)
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) {
		return "image/png"
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	// GIF87a
	if bytes.HasPrefix(data, []byte("GIF87a")) {
		return "image/gif"
	}
	// GIF89a
	if bytes.HasPrefix(data, []byte("GIF89a")) {
		return "image/gif"
	}
	// WebP: RIFF....WEBP
	if bytes.HasPrefix(data, []byte("RIFF")) && len(data) >= 12 &&
		bytes.HasPrefix(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	// BMP
	if bytes.HasPrefix(data, []byte("BM")) {
		return "image/bmp"
	}
	// TIFF (little endian)
	if bytes.HasPrefix(data, []byte("II\x2A\x00")) {
		return "image/tiff"
	}
	// TIFF (big endian)
	if bytes.HasPrefix(data, []byte("MM\x00\x2A")) {
		return "image/tiff"
	}

	// 回退到 http.DetectContentType
	return http.DetectContentType(data)
}

// resizeImage 将图片缩放到指定尺寸，使用 CatmullRom 插值保持质量
func resizeImage(src image.Image, size int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	if srcWidth <= 0 || srcHeight <= 0 {
		return image.NewRGBA(image.Rect(0, 0, size, size))
	}

	// 计算缩放比例，保持宽高比
	scale := float64(size) / float64(max(srcWidth, srcHeight))
	newWidth := int(float64(srcWidth) * scale)
	newHeight := int(float64(srcHeight) * scale)

	// 创建目标图像
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// 使用 CatmullRom 进行高质量缩放
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, srcBounds, draw.Src, nil)

	return dst
}
