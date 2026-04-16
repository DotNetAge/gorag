package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DownloadEvent 下载事件
type DownloadEvent struct {
	Type    EventType // 事件类型
	File    string    // 当前文件
	Current int64     // 已下载字节
	Total   int64     // 总字节
	Message string    // 消息
}

// EventType 事件类型
type EventType int

const (
	EventStart       EventType = iota // 开始下载
	EventProgress                     // 进度更新
	EventComplete                     // 单个文件完成
	EventError                        // 错误
	EventAllComplete                  // 全部完成
)

// DownloadObserver 下载观察者接口
type DownloadObserver interface {
	OnEvent(event DownloadEvent)
}

// ModelDownloader 模型下载器
type ModelDownloader struct {
	cacheDir  string
	authToken string
	observer  DownloadObserver
}

// getBaseDir 获取模型存储基础目录
func getBaseDir() string {
	if baseDir := os.Getenv("GORAG_MODEL_PATH"); baseDir != "" {
		return baseDir
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".embeddings")
	}
	return "./models"
}

// NewModelDownloader 创建模型下载器
func NewModelDownloader(cacheDir string) (*ModelDownloader, error) {
	if cacheDir == "" {
		cacheDir = getBaseDir()
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}
	return &ModelDownloader{
		cacheDir:  cacheDir,
		authToken: os.Getenv("HF_TOKEN"),
	}, nil
}

// WithObserver 设置观察者（链式调用）
func (d *ModelDownloader) WithObserver(observer DownloadObserver) *ModelDownloader {
	d.observer = observer
	return d
}

// notify 通知观察者
func (d *ModelDownloader) notify(event DownloadEvent) {
	if d.observer != nil {
		d.observer.OnEvent(event)
	}
}

// createRequest 创建带认证的 HTTP 请求
func (d *ModelDownloader) createRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	if d.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+d.authToken)
	}
	return req, nil
}

// Download 下载 HuggingFace 模型到本地目录
// modelID: HuggingFace 模型 ID，如 "Xenova/bge-base-zh-v1.5"
// files: 要下载的文件路径列表，如 []string{"config.json", "onnx/model.onnx"}
func (d *ModelDownloader) Download(modelID string, files []string) (string, error) {
	d.notify(DownloadEvent{
		Type:    EventStart,
		Message: fmt.Sprintf("开始下载模型: %s", modelID),
	})

	for _, file := range files {
		fileSize := d.getFileSize(modelID, file)
		d.notify(DownloadEvent{
			Type:    EventStart,
			File:    file,
			Total:   fileSize,
			Message: fmt.Sprintf("下载 %s", filepath.Base(file)),
		})

		if err := d.downloadFile(modelID, file, fileSize); err != nil {
			d.notify(DownloadEvent{
				Type:    EventError,
				File:    file,
				Message: err.Error(),
			})
			return "", fmt.Errorf("failed to download %s: %w", file, err)
		}

		d.notify(DownloadEvent{
			Type:    EventComplete,
			File:    file,
			Total:   fileSize,
			Current: fileSize,
		})
	}

	d.notify(DownloadEvent{
		Type:    EventAllComplete,
		Message: "所有文件下载完成",
	})

	return d.cacheDir, nil
}

// getFileSize 获取文件大小
func (d *ModelDownloader) getFileSize(modelID, file string) int64 {
	url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", modelID, file)
	req, err := d.createRequest("HEAD", url)
	if err != nil {
		return 0
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return resp.ContentLength
	}
	return 0
}

// downloadFile 下载单个文件
func (d *ModelDownloader) downloadFile(modelID, file string, expectedSize int64) error {
	url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", modelID, file)
	req, err := d.createRequest("GET", url)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	localPath := filepath.Join(d.cacheDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)), file)
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	tmpPath := localPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			d.notify(DownloadEvent{
				Type:    EventProgress,
				File:    file,
				Current: downloaded,
				Total:   expectedSize,
			})
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return os.Rename(tmpPath, localPath)
}

// GetModelPath 获取模型本地路径
func GetModelPath(modelID, file string) string {
	baseDir := getBaseDir()
	return filepath.Join(baseDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)), file)
}

// CheckAndDownload 检查模型是否存在，不存在则下载
// 返回模型文件的完整路径
func CheckAndDownload(modelID, modelFile string, observer DownloadObserver) (string, error) {
	baseDir := getBaseDir()
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create model directory: %w", err)
	}

	modelPath := GetModelPath(modelID, modelFile)

	if _, err := os.Stat(modelPath); err == nil {
		observer.OnEvent(DownloadEvent{
			Type:    EventComplete,
			File:    modelFile,
			Message: "模型已存在",
		})
		return modelPath, nil
	}

	downloader, err := NewModelDownloader(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to create downloader: %w", err)
	}
	downloader.WithObserver(observer)

	files := []string{modelFile}
	if filepath.Ext(modelFile) == ".onnx" {
		files = append(files, "config.json", "tokenizer.json", "vocab.txt")
	}

	if _, err := downloader.Download(modelID, files); err != nil {
		return "", fmt.Errorf("failed to download model: %w", err)
	}

	return modelPath, nil
}
