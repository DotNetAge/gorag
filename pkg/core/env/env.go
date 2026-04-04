// Package env provides environment configuration and management for the GoRAG runtime.
// It handles model downloads, directory setup, and environment verification.
package env

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gochat/pkg/embedding"
)

// Model represents a local AI model asset with its name and file path.
// Models are used for embeddings and other AI operations in the RAG pipeline.
//
// Fields:
//   - Name: The model identifier (e.g., "bge-small-zh-v1.5")
//   - File: The model file name (e.g., "model_fp16.onnx")
type Model struct {
	Name string
	File string
}

// Default model configurations for text and multimodal embeddings.
var (
	// DefaultTextModel is the default model for text embeddings.
	// Uses BGE-small Chinese model optimized for Chinese text.
	DefaultTextModel = Model{Name: "bge-small-zh-v1.5", File: "model_fp16.onnx"}

	// DefaultMultimodalModel is the default model for multimodal (image-text) embeddings.
	// Uses CLIP model for cross-modal retrieval.
	DefaultMultimodalModel = Model{Name: "clip-vit-base-patch32", File: "text_model_fp16.onnx"}
)

// Environment holds the configuration for the GoRAG runtime environment.
// It manages directories for models and working data.
//
// Fields:
//   - ModelDir: Directory where AI models are stored
//   - WorkDir: Working directory for temporary data and caches
type Environment struct {
	ModelDir string
	WorkDir  string
}

// DefaultEnvironment returns a preset environment configuration with default paths.
// The default configuration uses:
//   - ModelDir: ".test/models"
//   - WorkDir: "./data"
//
// Returns:
//   - *Environment: Environment with default settings
func DefaultEnvironment() *Environment {
	return &Environment{
		ModelDir: ".test/models",
		WorkDir:  "./data",
	}
}

// Check verifies if the essential models and directories are present.
// It validates that required models exist and the work directory is accessible.
//
// Parameters:
//   - ctx: Context for cancellation (currently unused but included for future extensibility)
//
// Returns:
//   - bool: True if environment is ready, false otherwise
//   - []string: List of missing components or errors encountered
func (e *Environment) Check(ctx context.Context) (bool, []string) {
	var missing []string
	ready := true

	models := []Model{DefaultTextModel, DefaultMultimodalModel}
	for _, m := range models {
		path := filepath.Join(e.ModelDir, m.Name, m.File)
		if _, err := os.Stat(path); err != nil {
			ready = false
			missing = append(missing, fmt.Sprintf("Model %s missing at %s", m.Name, path))
		}
	}

	if err := os.MkdirAll(e.WorkDir, 0755); err != nil {
		ready = false
		missing = append(missing, fmt.Sprintf("Failed to create work directory %s: %v", e.WorkDir, err))
	}

	return ready, missing
}

// Prepare downloads the required models if they are missing.
// It uses the embedding downloader to fetch models from HuggingFace or other sources.
//
// Parameters:
//   - ctx: Context for cancellation (currently unused but included for future extensibility)
//   - progress: Optional callback function to track download progress
//     - modelName: Name of the model being downloaded
//     - fileName: Name of the file being downloaded
//     - downloaded: Bytes downloaded so far
//     - total: Total bytes to download
//
// Returns:
//   - error: Any error that occurred during preparation
//
// Example:
//
//	env := env.DefaultEnvironment()
//	err := env.Prepare(ctx, func(modelName, fileName string, downloaded, total int64) {
//	    fmt.Printf("Downloading %s: %d/%d bytes\n", fileName, downloaded, total)
//	})
func (e *Environment) Prepare(ctx context.Context, progress func(modelName, fileName string, downloaded, total int64)) error {
	dl := embedding.NewDownloader(e.ModelDir)
	
	models := []string{DefaultTextModel.Name, DefaultMultimodalModel.Name}
	for _, m := range models {
		if _, err := dl.DownloadModel(m, progress); err != nil {
			return fmt.Errorf("failed to download model %s: %w", m, err)
		}
	}
	return nil
}
