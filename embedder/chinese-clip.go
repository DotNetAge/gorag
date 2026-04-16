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
	modelFile      string               // ONNX 模型路径
	tokenizer      *VocabTokenizer      // 分词器 (vocab 内嵌)
	imageProcessor *ImageProcessor      // 图像预处理器
	inputNames     []string             // 输入节点名称
	outputNames    []string             // 输出节点名称
	imageSize      int                  // ViT 图像尺寸 (固定 224)
	seqLength      int                  // 文本序列长度
	textSession    *ort.AdvancedSession // 复用的文本 ONNX session
	imageSession   *ort.AdvancedSession // 复用的图像 ONNX session
	mu             sync.RWMutex
}

// NewChineseClipEmbedder 创建 Chinese-CLIP 向量化器
// 默认从 ./models/model.onnx 加载模型，可通过 WithModel/WithModelDir 自定义
func NewChineseClipEmbedder(opts ...ChineseClipOption) (*ChineseClipEmbedder, error) {
	cfg := &chineseClipConfig{
		// modelDir:  defaultModelDir,
		// modelName: defaultModelName,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// 初始化 ONNX Runtime
	ort.SetSharedLibraryPath(getORTSharedLibraryPath())
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	// 初始化分词器
	var tokenizer *VocabTokenizer
	var err error
	if cfg.vocabPath != "" {
		// 使用外部 vocab 文件
		tokenizer, err = NewVocabTokenizerFromFile(cfg.vocabPath, 52)
		if err != nil {
			// 注意: ort.DestroyEnvironment() 采用引用计数机制，
			// 多次调用会导致崩溃，因此统一由调用方管理环境生命周期
			return nil, fmt.Errorf("failed to load external vocab: %w", err)
		}
	} else {
		// 使用内嵌 vocab
		tokenizer, err = NewVocabTokenizer(52)
		if err != nil {
			// 注意: ort.DestroyEnvironment() 采用引用计数机制，
			// 多次调用会导致崩溃，因此统一由调用方管理环境生命周期
			return nil, fmt.Errorf("failed to load embedded vocab: %w", err)
		}
	}

	// TODO: 如果模型文件不存在就返回nil
	modelFile := cfg.modelFile

	// Chinese-CLIP ONNX 模型的输入输出是固定的
	// 这是一个统一的多模态模型，需要同时提供两种输入
	// 文本编码器: input_ids(int64) + attention_mask(int64) + pixel_values(float32) -> text_embeds
	// 图像编码器: pixel_values(float32) + input_ids(int64) + attention_mask(int64) -> image_embeds
	// 注意: 运行时根据提供的输入决定使用哪个编码器
	inputNames := []string{"input_ids", "attention_mask", "pixel_values"}
	outputNames := []string{"text_embeds"}
	seqLength := 52

	// 验证模型是否可以正常加载
	validated := false
	if err := validateSession(modelFile, inputNames, outputNames); err != nil {
		// 如果验证失败，尝试其他输入组合
		altInputs := [][]string{
			{"input_ids"},
			{"input_ids", "attention_mask", "token_type_ids"},
		}
		for _, inputs := range altInputs {
			if err := validateSession(modelFile, inputs, outputNames); err == nil {
				inputNames = inputs
				validated = true
				break
			}
		}
	} else {
		validated = true
	}

	// 如果所有验证都失败，返回错误
	if !validated {
		tokenizer.Close()
		// 注意: ort.DestroyEnvironment() 采用引用计数机制，
		// 多次调用会导致崩溃，因此统一由调用方管理环境生命周期
		return nil, fmt.Errorf("failed to validate Chinese-CLIP model with any input configuration")
	}

	// 创建可复用的文本 session
	textSession, err := createChineseClipTextSession(modelFile, inputNames, outputNames, seqLength)
	if err != nil {
		tokenizer.Close()
		// 注意: ort.DestroyEnvironment() 采用引用计数机制，
		// 多次调用会导致崩溃，因此统一由调用方管理环境生命周期
		return nil, fmt.Errorf("failed to create Chinese-CLIP text session: %w", err)
	}

	// 创建可复用的图像 session
	imageSession, err := createChineseClipImageSession(modelFile, inputNames, []string{"image_embeds"}, seqLength, ViTImageSize)
	if err != nil {
		textSession.Destroy()
		tokenizer.Close()
		// 注意: ort.DestroyEnvironment() 采用引用计数机制，
		// 多次调用会导致崩溃，因此统一由调用方管理环境生命周期
		return nil, fmt.Errorf("failed to create Chinese-CLIP image session: %w", err)
	}

	e := &ChineseClipEmbedder{
		modelFile:      modelFile,
		tokenizer:      tokenizer,
		imageProcessor: NewImageProcessor(ViTImageSize),
		inputNames:     inputNames,
		outputNames:    outputNames,
		imageSize:      ViTImageSize,
		seqLength:      seqLength,
		textSession:    textSession,
		imageSession:   imageSession,
	}

	return e, nil
}

// createChineseClipTextSession 创建可复用的文本 session
func createChineseClipTextSession(modelPath string, inputNames, outputNames []string, seqLength int) (*ort.AdvancedSession, error) {
	batchSize := 1

	inputs := make([]ort.Value, len(inputNames))
	for i, name := range inputNames {
		var err error
		if name == "pixel_values" {
			inputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), 3, 224, 224))
		} else {
			inputs[i], err = ort.NewEmptyTensor[int64](ort.NewShape(int64(batchSize), int64(seqLength)))
		}
		if err != nil {
			for j := 0; j < i; j++ {
				inputs[j].Destroy()
			}
			return nil, err
		}
	}

	outputs := make([]ort.Value, len(outputNames))
	for i := range outputNames {
		var err error
		outputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), 512))
		if err != nil {
			for j := 0; j < len(inputs); j++ {
				inputs[j].Destroy()
			}
			for j := 0; j < i; j++ {
				outputs[j].Destroy()
			}
			return nil, err
		}
	}

	session, err := ort.NewAdvancedSession(modelPath, inputNames, outputNames, inputs, outputs, nil)
	if err != nil {
		for j := 0; j < len(inputs); j++ {
			inputs[j].Destroy()
		}
		for j := 0; j < len(outputs); j++ {
			outputs[j].Destroy()
		}
		return nil, err
	}

	return session, nil
}

