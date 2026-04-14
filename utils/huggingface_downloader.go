package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gomlx/go-huggingface/hub"
)

// ModelDownloader 模型下载器
type ModelDownloader struct {
	cacheDir  string
	authToken string
}

// NewModelDownloader 创建模型下载器
func NewModelDownloader(cacheDir string) (*ModelDownloader, error) {
	if cacheDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			cacheDir = "./models"
		} else {
			cacheDir = filepath.Join(homeDir, ".cache", "gorag", "models")
		}
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}
	return &ModelDownloader{cacheDir: cacheDir, authToken: os.Getenv("HF_TOKEN")}, nil
}

// Download 下载 HuggingFace 模型到本地目录
// modelID: HuggingFace 模型 ID，如 "Xenova/bge-base-zh-v1.5"
// files: 要下载的文件路径列表，如 []string{"config.json", "onnx/model.onnx"}
func (d *ModelDownloader) Download(modelID string, files []string) (string, error) {
	repo := hub.New(modelID)
	if d.authToken != "" {
		repo = repo.WithAuth(d.authToken)
	}
	repo = repo.WithCacheDir(d.cacheDir)

	fmt.Printf("Downloading from %s...\n", modelID)

	for _, file := range files {
		localPath, err := repo.DownloadFile(file)
		if err != nil {
			return "", fmt.Errorf("failed to download %s: %w", file, err)
		}
		fmt.Printf("  %s -> %s\n", file, localPath)
	}

	return d.cacheDir, nil
}
