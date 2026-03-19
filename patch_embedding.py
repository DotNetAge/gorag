import os
import re

base_dir = "/Users/ray/workspaces/ai-ecosystem/gochat/pkg/embedding"

# 1. Update embedding.go
with open(os.path.join(base_dir, "embedding.go"), "r") as f:
    content = f.read()

if "MultimodalProvider" not in content:
    content += """

// MultimodalProvider extends Provider with image embedding capabilities.
type MultimodalProvider interface {
	Provider

	// EmbedImages generates embeddings for the given images.
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - images: Slice of byte arrays, where each byte array is a raw image (JPEG/PNG)
	//
	// Returns:
	// - [][]float32: Slice of embeddings, one for each image
	// - error: Error if embedding generation fails
	EmbedImages(ctx context.Context, images [][]byte) ([][]float32, error)
}
"""
    with open(os.path.join(base_dir, "embedding.go"), "w") as f:
        f.write(content)

# 2. Update models.go
with open(os.path.join(base_dir, "models.go"), "r") as f:
    content = f.read()

if "ModelTypeCLIP" not in content:
    content = content.replace(
        'ModelTypeGloVe ModelType = "glove"\n)',
        'ModelTypeGloVe ModelType = "glove"\n\t// ModelTypeCLIP represents CLIP models\n\tModelTypeCLIP ModelType = "clip"\n)'
    )
    content = content.replace(
        'dimension = 300 // Default for GloVe\n\tdefault:',
        'dimension = 300 // Default for GloVe\n\tcase strings.Contains(lowerName, "clip"):\n\t\tmodelType = ModelTypeCLIP\n\t\tdimension = 512 // Default for CLIP\n\tdefault:'
    )
    with open(os.path.join(base_dir, "models.go"), "w") as f:
        f.write(content)

# 3. Update downloader.go
with open(os.path.join(base_dir, "downloader.go"), "r") as f:
    content = f.read()

if '"clip-vit-base-patch32"' not in content:
    clip_model = """		{
			Name: "clip-vit-base-patch32",
			Type: "clip",
			URLs: []string{
				"https://huggingface.co/Xenova/clip-vit-base-patch32/resolve/main/onnx/text_model_fp16.onnx",
				"https://huggingface.co/Xenova/clip-vit-base-patch32/resolve/main/onnx/vision_model_fp16.onnx",
			},
			Size:        "~300MB",
			Description: "OpenAI CLIP base model for text and image multimodal embeddings",
		},
"""
    content = content.replace('\t\t{\n\t\t\tName: "all-mpnet-base-v2",', clip_model + '\t\t{\n\t\t\tName: "all-mpnet-base-v2",')
    with open(os.path.join(base_dir, "downloader.go"), "w") as f:
        f.write(content)

# 4. Update factory.go
with open(os.path.join(base_dir, "factory.go"), "r") as f:
    content = f.read()

if "WithCLIP" not in content:
    content = content.replace(
        'case ModelTypeSentenceBERT:\n\t\treturn NewSentenceBERTProvider(modelPath)\n\tdefault:',
        'case ModelTypeSentenceBERT:\n\t\treturn NewSentenceBERTProvider(modelPath)\n\tcase ModelTypeCLIP:\n\t\treturn NewCLIPProvider(modelPath)\n\tdefault:'
    )
    
    with_clip = """

// WithCLIP 创建 CLIP Multimodal Embedding Provider
// modelName: 模型名称，例如 "clip-vit-base-patch32"
// modelPath: 模型路径，如果为空则自动下载
func WithCLIP(modelName, modelPath string) (MultimodalProvider, error) {
	if modelPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		modelPath = filepath.Join(homeDir, ".embedding", modelName)
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		fmt.Printf("Model not found, downloading %s to %s...\\n", modelName, modelPath)
		downloader := NewDownloader("")
		_, downloadErr := downloader.DownloadModel(modelName, nil)
		if downloadErr != nil {
			return nil, fmt.Errorf("failed to download model: %w", downloadErr)
		}
		fmt.Println("Model downloaded successfully")
	}

	p, err := NewProvider(modelPath)
	if err != nil {
		return nil, err
	}
	
	mp, ok := p.(MultimodalProvider)
	if !ok {
		return nil, fmt.Errorf("provider is not multimodal")
	}
	return mp, nil
}
"""
    content += with_clip
    with open(os.path.join(base_dir, "factory.go"), "w") as f:
        f.write(content)

print("Patching complete.")
