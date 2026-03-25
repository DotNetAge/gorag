package env

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gochat/pkg/embedding"
)

// Model represents a local AI model asset.
type Model struct {
	Name string
	File string
}

var (
	DefaultTextModel = Model{Name: "bge-small-zh-v1.5", File: "model_fp16.onnx"}
	DefaultMultimodalModel = Model{Name: "clip-vit-base-patch32", File: "text_model_fp16.onnx"}
)

// Environment holds the configuration for the GoRAG runtime environment.
type Environment struct {
	ModelDir string
	WorkDir  string
}

// DefaultEnvironment returns a preset environment configuration.
func DefaultEnvironment() *Environment {
	return &Environment{
		ModelDir: ".test/models",
		WorkDir:  "./data",
	}
}

// Check verifies if the essential models and directories are present.
func (e *Environment) Check(ctx context.Context) (bool, []string) {
	var missing []string
	ready := true

	// 1. Check Models
	models := []Model{DefaultTextModel, DefaultMultimodalModel}
	for _, m := range models {
		path := filepath.Join(e.ModelDir, m.Name, m.File)
		if _, err := os.Stat(path); err != nil {
			ready = false
			missing = append(missing, fmt.Sprintf("Model %s missing at %s", m.Name, path))
		}
	}

	// 2. Check Directories
	if err := os.MkdirAll(e.WorkDir, 0755); err != nil {
		ready = false
		missing = append(missing, fmt.Sprintf("Failed to create work directory %s: %v", e.WorkDir, err))
	}

	return ready, missing
}

// Prepare downloads the required models if they are missing.
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
