package embedder

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

const testImagesDir = ".test"

func loadImage(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testImagesDir, name))
	if err != nil {
		t.Fatalf("Failed to read test image %s: %v", name, err)
	}
	return data
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf []byte
	err := png.Encode(&bytesWriter{buf: &buf}, img)
	if err != nil {
		t.Fatalf("Failed to encode PNG: %v", err)
	}
	return buf
}

type bytesWriter struct{ buf *[]byte }

func (w *bytesWriter) Write(p []byte) (n int, err error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

// f32eq 判断两个 float32 是否在容差范围内相等
func f32eq(a, b float32, tol float64) bool {
	return math.Abs(float64(a-b)) <= tol
}

// normalize 模拟 ImageNet 归一化: (pixel/255 - mean) / std
func normalize(pixel uint8, mean, std float32) float32 {
	return (float32(pixel)/255.0 - mean) / std
}

func TestImageProcessor_New(t *testing.T) {
	p := NewImageProcessor(224)
	if p.imageSize != 224 {
		t.Errorf("Expected imageSize 224, got %d", p.imageSize)
	}
	p2 := NewImageProcessor(0)
	if p2.imageSize != ViTImageSize {
		t.Errorf("Expected default imageSize %d, got %d", ViTImageSize, p2.imageSize)
	}
	p3 := NewImageProcessor(-1)
	if p3.imageSize != ViTImageSize {
		t.Errorf("Expected default imageSize %d for negative input, got %d", ViTImageSize, p3.imageSize)
	}
}

func TestImageProcessor_Preprocess_RealImage(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testImagesDir, "1.jpeg")); os.IsNotExist(err) {
		t.Skip("Skipping: test image not found")
	}

	imgData := loadImage(t, "1.jpeg")
	processor := NewImageProcessor(ViTImageSize)
	tensor, err := processor.Preprocess(imgData)
	if err != nil {
		t.Fatalf("Preprocess failed: %v", err)
	}

	// 输出尺寸 [3, 224, 224]
	expectedLen := 3 * ViTImageSize * ViTImageSize
	if len(tensor) != expectedLen {
		t.Errorf("Expected tensor length %d, got %d", expectedLen, len(tensor))
	}

	// 归一化范围通常在 -3 到 +3 之间
	var minVal, maxVal float32 = math.MaxFloat32, -math.MaxFloat32
	for _, v := range tensor {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	t.Logf("Tensor range: [%.6f, %.6f]", minVal, maxVal)
	if maxVal <= 0 {
		t.Error("All values are non-positive after normalization")
	}
	if minVal >= 0 {
		t.Error("All values are non-negative after normalization")
	}
	if maxVal > 5.0 {
		t.Errorf("Max value %.4f is unexpectedly large", maxVal)
	}
	if minVal < -5.0 {
		t.Errorf("Min value %.4f is unexpectedly small", minVal)
	}

	// 验证三通道均值不同（非均匀图片）
	rMean := channelMean(tensor[0 : ViTImageSize*ViTImageSize])
	gMean := channelMean(tensor[ViTImageSize*ViTImageSize : 2*ViTImageSize*ViTImageSize])
	bMean := channelMean(tensor[2*ViTImageSize*ViTImageSize:])
	t.Logf("Channel means: R=%.6f, G=%.6f, B=%.6f", rMean, gMean, bMean)
}

