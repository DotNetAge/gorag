package embedder

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	defaultModelDir  = "./models"
	defaultModelName = "model.onnx"
)

// ChineseClipOption 是 ChineseClipEmbedder 的配置选项
type ChineseClipOption func(*chineseClipConfig)

type chineseClipConfig struct {
	modelDir  string
	modelName string
	vocabPath string // 空则使用内嵌 vocab
}

// WithModel 设置模型名称 (不含路径)
func WithModel(name string) ChineseClipOption {
	return func(c *chineseClipConfig) {
		c.modelName = name
	}
}

// WithModelDir 设置模型目录
func WithModelDir(dir string) ChineseClipOption {
	return func(c *chineseClipConfig) {
		c.modelDir = dir
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
	modelPath      string          // ONNX 模型路径
	tokenizer      *VocabTokenizer // 分词器 (vocab 内嵌)
	imageProcessor *ImageProcessor // 图像预处理器
	inputNames     []string        // 输入节点名称
	outputNames    []string        // 输出节点名称
	imageSize      int             // ViT 图像尺寸 (固定 224)
	mu             sync.RWMutex
}

// NewChineseClipEmbedder 创建 Chinese-CLIP 向量化器
// 默认从 ./models/model.onnx 加载模型，可通过 WithModel/WithModelDir 自定义
func NewChineseClipEmbedder(opts ...ChineseClipOption) (*ChineseClipEmbedder, error) {
	cfg := &chineseClipConfig{
		modelDir:  defaultModelDir,
		modelName: defaultModelName,
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
			ort.DestroyEnvironment()
			return nil, fmt.Errorf("failed to load external vocab: %w", err)
		}
	} else {
		// 使用内嵌 vocab
		tokenizer, err = NewVocabTokenizer(52)
		if err != nil {
			ort.DestroyEnvironment()
			return nil, fmt.Errorf("failed to load embedded vocab: %w", err)
		}
	}

	// 确定模型路径
	modelPath := filepath.Join(cfg.modelDir, cfg.modelName)

	// Chinese-CLIP ONNX 模型的输入输出是固定的
	// 这是一个统一的多模态模型，需要同时提供两种输入
	// 文本编码器: input_ids(int64) + attention_mask(int64) + pixel_values(float32) -> text_embeds
	// 图像编码器: pixel_values(float32) + input_ids(int64) + attention_mask(int64) -> image_embeds
	// 注意: 运行时根据提供的输入决定使用哪个编码器
	inputNames := []string{"input_ids", "attention_mask", "pixel_values"}
	outputNames := []string{"text_embeds"}

	// 验证模型是否可以正常加载
	if err := validateSession(modelPath, inputNames, outputNames); err != nil {
		// 如果验证失败，尝试其他输入组合
		altInputs := [][]string{
			{"input_ids"},
			{"input_ids", "attention_mask", "token_type_ids"},
		}
		for _, inputs := range altInputs {
			if err := validateSession(modelPath, inputs, outputNames); err == nil {
				inputNames = inputs
				break
			}
		}
	}

	e := &ChineseClipEmbedder{
		modelPath:      modelPath,
		tokenizer:      tokenizer,
		imageProcessor: NewImageProcessor(ViTImageSize),
		inputNames:     inputNames,
		outputNames:    outputNames,
		imageSize:      ViTImageSize,
	}

	return e, nil
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
	embeddings, err := e.embedTexts([]string{text})
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

// Bulk 批量计算 chunks 的向量
func (e *ChineseClipEmbedder) Bulk(chunks []*core.Chunk) ([]*core.Vector, error) {
	if len(chunks) == 0 {
		return []*core.Vector{}, nil
	}

	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	embeddings, err := e.embedTexts(texts)
	if err != nil {
		return nil, err
	}

	vectors := make([]*core.Vector, len(chunks))
	for i, chunk := range chunks {
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

		vectors[i] = &core.Vector{
			ID:       uuid.NewString(),
			Values:   embeddings[i],
			ChunkID:  chunk.ID,
			Metadata: meta,
		}
	}

	return vectors, nil
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

// embedTexts 对文本进行向量化
func (e *ChineseClipEmbedder) embedTexts(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	batchSize := len(texts)
	maxLen := e.tokenizer.maxLength

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
		for j := range allInputIDs[i] {
			inputData[i*maxLen+j] = allInputIDs[i][j]
		}
	}

	maskData := make([]int64, batchSize*maxLen)
	for i := range allMasks {
		for j := range allMasks[i] {
			maskData[i*maxLen+j] = allMasks[i][j]
		}
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

	// 推理 - Chinese-CLIP 统一模型需要三种输入
	session, err := ort.NewAdvancedSession(
		e.modelPath,
		e.inputNames,
		e.outputNames,
		[]ort.Value{inputTensor, maskTensor, pixelTensor},
		[]ort.Value{outputTensor},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Destroy()

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
	defer e.mu.RUnlock()

	batchSize := len(images)
	imageSize := e.imageSize
	maxLen := 52 // Chinese-CLIP 默认序列长度

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
	imageOutputNames := []string{"image_embeds"}

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

	// 6. ONNX 推理
	session, err := ort.NewAdvancedSession(
		e.modelPath,
		e.inputNames,
		imageOutputNames,
		[]ort.Value{inputTensor, maskTensor, pixelTensor},
		[]ort.Value{outputTensor},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create image session: %w", err)
	}
	defer session.Destroy()

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
	ort.DestroyEnvironment()
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
