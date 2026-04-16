package embedder

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // 注册 JPEG 解码器
	_ "image/png"  // 注册 PNG 解码器

	"golang.org/x/image/draw"
)

const (
	// ViT 标准输入尺寸
	ViTImageSize = 224

	// ImageNet 归一化参数 (CLIP 使用的)
	ImageNetMeanR = 0.48145466
	ImageNetMeanG = 0.45782750
	ImageNetMeanB = 0.40821073
	ImageNetStdR  = 0.26862954
	ImageNetStdG  = 0.26130258
	ImageNetStdB  = 0.27577711
)

// ImageProcessor 图像预处理器，用于 ViT/CLIP 图像编码
type ImageProcessor struct {
	imageSize int
}

// NewImageProcessor 创建图像预处理器
func NewImageProcessor(imageSize int) *ImageProcessor {
	if imageSize <= 0 {
		imageSize = ViTImageSize
	}
	return &ImageProcessor{
		imageSize: imageSize,
	}
}

// Preprocess 预处理图像：decode + resize + 归一化
// 输入: 原始图像数据 (JPEG/PNG等)
// 输出: [3, H, W] 的 float32 数组 (CHW 格式，符合 ViT 输入)
func (p *ImageProcessor) Preprocess(data []byte) ([]float32, error) {
	// 1. 解码图像 (使用标准库，自动注册 JPEG/PNG 解码器)
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// 2. Resize 到目标尺寸
	resized := p.resize(img, p.imageSize)

	// 3. 转换为 RGB 并归一化
	return p.toNormalizedTensor(resized)
}

// PreprocessFromImage 直接从 image.Image 预处理
func (p *ImageProcessor) PreprocessFromImage(img image.Image) ([]float32, error) {
	// 1. Resize 到目标尺寸
	resized := p.resize(img, p.imageSize)

	// 2. 转换为 RGB 并归一化
	return p.toNormalizedTensor(resized)
}

// resize 将图像 resize 到目标尺寸 (使用 CatmullRom 插值，质量更好)
func (p *ImageProcessor) resize(src image.Image, size int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// 防止除零
	if srcWidth <= 0 || srcHeight <= 0 {
		return image.NewRGBA(image.Rect(0, 0, size, size))
	}

	// 创建目标图像
	dst := image.NewRGBA(image.Rect(0, 0, size, size))

	// 使用 CatmullRom 进行高质量缩放
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, srcBounds, draw.Src, nil)

	return dst
}

// toNormalizedTensor 将图像转换为归一化的 float32 tensor [3, H, W]
func (p *ImageProcessor) toNormalizedTensor(img image.Image) ([]float32, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 创建输出 tensor [3, H, W] (CHW 格式)
	data := make([]float32, 3*height*width)

	// 复用已存在的 RGBA 图像，避免不必要的分配
	var rgba *image.RGBA
	if pr, ok := img.(*image.RGBA); ok {
		rgba = pr
	} else {
		rgba = image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	}

	// 转换为归一化的 float32
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := rgba.PixOffset(x, y)
			// RGBA 格式: R, G, B, A
			r := float32(rgba.Pix[idx+0]) / 255.0
			g := float32(rgba.Pix[idx+1]) / 255.0
			b := float32(rgba.Pix[idx+2]) / 255.0

			// ImageNet 归一化 (CLIP 使用的 mean/std)
			rNorm := (r - ImageNetMeanR) / ImageNetStdR
			gNorm := (g - ImageNetMeanG) / ImageNetStdG
			bNorm := (b - ImageNetMeanB) / ImageNetStdB

			// CHW 格式存储
			data[0*height*width+y*width+x] = rNorm
			data[1*height*width+y*width+x] = gNorm
			data[2*height*width+y*width+x] = bNorm
		}
	}

	return data, nil
}
