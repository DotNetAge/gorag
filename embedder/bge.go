package embedder

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	// BGE 默认配置 (base 模型)
	bgeDefaultModelDir   = "./models/bge-base-zh-v1.5"
	bgeDefaultModelName  = "onnx/model.onnx"
	bgeDefaultDimension  = 768  // BGE-base hidden size
	bgeDefaultMaxLength  = 512  // BGE 最大序列长度
)

// BGEOption 是 BGEEmbedder 的配置选项
type BGEOption func(*bgeConfig)

type bgeConfig struct {
	modelDir  string
	modelName string
	vocabPath string // 空则使用内嵌 vocab
	dimension int    // 输出向量维度
}

// WithBGEModel 设置模型名称 (不含路径)
func WithBGEModel(name string) BGEOption {
	return func(c *bgeConfig) {
		c.modelName = name
	}
}

// WithBGEModelDir 设置模型目录
func WithBGEModelDir(dir string) BGEOption {
	return func(c *bgeConfig) {
		c.modelDir = dir
	}
}

// WithBGAVocab 设置外部 vocab 文件路径（支持 vocab.txt 或 tokenizer.json）
func WithBGAVocab(path string) BGEOption {
	return func(c *bgeConfig) {
		c.vocabPath = path
	}
}

// WithBGEDimension 设置输出向量维度 (base=768, small=384)
func WithBGEDimension(dim int) BGEOption {
	return func(c *bgeConfig) {
		c.dimension = dim
	}
}

// BGEEmbedder 使用 onnxruntime-go 进行 BGE ONNX 模型推理
type BGEEmbedder struct {
	modelPath  string           // ONNX 模型路径
	tokenizer  *VocabTokenizer  // 分词器
	inputNames []string         // 输入节点名称
	outputNames []string        // 输出节点名称
	dimension  int              // 输出向量维度
	mu         sync.RWMutex
}

// NewBGEEmbedder 创建 BGE 向量化器
// 默认从 ./models/bge-base-zh-v1.5/onnx/model.onnx 加载模型
func NewBGEEmbedder(opts ...BGEOption) (*BGEEmbedder, error) {
	cfg := &bgeConfig{
		modelDir:  bgeDefaultModelDir,
		modelName: bgeDefaultModelName,
		dimension: bgeDefaultDimension,
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
		tokenizer, err = NewVocabTokenizerFromFile(cfg.vocabPath, bgeDefaultMaxLength)
		if err != nil {
			ort.DestroyEnvironment()
			return nil, fmt.Errorf("failed to load external vocab: %w", err)
		}
	} else {
		// 使用内嵌 vocab
		tokenizer, err = NewBGEVocabTokenizer(bgeDefaultMaxLength)
		if err != nil {
			ort.DestroyEnvironment()
			return nil, fmt.Errorf("failed to load BGE embedded vocab: %w", err)
		}
	}

	// 确定模型路径
	modelPath := filepath.Join(cfg.modelDir, cfg.modelName)

	// BGE ONNX 模型的输入输出
	inputNames := []string{"input_ids", "attention_mask"}
	outputNames := []string{"last_hidden_state"}

	// 验证模型
	if err := validateBGESession(modelPath, inputNames, outputNames, cfg.dimension); err != nil {
		// 尝试其他输入组合
		altInputs := [][]string{
			{"input_ids"},
			{"input_ids", "attention_mask", "token_type_ids"},
		}
		for _, inputs := range altInputs {
			if err := validateBGESession(modelPath, inputs, outputNames, cfg.dimension); err == nil {
				inputNames = inputs
				break
			}
		}
	}

	e := &BGEEmbedder{
		modelPath:   modelPath,
		tokenizer:   tokenizer,
		inputNames:  inputNames,
		outputNames: outputNames,
		dimension:   cfg.dimension,
	}

	return e, nil
}

// validateBGESession 验证 BGE session 是否可以创建
func validateBGESession(modelPath string, inputNames, outputNames []string, dimension int) error {
	batchSize := 1
	seqLen := bgeDefaultMaxLength

	inputs := make([]ort.Value, len(inputNames))
	for i := range inputNames {
		var err error
		inputs[i], err = ort.NewEmptyTensor[int64](ort.NewShape(int64(batchSize), int64(seqLen)))
		if err != nil {
			return err
		}
		defer inputs[i].Destroy()
	}

	outputs := make([]ort.Value, len(outputNames))
	for i := range outputNames {
		var err error
		outputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), int64(seqLen), int64(dimension)))
		if err != nil {
			return err
		}
		defer outputs[i].Destroy()
	}

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
func (e *BGEEmbedder) Calc(chunk *core.Chunk) (*core.Vector, error) {
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
func (e *BGEEmbedder) CalcText(text string) (*core.Vector, error) {
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
func (e *BGEEmbedder) Bulk(chunks []*core.Chunk) ([]*core.Vector, error) {
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

// CalcImage BGE 不支持图像向量化
func (e *BGEEmbedder) CalcImage(data []byte) (*core.Vector, error) {
	return nil, fmt.Errorf("BGEEmbedder does not support image embedding")
}

// CalcImages BGE 不支持图像向量化
func (e *BGEEmbedder) CalcImages(data [][]byte) ([][]float32, error) {
	return nil, fmt.Errorf("BGEEmbedder does not support image embedding")
}

// embedTexts 对文本进行向量化
func (e *BGEEmbedder) embedTexts(texts []string) ([][]float32, error) {
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

	// 构建输入张量
	inputData := make([]int64, batchSize*maxLen)
	for i := range allInputIDs {
		copy(inputData[i*maxLen:(i+1)*maxLen], allInputIDs[i])
	}

	maskData := make([]int64, batchSize*maxLen)
	for i := range allMasks {
		copy(maskData[i*maxLen:(i+1)*maxLen], allMasks[i])
	}

	// 创建张量
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

	// 创建输出张量 - [batch, seq, hidden]
	outputShape := ort.NewShape(int64(batchSize), int64(maxLen), int64(e.dimension))
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// 推理
	session, err := ort.NewAdvancedSession(
		e.modelPath,
		e.inputNames,
		e.outputNames,
		[]ort.Value{inputTensor, maskTensor},
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

	// 提取结果 - 使用 [CLS] token 的 hidden state 作为句子向量
	embeddings := outputTensor.GetData()

	result := make([][]float32, batchSize)
	for i := 0; i < batchSize; i++ {
		result[i] = make([]float32, e.dimension)
		// 取第一个 token ([CLS]) 的 hidden state
		copy(result[i], embeddings[i*e.dimension:(i+1)*e.dimension])
	}

	return result, nil
}

// Close 释放资源
func (e *BGEEmbedder) Close() error {
	ort.DestroyEnvironment()
	return nil
}
