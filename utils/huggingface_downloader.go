package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// maxRetries 最大重试次数
const maxRetries = 3

// DownloadObserver 下载观察者接口
type DownloadObserver interface {
	OnEvent(event DownloadEvent)
}

// ModelDownloader 模型下载器
type ModelDownloader struct {
	cacheDir  string
	authToken string
	baseURL   string
	client    *http.Client
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

// getMirrorEndpoint 获取 HuggingFace 镜像地址
// 优先使用 HF_ENDPOINT 环境变量，否则默认 huggingface.co
func getMirrorEndpoint() string {
	if endpoint := os.Getenv("HF_ENDPOINT"); endpoint != "" {
		// 去掉末尾斜杠
		return strings.TrimRight(endpoint, "/")
	}
	return "https://huggingface.co"
}

// NewModelDownloader 创建模型下载器
func NewModelDownloader(cacheDir string) (*ModelDownloader, error) {
	if cacheDir == "" {
		cacheDir = getBaseDir()
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建模型缓存目录失败: %w", err)
	}
	return &ModelDownloader{
		cacheDir:  cacheDir,
		authToken: os.Getenv("HF_TOKEN"),
		baseURL:   getMirrorEndpoint(),
		client: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
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

// buildURL 构建完整的文件下载地址
func (d *ModelDownloader) buildURL(modelID, file string) string {
	return fmt.Sprintf("%s/%s/resolve/main/%s", d.baseURL, modelID, file)
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
	if len(files) == 0 {
		return d.cacheDir, nil
	}

	d.notify(DownloadEvent{
		Type:    EventStart,
		Message: fmt.Sprintf("开始下载模型: %s（镜像源: %s）", modelID, d.baseURL),
	})

	for _, file := range files {
		fileSize, err := d.getFileSize(modelID, file)
		if err != nil {
			return "", fmt.Errorf("获取文件大小失败 %s: %w", file, err)
		}
		d.notify(DownloadEvent{
			Type:    EventStart,
			File:    file,
			Total:   fileSize,
			Message: fmt.Sprintf("下载 %s（%d 字节）", filepath.Base(file), fileSize),
		})

		if err := d.downloadFile(modelID, file, fileSize); err != nil {
			d.notify(DownloadEvent{
				Type:    EventError,
				File:    file,
				Message: err.Error(),
			})
			return "", fmt.Errorf("下载失败 %s: %w", file, err)
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
func (d *ModelDownloader) getFileSize(modelID, file string) (int64, error) {
	url := d.buildURL(modelID, file)
	req, err := d.createRequest("HEAD", url)
	if err != nil {
		return 0, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("请求文件大小失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return resp.ContentLength, nil
	}
	return 0, fmt.Errorf("获取文件大小返回状态码 %d", resp.StatusCode)
}

// downloadFile 下载单个文件（带重试）
func (d *ModelDownloader) downloadFile(modelID, file string, expectedSize int64) error {
	localPath := filepath.Join(d.cacheDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)), file)
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 如果文件已存在且大小匹配，跳过
	if fi, err := os.Stat(localPath); err == nil && fi.Size() == expectedSize {
		return nil
	}

	url := d.buildURL(modelID, file)
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			backoff := time.Duration(retry) * 2 * time.Second
			d.notify(DownloadEvent{
				Type:    EventProgress,
				File:    file,
				Message: fmt.Sprintf("重试第 %d 次（等待 %v）", retry, backoff),
			})
			time.Sleep(backoff)
		}

		lastErr = d.downloadOnce(url, file, localPath, expectedSize)
		if lastErr == nil {
			return nil
		}

		d.notify(DownloadEvent{
			Type:    EventProgress,
			File:    file,
			Message: fmt.Sprintf("下载失败: %v", lastErr),
		})
	}

	return fmt.Errorf("重试 %d 次后仍然失败: %w", maxRetries, lastErr)
}

// downloadOnce 执行单次下载
func (d *ModelDownloader) downloadOnce(url, file, localPath string, expectedSize int64) error {
	req, err := d.createRequest("GET", url)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmpPath := localPath + ".tmp." + fmt.Sprintf("%d", time.Now().UnixNano())
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}

	var downloaded int64
	buf := make([]byte, 32*1024)
	writeErr := error(nil)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := out.Write(buf[:n]); wErr != nil {
				writeErr = fmt.Errorf("写入文件失败: %w", wErr)
				break
			}
			downloaded += int64(n)
			d.notify(DownloadEvent{
				Type:    EventProgress,
				File:    file,
				Current: downloaded,
				Total:   expectedSize,
			})
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			writeErr = fmt.Errorf("读取响应失败: %w", readErr)
			break
		}
	}

	out.Close()

	if writeErr != nil {
		os.Remove(tmpPath)
		return writeErr
	}

	// 验证文件完整性
	if expectedSize > 0 && downloaded != expectedSize {
		os.Remove(tmpPath)
		return fmt.Errorf("文件大小不匹配: 期望 %d, 实际 %d", expectedSize, downloaded)
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
		return "", fmt.Errorf("创建模型目录失败: %w", err)
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
		return "", fmt.Errorf("创建下载器失败: %w", err)
	}
	downloader.WithObserver(observer)

	files := []string{modelFile}
	if filepath.Ext(modelFile) == ".onnx" {
		files = append(files, "config.json", "tokenizer.json", "vocab.txt")
	}

	if _, err := downloader.Download(modelID, files); err != nil {
		return "", fmt.Errorf("下载模型失败: %w", err)
	}

	return modelPath, nil
}
