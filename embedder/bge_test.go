package embedder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/core"
)

const (
	bgeTestModelDir = "../models/bge-base-zh-v1.5"
	bgeTestONNXName = "model_q4.onnx" // 使用量化版本测试，更快
	bgeTestDim      = 768
)

// skipIfBGEModelNotFound 如果 BGE 模型文件不存在或是 LFS 占位符则跳过测试
func skipIfBGEModelNotFound(t *testing.T) {
	t.Helper()
	modelPath := filepath.Join(bgeTestModelDir, "onnx", bgeTestONNXName)
	info, err := os.Stat(modelPath)
	if os.IsNotExist(err) {
		t.Skipf("Skipping test: BGE model not found at %s", modelPath)
		return
	}
	// Git LFS 占位符通常小于 1KB
	if info.Size() < 1024 {
		t.Skipf("Skipping test: BGE model at %s is a Git LFS placeholder (only %d bytes), download it first", modelPath, info.Size())
	}
}

// newBGEEmbedderForTest 创建用于测试的 BGEEmbedder
func newBGEEmbedderForTest(t *testing.T) *BGEEmbedder {
	t.Helper()
	modelFile := filepath.Join(bgeTestModelDir, "onnx", bgeTestONNXName)
	embedder, err := NewBGEEmbedder(
		WithBGEModelFile(modelFile),
		WithBGEDimension(bgeTestDim),
	)
	if err != nil {
		t.Fatalf("Failed to create BGE embedder: %v", err)
	}
	return embedder
}

func TestBGEEmbedder_CalcText(t *testing.T) {
	skipIfBGEModelNotFound(t)

	embedder := newBGEEmbedderForTest(t)
	defer embedder.Close()

	testTexts := []string{
		"你好世界",
		"Hello world",
		"这是一个测试",
		"人工智能技术在各个领域得到了广泛应用",
	}

	for _, text := range testTexts {
		t.Run(text, func(t *testing.T) {
			vector, err := embedder.CalcText(text)
			if err != nil {
				t.Fatalf("CalcText failed: %v", err)
			}

			if vector == nil || len(vector.Values) == 0 {
				t.Fatal("Got nil or empty vector")
			}

			if len(vector.Values) != bgeTestDim {
				t.Errorf("Expected vector dimension %d, got %d", bgeTestDim, len(vector.Values))
			}

			norm := sqrtFloat32(vectorNorm(vector.Values))
			if norm < 0.001 {
				t.Errorf("Vector norm is %.4f (near zero), embedding output is likely invalid", norm)
			}

			t.Logf("Text: %q -> vector dim=%d, norm=%.4f, first 5 dims: %v",
				text, len(vector.Values), norm, vector.Values[:5])
		})
	}
}

func TestBGEEmbedder_Bulk(t *testing.T) {
	skipIfBGEModelNotFound(t)

	embedder := newBGEEmbedderForTest(t)
	defer embedder.Close()

	chunks := []*core.Chunk{
		{ID: "bge_chunk1", Content: "第一段文本用于测试", DocID: "bge_doc1"},
		{ID: "bge_chunk2", Content: "第二段文本用于测试", DocID: "bge_doc1"},
		{ID: "bge_chunk3", Content: "第三段文本用于测试", DocID: "bge_doc2"},
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
		if len(v.Values) == 0 {
			t.Errorf("Vector %d has empty values", i)
			continue
		}
		if len(v.Values) != bgeTestDim {
			t.Errorf("Vector %d expected dim %d, got %d", i, bgeTestDim, len(v.Values))
		}
		norm := sqrtFloat32(vectorNorm(v.Values))
		if norm < 0.001 {
			t.Errorf("Vector %d norm is %.4f (near zero), embedding output is likely invalid", i, norm)
		}
		t.Logf("Chunk %q -> dim=%d, norm=%.4f", chunks[i].Content, len(v.Values), norm)
	}
}

func TestBGEEmbedder_Bulk_SkipImageChunks(t *testing.T) {
	skipIfBGEModelNotFound(t)

	embedder := newBGEEmbedderForTest(t)
	defer embedder.Close()

	chunks := []*core.Chunk{
		{ID: "text1", Content: "文本内容", DocID: "doc1"},
		{ID: "img1", Content: "fake_image_data", DocID: "doc1", MIMEType: "image/jpeg"},
		{ID: "text2", Content: "另一段文本", DocID: "doc2"},
	}

	vectors, err := embedder.Bulk(chunks)
	if err != nil {
		t.Fatalf("Bulk failed: %v", err)
	}

	// 图片 chunk 应被跳过，只返回 2 个文本向量
	if len(vectors) != 2 {
		t.Errorf("Expected 2 vectors (image chunk should be skipped), got %d", len(vectors))
	}
}

