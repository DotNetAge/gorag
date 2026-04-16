package embedder

import (
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// TextEncoderConfig 文本编码器配置
type TextEncoderConfig struct {
	ModelPath     string   // ONNX 模型路径
	InputNames    []string // 输入节点名称
	OutputNames   []string // 输出节点名称
	SeqLength     int      // 序列长度
	Dimension     int      // 输出向量维度
	ImageSize     int      // 图像尺寸 (0 表示不需要 pixel_values)
	UseCLSPooling bool     // true: 从 [batch, seq, hidden] 提取 CLS; false: 直接使用 [batch, dim]
}

// TextEncoder 通用文本编码器
type TextEncoder struct {
	config    TextEncoderConfig
	tokenizer *VocabTokenizer
	session   *ort.AdvancedSession
	mu        sync.RWMutex
}

// NewTextEncoder 创建文本编码器
func NewTextEncoder(config TextEncoderConfig, tokenizer *VocabTokenizer) (*TextEncoder, error) {
	session, err := createTextSession(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create text session: %w", err)
	}
	return &TextEncoder{
		config:    config,
		tokenizer: tokenizer,
		session:   session,
	}, nil
}

// createTextSession 创建文本编码 session
func createTextSession(config TextEncoderConfig) (*ort.AdvancedSession, error) {
	batchSize := 1

	inputs := make([]ort.Value, len(config.InputNames))
	for i, name := range config.InputNames {
		var err error
		switch name {
		case "pixel_values":
			if config.ImageSize <= 0 {
				config.ImageSize = 224
			}
			inputs[i], err = ort.NewEmptyTensor[float32](
				ort.NewShape(int64(batchSize), 3, int64(config.ImageSize), int64(config.ImageSize)),
			)
		default:
			inputs[i], err = ort.NewEmptyTensor[int64](
				ort.NewShape(int64(batchSize), int64(config.SeqLength)),
			)
		}
		if err != nil {
			for j := 0; j < i; j++ {
				inputs[j].Destroy()
			}
			return nil, fmt.Errorf("failed to create input tensor %s: %w", name, err)
		}
	}

	outputs := make([]ort.Value, len(config.OutputNames))
	for i := range config.OutputNames {
		var err error
		if config.UseCLSPooling {
			// 输出形状: [batch, seq, hidden]
			outputs[i], err = ort.NewEmptyTensor[float32](
				ort.NewShape(int64(batchSize), int64(config.SeqLength), int64(config.Dimension)),
			)
		} else {
			// 输出形状: [batch, dimension]
			outputs[i], err = ort.NewEmptyTensor[float32](
				ort.NewShape(int64(batchSize), int64(config.Dimension)),
			)
		}
		if err != nil {
			for j := 0; j < len(inputs); j++ {
				inputs[j].Destroy()
			}
			for j := 0; j < i; j++ {
				outputs[j].Destroy()
			}
			return nil, fmt.Errorf("failed to create output tensor: %w", err)
		}
	}

	session, err := ort.NewAdvancedSession(
		config.ModelPath,
		config.InputNames,
		config.OutputNames,
		inputs,
		outputs,
		nil,
	)
	if err != nil {
		for j := 0; j < len(inputs); j++ {
			inputs[j].Destroy()
		}
		for j := 0; j < len(outputs); j++ {
			outputs[j].Destroy()
		}
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// Embed 文本向量化
func (e *TextEncoder) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	e.mu.RLock()
	session := e.session
	e.mu.RUnlock()

	if session == nil {
		return nil, fmt.Errorf("text encoder session not initialized")
	}

	batchSize := len(texts)
	maxLen := e.config.SeqLength

	// 1. 分词
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

	// 2. 构建输入数据
	inputData := make([]int64, batchSize*maxLen)
	for i := range allInputIDs {
		copy(inputData[i*maxLen:], allInputIDs[i])
	}

	maskData := make([]int64, batchSize*maxLen)
	for i := range allMasks {
		copy(maskData[i*maxLen:], allMasks[i])
	}

	// 3. 创建输入张量
	inputShape := ort.NewShape(int64(batchSize), int64(maxLen))
	inputTensor, err := ort.NewTensor(inputShape, inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to create input_ids tensor: %w", err)
	}
	defer inputTensor.Destroy()

	maskTensor, err := ort.NewTensor(inputShape, maskData)
	if err != nil {
		return nil, fmt.Errorf("failed to create attention_mask tensor: %w", err)
	}
	defer maskTensor.Destroy()

	// 4. 如果需要 pixel_values，创建占位符
	var pixelTensor *ort.Tensor[float32]
	if e.config.ImageSize > 0 {
		pixelShape := ort.NewShape(int64(batchSize), 3, int64(e.config.ImageSize), int64(e.config.ImageSize))
		pixelData := make([]float32, batchSize*3*e.config.ImageSize*e.config.ImageSize)
		pixelTensor, err = ort.NewTensor(pixelShape, pixelData)
		if err != nil {
			return nil, fmt.Errorf("failed to create pixel_values tensor: %w", err)
		}
		defer pixelTensor.Destroy()
	}

	// 5. 创建输出张量
	var outputTensor *ort.Tensor[float32]
	if e.config.UseCLSPooling {
		outputShape := ort.NewShape(int64(batchSize), int64(maxLen), int64(e.config.Dimension))
		outputTensor, err = ort.NewEmptyTensor[float32](outputShape)
	} else {
		outputShape := ort.NewShape(int64(batchSize), int64(e.config.Dimension))
		outputTensor, err = ort.NewEmptyTensor[float32](outputShape)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// 6. 推理
	if err := session.Run(); err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// 7. 提取结果
	embeddings := outputTensor.GetData()
	result := make([][]float32, batchSize)
	dim := e.config.Dimension

	if e.config.UseCLSPooling {
		// 从 [batch, seq, hidden] 提取每个样本的第一个 token (CLS) 的向量
		for i := 0; i < batchSize; i++ {
			result[i] = make([]float32, dim)
			copy(result[i], embeddings[i*dim:(i+1)*dim])
		}
	} else {
		// 直接使用 [batch, dimension]
		for i := 0; i < batchSize; i++ {
			result[i] = make([]float32, dim)
			copy(result[i], embeddings[i*dim:(i+1)*dim])
		}
	}

	return result, nil
}

// Close 释放资源
func (e *TextEncoder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.session != nil {
		e.session.Destroy()
		e.session = nil
	}
	return nil
}

// Dim 返回向量维度
func (e *TextEncoder) Dim() int {
	return e.config.Dimension
}
