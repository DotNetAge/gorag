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
	config  ImageEncoderConfig
	session *ort.AdvancedSession
	mu      sync.RWMutex
}

// NewImageEncoder 创建图像编码器
func NewImageEncoder(config ImageEncoderConfig) (*ImageEncoder, error) {
	session, err := createImageSession(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create image session: %w", err)
	}
	return &ImageEncoder{
		config:  config,
		session: session,
	}, nil
}

// createImageSession 创建图像编码 session
func createImageSession(config ImageEncoderConfig) (*ort.AdvancedSession, error) {
	batchSize := 1

	inputs := make([]ort.Value, len(config.InputNames))
	for i, name := range config.InputNames {
		var err error
		switch name {
		case "pixel_values":
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
		outputs[i], err = ort.NewEmptyTensor[float32](
			ort.NewShape(int64(batchSize), int64(config.Dimension)),
		)
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

// Embed 图像向量化
// 输入: 预处理后的图像数据 [3, H, W] 格式的 float32 数组
func (e *ImageEncoder) Embed(images [][]float32) ([][]float32, error) {
	if len(images) == 0 {
		return [][]float32{}, nil
	}

	e.mu.RLock()
	session := e.session
	e.mu.RUnlock()

	if session == nil {
		return nil, fmt.Errorf("image encoder session not initialized")
	}

	batchSize := len(images)
	imageSize := e.config.ImageSize
	maxLen := e.config.SeqLength

	// 1. 构建 pixel_values 张量 [batch, 3, H, W]
	pixelData := make([]float32, batchSize*3*imageSize*imageSize)
	for b := 0; b < batchSize; b++ {
		copy(pixelData[b*3*imageSize*imageSize:], images[b])
	}

	pixelShape := ort.NewShape(int64(batchSize), 3, int64(imageSize), int64(imageSize))
	pixelTensor, err := ort.NewTensor(pixelShape, pixelData)
	if err != nil {
		return nil, fmt.Errorf("failed to create pixel_values tensor: %w", err)
	}
	defer pixelTensor.Destroy()

	// 2. 构建占位符文本输入 (全零，带 [CLS]...[SEP] 结构)
	textData := make([]int64, batchSize*maxLen)
	maskData := make([]int64, batchSize*maxLen)
	for b := 0; b < batchSize; b++ {
		textData[b*maxLen] = 101          // [CLS]
		textData[b*maxLen+maxLen-1] = 102 // [SEP]
		maskData[b*maxLen] = 1
		maskData[b*maxLen+maxLen-1] = 1
	}

	inputShape := ort.NewShape(int64(batchSize), int64(maxLen))
	inputTensor, err := ort.NewTensor(inputShape, textData)
	if err != nil {
		return nil, fmt.Errorf("failed to create input_ids tensor: %w", err)
	}
	defer inputTensor.Destroy()

	maskTensor, err := ort.NewTensor(inputShape, maskData)
	if err != nil {
		return nil, fmt.Errorf("failed to create attention_mask tensor: %w", err)
	}
	defer maskTensor.Destroy()

	// 3. 创建输出张量
	outputShape := ort.NewShape(int64(batchSize), int64(e.config.Dimension))
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// 4. 推理
	if err := session.Run(); err != nil {
		return nil, fmt.Errorf("image inference failed: %w", err)
	}

	// 5. 提取结果
	embeddings := outputTensor.GetData()
	result := make([][]float32, batchSize)
	dim := e.config.Dimension
	for i := 0; i < batchSize; i++ {
		result[i] = make([]float32, dim)
		copy(result[i], embeddings[i*dim:(i+1)*dim])
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
	return nil
}

// Dim 返回向量维度
func (e *ImageEncoder) Dim() int {
	return e.config.Dimension
}