func TestBGEEmbedder_CalcImage_NotSupported(t *testing.T) {
	skipIfBGEModelNotFound(t)

	embedder := newBGEEmbedderForTest(t)
	defer embedder.Close()

	_, err := embedder.CalcImage([]byte("fake image"))
	if err == nil {
		t.Error("Expected error for CalcImage, got nil")
	}

	_, err = embedder.CalcImages([][]byte{{0x00}})
	if err == nil {
		t.Error("Expected error for CalcImages, got nil")
	}
}

func TestBGEEmbedder_CalcText_EmptyInput(t *testing.T) {
	skipIfBGEModelNotFound(t)

	embedder := newBGEEmbedderForTest(t)
	defer embedder.Close()

	vector, err := embedder.CalcText("")
	if err != nil {
		t.Fatalf("CalcText with empty string should not error, got: %v", err)
	}
	if vector != nil {
		t.Error("Expected nil vector for empty string")
	}
}

func TestBGEEmbedder_Bulk_EmptyInput(t *testing.T) {
	skipIfBGEModelNotFound(t)

	embedder := newBGEEmbedderForTest(t)
	defer embedder.Close()

	vectors, err := embedder.Bulk(nil)
	if err != nil {
		t.Fatalf("Bulk with nil should not error, got: %v", err)
	}
	if len(vectors) != 0 {
		t.Errorf("Expected 0 vectors for nil input, got %d", len(vectors))
	}
}

func TestBGEEmbedder_Dim(t *testing.T) {
	skipIfBGEModelNotFound(t)

	embedder := newBGEEmbedderForTest(t)
	defer embedder.Close()

	if embedder.Dim() != bgeTestDim {
		t.Errorf("Expected Dim()=%d, got %d", bgeTestDim, embedder.Dim())
	}
	if embedder.Multimoding() {
		t.Error("BGE should not be multimodal")
	}
}

func TestBGEEmbedder_CalcText_SemanticSimilarity(t *testing.T) {
	skipIfBGEModelNotFound(t)

	embedder := newBGEEmbedderForTest(t)
	defer embedder.Close()

	// 语义相近的文本应该有更高的余弦相似度
	pairs := [][2]string{
		{"我喜欢吃苹果", "我爱吃苹果"},
		{"机器学习是人工智能的一个分支", "深度学习属于人工智能领域"},
	}

	for _, pair := range pairs {
		t.Run(pair[0], func(t *testing.T) {
			v1, err := embedder.CalcText(pair[0])
			if err != nil {
				t.Fatalf("CalcText failed for %q: %v", pair[0], err)
			}
			v2, err := embedder.CalcText(pair[1])
			if err != nil {
				t.Fatalf("CalcText failed for %q: %v", pair[1], err)
			}

			sim := cosineSimilarity(v1.Values, v2.Values)
			t.Logf("Similarity between %q and %q: %.4f", pair[0], pair[1], sim)

			// 语义相近的文本相似度应该大于 0
			if sim <= 0 {
				t.Errorf("Semantically similar texts should have positive cosine similarity, got %.4f", sim)
			}
		})
	}
}

func TestBGEVocabTokenizer(t *testing.T) {
	tokenizer, err := NewBGEVocabTokenizer(128)
	if err != nil {
		t.Fatalf("Failed to create BGE tokenizer: %v", err)
	}

	tests := []struct {
		name string
		text string
	}{
		{"Chinese", "自然语言处理"},
		{"English", "transformer model"},
		{"Mixed", "BERT模型用于NLP任务"},
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
			if len(inputIDs) != 128 {
				t.Errorf("Expected padded length 128, got %d", len(inputIDs))
			}

			t.Logf("Text: %q -> %d tokens, first 10 IDs: %v", tt.text, len(inputIDs), inputIDs[:10])
		})
	}
}

// vectorNorm 计算向量的 L2 范数平方
func vectorNorm(values []float32) float32 {
	var sum float32
	for _, v := range values {
		sum += v * v
	}
	return sum
}

// cosineSimilarity 计算两个向量的余弦相似度
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (sqrtFloat32(normA) * sqrtFloat32(normB))
}