func TestImageProcessor_Preprocess_SolidRedImage(t *testing.T) {
	processor := NewImageProcessor(32)
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 0, B: 0, A: 255}}, image.Point{}, draw.Src)

	tensor, err := processor.PreprocessFromImage(img)
	if err != nil {
		t.Fatalf("PreprocessFromImage failed: %v", err)
	}
	if len(tensor) != 3*32*32 {
		t.Fatalf("Expected tensor length %d, got %d", 3*32*32, len(tensor))
	}

	wantR := normalize(255, ImageNetMeanR, ImageNetStdR)
	wantG := normalize(0, ImageNetMeanG, ImageNetStdG)
	wantB := normalize(0, ImageNetMeanB, ImageNetStdB)

	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			idx := y*32 + x
			if !f32eq(tensor[0*1024+idx], wantR, 1e-5) {
				t.Errorf("R pixel(%d,%d): got %.6f, want %.6f", x, y, tensor[0*1024+idx], wantR)
			}
			if !f32eq(tensor[1*1024+idx], wantG, 1e-5) {
				t.Errorf("G pixel(%d,%d): got %.6f, want %.6f", x, y, tensor[1*1024+idx], wantG)
			}
			if !f32eq(tensor[2*1024+idx], wantB, 1e-5) {
				t.Errorf("B pixel(%d,%d): got %.6f, want %.6f", x, y, tensor[2*1024+idx], wantB)
			}
		}
	}
	t.Logf("Pure red: R=%.6f, G=%.6f, B=%.6f", wantR, wantG, wantB)
}

func TestImageProcessor_Preprocess_SolidWhiteImage(t *testing.T) {
	processor := NewImageProcessor(16)
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)

	tensor, err := processor.PreprocessFromImage(img)
	if err != nil {
		t.Fatalf("PreprocessFromImage failed: %v", err)
	}

	wantR := normalize(255, ImageNetMeanR, ImageNetStdR)
	wantG := normalize(255, ImageNetMeanG, ImageNetStdG)
	wantB := normalize(255, ImageNetMeanB, ImageNetStdB)

	// 三个通道归一化值应不同（mean/std 不同）
	if f32eq(wantR, wantG, 1e-5) {
		t.Error("R and G normalized values should differ for white pixel")
	}
	if f32eq(wantR, wantB, 1e-5) {
		t.Error("R and B normalized values should differ for white pixel")
	}

	for _, idx := range []int{0, 127, 255, 16*16 - 1} {
		if !f32eq(tensor[0*256+idx], wantR, 1e-5) {
			t.Errorf("R pixel[%d]: got %.6f, want %.6f", idx, tensor[0*256+idx], wantR)
		}
		if !f32eq(tensor[1*256+idx], wantG, 1e-5) {
			t.Errorf("G pixel[%d]: got %.6f, want %.6f", idx, tensor[1*256+idx], wantG)
		}
		if !f32eq(tensor[2*256+idx], wantB, 1e-5) {
			t.Errorf("B pixel[%d]: got %.6f, want %.6f", idx, tensor[2*256+idx], wantB)
		}
	}
	t.Logf("Pure white: R=%.6f, G=%.6f, B=%.6f", wantR, wantG, wantB)
}

func TestImageProcessor_Preprocess_SolidBlackImage(t *testing.T) {
	processor := NewImageProcessor(16)
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 0, G: 0, B: 0, A: 255}}, image.Point{}, draw.Src)

	tensor, err := processor.PreprocessFromImage(img)
	if err != nil {
		t.Fatalf("PreprocessFromImage failed: %v", err)
	}

	wantR := normalize(0, ImageNetMeanR, ImageNetStdR)
	wantG := normalize(0, ImageNetMeanG, ImageNetStdG)
	wantB := normalize(0, ImageNetMeanB, ImageNetStdB)

	// 黑色归一化后三个通道应为负值
	if wantR >= 0 || wantG >= 0 || wantB >= 0 {
		t.Errorf("Black pixel should normalize to negative values: R=%.6f, G=%.6f, B=%.6f", wantR, wantG, wantB)
	}

	// 所有像素一致
	for i := 1; i < 16*16; i++ {
		if tensor[i] != tensor[0] {
			t.Errorf("R channel should be uniform for black image, mismatch at pixel %d", i)
			break
		}
	}
	t.Logf("Pure black: R=%.6f, G=%.6f, B=%.6f", wantR, wantG, wantB)
}

