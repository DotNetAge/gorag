package embedder

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
	ort "github.com/yalue/onnxruntime_go"
)

// onnxInitOnce 确保 ONNX Runtime 全局只初始化一次
var onnxInitOnce sync.Once
var onnxInitErr error

// initONNX 幂等初始化 ONNX Runtime（全局只执行一次）
func initONNX() error {
	onnxInitOnce.Do(func() {
		ort.SetSharedLibraryPath(getORTSharedLibraryPath())
		onnxInitErr = ort.InitializeEnvironment()
	})
	return onnxInitErr
}

// ChineseClipOption 是 ChineseClipEmbedder 的配置选项
type ChineseClipOption func(*chineseClipConfig)

type chineseClipConfig struct {
	modelFile string
	vocabPath string // 空则使用内嵌 vocab
}

// WithModelFile 设置模型目录
func WithModelFile(filePath string) ChineseClipOption {
	return func(c *chineseClipConfig) {
		c.modelFile = filePath
	}
}

// WithVocab 设置外部 vocab 文件路径（覆盖内嵌 vocab）
func WithVocab(path string) ChineseClipOption {
	return func(c *chineseClipConfig) {
		c.vocabPath = path
	}
}

// ChineseClipEmbedder 使用 onnxruntime-go 进行 Chinese-CLIP ONNX 模型推理
type ChineseClipEmbedder struct {
	textEncoder    *TextEncoder    // 通用文本编码器
	imageEncoder   *ImageEncoder   // 通用图像编码器
	tokenizer      *VocabTokenizer
	imageProcessor *ImageProcessor
	imageSize      int
	seqLength      int
}

// NewChineseClipEmbedder 创建 Chinese-CLIP 向量化器
// 默认从 ./models/model.onnx 加载模型，可通过 WithModel/WithModelDir 自定义
func NewChineseClipEmbedder(opts ...ChineseClipOption) (*ChineseClipEmbedder, error) {
	cfg := &chineseClipConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// 初始化 ONNX Runtime（幂等，全局只执行一次）
	if err := initONNX(); err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	// 初始化分词器
	var tokenizer *VocabTokenizer
	var err error
	seqLength := 52
	if cfg.vocabPath != "" {
		tokenizer, err = NewVocabTokenizerFromFile(cfg.vocabPath, seqLength)
		if err != nil {
			return nil, fmt.Errorf("failed to load external vocab: %w", err)
		}
	} else {
		tokenizer, err = NewVocabTokenizer(seqLength)
		if err != nil {
			return nil, fmt.Errorf("failed to load embedded vocab: %w", err)
		}
	}

	// TODO: 如果模型文件不存在就返回nil
	modelFile := cfg.modelFile

	// Chinese-CLIP ONNX 模型的输入输出是固定的
	// 文本编码器: input_ids + attention_mask + pixel_values -> text_embeds
	// 图像编码器: pixel_values + input_ids + attention_mask -> image_embeds
	inputNames := []string{"input_ids", "attention_mask", "pixel_values"}
	textOutputNames := []string{"text_embeds"}
	imageOutputNames := []string{"image_embeds"}
	imageSize := ViTImageSize
	dimension := 512

	// 验证模型是否可以正常加载
	validated := validateChineseClipModel(modelFile, inputNames, textOutputNames, seqLength, imageSize, dimension)
	if !validated {
		altInputs := [][]string{
			{"input_ids"},
			{"input_ids", "attention_mask", "token_type_ids"},
		}
		for _, inputs := range altInputs {
			if validateChineseClipModel(modelFile, inputs, textOutputNames, seqLength, imageSize, dimension) {
				inputNames = inputs
				validated = true
				break
			}
		}
	}

	if !validated {
		tokenizer.Close()
		return nil, fmt.Errorf("failed to validate Chinese-CLIP model with any input configuration")
	}

	// 创建文本编码器
	textEncoder, err := NewTextEncoder(TextEncoderConfig{
		ModelPath:     modelFile,
		InputNames:    inputNames,
		OutputNames:   textOutputNames,
		SeqLength:     seqLength,
		Dimension:     dimension,
		ImageSize:     imageSize,
		UseCLSPooling: false, // CLIP 输出已经是 pooled
	}, tokenizer)
	if err != nil {
		tokenizer.Close()
		return nil, fmt.Errorf("failed to create Chinese-CLIP text encoder: %w", err)
	}

	// 创建图像编码器
	imageEncoder, err := NewImageEncoder(ImageEncoderConfig{
		ModelPath:   modelFile,
		InputNames:  inputNames,
		OutputNames: imageOutputNames,
		ImageSize:   imageSize,
		SeqLength:   seqLength,
		Dimension:   dimension,
	})
	if err != nil {
		textEncoder.Close()
		tokenizer.Close()
		return nil, fmt.Errorf("failed to create Chinese-CLIP image encoder: %w", err)
	}

	return &ChineseClipEmbedder{
		textEncoder:    textEncoder,
		imageEncoder:   imageEncoder,
		tokenizer:      tokenizer,
		imageProcessor: NewImageProcessor(imageSize),
		imageSize:      imageSize,
		seqLength:      seqLength,
	}, nil
}