// createChineseClipImageSession 创建可复用的图像 session
func createChineseClipImageSession(modelPath string, inputNames, outputNames []string, seqLength, imageSize int) (*ort.AdvancedSession, error) {
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
			for j := 0; j < i; j++ {
				inputs[j].Destroy()
			}
			return nil, err
		}
	}

	outputs := make([]ort.Value, len(outputNames))
	for i := range outputNames {
		var err error
		outputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), 512))
		if err != nil {
			for j := 0; j < len(inputs); j++ {
				inputs[j].Destroy()
			}
			for j := 0; j < i; j++ {
				outputs[j].Destroy()
			}
			return nil, err
		}
	}

	session, err := ort.NewAdvancedSession(modelPath, inputNames, outputNames, inputs, outputs, nil)
	if err != nil {
		for j := 0; j < len(inputs); j++ {
			inputs[j].Destroy()
		}
		for j := 0; j < len(outputs); j++ {
			outputs[j].Destroy()
		}
		return nil, err
	}

	return session, nil
}

// validateSession 验证 session 是否可以创建
func validateSession(modelPath string, inputNames, outputNames []string) error {
	// 创建临时张量来测试
	batchSize := 1
	seqLen := 52

	// 根据输入类型创建张量
	inputs := make([]ort.Value, len(inputNames))
	for i, name := range inputNames {
		var err error
		if name == "pixel_values" {
			inputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), 3, 224, 224))
		} else {
			inputs[i], err = ort.NewEmptyTensor[int64](ort.NewShape(int64(batchSize), int64(seqLen)))
		}
		if err != nil {
			return err
		}
		defer inputs[i].Destroy()
	}

	// 创建输出 (text_embeds/image_embeds 已经是 pooled 输出 [batch, 512])
	outputs := make([]ort.Value, len(outputNames))
	for i := range outputNames {
		var err error
		outputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), 512))
		if err != nil {
			return err
		}
		defer outputs[i].Destroy()
	}

	// 尝试创建 session
	session, err := ort.NewAdvancedSession(
		modelPath,
		inputNames,
		outputNames,
		inputs,
		outputs,
		nil,
	)
	if err != nil {
		return err
	}
	session.Destroy()
	return nil
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
	embeddings, err := e.embedTexts([]string{text})
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
// 根据 MIMEType 判断类型：图片使用 embedImages，文本使用 embedTexts
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
		textEmbeddings, err := e.embedTexts(texts)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text chunks: %w", err)
		}
		for i, chunk := range textChunks {
			vectors = append(vectors, e.newVector(chunk, textEmbeddings[i]))
		}
	}

	// 处理图片 chunks（Content 是无头 base64）
	if len(imageChunks) > 0 {
		images := make([][]byte, len(imageChunks))
		for i, chunk := range imageChunks {
			// base64 解码（无头 base64）
			imgData, err := base64.StdEncoding.DecodeString(chunk.Content)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 for chunk %s: %w", chunk.ID, err)
			}
			images[i] = imgData
		}
		imageEmbeddings, err := e.embedImages(images)
		if err != nil {
			return nil, fmt.Errorf("failed to embed image chunks: %w", err)
		}
		for i, chunk := range imageChunks {
			vectors = append(vectors, e.newVector(chunk, imageEmbeddings[i]))
		}
	}

	return vectors, nil
}