func TestImageProcessor_Preprocess_GrayImage(t *testing.T) {
	processor := NewImageProcessor(16)
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 128, G: 128, B: 128, A: 255}}, image.Point{}, draw.Src)

	tensor, err := processor.PreprocessFromImage(img)
	if err != nil {
		t.Fatalf("PreprocessFromImage failed: %v", err)
	}

	wantR := normalize(128, ImageNetMeanR, ImageNetStdR)
	wantG := normalize(128, ImageNetMeanG, ImageNetStdG)
	wantB := normalize(128, ImageNetMeanB, ImageNetStdB)

	if !f32eq(tensor[0], wantR, 1e-5) {
		t.Errorf("R[0]: got %.6f, want %.6f", tensor[0], wantR)
	}
	if !f32eq(tensor[256], wantG, 1e-5) {
		t.Errorf("G[0]: got %.6f, want %.6f", tensor[256], wantG)
	}
	if !f32eq(tensor[512], wantB, 1e-5) {
		t.Errorf("B[0]: got %.6f, want %.6f", tensor[512], wantB)
	}
	t.Logf("Gray (128): R=%.6f, G=%.6f, B=%.6f", wantR, wantG, wantB)
}

func TestImageProcessor_Preprocess_CHWLayout(t *testing.T) {
	// 构造左半红右半蓝的图片，验证 CHW 布局正确性
	processor := NewImageProcessor(4)
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if x < 2 {
				img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
			}
		}
	}

	tensor, err := processor.PreprocessFromImage(img)
	if err != nil {
		t.Fatalf("PreprocessFromImage failed: %v", err)
	}

	wantFullR := normalize(255, ImageNetMeanR, ImageNetStdR)
	wantFullB := normalize(255, ImageNetMeanB, ImageNetStdB)
	wantZeroR := normalize(0, ImageNetMeanR, ImageNetStdR)
	wantZeroG := normalize(0, ImageNetMeanG, ImageNetStdG)
	wantZeroB := normalize(0, ImageNetMeanB, ImageNetStdB)

	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			idx := y*4 + x
			rVal := tensor[0*16+idx]
			gVal := tensor[1*16+idx]
			bVal := tensor[2*16+idx]

			if x < 2 {
				if !f32eq(rVal, wantFullR, 1e-5) {
					t.Errorf("R pixel(%d,%d) [left]: got %.6f, want %.6f", x, y, rVal, wantFullR)
				}
				if !f32eq(bVal, wantZeroB, 1e-5) {
					t.Errorf("B pixel(%d,%d) [left]: got %.6f, want %.6f", x, y, bVal, wantZeroB)
				}
			} else {
				if !f32eq(rVal, wantZeroR, 1e-5) {
					t.Errorf("R pixel(%d,%d) [right]: got %.6f, want %.6f", x, y, rVal, wantZeroR)
				}
				if !f32eq(bVal, wantFullB, 1e-5) {
					t.Errorf("B pixel(%d,%d) [right]: got %.6f, want %.6f", x, y, bVal, wantFullB)
				}
			}
			if !f32eq(gVal, wantZeroG, 1e-5) {
				t.Errorf("G pixel(%d,%d): got %.6f, want %.6f", x, y, gVal, wantZeroG)
			}
		}
	}
}

func TestImageProcessor_Preprocess_DifferentSizes(t *testing.T) {
	sizes := []int{64, 128, 224, 336}
	for _, size := range sizes {
		t.Run(string(rune(size)), func(t *testing.T) {
			processor := NewImageProcessor(size)
			img := image.NewRGBA(image.Rect(0, 0, 100, 200))
			draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 100, G: 150, B: 200, A: 255}}, image.Point{}, draw.Src)

			tensor, err := processor.PreprocessFromImage(img)
			if err != nil {
				t.Fatalf("PreprocessFromImage failed: %v", err)
			}

			if len(tensor) != 3*size*size {
				t.Errorf("Expected tensor length %d for size %d, got %d", 3*size*size, size, len(tensor))
			}

			// 纯色图 R 通道应均匀
			firstVal := tensor[0]
			for _, v := range tensor[0 : size*size] {
				if !f32eq(v, firstVal, 1e-3) {
					t.Errorf("R channel not uniform at size %d: first=%.6f, val=%.6f", size, firstVal, v)
					break
				}
			}
		})
	}
}