// validateChineseClipModel 验证 Chinese-CLIP 模型
func validateChineseClipModel(modelPath string, inputNames, outputNames []string, seqLength, imageSize, dimension int) bool {
	batchSize := 1

	inputs := make([]ort.Value, len(inputNames))
	for i, name := range inputNames {
		var err error
		if name == "pixel_values" {
			inputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), 3, int64(imageSize), int64(imageSize)))
		} else {
			inputs[i], err = ort.NewEmptyTensor[int64](ort.NewShape(int64(batchSize), int64(seqLength)))
		}
		if err != nil {
			return false
		}
		defer inputs[i].Destroy()
	}

	outputs := make([]ort.Value, len(outputNames))
	for i := range outputNames {
		var err error
		outputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), int64(dimension)))
		if err != nil {
			return false
		}
		defer outputs[i].Destroy()
	}

	session, err := ort.NewAdvancedSession(modelPath, inputNames, outputNames, inputs, outputs, nil)
	if err != nil {
		return false
	}
	session.Destroy()
	return true
}

// Calc 计算单个 chunk 的向量
func (e *ChineseClipEmbedder) Calc(chunk *core.Chunk) (*core.Vector, error) {
	if chunk == nil || chunk.Content == "" {
		return nil, nil
	}
	vectors, err := e.Bulk([]*core.Chunk{chunk})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, nil
	}
	return vectors[0], nil
}

// CalcText 计算文本的向量
func (e *ChineseClipEmbedder) CalcText(text string) (*core.Vector, error) {
	if text == "" {
		return nil, nil
	}
	embeddings, err := e.textEncoder.Embed([]string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, nil
	}
	return &core.Vector{
		ID:     uuid.NewString(),
		Values: embeddings[0],
	}, nil
}

