package gorag

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"slices"
	"sync"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/indexer"
	"github.com/DotNetAge/gorag/logging"
	"github.com/fsnotify/fsnotify"
)

// indexedDoc 已索引文件的元数据记录
type indexedDoc struct {
	Chunks    []string `json:"chunks"`    // 文件产生的所有 Chunk ID
	Timestamp string   `json:"timestamp"` // 索引时间 (RFC3339)
}

// IndexingService RAG 索引服务
// 支持批量索引和文件监控两种模式
type IndexingService struct {
	dataDir      string              // 索引数据目录
	watchs       []string            // 监控的文件目录
	pendingFiles []string            // 待索引文件列表
	indexedFiles map[string]*indexedDoc // 已索引文件记录（filePath → 元数据）
	indexer      core.Indexer        // 索引器实例
	logger       logging.Logger      // 日志记录器
	workerCount  int                 // worker pool 大小

	// 内部状态
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	indexFile string // 已索引文件记录文件路径（JSON）
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
		indexedFiles: make(map[string]*indexedDoc),
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
	svc.indexFile = filepath.Join(dataDir, "history", "doc_index.json")
	if err := svc.loadIndexedFiles(); err != nil {
		svc.logger.Warn("failed to load indexed files history", "error", err.Error())
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

	s.logger.Info("starting batch indexing", "watch_dirs", s.watchs, "worker_count", s.workerCount)

	// 1. 扫描所有待索引文件
	for _, watchDir := range s.watchs {
		if err := s.scanDirectory(watchDir); err != nil {
			s.logger.Error("failed to scan directory", err, "dir", watchDir)
			continue
		}
	}

	if len(s.pendingFiles) == 0 {
		s.logger.Info("no files to index")
		return nil
	}

	s.logger.Info("files found for indexing", "count", len(s.pendingFiles))

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
			s.logger.Error("failed to index file", result.err, "file", result.file)
		} else {
			s.logger.Info("file indexed successfully", "file", result.file, "chunk_id", result.chunkID, "chunk_count", result.count)
		}
	}

	// 批量保存已索引记录（避免 worker 并发写文件）
	if err := s.saveIndexedFiles(); err != nil {
		s.logger.Warn("failed to persist indexed files after batch", "error", err)
	}

	s.logger.Info("batch indexing completed", "total", len(s.pendingFiles), "failed", failedCount, "success", len(s.pendingFiles)-failedCount)

	// 3. 清空待索引列表
	s.pendingFiles = []string{}

	return nil
}

// indexResult 索引结果
type indexResult struct {
	file    string
	chunkID string
	count   int
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
			if _, exists := s.indexedFiles[file]; exists {
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

			// 获取 chunk ID 列表并记录
			chunkIDs := s.recordFileChunks(ctx, file)
			results <- indexResult{
				file:    file,
				chunkID: chunk.ID,
				count:   len(chunkIDs),
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

	return slices.Contains(textExts, ext)
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
		s.logger.Info("watching directory", "dir", dir)
	}

	// 首次执行全量索引
	if err := s.Index(); err != nil {
		s.logger.Error("initial indexing failed", err)
	}

	// 启动事件处理 goroutine
	s.wg.Add(1)
	go s.handleWatchEvents(watcher)

	s.logger.Info("file watch service started", "watch_dirs", s.watchs)

	// 阻塞等待
	<-s.ctx.Done()

	s.logger.Info("file watch service stopped")

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

			// 处理创建、写入和删除事件
			if event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Remove == fsnotify.Remove {
				if s.isTextFile(event.Name) {
					pendingEvents = append(pendingEvents, event.Name)
					debounceTimer.Reset(2 * time.Second)
				}
			}

		case <-debounceTimer.C:
			if len(pendingEvents) > 0 {
				s.processFileChanges(s.ctx, pendingEvents)
				pendingEvents = []string{}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error("watcher error", err)
		}
	}
}

// processFileChanges 处理文件变更（创建/修改/删除）
func (s *IndexingService) processFileChanges(ctx context.Context, files []string) {
	s.logger.Info("processing file changes", "count", len(files))

	for _, file := range files {
		// 文件已删除 → 清理索引
		if _, err := os.Stat(file); os.IsNotExist(err) {
			if err := s.removeFileIndex(ctx, file); err != nil {
				s.logger.Error("failed to cleanup deleted file index", err, "file", file)
			} else {
				s.logger.Info("cleaned up index for deleted file", "file", file)
			}
			continue
		}

		// 检查文件是否已索引
		s.mu.RLock()
		_, exists := s.indexedFiles[file]
		s.mu.RUnlock()

		if exists {
			// 已索引 → 更新（先删后加）
			s.logger.Info("reindexing changed file", "file", file)
			chunk, err := s.reindexFile(ctx, file)
			if err != nil {
				s.logger.Error("failed to reindex file", err, "file", file)
				continue
			}
			s.logger.Info("file reindexed", "file", file,
				"chunk_count", s.getChunkCount(file), "first_chunk_id", chunk.ID)
		} else {
			// 新文件 → 直接索引
			s.indexNewFile(ctx, file)
		}
	}
}