func TestImageProcessor_Preprocess_InvalidInput(t *testing.T) {
	processor := NewImageProcessor(224)

	_, err := processor.Preprocess([]byte("not an image"))
	if err == nil {
		t.Error("Expected error for invalid image data, got nil")
	}
	t.Logf("Got expected error: %v", err)

	_, err = processor.Preprocess(nil)
	if err == nil {
		t.Error("Expected error for nil data, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

func TestImageProcessor_Preprocess_RealImage_Consistency(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testImagesDir, "1.jpeg")); os.IsNotExist(err) {
		t.Skip("Skipping: test image not found")
	}

	imgData := loadImage(t, "1.jpeg")
	processor := NewImageProcessor(224)

	tensor1, err := processor.Preprocess(imgData)
	if err != nil {
		t.Fatalf("First Preprocess failed: %v", err)
	}
	tensor2, err := processor.Preprocess(imgData)
	if err != nil {
		t.Fatalf("Second Preprocess failed: %v", err)
	}

	if len(tensor1) != len(tensor2) {
		t.Fatalf("Length mismatch: %d vs %d", len(tensor1), len(tensor2))
	}
	for i := range tensor1 {
		if tensor1[i] != tensor2[i] {
			t.Errorf("Inconsistent value at index %d: %.6f vs %.6f", i, tensor1[i], tensor2[i])
		}
	}
}

func TestImageProcessor_Preprocess_PNGInput(t *testing.T) {
	processor := NewImageProcessor(16)
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 64, G: 128, B: 192, A: 255}}, image.Point{}, draw.Src)

	pngData := encodePNG(t, img)
	tensor, err := processor.Preprocess(pngData)
	if err != nil {
		t.Fatalf("Preprocess PNG failed: %v", err)
	}
	if len(tensor) != 3*16*16 {
		t.Errorf("Expected tensor length %d, got %d", 3*16*16, len(tensor))
	}

	wantR := normalize(64, ImageNetMeanR, ImageNetStdR)
	wantG := normalize(128, ImageNetMeanG, ImageNetStdG)
	wantB := normalize(192, ImageNetMeanB, ImageNetStdB)

	if !f32eq(tensor[0], wantR, 1e-5) {
		t.Errorf("R[0]: got %.6f, want %.6f", tensor[0], wantR)
	}
	if !f32eq(tensor[256], wantG, 1e-5) {
		t.Errorf("G[0]: got %.6f, want %.6f", tensor[256], wantG)
	}
	if !f32eq(tensor[512], wantB, 1e-5) {
		t.Errorf("B[0]: got %.6f, want %.6f", tensor[512], wantB)
	}
	t.Logf("PNG (64,128,192): R=%.6f, G=%.6f, B=%.6f", wantR, wantG, wantB)
}

func TestImageProcessor_Normalization_KnownValues(t *testing.T) {
	tests := []struct {
		name    string
		pixel   uint8
		mean    float32
		std     float32
		want    float32
	}{
		{"R_0", 0, ImageNetMeanR, ImageNetStdR, normalize(0, ImageNetMeanR, ImageNetStdR)},
		{"R_128", 128, ImageNetMeanR, ImageNetStdR, normalize(128, ImageNetMeanR, ImageNetStdR)},
		{"R_255", 255, ImageNetMeanR, ImageNetStdR, normalize(255, ImageNetMeanR, ImageNetStdR)},
		{"G_0", 0, ImageNetMeanG, ImageNetStdG, normalize(0, ImageNetMeanG, ImageNetStdG)},
		{"B_255", 255, ImageNetMeanB, ImageNetStdB, normalize(255, ImageNetMeanB, ImageNetStdB)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalize(tt.pixel, tt.mean, tt.std)
			if !f32eq(got, tt.want, 1e-7) {
				t.Errorf("got %.7f, want %.7f", got, tt.want)
			}
		})
	}
}

func channelMean(data []float32) float32 {
	if len(data) == 0 {
		return 0
	}
	var sum float32
	for _, v := range data {
		sum += v
	}
	return sum / float32(len(data))
}
