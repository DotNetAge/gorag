package gorag

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/logging"
	"github.com/fsnotify/fsnotify"
)

// IndexingService RAG 索引服务
// 支持批量索引和文件监控两种模式
type IndexingService struct {
	dataDir      string         // 索引数据目录
	watchs       []string       // 监控的文件目录
	pendingFiles []string       // 待索引文件列表
	indexedFiles map[string]bool // 已索引文件路径集合
	indexer      core.Indexer   // 索引器实例
	logger       logging.Logger // 日志记录器
	workerCount  int            // worker pool 大小

	// 内部状态
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	indexFile   string // 已索引文件记录文件路径
}

// ServiceOption 服务配置选项
type ServiceOption func(*IndexingService)

// WithWatchs 设置监控目录
func WithWatchs(dirs ...string) ServiceOption {
	return func(s *IndexingService) {
		s.watchs = append(s.watchs, dirs...)
	}
}

// WithLogger 设置日志记录器
func WithLogger(logger logging.Logger) ServiceOption {
	return func(s *IndexingService) {
		s.logger = logger
	}
}

// WithWorkerCount 设置 worker pool 大小
func WithWorkerCount(count int) ServiceOption {
	return func(s *IndexingService) {
		if count > 0 {
			s.workerCount = count
		}
	}
}

// NewRAGService 创建 RAG 索引服务
// dataDir: RAG 数据目录（必须）
// opts: 可选配置项
func NewRAGService(dataDir string, opts ...ServiceOption) (*IndexingService, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("dataDir is required")
	}

	// 打开 RAG 库，获取索引器实例
	indexer, err := Open(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open RAG: %w", err)
	}

	// 创建服务实例
	svc := &IndexingService{
		dataDir:      dataDir,
		watchs:       []string{},
		pendingFiles: []string{},
		indexedFiles: make(map[string]bool),
		indexer:      indexer,
		workerCount:  4, // 默认 4 个 worker
	}

	// 应用配置选项
	for _, opt := range opts {
		opt(svc)
	}

	// 设置默认日志记录器
	if svc.logger == nil {
		logDir := filepath.Join(dataDir, "logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		// 使用默认的文件 logger
		logFile := filepath.Join(logDir, "indexing.log")
		logger, err := logging.DefaultFileLogger(logFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create logger: %w", err)
		}
		svc.logger = logger
	}

	// 加载已索引文件记录
	svc.indexFile = filepath.Join(dataDir, "history", "indexed_files.txt")
	if err := svc.loadIndexedFiles(); err != nil {
		svc.logger.Warn("failed to load indexed files history", map[string]any{"error": err.Error()})
	}

	// 创建 history 目录
	historyDir := filepath.Join(dataDir, "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	return svc, nil
}

// Index 执行批量索引
// 对监控目录下的全部文件进行全量索引
func (s *IndexingService) Index() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("starting batch indexing", map[string]any{
		"watch_dirs":  s.watchs,
		"worker_count": s.workerCount,
	})

	// 1. 扫描所有待索引文件
	for _, watchDir := range s.watchs {
		if err := s.scanDirectory(watchDir); err != nil {
			s.logger.Error("failed to scan directory", err, map[string]any{"dir": watchDir})
			continue
		}
	}

	if len(s.pendingFiles) == 0 {
		s.logger.Info("no files to index", nil)
		return nil
	}

	s.logger.Info("files found for indexing", map[string]any{"count": len(s.pendingFiles)})

	// 2. 使用 worker pool 并发索引
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan string, len(s.pendingFiles))
	results := make(chan indexResult, len(s.pendingFiles))

	// 启动 workers
	var wg sync.WaitGroup
	for i := 0; i < s.workerCount; i++ {
		wg.Add(1)
		go s.indexWorker(ctx, &wg, jobs, results)
	}

	// 发送任务
	for _, file := range s.pendingFiles {
		jobs <- file
	}
	close(jobs)

	// 等待所有 workers 完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var failedCount int
	for result := range results {
		if result.err != nil {
			failedCount++
			s.logger.Error("failed to index file", result.err, map[string]any{"file": result.file})
		} else {
			s.logger.Info("file indexed successfully", map[string]any{
				"file":     result.file,
				"chunk_id": result.chunkID,
			})
		}
	}

	s.logger.Info("batch indexing completed", map[string]any{
		"total":  len(s.pendingFiles),
		"failed": failedCount,
		"success": len(s.pendingFiles) - failedCount,
	})

	// 3. 清空待索引列表
	s.pendingFiles = []string{}

	return nil
}

// indexResult 索引结果
type indexResult struct {
	file    string
	chunkID string
	err     error
}

