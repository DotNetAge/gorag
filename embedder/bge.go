package embedder

import (
	"fmt"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	// BGE 默认配置 (base 模型)
	bgeDefaultModelFile = "./models/bge-base-zh-v1.5/onnx/model.onnx"
	bgeDefaultDimension = 768 // BGE-base hidden size
	bgeDefaultMaxLength = 512 // BGE 最大序列长度
)

// BGEOption 是 BGEEmbedder 的配置选项
type BGEOption func(*bgeConfig)

type bgeConfig struct {
	modelFile string
	vocabPath string // 空则使用内嵌 vocab
	dimension int    // 输出向量维度
}

// WithBGEModel 设置模型名称 (不含路径)
func WithBGEModelFile(name string) BGEOption {
	return func(c *bgeConfig) {
		c.modelFile = name
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
	encoder   *TextEncoder   // 通用文本编码器
	tokenizer *VocabTokenizer
	dimension int
}

// NewBGEEmbedder 创建 BGE 向量化器
// 默认从 ./models/bge-base-zh-v1.5/onnx/model.onnx 加载模型
func NewBGEEmbedder(opts ...BGEOption) (*BGEEmbedder, error) {
	cfg := &bgeConfig{
		modelFile: bgeDefaultModelFile,
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
		tokenizer, err = NewVocabTokenizerFromFile(cfg.vocabPath, bgeDefaultMaxLength)
		if err != nil {
			return nil, fmt.Errorf("failed to load external vocab: %w", err)
		}
	} else {
		tokenizer, err = NewBGEVocabTokenizer(bgeDefaultMaxLength)
		if err != nil {
			return nil, fmt.Errorf("failed to load BGE embedded vocab: %w", err)
		}
	}

	// 确定模型路径
	modelPath := cfg.modelFile

	// 验证模型
	inputNames := []string{"input_ids", "attention_mask"}
	outputNames := []string{"last_hidden_state"}

	// 尝试验证，如果失败则尝试其他输入组合
	validated := validateTextEncoderConfig(modelPath, inputNames, outputNames, cfg.dimension, bgeDefaultMaxLength)
	if !validated {
		altInputs := [][]string{
			{"input_ids"},
			{"input_ids", "attention_mask", "token_type_ids"},
		}
		for _, inputs := range altInputs {
			if validateTextEncoderConfig(modelPath, inputs, outputNames, cfg.dimension, bgeDefaultMaxLength) {
				inputNames = inputs
				validated = true
				break
			}
		}
	}

	if !validated {
		tokenizer.Close()
		return nil, fmt.Errorf("failed to validate BGE model with any input configuration")
	}

	// 创建文本编码器
	encoder, err := NewTextEncoder(TextEncoderConfig{
		ModelPath:     modelPath,
		InputNames:    inputNames,
		OutputNames:   outputNames,
		SeqLength:     bgeDefaultMaxLength,
		Dimension:     cfg.dimension,
		ImageSize:     0, // 不需要 pixel_values
		UseCLSPooling: true,
	}, tokenizer)
	if err != nil {
		tokenizer.Close()
		return nil, fmt.Errorf("failed to create BGE encoder: %w", err)
	}

	return &BGEEmbedder{
		encoder:   encoder,
		tokenizer: tokenizer,
		dimension: cfg.dimension,
	}, nil
}

// validateTextEncoderConfig 验证编码器配置
func validateTextEncoderConfig(modelPath string, inputNames, outputNames []string, dimension, seqLength int) bool {
	batchSize := 1

	inputs := make([]ort.Value, len(inputNames))
	for i := range inputNames {
		var err error
		inputs[i], err = ort.NewEmptyTensor[int64](ort.NewShape(int64(batchSize), int64(seqLength)))
		if err != nil {
			return false
		}
		defer inputs[i].Destroy()
	}

	outputs := make([]ort.Value, len(outputNames))
	for i := range outputNames {
		var err error
		outputs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(int64(batchSize), int64(seqLength), int64(dimension)))
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
func (e *BGEEmbedder) Calc(chunk *core.Chunk) (*core.Vector, error) {
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
func (e *BGEEmbedder) CalcText(text string) (*core.Vector, error) {
	if text == "" {
		return nil, nil
	}
	embeddings, err := e.encoder.Embed([]string{text})
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

// Bulk 批量计算 chunks 的向量（只处理非图片 chunk）
// 图片 chunk 会被忽略（由多模态 embedder 如 ChineseClip 处理）
func (e *BGEEmbedder) Bulk(chunks []*core.Chunk) ([]*core.Vector, error) {
	if len(chunks) == 0 {
		return []*core.Vector{}, nil
	}

	// 过滤掉图片 chunks（只处理文本）
	var textChunks []*core.Chunk
	for _, chunk := range chunks {
		if isImageMimeType(chunk.MIMEType) {
			continue // 跳过图片 chunk
		}
		textChunks = append(textChunks, chunk)
	}

	if len(textChunks) == 0 {
		return []*core.Vector{}, nil
	}

	texts := make([]string, len(textChunks))
	for i, chunk := range textChunks {
		texts[i] = chunk.Content
	}

	embeddings, err := e.encoder.Embed(texts)
	if err != nil {
		return nil, err
	}

	vectors := make([]*core.Vector, len(textChunks))
	for i, chunk := range textChunks {
		vectors[i] = newVector(chunk, embeddings[i])
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

func (e *BGEEmbedder) Multimoding() bool {
	return false
}

// Close 释放资源
func (e *BGEEmbedder) Close() error {
	if e.encoder != nil {
		e.encoder.Close()
	}
	if e.tokenizer != nil {
		e.tokenizer.Close()
	}
	return nil
}

func (e *BGEEmbedder) Dim() int {
	return e.dimension
}

// newVector 从 chunk 和 embedding 创建 Vector
func newVector(chunk *core.Chunk, embedding []float32) *core.Vector {
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