// newVector 从 chunk 和 embedding 创建 Vector
func (e *ChineseClipEmbedder) newVector(chunk *core.Chunk, embedding []float32) *core.Vector {
	meta := make(map[string]any)
	if chunk.Metadata != nil {
		for k, v := range chunk.Metadata {
			meta[k] = v
		}
	}
	meta["doc_id"] = chunk.DocID
	meta["parent_id"] = chunk.ParentID
	meta["content"] = chunk.Content
	meta["mime_type"] = chunk.MIMEType

	return &core.Vector{
		ID:       uuid.NewString(),
		Values:   embedding,
		ChunkID:  chunk.ID,
		Metadata: meta,
	}
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
	embeddings, err := e.embedImages([][]byte{data})
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
	return e.embedImages(data)
}

func (e *ChineseClipEmbedder) Multimoding() bool {
	return true
}

// embedTexts 对文本进行向量化
func (e *ChineseClipEmbedder) embedTexts(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	e.mu.RLock()
	session := e.textSession
	e.mu.RUnlock()

	if session == nil {
		return nil, fmt.Errorf("Chinese-CLIP text session not initialized")
	}

	batchSize := len(texts)
	maxLen := e.seqLength

	// 分词
	allInputIDs := make([][]int64, batchSize)
	allMasks := make([][]int64, batchSize)

	for i, text := range texts {
		inputIDs, mask, err := e.tokenizer.Tokenize(text)
		if err != nil {
			return nil, fmt.Errorf("tokenization failed: %w", err)
		}
		allInputIDs[i] = inputIDs
		allMasks[i] = mask
	}

	// 构建输入张量 (int64 类型)
	inputData := make([]int64, batchSize*maxLen)
	for i := range allInputIDs {
		copy(inputData[i*maxLen:], allInputIDs[i])
	}

	maskData := make([]int64, batchSize*maxLen)
	for i := range allMasks {
		copy(maskData[i*maxLen:], allMasks[i])
	}

	// 创建张量 (int64 类型)
	inputShape := ort.NewShape(int64(batchSize), int64(maxLen))
	inputTensor, err := ort.NewTensor(inputShape, inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	maskShape := ort.NewShape(int64(batchSize), int64(maxLen))
	maskTensor, err := ort.NewTensor(maskShape, maskData)
	if err != nil {
		return nil, fmt.Errorf("failed to create mask tensor: %w", err)
	}
	defer maskTensor.Destroy()

	// 创建 pixel_values 占位符 (全零)
	pixelShape := ort.NewShape(int64(batchSize), 3, int64(e.imageSize), int64(e.imageSize))
	pixelData := make([]float32, batchSize*3*e.imageSize*e.imageSize)
	pixelTensor, err := ort.NewTensor(pixelShape, pixelData)
	if err != nil {
		return nil, fmt.Errorf("failed to create pixel_values tensor: %w", err)
	}
	defer pixelTensor.Destroy()

	// 创建输出张量 - 模型输出 [batch, 512] (text_embeds 已经是 pooled)
	outputShape := ort.NewShape(int64(batchSize), 512)
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// 使用复用的 session 推理
	if err := session.Run(); err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// 提取结果
	embeddings := outputTensor.GetData()

	// 转换为 [][]float32 (dimension 固定为 512)
	const dimension = 512
	result := make([][]float32, batchSize)
	for i := 0; i < batchSize; i++ {
		result[i] = make([]float32, dimension)
		copy(result[i], embeddings[i*dimension:(i+1)*dimension])
	}

	return result, nil
}

// embedImages 对图像进行向量化（ViT 编码器）
// 输入: 原始图像数据 (JPEG/PNG 等)
// 输出: 图像向量 [batch, 512]
func (e *ChineseClipEmbedder) embedImages(images [][]byte) ([][]float32, error) {
	if len(images) == 0 {
		return [][]float32{}, nil
	}

	e.mu.RLock()
	session := e.imageSession
	e.mu.RUnlock()

	if session == nil {
		return nil, fmt.Errorf("Chinese-CLIP image session not initialized")
	}

	batchSize := len(images)
	imageSize := e.imageSize
	maxLen := e.seqLength

	// 1. 预处理所有图像
	allTensorData := make([][]float32, batchSize)
	for i, imgData := range images {
		tensorData, err := e.imageProcessor.Preprocess(imgData)
		if err != nil {
			return nil, fmt.Errorf("failed to preprocess image %d: %w", i, err)
		}
		allTensorData[i] = tensorData
	}

	// 2. 构建 pixel_values 张量 [batch, 3, H, W]
	pixelData := make([]float32, batchSize*3*imageSize*imageSize)
	for b := 0; b < batchSize; b++ {
		copy(pixelData[b*3*imageSize*imageSize:(b+1)*3*imageSize*imageSize], allTensorData[b])
	}

	// 3. 构建占位符文本输入 (全零)
	textData := make([]int64, batchSize*maxLen)
	// 填充 [CLS]...[SEP] 结构
	for b := 0; b < batchSize; b++ {
		textData[b*maxLen] = 101          // [CLS]
		textData[b*maxLen+maxLen-1] = 102 // [SEP]
	}

	maskData := make([]int64, batchSize*maxLen)
	for b := 0; b < batchSize; b++ {
		maskData[b*maxLen] = 1
		maskData[b*maxLen+maxLen-1] = 1
	}

	// 4. 创建 ONNX 输入 (与文本编码相同的输入节点)
	// pixel_values
	pixelShape := ort.NewShape(int64(batchSize), 3, int64(imageSize), int64(imageSize))
	pixelTensor, err := ort.NewTensor(pixelShape, pixelData)
	if err != nil {
		return nil, fmt.Errorf("failed to create pixel_values tensor: %w", err)
	}
	defer pixelTensor.Destroy()

	// input_ids
	inputShape := ort.NewShape(int64(batchSize), int64(maxLen))
	inputTensor, err := ort.NewTensor(inputShape, textData)
	if err != nil {
		return nil, fmt.Errorf("failed to create input_ids tensor: %w", err)
	}
	defer inputTensor.Destroy()

	// attention_mask
	maskShape := ort.NewShape(int64(batchSize), int64(maxLen))
	maskTensor, err := ort.NewTensor(maskShape, maskData)
	if err != nil {
		return nil, fmt.Errorf("failed to create attention_mask tensor: %w", err)
	}
	defer maskTensor.Destroy()

	// 5. 创建输出张量 (dimension 固定为 512)
	outputShape := ort.NewShape(int64(batchSize), 512)
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create image output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// 6. 使用复用的 session 推理
	if err := session.Run(); err != nil {
		return nil, fmt.Errorf("image inference failed: %w", err)
	}

	// 7. 提取结果
	embeddings := outputTensor.GetData()

	result := make([][]float32, batchSize)
	for i := 0; i < batchSize; i++ {
		result[i] = make([]float32, 512)
		copy(result[i], embeddings[i*512:(i+1)*512])
	}

	return result, nil
}

// Close 释放资源
func (e *ChineseClipEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.textSession != nil {
		e.textSession.Destroy()
		e.textSession = nil
	}
	if e.imageSession != nil {
		e.imageSession.Destroy()
		e.imageSession = nil
	}
	if e.tokenizer != nil {
		e.tokenizer.Close()
	}
	// 注意: ort.DestroyEnvironment() 采用引用计数机制，
	// 多次调用会导致崩溃，因此统一由调用方管理环境生命周期
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