// removeFileIndex 删除文件的索引记录并移除所有关联 chunks
func (s *IndexingService) removeFileIndex(ctx context.Context, file string) error {
	s.mu.Lock()
	doc, exists := s.indexedFiles[file]
	if !exists {
		s.mu.Unlock()
		return nil
	}
	chunkIDs := make([]string, len(doc.Chunks))
	copy(chunkIDs, doc.Chunks)
	delete(s.indexedFiles, file)
	s.mu.Unlock()

	if err := s.saveIndexedFiles(); err != nil {
		s.logger.Warn("failed to persist index after remove", "file", file, "error", err)
	}

	var errs []error
	for _, chunkID := range chunkIDs {
		if err := s.indexer.Remove(ctx, chunkID); err != nil {
			errs = append(errs, err)
			s.logger.Warn("failed to remove chunk", "file", file, "chunkID", chunkID, "error", err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d/%d chunks failed to remove: %w", len(errs), len(chunkIDs), errs[0])
	}
	return nil
}

// reindexFile 更新文件索引（删除旧数据 → 重新分块索引）
func (s *IndexingService) reindexFile(ctx context.Context, file string) (*core.Chunk, error) {
	// 1. 删除旧索引
	if err := s.removeFileIndex(ctx, file); err != nil {
		s.logger.Warn("partial failure removing old index, continuing with reindex",
			"file", file, "error", err)
	}

	// 2. 预分块获取所有 chunk ID（记录映射关系）
	chunks, err := indexer.GetFileChunks(file)
	if err != nil {
		return nil, fmt.Errorf("get chunks: %w", err)
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	// 3. 执行索引
	chunk, err := s.indexer.AddFile(ctx, file)
	if err != nil {
		return nil, err
	}

	// 4. 记录文件 → chunkIDs 映射
	chunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		chunkIDs[i] = c.ID
	}
	s.mu.Lock()
	s.indexedFiles[file] = &indexedDoc{
		Chunks:    chunkIDs,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	s.mu.Unlock()

	if err := s.saveIndexedFiles(); err != nil {
		s.logger.Warn("failed to persist reindex record", "file", file, "error", err)
	}

	return chunk, nil
}

// indexNewFile 索引新文件并记录映射
func (s *IndexingService) indexNewFile(ctx context.Context, file string) {
	chunks, err := indexer.GetFileChunks(file)
	if err != nil {
		s.logger.Error("failed to get chunks", err, "file", file)
		return
	}
	if len(chunks) == 0 {
		return
	}

	chunk, err := s.indexer.AddFile(ctx, file)
	if err != nil {
		s.logger.Error("failed to index file", err, "file", file)
		return
	}

	chunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		chunkIDs[i] = c.ID
	}
	s.mu.Lock()
	s.indexedFiles[file] = &indexedDoc{
		Chunks:    chunkIDs,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	s.mu.Unlock()

	if err := s.saveIndexedFiles(); err != nil {
		s.logger.Warn("failed to persist indexed file record", "file", file, "error", err)
	}

	s.logger.Info("file indexed", "file", file, "chunk_count", len(chunkIDs), "first_chunk_id", chunk.ID)
}

// Reindex 重新索引指定文件（公开 API）
// 删除旧索引数据后重新分块并索引，适用于文件内容变更后的更新场景
func (s *IndexingService) Reindex(ctx context.Context, file string) error {
	_, err := s.reindexFile(ctx, file)
	return err
}

// recordFileChunks 获取文件的 chunk IDs 并记录到内存（持久化由 Index 批量保存）
func (s *IndexingService) recordFileChunks(_ context.Context, file string) []string {
	chunks, err := indexer.GetFileChunks(file)
	if err != nil {
		s.logger.Warn("failed to get chunk IDs for recording", "file", file, "error", err)
		return nil
	}

	chunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		chunkIDs[i] = c.ID
	}
	s.mu.Lock()
	s.indexedFiles[file] = &indexedDoc{
		Chunks:    chunkIDs,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	s.mu.Unlock()

	return chunkIDs
}

// getChunkCount 获取文件的已索引 chunk 数量
func (s *IndexingService) getChunkCount(file string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if doc, ok := s.indexedFiles[file]; ok {
		return len(doc.Chunks)
	}
	return 0
}

// loadIndexedFiles 从 JSON 文件加载已索引记录，不存在时尝试从旧格式迁移
func (s *IndexingService) loadIndexedFiles() error {
	data, err := os.ReadFile(s.indexFile)
	if os.IsNotExist(err) {
		return s.migrateLegacyIndexedFiles()
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.indexedFiles)
}

// migrateLegacyIndexedFiles 从旧的纯文本格式迁移到 JSON 格式
func (s *IndexingService) migrateLegacyIndexedFiles() error {
	legacyFile := filepath.Join(filepath.Dir(s.indexFile), "indexed_files.txt")
	data, err := os.ReadFile(legacyFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	count := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		path := strings.TrimSpace(line)
		if path != "" {
			s.indexedFiles[path] = &indexedDoc{Timestamp: time.Now().Format(time.RFC3339)}
			count++
		}
	}

	if count > 0 {
		if err := s.saveIndexedFiles(); err != nil {
			return err
		}
		s.logger.Info("migrated indexed files to new format", "count", count)
	}

	return nil
}

// saveIndexedFiles 将已索引记录持久化为 JSON 文件（原子写入）
func (s *IndexingService) saveIndexedFiles() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.indexedFiles, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.indexFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmpFile := s.indexFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, s.indexFile)
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
