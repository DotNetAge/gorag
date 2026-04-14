package embedder

import (
	"os"
	"testing"

	"github.com/DotNetAge/gorag/core"
)

const (
	testModelDir = "../models/chinese-clip-vit-base-patch16"
	testONNXName = "model_q4.onnx" // 使用量化版本测试，更快
)

func TestVocabTokenizer(t *testing.T) {
	tokenizer, err := NewVocabTokenizer(52)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name     string
		text     string
		wantLen  int // 期望的 token 数量（大致）
	}{
		{"Chinese", "你好世界", 4},
		{"English", "hello world", 3},
		{"Mixed", "Hello 你好 world 世界", 6},
		{"With punctuation", "你好，世界！", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputIDs, mask, err := tokenizer.Tokenize(tt.text)
			if err != nil {
				t.Fatalf("Tokenize failed: %v", err)
			}

			if len(inputIDs) == 0 {
				t.Error("Empty input IDs")
			}

			if len(inputIDs) != len(mask) {
				t.Errorf("Input IDs length %d != mask length %d", len(inputIDs), len(mask))
			}

			t.Logf("Text: %q -> %d tokens, IDs: %v", tt.text, len(inputIDs), inputIDs)

			// 验证特殊 token
			if inputIDs[0] != int64(tokenizer.clsID) {
				t.Errorf("First token should be [CLS], got %d", inputIDs[0])
			}
			// 找到最后一个非PAD的token，应该是[SEP]
			lastNonPad := len(inputIDs) - 1
			for lastNonPad > 0 && inputIDs[lastNonPad] == int64(tokenizer.padID) {
				lastNonPad--
			}
			if inputIDs[lastNonPad] != int64(tokenizer.sepID) {
				t.Errorf("Last non-pad token should be [SEP], got %d", inputIDs[lastNonPad])
			}
		})
	}
}

func TestImageProcessor(t *testing.T) {
	processor := NewImageProcessor(224)

	// 读取测试图片
	imgData, err := os.ReadFile("../inputs/sample_1.jpg")
	if err != nil {
		t.Fatalf("Failed to read test image: %v", err)
	}

	// 预处理
	tensor, err := processor.Preprocess(imgData)
	if err != nil {
		t.Fatalf("Failed to preprocess image: %v", err)
	}

	// 验证输出尺寸
	expectedLen := 3 * 224 * 224
	if len(tensor) != expectedLen {
		t.Errorf("Expected tensor length %d, got %d", expectedLen, len(tensor))
	}

	// 检查归一化后的值（应该在 -2 到 2 之间）
	var maxVal, minVal float32
	for _, v := range tensor {
		if v > maxVal {
			maxVal = v
		}
		if v < minVal {
			minVal = v
		}
	}

	t.Logf("Tensor range: [%.3f, %.3f]", minVal, maxVal)

	// 验证有正有负（已归一化）
	if maxVal <= 0 {
		t.Error("All values should not be non-positive after normalization")
	}
}

func TestONNXEmbedder_Text(t *testing.T) {
	// 跳过如果模型文件不存在
	if _, err := os.Stat(testModelDir + "/onnx/" + testONNXName); os.IsNotExist(err) {
		t.Skipf("Skipping test: ONNX model not found at %s", testONNXName)
	}

	embedder, err := NewChineseClipEmbedder(WithModelDir(testModelDir), WithModel("onnx/"+testONNXName))
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}
	defer embedder.Close()

	// 测试文本向量化
	testTexts := []string{
		"你好世界",
		"Hello world",
		"这是一个测试",
	}

	for _, text := range testTexts {
		t.Run(text, func(t *testing.T) {
			vector, err := embedder.CalcText(text)
			if err != nil {
				t.Fatalf("CalcText failed: %v", err)
			}

			if vector == nil || len(vector.Values) == 0 {
				t.Fatal("Got empty vector")
			}

			if len(vector.Values) != 512 {
				t.Errorf("Expected vector dimension 512, got %d", len(vector.Values))
			}

			// 计算向量范数
			var norm float32
			for _, v := range vector.Values {
				norm += v * v
			}
			norm = sqrtFloat32(norm)

			t.Logf("Text: %q -> vector norm: %.4f, first 5 dims: %v", text, norm, vector.Values[:5])
		})
	}
}

func TestONNXEmbedder_Image(t *testing.T) {
	// 跳过如果模型文件不存在
	if _, err := os.Stat(testModelDir + "/onnx/" + testONNXName); os.IsNotExist(err) {
		t.Skipf("Skipping test: ONNX model not found at %s", testONNXName)
	}

	embedder, err := NewChineseClipEmbedder(WithModelDir(testModelDir), WithModel("onnx/"+testONNXName))
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}
	defer embedder.Close()

	// 测试图像向量化
	imgFiles := []string{"../inputs/sample_1.jpg", "../inputs/sample_2.jpg"}

	for _, imgFile := range imgFiles {
		t.Run(imgFile, func(t *testing.T) {
			imgData, err := os.ReadFile(imgFile)
			if err != nil {
				t.Fatalf("Failed to read image: %v", err)
			}

			vector, err := embedder.CalcImage(imgData)
			if err != nil {
				t.Fatalf("CalcImage failed: %v", err)
			}

			if vector == nil || len(vector.Values) == 0 {
				t.Fatal("Got empty vector")
			}

			if len(vector.Values) != 512 {
				t.Errorf("Expected vector dimension 512, got %d", len(vector.Values))
			}

			// 计算向量范数
			var norm float32
			for _, v := range vector.Values {
				norm += v * v
			}
			norm = sqrtFloat32(norm)

			t.Logf("Image: %s -> vector norm: %.4f, first 5 dims: %v", imgFile, norm, vector.Values[:5])
		})
	}
}

func TestONNXEmbedder_Bulk(t *testing.T) {
	// 跳过如果模型文件不存在
	if _, err := os.Stat(testModelDir + "/onnx/" + testONNXName); os.IsNotExist(err) {
		t.Skipf("Skipping test: ONNX model not found at %s", testONNXName)
	}

	embedder, err := NewChineseClipEmbedder(WithModelDir(testModelDir), WithModel("onnx/"+testONNXName))
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}
	defer embedder.Close()

	chunks := []*core.Chunk{
		{
			ID:      "chunk1",
			Content: "第一段文本",
			DocID:   "doc1",
		},
		{
			ID:      "chunk2",
			Content: "第二段文本",
			DocID:   "doc1",
		},
		{
			ID:      "chunk3",
			Content: "第三段文本",
			DocID:   "doc2",
		},
	}

	vectors, err := embedder.Bulk(chunks)
	if err != nil {
		t.Fatalf("Bulk failed: %v", err)
	}

	if len(vectors) != len(chunks) {
		t.Errorf("Expected %d vectors, got %d", len(chunks), len(vectors))
	}

	for i, v := range vectors {
		if v.ChunkID != chunks[i].ID {
			t.Errorf("Vector %d ChunkID mismatch: expected %s, got %s", i, chunks[i].ID, v.ChunkID)
		}
		t.Logf("Chunk %s -> vector len=%d", chunks[i].Content, len(v.Values))
	}
}

func sqrtFloat32(x float32) float32 {
	if x < 0 {
		return 0
	}
	// 简单牛顿法
	z := x / 2
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}