// indexWorker 索引 worker
func (s *IndexingService) indexWorker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan string, results chan<- indexResult) {
	defer wg.Done()

	for file := range jobs {
		select {
		case <-ctx.Done():
			results <- indexResult{file: file, err: ctx.Err()}
			return
		default:
			// 检查文件是否已索引
			s.mu.RLock()
			if s.indexedFiles[file] {
				s.mu.RUnlock()
				results <- indexResult{file: file}
				continue
			}
			s.mu.RUnlock()

			// 执行索引
			chunk, err := s.indexer.AddFile(ctx, file)
			if err != nil {
				results <- indexResult{file: file, err: err}
				continue
			}

			// 记录已索引文件
			s.mu.Lock()
			s.indexedFiles[file] = true
			s.mu.Unlock()

			// 持久化到文件
			if err := s.appendIndexedFile(file); err != nil {
				s.logger.Warn("failed to record indexed file", map[string]any{
					"file":  file,
					"error": err.Error(),
				})
			}

			results <- indexResult{
				file:    file,
				chunkID: chunk.ID,
			}
		}
	}
}

// scanDirectory 扫描目录下的所有文件
func (s *IndexingService) scanDirectory(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// 检查文件是否为文本文件
		if s.isTextFile(path) {
			s.pendingFiles = append(s.pendingFiles, path)
		}

		return nil
	})
}

// isTextFile 判断是否为文本文件
func (s *IndexingService) isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	textExts := []string{
		".txt", ".md", ".json", ".yaml", ".yml",
		".html", ".xml", ".css",
		".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h",
		".sh", ".bash", ".zsh",
		".sql", ".conf", ".cfg", ".ini",
	}

	for _, e := range textExts {
		if ext == e {
			return true
		}
	}
	return false
}

// Watch 启动文件监控服务
// 当文件发生变更时自动进行索引
// 首次启动时会执行全量索引
func (s *IndexingService) Watch() error {
	if len(s.watchs) == 0 {
		return fmt.Errorf("no directories to watch")
	}

	// 创建文件监控器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// 设置上下文
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	// 添加监控目录
	for _, dir := range s.watchs {
		if err := watcher.Add(dir); err != nil {
			return fmt.Errorf("failed to watch directory %s: %w", dir, err)
		}
		s.logger.Info("watching directory", map[string]any{"dir": dir})
	}

	// 首次执行全量索引
	if err := s.Index(); err != nil {
		s.logger.Error("initial indexing failed", err, nil)
	}

	// 启动事件处理 goroutine
	s.wg.Add(1)
	go s.handleWatchEvents(watcher)

	s.logger.Info("file watch service started", map[string]any{"watch_dirs": s.watchs})

	// 阻塞等待
	<-s.ctx.Done()

	s.logger.Info("file watch service stopped", nil)

	return nil
}

// handleWatchEvents 处理文件监控事件
func (s *IndexingService) handleWatchEvents(watcher *fsnotify.Watcher) {
	defer s.wg.Done()

	debounceTimer := time.NewTimer(2 * time.Second)
	defer debounceTimer.Stop()

	var pendingEvents []string

	for {
		select {
		case <-s.ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// 只处理创建和写入事件
			if event.Op&fsnotify.Create == fsnotify.Create || 
			   event.Op&fsnotify.Write == fsnotify.Write {
				// 检查是否为文本文件
				if s.isTextFile(event.Name) {
					pendingEvents = append(pendingEvents, event.Name)
					// 重置防抖定时器
					debounceTimer.Reset(2 * time.Second)
				}
			}

		case <-debounceTimer.C:
			// 处理累积的事件
			if len(pendingEvents) > 0 {
				s.processFileChanges(pendingEvents)
				pendingEvents = []string{}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error("watcher error", err, nil)
		}
	}
}

// processFileChanges 处理文件变更
func (s *IndexingService) processFileChanges(files []string) {
	s.logger.Info("processing file changes", map[string]any{"count": len(files)})

	for _, file := range files {
		// 检查文件是否已索引
		s.mu.RLock()
		if s.indexedFiles[file] {
			s.mu.RUnlock()
			// 文件已存在，需要更新（先删除再添加）
			// 注意：当前实现暂不支持更新，跳过
			s.logger.Warn("file already indexed, skip update", map[string]any{"file": file})
			continue
		}
		s.mu.RUnlock()

		// 索引新文件
		chunk, err := s.indexer.AddFile(s.ctx, file)
		if err != nil {
			s.logger.Error("failed to index file", err, map[string]any{"file": file})
			continue
		}

		// 记录已索引文件
		s.mu.Lock()
		s.indexedFiles[file] = true
		s.mu.Unlock()

		if err := s.appendIndexedFile(file); err != nil {
			s.logger.Warn("failed to record indexed file", map[string]any{
				"file":  file,
				"error": err.Error(),
			})
		}

		s.logger.Info("file indexed successfully", map[string]any{
			"file":     file,
			"chunk_id": chunk.ID,
		})
	}
}

// loadIndexedFiles 加载已索引文件记录
func (s *IndexingService) loadIndexedFiles() error {
	file, err := os.Open(s.indexFile)
	if os.IsNotExist(err) {
		return nil // 文件不存在是正常情况
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path != "" {
			s.indexedFiles[path] = true
		}
	}

	return scanner.Err()
}

// appendIndexedFile 追加已索引文件记录
func (s *IndexingService) appendIndexedFile(path string) error {
	file, err := os.OpenFile(s.indexFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintln(file, path)
	return err
}

// Stop 停止服务
func (s *IndexingService) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()

	// 关闭索引器
	if closer, ok := s.indexer.(interface{ Close() error }); ok {
		return closer.Close()
	}

	return nil
}
