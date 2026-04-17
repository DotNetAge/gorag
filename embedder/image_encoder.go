package embedder

import (
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// ImageEncoderConfig 图像编码器配置
type ImageEncoderConfig struct {
	ModelPath   string   // ONNX 模型路径
	InputNames  []string // 输入节点名称
	OutputNames []string // 输出节点名称
	ImageSize   int      // 图像尺寸 (224)
	SeqLength   int      // 文本序列长度 (用于占位符)
	Dimension   int      // 输出向量维度 (512)
}

// ImageEncoder 通用图像编码器
type ImageEncoder struct {
	config       ImageEncoderConfig
	session      *ort.AdvancedSession
	inputIDs     *ort.Tensor[int64]   // session 绑定的 input_ids 张量 (文本占位)
	attentionMask *ort.Tensor[int64]  // session 绑定的 attention_mask 张量 (文本占位)
	pixelValues  *ort.Tensor[float32] // session 绑定的 pixel_values 张量
	imageEmbeds  *ort.Tensor[float32] // session 绑定的 image_embeds 输出张量
	mu           sync.RWMutex
}

// NewImageEncoder 创建图像编码器
func NewImageEncoder(config ImageEncoderConfig) (*ImageEncoder, error) {
	session, inputIDs, attentionMask, pixelValues, imageEmbeds, err := createImageSession(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create image session: %w", err)
	}
	return &ImageEncoder{
		config:       config,
		session:      session,
		inputIDs:     inputIDs,
		attentionMask: attentionMask,
		pixelValues:  pixelValues,
		imageEmbeds:  imageEmbeds,
	}, nil
}

// createImageSession 创建图像编码 session，返回 session 和绑定的类型化张量
func createImageSession(config ImageEncoderConfig) (
	session *ort.AdvancedSession,
	inputIDs *ort.Tensor[int64],
	attentionMask *ort.Tensor[int64],
	pixelValues *ort.Tensor[float32],
	imageEmbeds *ort.Tensor[float32],
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
		if imageEmbeds != nil {
			imageEmbeds.Destroy()
		}
	}

	// 创建绑定张量
	type namedInput struct {
		name  string
		value ort.Value
	}
	namedInputs := make([]namedInput, 0, len(config.InputNames))

	for _, name := range config.InputNames {
		switch name {
		case "pixel_values":
			pixelValues, err = ort.NewEmptyTensor[float32](
				ort.NewShape(batchSize, 3, int64(config.ImageSize), int64(config.ImageSize)),
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
			// 其他输入按 int64 处理
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

	// 按 InputNames 顺序组装 inputs
	inputOrder := make(map[string]int)
	for i, name := range config.InputNames {
		inputOrder[name] = i
	}
	inputs := make([]ort.Value, len(config.InputNames))
	for _, ni := range namedInputs {
		inputs[inputOrder[ni.name]] = ni.value
	}

	// 输出张量
	imageEmbeds, err = ort.NewEmptyTensor[float32](
		ort.NewShape(batchSize, int64(config.Dimension)),
	)
	if err != nil {
		cleanup()
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create output tensor: %w", err)
	}

	outputs := []ort.Value{imageEmbeds}

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

	return session, inputIDs, attentionMask, pixelValues, imageEmbeds, nil
}

// Embed 图像向量化
// 逐张处理图像，将预处理后的数据写入 session 绑定的 pixel_values 张量，
// 调用 Run() 推理后从绑定的输出张量读取结果。
func (e *ImageEncoder) Embed(images [][]float32) ([][]float32, error) {
	if len(images) == 0 {
		return [][]float32{}, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.session == nil {
		return nil, fmt.Errorf("image encoder session not initialized")
	}

	maxLen := e.config.SeqLength
	dim := e.config.Dimension
	result := make([][]float32, len(images))

	for i, imageData := range images {
		// 1. 将图像数据写入绑定的 pixel_values 张量
		copy(e.pixelValues.GetData(), imageData)

		// 2. 设置文本占位符输入（全零，带 [CLS]...[SEP] 结构）
		if e.inputIDs != nil {
			idsData := e.inputIDs.GetData()
			for j := range idsData {
				idsData[j] = 0
			}
			idsData[0] = 101                   // [CLS]
			idsData[maxLen-1] = 102             // [SEP]
		}
		if e.attentionMask != nil {
			maskData := e.attentionMask.GetData()
			for j := range maskData {
				maskData[j] = 0
			}
			maskData[0] = 1
			maskData[maxLen-1] = 1
		}

		// 3. 推理
		if err := e.session.Run(); err != nil {
			return nil, fmt.Errorf("image inference failed for image %d: %w", i, err)
		}

		// 4. 从绑定的输出张量读取结果
		outputData := e.imageEmbeds.GetData()
		result[i] = make([]float32, dim)
		copy(result[i], outputData[:dim])
	}

	return result, nil
}

// Close 释放资源
func (e *ImageEncoder) Close() error {
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
	if e.imageEmbeds != nil {
		e.imageEmbeds.Destroy()
		e.imageEmbeds = nil
	}
	return nil
}

// Dim 返回向量维度
func (e *ImageEncoder) Dim() int {
	return e.config.Dimension
}