// Bulk 批量计算 chunks 的向量
// 根据 MIMEType 判断类型：图片使用 imageEncoder，文本使用 textEncoder
func (e *ChineseClipEmbedder) Bulk(chunks []*core.Chunk) ([]*core.Vector, error) {
	if len(chunks) == 0 {
		return []*core.Vector{}, nil
	}

	// 分离文本 chunks 和图片 chunks
	var textChunks []*core.Chunk
	var imageChunks []*core.Chunk
	for _, chunk := range chunks {
		if isImageMimeType(chunk.MIMEType) {
			imageChunks = append(imageChunks, chunk)
		} else {
			textChunks = append(textChunks, chunk)
		}
	}

	vectors := make([]*core.Vector, 0, len(chunks))

	// 处理文本 chunks
	if len(textChunks) > 0 {
		texts := make([]string, len(textChunks))
		for i, chunk := range textChunks {
			texts[i] = chunk.Content
		}
		textEmbeddings, err := e.textEncoder.Embed(texts)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text chunks: %w", err)
		}
		for i, chunk := range textChunks {
			vectors = append(vectors, newVector(chunk, textEmbeddings[i]))
		}
	}

	// 处理图片 chunks（Content 是无头 base64）
	if len(imageChunks) > 0 {
		// 预处理图片
		preprocessedImages := make([][]float32, len(imageChunks))
		for i, chunk := range imageChunks {
			// base64 解码（无头 base64）
			imgData, err := base64.StdEncoding.DecodeString(chunk.Content)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 for chunk %s: %w", chunk.ID, err)
			}
			// 预处理图像
			tensorData, err := e.imageProcessor.Preprocess(imgData)
			if err != nil {
				return nil, fmt.Errorf("failed to preprocess image for chunk %s: %w", chunk.ID, err)
			}
			preprocessedImages[i] = tensorData
		}

		imageEmbeddings, err := e.imageEncoder.Embed(preprocessedImages)
		if err != nil {
			return nil, fmt.Errorf("failed to embed image chunks: %w", err)
		}
		for i, chunk := range imageChunks {
			vectors = append(vectors, newVector(chunk, imageEmbeddings[i]))
		}
	}

	return vectors, nil
}

// isImageMimeType 判断 MIMEType 是否为图片类型
func isImageMimeType(mimeType string) bool {
	if mimeType == "" {
		return false
	}
	return strings.HasPrefix(mimeType, "image")
}

// CalcImage 计算图片的向量
func (e *ChineseClipEmbedder) CalcImage(data []byte) (*core.Vector, error) {
	// 预处理图像
	tensorData, err := e.imageProcessor.Preprocess(data)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess image: %w", err)
	}

	embeddings, err := e.imageEncoder.Embed([][]float32{tensorData})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, nil
	}
	return &core.Vector{
		ID:     uuid.NewString(),
		Values: embeddings[0],
	}, nil
}

// CalcImages 批量计算图片的向量
func (e *ChineseClipEmbedder) CalcImages(data [][]byte) ([][]float32, error) {
	// 预处理所有图像
	preprocessed := make([][]float32, len(data))
	for i, imgData := range data {
		tensorData, err := e.imageProcessor.Preprocess(imgData)
		if err != nil {
			return nil, fmt.Errorf("failed to preprocess image %d: %w", i, err)
		}
		preprocessed[i] = tensorData
	}
	return e.imageEncoder.Embed(preprocessed)
}

func (e *ChineseClipEmbedder) Multimoding() bool {
	return true
}

// Close 释放资源
func (e *ChineseClipEmbedder) Close() error {
	if e.textEncoder != nil {
		e.textEncoder.Close()
	}
	if e.imageEncoder != nil {
		e.imageEncoder.Close()
	}
	if e.tokenizer != nil {
		e.tokenizer.Close()
	}
	return nil
}

// getORTSharedLibraryPath 获取 ONNX Runtime 共享库路径
func getORTSharedLibraryPath() string {
	// macOS Homebrew 安装路径
	macPaths := []string{
		"/usr/local/lib/libonnxruntime.dylib",
		"/usr/local/lib/libonnxruntime.1.24.4.dylib",
		"/opt/homebrew/lib/libonnxruntime.dylib",
		"/opt/homebrew/lib/libonnxruntime.1.24.4.dylib",
		"onnxruntime/libonnxruntime.dylib",
		"./libonnxruntime.dylib",
	}
	for _, p := range macPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Linux 路径
	linuxPaths := []string{
		"/usr/local/lib/libonnxruntime.so",
		"/usr/local/lib/libonnxruntime.so.1.24",
		"onnxruntime/libonnxruntime.so",
		"./libonnxruntime.so",
	}
	for _, p := range linuxPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 默认路径
	return "/usr/local/lib/libonnxruntime.dylib"
}

func (e *ChineseClipEmbedder) Dim() int {
	return 512
}
