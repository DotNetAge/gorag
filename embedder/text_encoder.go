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
	config        TextEncoderConfig
	tokenizer     *VocabTokenizer
	session       *ort.AdvancedSession
	inputIDs      *ort.Tensor[int64]   // session 绑定的 input_ids 张量
	attentionMask *ort.Tensor[int64]   // session 绑定的 attention_mask 张量
	pixelValues   *ort.Tensor[float32] // session 绑定的 pixel_values 张量 (图像占位，可为 nil)
	textEmbeds    *ort.Tensor[float32] // session 绑定的 text_embeds 输出张量
	mu            sync.RWMutex
}

// NewTextEncoder 创建文本编码器
func NewTextEncoder(config TextEncoderConfig, tokenizer *VocabTokenizer) (*TextEncoder, error) {
	session, inputIDs, attentionMask, pixelValues, textEmbeds, err := createTextSession(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create text session: %w", err)
	}
	return &TextEncoder{
		config:        config,
		tokenizer:     tokenizer,
		session:       session,
		inputIDs:      inputIDs,
		attentionMask: attentionMask,
		pixelValues:   pixelValues,
		textEmbeds:    textEmbeds,
	}, nil
}

// createTextSession 创建文本编码 session，返回 session 和绑定的类型化张量
func createTextSession(config TextEncoderConfig) (
	session *ort.AdvancedSession,
	inputIDs *ort.Tensor[int64],
	attentionMask *ort.Tensor[int64],
	pixelValues *ort.Tensor[float32],
	textEmbeds *ort.Tensor[float32],
	err error,
) {
	batchSize := int64(1)

	cleanup := func() {
		if inputIDs != nil {
			inputIDs.Destroy()
		}
		if attentionMask != nil {
			attentionMask.Destroy()
		}
		if pixelValues != nil {
			pixelValues.Destroy()
		}
		if textEmbeds != nil {
			textEmbeds.Destroy()
		}
	}

	// 创建绑定张量并记录对应的 ort.Value
	type namedInput struct {
		name  string
		value ort.Value
	}
	namedInputs := make([]namedInput, 0, len(config.InputNames))

	for _, name := range config.InputNames {
		switch name {
		case "pixel_values":
			imageSize := config.ImageSize
			if imageSize <= 0 {
				imageSize = 224
			}
			pixelValues, err = ort.NewEmptyTensor[float32](
				ort.NewShape(batchSize, 3, int64(imageSize), int64(imageSize)),
			)
			if err != nil {
				cleanup()
				return nil, nil, nil, nil, nil, fmt.Errorf("failed to create pixel_values tensor: %w", err)
			}
			namedInputs = append(namedInputs, namedInput{name: name, value: pixelValues})
		case "input_ids":
			inputIDs, err = ort.NewEmptyTensor[int64](
				ort.NewShape(batchSize, int64(config.SeqLength)),
			)
			if err != nil {
				cleanup()
				return nil, nil, nil, nil, nil, fmt.Errorf("failed to create input_ids tensor: %w", err)
			}
			namedInputs = append(namedInputs, namedInput{name: name, value: inputIDs})
		case "attention_mask":
			attentionMask, err = ort.NewEmptyTensor[int64](
				ort.NewShape(batchSize, int64(config.SeqLength)),
			)
			if err != nil {
				cleanup()
				return nil, nil, nil, nil, nil, fmt.Errorf("failed to create attention_mask tensor: %w", err)
			}
			namedInputs = append(namedInputs, namedInput{name: name, value: attentionMask})
		default:
			// 其他文本输入 (如 token_type_ids)，按 int64 处理
			var t *ort.Tensor[int64]
			t, err = ort.NewEmptyTensor[int64](
				ort.NewShape(batchSize, int64(config.SeqLength)),
			)
			if err != nil {
				cleanup()
				return nil, nil, nil, nil, nil, fmt.Errorf("failed to create input tensor %s: %w", name, err)
			}
			namedInputs = append(namedInputs, namedInput{name: name, value: t})
		}
	}

	// 按原始 InputNames 顺序组装 inputs
	inputOrder := make(map[string]int)
	for i, name := range config.InputNames {
		inputOrder[name] = i
	}
	inputs := make([]ort.Value, len(config.InputNames))
	for _, ni := range namedInputs {
		inputs[inputOrder[ni.name]] = ni.value
	}

	// 输出张量
	if config.UseCLSPooling {
		textEmbeds, err = ort.NewEmptyTensor[float32](
			ort.NewShape(batchSize, int64(config.SeqLength), int64(config.Dimension)),
		)
	} else {
		textEmbeds, err = ort.NewEmptyTensor[float32](
			ort.NewShape(batchSize, int64(config.Dimension)),
		)
	}
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create output tensor: %w", err)
	}

	outputs := []ort.Value{textEmbeds}

	session, err = ort.NewAdvancedSession(
		config.ModelPath,
		config.InputNames,
		config.OutputNames,
		inputs,
		outputs,
		nil,
	)
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, inputIDs, attentionMask, pixelValues, textEmbeds, nil
}

// Embed 文本向量化
// 逐条处理文本，将 tokenized 数据写入 session 绑定的输入张量，
// 调用 Run() 推理后从绑定的输出张量读取结果。
func (e *TextEncoder) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.session == nil {
		return nil, fmt.Errorf("text encoder session not initialized")
	}

	dim := e.config.Dimension
	result := make([][]float32, len(texts))

	for i, text := range texts {
		// 1. 分词
		inputIDs, mask, err := e.tokenizer.Tokenize(text)
		if err != nil {
			return nil, fmt.Errorf("tokenization failed for text %d: %w", i, err)
		}

		// 2. 将数据写入 session 绑定的输入张量
		copy(e.inputIDs.GetData(), inputIDs)
		copy(e.attentionMask.GetData(), mask)

		// 3. 推理（在绑定的张量上执行）
		if err := e.session.Run(); err != nil {
			return nil, fmt.Errorf("inference failed for text %d: %w", i, err)
		}

		// 4. 从 session 绑定的输出张量读取结果
		outputData := e.textEmbeds.GetData()
		result[i] = make([]float32, dim)
		copy(result[i], outputData[:dim])
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
	if e.inputIDs != nil {
		e.inputIDs.Destroy()
		e.inputIDs = nil
	}
	if e.attentionMask != nil {
		e.attentionMask.Destroy()
		e.attentionMask = nil
	}
	if e.pixelValues != nil {
		e.pixelValues.Destroy()
		e.pixelValues = nil
	}
	if e.textEmbeds != nil {
		e.textEmbeds.Destroy()
		e.textEmbeds = nil
	}
	return nil
}

// Dim 返回向量维度
func (e *TextEncoder) Dim() int {
	return e.config.Dimension
}
