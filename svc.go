package gorag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/llm"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/store/meta"
	"github.com/DotNetAge/gorag/v2/utils"
	gvcore "github.com/DotNetAge/govector/core"
)

// textExts 可索引的文本文件扩展名列表
var textExts = []string{
	".txt", ".md", ".json", ".yaml", ".yml",
	".html", ".xml", ".css",
	".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h",
	".sh", ".bash", ".zsh",
	".sql", ".conf", ".cfg", ".ini",
}

// ── 初始化 RAG 库 ────────────────────────────────────────────────────

// InitOptions 初始化 RAG 库的配置选项。
type InitOptions struct {
	RagDir    string                 // .rag 库目录
	IndexType string                 // 索引器类型: semantic/graph/hyper
	ModelPath string                 // 本地模型文件路径（与 ModelID 二选一）
	ModelID   string                 // HuggingFace 模型 ID（与 ModelPath 二选一）
	ModelFile string                 // HuggingFace 模型文件名
	Observer  utils.DownloadObserver // 下载进度观察者（可选）
}

// InitResult 初始化结果。
type InitResult struct {
	RagDir      string
	IndexType   string
	ModelPath   string
	IndexerName string
	Config      *Config
	ConfigYAML  string
}

// InitRAG 初始化 RAG 库。
// 包含完整的初始化流程：模型下载、目录创建、配置写入、索引器验证。
func InitRAG(opts InitOptions) (*InitResult, error) {
	// 1. 确定模型路径
	modelPath, err := resolveModelPath(opts)
	if err != nil {
		return nil, err
	}

	// 2. 创建 .rag 目录结构
	if err := Init(opts.RagDir); err != nil {
		return nil, fmt.Errorf("创建 RAG 库失败: %w", err)
	}

	// 3. 写入配置
	cfg, err := LoadConfig(opts.RagDir)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	cfg.Indexer.Type = opts.IndexType
	if modelPath != "" {
		cfg.Embedding.ModelFile = modelPath
	}
	if err := SaveConfig(opts.RagDir, cfg); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	// 4. 打开索引器验证（仅当有模型路径时）
	var indexerName string
	if modelPath != "" {
		idx, err := Open(opts.RagDir)
		if err == nil {
			indexerName = idx.Name()
			if closer, ok := idx.(indexer.IndexerCloser); ok {
				closer.Close(context.Background())
			}
		}
	}

	// 5. 返回结果
	_, raw, _ := loadConfigRaw(opts.RagDir)

	return &InitResult{
		RagDir:      opts.RagDir,
		IndexType:   opts.IndexType,
		ModelPath:   modelPath,
		IndexerName: indexerName,
		Config:      cfg,
		ConfigYAML:  raw,
	}, nil
}

// resolveModelPath 确定模型路径。
// 策略：优先使用 ModelID 从 HuggingFace 下载，其次使用 ModelPath。
func resolveModelPath(opts InitOptions) (string, error) {
	if !needsModel(opts.IndexType) {
		return "", nil
	}

	if opts.ModelID != "" {
		modelFile := opts.ModelFile
		if modelFile == "" {
			modelFile = "onnx/model.onnx"
		}
		path, err := utils.CheckAndDownload(opts.ModelID, modelFile, opts.Observer)
		if err != nil {
			return "", fmt.Errorf("模型下载失败: %w", err)
		}
		return path, nil
	}

	if opts.ModelPath != "" {
		if _, err := os.Stat(opts.ModelPath); os.IsNotExist(err) {
			return "", fmt.Errorf("模型文件不存在: %s", opts.ModelPath)
		}
		return opts.ModelPath, nil
	}

	return "", fmt.Errorf("%s 索引器需要模型，请指定模型路径或 HuggingFace 模型 ID", opts.IndexType)
}

// needsModel 判断索引器类型是否需要模型。
func needsModel(indexerType string) bool {
	return indexerType == "hyper" || indexerType == "semantic"
}

// ── 索引服务 ──────────────────────────────────────────────────────────
// 提供与 CLI 命令一一对应的业务方法。
type IndexingService struct {
	dataDir   string          // 索引数据目录
	metaStore meta.Store      // 元数据存储
	indexer   indexer.Indexer // 索引器实例
	logger    logging.Logger  // 日志记录器

	mu sync.RWMutex // 保护内部状态
}

// ServiceOption 服务配置选项
type ServiceOption func(*IndexingService)

// WithMetaStore 设置元数据存储
func WithMetaStore(store meta.Store) ServiceOption {
	return func(s *IndexingService) {
		s.metaStore = store
	}
}

// WithLogger 设置日志记录器
func WithLogger(logger logging.Logger) ServiceOption {
	return func(s *IndexingService) {
		s.logger = logger
	}
}

// NewRAGService 创建 RAG 索引服务。
//
// 参数：
//   - dataDir: RAG 库目录（必须是以 .rag 结尾的绝对路径）
//   - opts: 可选配置项
//
// 若未通过 WithMetaStore 注入元数据存储，会自动创建 SQLite 存储。
// 若未通过 WithLogger 注入日志器，会自动创建文件日志。
func NewRAGService(dataDir string, opts ...ServiceOption) (*IndexingService, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("dataDir 不能为空")
	}

	// 打开 RAG 库，获取索引器实例
	idx, err := Open(dataDir)
	if err != nil {
		return nil, fmt.Errorf("打开 RAG 库失败: %w", err)
	}

	svc := &IndexingService{
		dataDir: dataDir,
		indexer: idx,
	}

	// 应用配置选项
	for _, opt := range opts {
		opt(svc)
	}

	// 设置默认元数据存储
	if svc.metaStore == nil {
		metaDB := filepath.Join(dataDir, "meta.db")
		store, err := meta.NewSQLiteStore(metaDB)
		if err != nil {
			// 关闭已打开的索引器
			if closer, ok := idx.(indexer.IndexerCloser); ok {
				closer.Close(context.Background())
			}
			return nil, fmt.Errorf("创建元数据存储失败: %w", err)
		}
		svc.metaStore = store
	}

	// 设置默认日志记录器
	if svc.logger == nil {
		logDir := filepath.Join(dataDir, "logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}
		logFile := filepath.Join(logDir, "gorag.log")
		logger, err := logging.DefaultFileLogger(logFile)
		if err != nil {
			return nil, fmt.Errorf("创建日志记录器失败: %w", err)
		}
		svc.logger = logger
	}

	return svc, nil
}

// Indexer 返回底层索引器实例。
func (s *IndexingService) Indexer() indexer.Indexer {
	return s.indexer
}

// reindexChangedFiles 对指定路径执行增量重新索引（扫描、分块、向量化）。
//
// 复用 Index 的核心扫描 + worker pool 逻辑，但不加锁（调用方负责上锁）。
// 用于 Index 和 Update 双阶段流程的第一阶段。
//
// targetPath 可以是文件或目录。
func (s *IndexingService) reindexChangedFiles(ctx context.Context, targetPath string) error {
	// 0. 加载 .ragignore 规则
	ragignore := loadRagignore(s.dataDir)

	// 1. 扫描文件
	var files []string
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("无法访问目标路径: %w", err)
	}
	if info.IsDir() {
		files, err = scanDir(targetPath, ragignore)
	} else {
		if isTextFile(targetPath) {
			files = append(files, targetPath)
		}
	}
	if err != nil {
		return fmt.Errorf("扫描文件失败: %w", err)
	}

	if len(files) == 0 {
		s.logger.Info("无待索引文件")
		return nil
	}

	s.logger.Info("开始重新索引变更的文件", "target", targetPath, "files", len(files))

	// 2. 使用 worker pool 并发索引
	workerCount := 4
	jobs := make(chan string, len(files))
	results := make(chan indexResult, len(files))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go s.indexWorker(ctx, &wg, jobs, results)
	}

	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var failedCount int
	for result := range results {
		if result.err != nil {
			failedCount++
			s.logger.Error("索引文件失败", result.err, "file", result.file)
		} else {
			s.logger.Info("文件索引成功", "file", result.file, "chunk_count", result.count)
		}
	}

	s.logger.Info("重新索引完成",
		"total", len(files),
		"failed", failedCount,
		"success", len(files)-failedCount)

	// 3. 为没有 README.md 的目录生成 Region 摘要文件
	indexedDirs := collectIndexedDirs(files, targetPath)
	for _, dir := range indexedDirs {
		readmePath := filepath.Join(dir, "README.md")
		if fileExists(readmePath) {
			continue
		}
		if err := s.generateRegionReadme(ctx, dir); err != nil {
			s.logger.Warn("生成目录 README 失败", "dir", dir, "error", err)
		}
	}

	return nil
}

// Index 对指定路径执行批量索引（快路径，无 LLM）。
// targetPath 可以是文件或目录。
func (s *IndexingService) Index(ctx context.Context, targetPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reindexChangedFiles(ctx, targetPath)
}

// indexResult 索引结果
type indexResult struct {
	file  string
	count int
	err   error
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
			chunks, err := s.processFile(ctx, file)
			if err != nil {
				results <- indexResult{file: file, err: err}
				continue
			}
			results <- indexResult{
				file:  file,
				count: len(chunks),
			}
		}
	}
}

// processFile 对单个文件执行完整索引流程（含两级增量预检）。
//
// 流程：
//  1. 两级预检：mtime + size → hash
//  2. 清理旧数据
//  3. 调用 indexer.AddFile 写入新数据
//  4. 成功/失败写入 meta.db
func (s *IndexingService) processFile(ctx context.Context, absPath string) ([]*core.Chunk, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat 文件失败: %w", err)
	}

	// 1. 两级预检：先查 mtime+size，不匹配才算 hash
	existing, _ := s.metaStore.GetDocumentByPath(absPath)
	if existing != nil && existing.Status == "indexed" {
		if existing.ModifiedAt.Equal(info.ModTime()) && existing.SizeBytes == info.Size() {
			s.logger.Info("文件未变更（mtime+size 命中），跳过", "path", absPath)
			return nil, nil
		}
	}

	// 2. 计算内容 hash（仅 mtime/size 变化时计算）
	contentHash, err := computeFileHash(absPath)
	if err != nil {
		s.recordFailure(absPath, "", err)
		return nil, fmt.Errorf("计算文件 hash 失败: %w", err)
	}

	if existing != nil && existing.ContentHash == contentHash && existing.Status == "indexed" {
		s.logger.Info("文件未变更（hash 命中），跳过", "path", absPath)
		return nil, nil
	}

	// 3. 先清理旧数据
	if existing != nil && len(existing.ChunkIDs) > 0 {
		if err := s.removeFileIndex(ctx, absPath); err != nil {
			s.logger.Warn("清理旧索引失败，继续写入新数据", "path", absPath, "err", err)
		}
	}

	// 4. 调用 indexer.AddFile 写入新数据
	chunks, err := s.indexer.AddFile(ctx, absPath)
	if err != nil {
		s.recordFailure(absPath, contentHash, err)
		return nil, fmt.Errorf("索引文件失败: %w", err)
	}

	// 5. 成功状态写入 meta.db
	chunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		chunkIDs[i] = c.ID
	}
	if err := s.metaStore.SaveDocument(&meta.Document{
		AbsolutePath: absPath,
		FileName:     info.Name(),
		Extension:    filepath.Ext(absPath),
		SizeBytes:    info.Size(),
		ModifiedAt:   info.ModTime(),
		ContentHash:  contentHash,
		Status:       "indexed",
		ChunkIDs:     chunkIDs,
		IndexedAt:    timePtr(time.Now()),
	}); err != nil {
		s.logger.Warn("保存元数据失败", "path", absPath, "error", err)
	}

	// 6. 写入 chunk_llm_status（初始状态：未摘要、未实体提取）
	for _, c := range chunks {
		if err := s.metaStore.SaveChunkLLMStatus(&meta.ChunkLLMStatus{
			ChunkID:       c.ID,
			DocPath:       absPath,
			DocID:         c.DocID,
			ContentHash:   computeChunkContentHash(c.Content),
			ContentLength: len(c.Content),
			Summarized:    false,
			Refilled:      false,
		}); err != nil {
			s.logger.Warn("保存 chunk_llm_status 失败", "chunk_id", c.ID, "error", err)
		}
	}

	return chunks, nil
}

// recordFailure 将索引失败记录写入 meta.db。
func (s *IndexingService) recordFailure(absPath, contentHash string, err error) {
	if saveErr := s.metaStore.SaveDocument(&meta.Document{
		AbsolutePath: absPath,
		ContentHash:  contentHash,
		Status:       "failed",
		ErrorMessage: err.Error(),
	}); saveErr != nil {
		s.logger.Warn("保存失败记录到 meta.db 失败", "path", absPath, "error", saveErr)
	}
}

// scanDir 扫描目录下的所有文本文件，跳过 .ragignore 匹配的目录。
func scanDir(dir string, ragignore []string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// 跳过 .rag 子目录（避免索引库索引自己）
			if strings.HasSuffix(path, ".rag") {
				return filepath.SkipDir
			}
			// 跳过 .ragignore 匹配的目录
			if matchRagignoreDir(path, dir, ragignore) {
				return filepath.SkipDir
			}
			return nil
		}
		if isTextFile(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// loadRagignore 从 .rag 目录加载 .ragignore 忽略规则。
// 返回非空、非注释的规则行列表。文件不存在时返回空切片。
func loadRagignore(ragDir string) []string {
	data, err := os.ReadFile(filepath.Join(ragDir, ".ragignore"))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// matchRagignoreDir 判断目录是否匹配任一 .ragignore 规则。
// 规则支持目录匹配（尾随 /）和文件名匹配。
func matchRagignoreDir(dirPath, scanRoot string, patterns []string) bool {
	rel, err := filepath.Rel(scanRoot, dirPath)
	if err != nil {
		return false
	}
	dirName := filepath.Base(dirPath)
	for _, pattern := range patterns {
		// 通配符规则：**.pyc → 检查是否以 .pyc 结尾
		if strings.HasPrefix(pattern, "**.") {
			suffix := strings.TrimPrefix(pattern, "**")
			if strings.HasSuffix(dirName, suffix) {
				return true
			}
			if strings.HasSuffix(rel, suffix) {
				return true
			}
			continue
		}
		// *.swp, *.swo 等通配符
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(dirName, suffix) {
				return true
			}
			continue
		}
		cleanPattern := strings.TrimSuffix(pattern, "/")
		// 路径中的任意一级匹配
		if strings.HasPrefix(rel, cleanPattern) || strings.Contains("/"+rel+"/", "/"+cleanPattern+"/") {
			return true
		}
	}
	return false
}

// isTextFile 判断是否为可索引的文本文件
func isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return slices.Contains(textExts, ext)
}

// removeFileIndex 删除文件的索引记录并清理所有关联 chunks。
// 全部 Remove 成功后才删除 meta.db 记录，部分失败则标记 partial_deleted。
func (s *IndexingService) removeFileIndex(ctx context.Context, absPath string) error {
	doc, err := s.metaStore.GetDocumentByPath(absPath)
	if err != nil {
		return fmt.Errorf("查询文档元数据失败: %w", err)
	}
	if doc == nil {
		return nil
	}

	if len(doc.ChunkIDs) == 0 {
		return s.metaStore.DeleteDocument(absPath)
	}

	var failedChunks []string
	if admin, ok := s.indexer.(indexer.IndexerAdmin); ok {
		for _, chunkID := range doc.ChunkIDs {
			if err := admin.Remove(ctx, chunkID); err != nil {
				s.logger.Error("删除 chunk 失败", err, "chunk_id", chunkID)
				failedChunks = append(failedChunks, chunkID)
			}
		}
	}

	if len(failedChunks) > 0 {
		return s.metaStore.SaveDocument(&meta.Document{
			AbsolutePath: doc.AbsolutePath,
			ContentHash:  doc.ContentHash,
			Status:       "partial_deleted",
			ChunkIDs:     failedChunks,
			ErrorMessage: fmt.Sprintf("部分 chunk 删除失败：%d/%d", len(failedChunks), len(doc.ChunkIDs)),
		})
	}

	if err := s.metaStore.DeleteDocument(absPath); err != nil {
		return err
	}
	// 清理 chunk_llm_status
	if err := s.metaStore.DeleteChunkLLMStatusByDocPath(absPath); err != nil {
		s.logger.Warn("清理 chunk_llm_status 失败", "path", absPath, "error", err)
	}
	return nil
}

// SetLLMUsageRecorder 为 Summarizer 和 Refiller 设置 token 用量记录回调。
// 在每次成功 LLM 调用后自动将 token 用量写入 meta.db 的 usages 表。
// 通过类型断言检测传入对象是否支持 SetUsageRecorder，不支持时静默跳过。
func (s *IndexingService) SetLLMUsageRecorder(summarizer llm.Summarizer, refiller llm.Refiller) {
	recorder := func(ctx context.Context, model string, usage *chat.Usage, label string) {
		if usage == nil {
			return
		}

		u := &meta.Usage{
			Model:            model,
			Label:            label,
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			CreatedAt:        time.Now(),
		}
		if usage.PromptTokensDetails != nil {
			u.CachedTokens = usage.PromptTokensDetails.CachedTokens
			u.PromptAudioTokens = usage.PromptTokensDetails.AudioTokens
		}
		if usage.CompletionTokensDetails != nil {
			u.ReasoningTokens = usage.CompletionTokensDetails.ReasoningTokens
			u.CompletionAudioTokens = usage.CompletionTokensDetails.AudioTokens
			u.AcceptedPredictionTokens = usage.CompletionTokensDetails.AcceptedPredictionTokens
			u.RejectedPredictionTokens = usage.CompletionTokensDetails.RejectedPredictionTokens
		}

		if err := s.metaStore.SaveUsage(u); err != nil {
			s.logger.Warn("保存 token 用量记录失败", "error", err)
		}
	}

	if s, ok := summarizer.(interface{ SetUsageRecorder(llm.UsageRecorder) }); ok {
		s.SetUsageRecorder(recorder)
	}
	if r, ok := refiller.(interface{ SetUsageRecorder(llm.UsageRecorder) }); ok {
		r.SetUsageRecorder(recorder)
	}
}

// QueryUsages 查询最近的 token 用量记录，按时间倒序。
// limit 限制返回条数，<= 0 时返回全部。
func (s *IndexingService) QueryUsages(limit int) ([]*meta.Usage, error) {
	return s.metaStore.QueryUsages(limit)
}

// Stop 停止服务，关闭所有资源。
func (s *IndexingService) Stop() error {
	// 关闭索引器
	if closer, ok := s.indexer.(indexer.IndexerCloser); ok {
		if err := closer.Close(context.Background()); err != nil {
			s.logger.Error("关闭索引器失败", err)
		}
	}

	// 关闭元数据存储
	if s.metaStore != nil {
		if err := s.metaStore.Close(); err != nil {
			s.logger.Error("关闭元数据存储失败", err)
		}
	}

	return nil
}

// computeFileHash 计算文件的 SHA256 哈希值。
func computeFileHash(absPath string) (string, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("计算哈希失败: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// timePtr 返回 time.Time 的指针。
func timePtr(t time.Time) *time.Time {
	return &t
}

// ── 内容变更阈值 ──────────────────────────────────────────────────

// minContentChangeForLLM 触发 LLM 重新处理的最小内容长度变化量（字符数）。
// 已处理过的分片再次发生内容变更时，若长度变化低于此阈值，视为微小改动，
// 跳过 LLM 处理以避免浪费调用。
const minContentChangeForLLM = 50

// computeChunkContentHash 计算分片内容的简短哈希（SHA256 前 16 位十六进制）。
func computeChunkContentHash(content string) string {
	if content == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

// abs 返回 int 绝对值。
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// contains 检查字符串切片是否包含指定值。
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// =====================================================================
// 配置读取与文件系统工具（包内辅助函数）
// =====================================================================

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// calcDirSizes 递归计算各子目录大小。
func calcDirSizes(dataDir string) map[string]int64 {
	sizes := make(map[string]int64)
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return sizes
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			fi, err := entry.Info()
			if err == nil {
				sizes["total"] += fi.Size()
			}
			continue
		}
		subSize := dirSize(filepath.Join(dataDir, entry.Name()))
		sizes[entry.Name()] = subSize
		sizes["total"] += subSize
	}
	return sizes
}

// dirSize 递归计算目录总大小。
func dirSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err == nil {
			size += fi.Size()
		}
		return nil
	})
	return size
}

// getVectorCount 获取向量索引中的条目数。
func getVectorCount(dbPath string) int {
	storage, err := gvcore.NewStorage(dbPath)
	if err != nil {
		return -1
	}
	defer storage.Close()

	collections, err := storage.ListCollections()
	if err != nil || len(collections) == 0 {
		return -1
	}

	meta, err := storage.LoadCollectionMeta(collections[0])
	if err != nil {
		return -1
	}

	col, err := gvcore.NewCollection(meta.Name, meta.VectorLen, meta.Metric, storage, meta.UseHNSW)
	if err != nil {
		return -1
	}
	return col.Count()
}

// getGraphCount 获取图索引的节点和边数量。
func getGraphCount(dbPath string) (nodes int64, edges int64) {
	db, err := api.Open(dbPath)
	if err != nil {
		return -1, -1
	}
	defer db.Close()

	ctx := context.Background()
	nodes = queryGraphCount(ctx, db, "MATCH (n) RETURN count(n) AS cnt")
	edges = queryGraphCount(ctx, db, "MATCH ()-[r]->() RETURN count(r) AS cnt")
	return nodes, edges
}

// queryGraphCount 执行图查询获取计数值。
func queryGraphCount(ctx context.Context, db *api.DB, query string) int64 {
	rows, err := db.Query(ctx, query)
	if err != nil {
		return -1
	}
	defer rows.Close()

	if !rows.Next() {
		return 0
	}

	var count int64
	if err := rows.Scan(&count); err != nil {
		var val any
		_ = rows.Scan(&val)
		if val != nil {
			switch v := val.(type) {
			case int64:
				return v
			case int:
				return int64(v)
			case float64:
				return int64(v)
			}
		}
		return -1
	}
	return count
}

// =====================================================================
// 服务方法（与 CLI 命令一一对应）
// =====================================================================

// Query 执行搜索查询，可选按 source 路径前缀过滤。
func (s *IndexingService) Query(ctx context.Context, text string, filterPath string) (*core.Hit, error) {
	if text == "" {
		return nil, fmt.Errorf("查询内容不能为空")
	}
	hit, err := s.indexer.Search(ctx, s.indexer.NewQuery(text))
	if err != nil {
		return nil, err
	}
	if filterPath != "" {
		hit = filterHitBySourcePrefix(hit, filterPath)
	}
	return hit, nil
}

// QueryMulti 执行多个查询并融合结果，可选按 source 路径前缀过滤。
// 融合规则：相同 chunk 取最高分数，按分数降序排列。
func (s *IndexingService) QueryMulti(ctx context.Context, queries []string, filterPath string) (*core.Hit, error) {
	if len(queries) == 0 {
		return nil, fmt.Errorf("查询内容不能为空")
	}

	merged := make(map[string]core.ChunkHit)
	for _, text := range queries {
		if strings.TrimSpace(text) == "" {
			continue
		}
		hit, err := s.indexer.Search(ctx, s.indexer.NewQuery(text))
		if err != nil {
			return nil, fmt.Errorf("查询 %q 失败: %w", text, err)
		}
		if hit == nil {
			continue
		}
		for _, ch := range hit.Chunks {
			if filterPath != "" && !sourceHasPrefix(ch.Chunk.Source, filterPath) {
				continue
			}
			existing, ok := merged[ch.Chunk.ID]
			if !ok || ch.Score > existing.Score {
				merged[ch.Chunk.ID] = ch
			}
		}
	}

	chunks := make([]core.ChunkHit, 0, len(merged))
	for _, ch := range merged {
		chunks = append(chunks, ch)
	}

	// 按分数降序排列
	for i := 0; i < len(chunks); i++ {
		for j := i + 1; j < len(chunks); j++ {
			if chunks[j].Score > chunks[i].Score {
				chunks[i], chunks[j] = chunks[j], chunks[i]
			}
		}
	}

	combinedText := strings.Join(queries, " | ")
	return &core.Hit{
		Query:  s.indexer.NewQuery(combinedText),
		Score:  topChunkScore(chunks),
		Chunks: chunks,
	}, nil
}

// filterHitBySourcePrefix 按 source 路径前缀过滤命中结果。
func filterHitBySourcePrefix(hit *core.Hit, filterPath string) *core.Hit {
	if hit == nil {
		return nil
	}
	var filtered []core.ChunkHit
	for _, ch := range hit.Chunks {
		if sourceHasPrefix(ch.Chunk.Source, filterPath) {
			filtered = append(filtered, ch)
		}
	}
	hit.Chunks = filtered
	return hit
}

// sourceHasPrefix 判断 chunk.Source 是否以指定路径开头。
// filterPath 会先转换为绝对路径，再与 Source（绝对路径）比较。
func sourceHasPrefix(source, filterPath string) bool {
	if source == "" {
		return false
	}
	absFilter, err := filepath.Abs(filterPath)
	if err != nil {
		absFilter = filterPath
	}
	return strings.HasPrefix(source, absFilter)
}

// topChunkScore 返回 ChunkHit 切片中的最高分。
func topChunkScore(hits []core.ChunkHit) float32 {
	if len(hits) == 0 {
		return 0
	}
	max := hits[0].Score
	for _, h := range hits[1:] {
		if h.Score > max {
			max = h.Score
		}
	}
	return max
}

// ListChunks 分页列出已索引的 Chunk，可选按 source 路径前缀过滤。
// page 从 1 开始，size 为每页数量（<=0 时使用默认值 20）。
// 返回当前页 Chunk 列表、过滤后总数和错误。
func (s *IndexingService) ListChunks(ctx context.Context, page, size int, filterPath string) ([]core.Chunk, int, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	admin, ok := s.indexer.(indexer.IndexerAdmin)
	if !ok {
		return nil, 0, fmt.Errorf("当前索引器不支持列表查询")
	}

	var filters []core.FilterCondition
	if filterPath != "" {
		absFilter, err := filepath.Abs(filterPath)
		if err != nil {
			absFilter = filterPath
		}
		filters = append(filters, core.FilterCondition{
			Key:   core.VecMetaSource,
			Type:  "prefix",
			Value: absFilter,
		})
	}

	offset := (page - 1) * size
	chunks, total, err := admin.List(ctx, offset, size, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("列出分片失败: %w", err)
	}
	return chunks, total, nil
}

// NodeQueryResult 是 nodes 命令的查询结果容器。
type NodeQueryResult struct {
	RegionID   string       `json:"region_id"`
	RegionName string       `json:"region_name"`
	Region     *core.Node   `json:"region,omitempty"`
	Nodes      []*core.Node `json:"nodes"`
	Edges      []*core.Edge `json:"edges"`
}

// Nodes 按目录查询 Region 节点及其 N 跳相邻节点。
// dir 为空时使用当前工作目录；hops 范围 1-3，默认 1。
func (s *IndexingService) Nodes(ctx context.Context, dir string, hops int) (*NodeQueryResult, error) {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
		}
		dir = cwd
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("解析目录路径失败: %w", err)
	}

	if hops < 1 {
		hops = 1
	}
	if hops > 3 {
		hops = 3
	}

	// RegionID 与 Chunk.RegionID 保持一致：目录路径的 SHA256
	regionID := utils.GenerateID([]byte(absDir))

	nav, ok := s.indexer.(indexer.GraphNavigator)
	if !ok {
		return nil, fmt.Errorf("当前索引器不支持图导航")
	}

	region, err := nav.GetNode(ctx, regionID)
	if err != nil {
		return nil, fmt.Errorf("获取 Region 节点失败: %w", err)
	}

	neighborNodes, edges, err := nav.Neighbors(ctx, regionID, hops, 100)
	if err != nil {
		return nil, fmt.Errorf("邻居遍历失败: %w", err)
	}

	// 合并 Region 节点与邻居节点，并去重
	nodeMap := make(map[string]*core.Node, len(neighborNodes)+1)
	if region != nil {
		nodeMap[region.ID] = region
	}
	for _, n := range neighborNodes {
		if n != nil {
			nodeMap[n.ID] = n
		}
	}
	nodes := make([]*core.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	regionName := filepath.Base(absDir)
	if region != nil && region.Name != "" {
		regionName = region.Name
	}

	return &NodeQueryResult{
		RegionID:   regionID,
		RegionName: regionName,
		Region:     region,
		Nodes:      nodes,
		Edges:      edges,
	}, nil
}

// Cypher 执行原始 Cypher 查询并返回结果。
// 仅当索引器支持图存储时可用（如 GraphIndexer / HyperIndexer）。
func (s *IndexingService) Cypher(ctx context.Context, query string) ([]map[string]any, error) {
	if query == "" {
		return nil, fmt.Errorf("Cypher 查询不能为空")
	}

	type cypherer interface {
		CypherQuery(ctx context.Context, q string, params map[string]any) ([]map[string]any, error)
	}

	c, ok := s.indexer.(cypherer)
	if !ok {
		return nil, fmt.Errorf("当前索引器不支持 Cypher 查询")
	}

	rows, err := c.CypherQuery(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("Cypher 查询执行失败: %w", err)
	}
	return rows, nil
}

// RAGInfo RAG 库信息
type RAGInfo struct {
	Config      *Config
	ConfigYAML  string
	AbsPath     string
	Sizes       map[string]int64
	VectorCount int
	GraphNodes  int64
	GraphEdges  int64
}

// Info 获取 RAG 库的完整信息。
func (s *IndexingService) Info() (*RAGInfo, error) {
	info := &RAGInfo{AbsPath: s.dataDir}

	// 1. 读取配置
	cfg, raw, err := loadConfigRaw(s.dataDir)
	if err != nil {
		return nil, err
	}
	info.Config = cfg
	info.ConfigYAML = raw

	// 2. 目录大小
	info.Sizes = calcDirSizes(s.dataDir)

	// 3. 向量索引统计
	name := filepath.Base(s.dataDir)
	vecDB := filepath.Join(s.dataDir, "vectors", name+".db")
	if _, err := os.Stat(vecDB); err == nil {
		info.VectorCount = getVectorCount(vecDB)
	}

	// 4. 图索引统计
	graphDB := filepath.Join(s.dataDir, "graphs", name+".db")
	if _, err := os.Stat(graphDB); err == nil {
		info.GraphNodes, info.GraphEdges = getGraphCount(graphDB)
	}

	return info, nil
}

// CheckResult 诊断项结果
type CheckResult struct {
	Name string
	OK   bool
	Hint string
}

// Doctor 诊断 RAG 库的配置完整性。
func (s *IndexingService) Doctor() []CheckResult {
	cfg, err := loadConfig(s.dataDir)
	if err != nil {
		return []CheckResult{{Name: "config.yml", OK: false, Hint: "读取失败: " + err.Error()}}
	}

	return []CheckResult{
		{Name: "config.yml", OK: true},
		{Name: "embedding.model_file", OK: cfg.Embedding.ModelFile != "", Hint: "运行：grag config embedder <path>"},
		{Name: "向量库目录", OK: dirExists(filepath.Join(s.dataDir, "vectors"))},
		{Name: "图库目录", OK: dirExists(filepath.Join(s.dataDir, "graphs"))},
		{Name: "meta.db", OK: fileExists(filepath.Join(s.dataDir, "meta.db"))},
	}
}

// Logs 返回 RAG 库的日志内容。
func (s *IndexingService) Logs() (string, error) {
	logFile := filepath.Join(s.dataDir, "logs", "gorag.log")
	data, err := os.ReadFile(logFile)
	if err != nil {
		return "", fmt.Errorf("读取日志失败: %w", err)
	}
	return string(data), nil
}

// Update 对已索引的文件分片执行增量 LLM 处理（摘要 + 实体提取）。
//
// 流程：
//  1. 从 meta.db 查询所有需要 LLM 处理的分片（summarized=false 或 refilled=false）
//  2. 按 DocID 分组，从 VectorStore 加载分片完整数据
//  3. 内容变更阈值检查：已处理过的分片再次发生内容变更但长度变化低于阈值时跳过
//  4. 调用 HyperIndexer.ProcessChunks 执行 LLM 处理
//  5. 更新 chunk_llm_status（标记 summarized/refilled=true）
//
// path 参数当前暂未使用（处理所有已索引文件的分片）。
func (s *IndexingService) Update(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// ── 第一阶段：重新索引变更的文件 ──
	// 检测文件系统变更（mtime+size → content_hash），对变更文件重新分块 + 向量化。
	// 同时自动更新 chunk_llm_status（新分片标记为 summarized=false, refilled=false）。
	s.logger.Info("Update: 第一阶段 - 重新索引变更的文件", "target", path)
	if err := s.reindexChangedFiles(ctx, path); err != nil {
		s.logger.Warn("Update: 重新索引阶段有部分错误", "error", err)
	}

	// ── 第二阶段：增量 LLM 处理（摘要 + 实体提取） ──
	// 从 chunk_llm_status 查询需要 LLM 的分片并处理。
	// LLM 不可用时（索引器不是 HyperIndexer）降级退出，仅完成第一阶段的重新索引。
	hyper, ok := s.indexer.(*indexer.HyperIndexer)
	if !ok {
		s.logger.Info("Update: 索引器不是 HyperIndexer，跳过 LLM 增强（仅完成重新索引）")
		return nil
	}

	admin, ok := s.indexer.(indexer.IndexerAdmin)
	if !ok {
		return fmt.Errorf("Update: 索引器不支持管理接口")
	}

	// 查询所有需要 LLM 处理的分片
	needsLLM, err := s.metaStore.GetChunksNeedingLLM("", false, false, -1)
	if err != nil {
		return fmt.Errorf("Update: 查询需要 LLM 处理的分片失败: %w", err)
	}

	if len(needsLLM) == 0 {
		s.logger.Info("Update: 所有分片已完成 LLM 处理，无需更新")
		return nil
	}

	s.logger.Info("Update: 第二阶段 - 增量 LLM 处理", "待处理分片", len(needsLLM))

	// 按 DocID 分组（避免重复加载同一文档的 chunks）
	byDocID := make(map[string][]*meta.ChunkLLMStatus)
	for _, st := range needsLLM {
		byDocID[st.DocID] = append(byDocID[st.DocID], st)
	}

	var totalProcessed int

	for docID, statuses := range byDocID {
		// 加载该文档的所有分片
		allChunks, err := admin.GetChunks(ctx, docID)
		if err != nil {
			s.logger.Warn("Update: 加载文档分片失败", "doc_id", docID, "error", err)
			continue
		}

		// 按 chunkID 建立索引
		chunkByID := make(map[string]*core.Chunk, len(allChunks))
		for _, c := range allChunks {
			chunkByID[c.ID] = c
		}

		// 筛选需要处理的分片 + 内容变更阈值检查
		var toProcess []core.Chunk
		for _, st := range statuses {
			chunk, ok := chunkByID[st.ChunkID]
			if !ok {
				s.logger.Debug("Update: 分片不在 VectorStore 中，跳过", "chunk_id", st.ChunkID)
				continue
			}

			// 已处理过的分片：检查是否需要重新处理
			if st.Summarized && st.Refilled {
				currentHash := computeChunkContentHash(chunk.Content)
				if currentHash == st.ContentHash {
					continue // 内容无变更
				}
				diff := abs(len(chunk.Content) - st.ContentLength)
				if diff < minContentChangeForLLM {
					s.logger.Debug("Update: 分片内容微小变更，跳过 LLM",
						"chunk_id", st.ChunkID, "长度变化", diff)
					continue
				}
				s.logger.Info("Update: 分片内容实质性变更，重新 LLM 处理",
					"chunk_id", st.ChunkID, "长度变化", diff)
			}

			toProcess = append(toProcess, *chunk)
		}

		if len(toProcess) == 0 {
			continue
		}

		// 调用 ProcessChunks
		s.logger.Info("Update: 处理文档分片", "doc_id", docID, "分片数", len(toProcess))
		processed, _, _, pErr := hyper.ProcessChunks(ctx, toProcess)
		if pErr != nil {
			s.logger.Warn("Update: ProcessChunks 返回错误", "error", pErr)
		}

		// 记录已处理的分片 ID 集合
		processedIDs := make(map[string]bool, len(toProcess))
		for _, c := range toProcess {
			processedIDs[c.ID] = true
		}

		// 更新 chunk_llm_status
		now := time.Now()
		for _, pc := range processed {
			if !processedIDs[pc.ID] {
				continue
			}
			st := findStatus(statuses, pc.ID)
			if st == nil {
				continue
			}

			update := &meta.ChunkLLMStatus{
				ChunkID:       pc.ID,
				DocPath:       st.DocPath,
				DocID:         st.DocID,
				ContentHash:   computeChunkContentHash(pc.Content),
				ContentLength: len(pc.Content),
				Summarized:    true,
				Refilled:      true,
			}
			if !st.Summarized {
				update.LastSummarizedAt = &now
			} else {
				update.LastSummarizedAt = st.LastSummarizedAt
			}
			if !st.Refilled {
				update.LastRefilledAt = &now
			} else {
				update.LastRefilledAt = st.LastRefilledAt
			}

			if err := s.metaStore.SaveChunkLLMStatus(update); err != nil {
				s.logger.Warn("Update: 保存分片状态失败", "chunk_id", pc.ID, "error", err)
			}
		}
		totalProcessed += len(toProcess)
	}

	s.logger.Info("Update: 完成", "已重新索引的文件 + LLM 增强分片", totalProcessed)
	return nil
}

// findStatus 在 ChunkLLMStatus 切片中按 chunkID 查找。
func findStatus(statuses []*meta.ChunkLLMStatus, chunkID string) *meta.ChunkLLMStatus {
	for _, st := range statuses {
		if st.ChunkID == chunkID {
			return st
		}
	}
	return nil
}

// ── 目录树 ────────────────────────────────────────────────────────────

// SourceTreeNode 目录树节点，对应一个文件或目录。
type SourceTreeNode struct {
	Name     string            // 文件/目录名
	Path     string            // 完整路径
	Size     int64             // 文件内容总大小（所有 Chunk Content 长度之和）
	IsDir    bool              // 是否为目录
	Summary  string            // 目录摘要（来自 README.md）
	Chunks   []SourceChunkNode // 该文件下的顶层 Chunk（ParentID==""）
	Children []*SourceTreeNode // 子目录
}

// SourceChunkNode Chunk 树节点。
type SourceChunkNode struct {
	Type      string            // 数据|文档|图片|代码
	Title     string            // Chunk 标题
	Summary   string            // Chunk 摘要
	StartLine int               // 在源文件中的起始行号
	EndLine   int               // 在源文件中的结束行号
	Children  []SourceChunkNode // 通过 ParentID 连结的子块
}

// Tree 基于所有 Chunk 的 Source 属性重建文件目录树。
//
// 获取全部已索引的 Chunk，按 Source 分组后重建目录层级。
// 每个文件节点下挂载该文件的顶层 Chunk（ParentID=""），
// 再通过 ParentID 连结子块形成块子树。
func (s *IndexingService) Tree(ctx context.Context) (*SourceTreeNode, error) {
	admin, ok := s.indexer.(indexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("索引器不支持列表查询")
	}

	total, err := admin.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 Chunk 总数失败: %w", err)
	}

	if total == 0 {
		return &SourceTreeNode{Name: ".", IsDir: true}, nil
	}

	allChunks, _, err := admin.List(ctx, 0, total, nil)
	if err != nil {
		return nil, fmt.Errorf("获取 Chunk 列表失败: %w", err)
	}

	// 按 ChunkID 去重（List 可能返回多个维度的向量对应的 Chunk）
	seen := make(map[string]bool, len(allChunks))
	deduped := make([]core.Chunk, 0, len(allChunks))
	for _, c := range allChunks {
		if !seen[c.ID] {
			seen[c.ID] = true
			deduped = append(deduped, c)
		}
	}

	if len(deduped) == 0 {
		return &SourceTreeNode{Name: ".", IsDir: true}, nil
	}

	// 按 Source 分组
	sourceChunks := make(map[string][]core.Chunk) // source -> 顶层 Chunk（ParentID==""）
	allBySource := make(map[string][]core.Chunk)  // source -> 全部 Chunk
	for _, chunk := range deduped {
		src := chunk.Source
		if src == "" {
			continue
		}
		allBySource[src] = append(allBySource[src], chunk)
		if chunk.ParentID == "" {
			sourceChunks[src] = append(sourceChunks[src], chunk)
		}
	}

	// 构建目录树
	root := &SourceTreeNode{Name: ".", IsDir: true}
	for source, topChunks := range sourceChunks {
		allForSource := allBySource[source]

		var fileSize int64
		for _, c := range allForSource {
			fileSize += int64(len(c.Content))
		}

		fileNode := &SourceTreeNode{
			Name:   filepath.Base(source),
			Path:   source,
			Size:   fileSize,
			Chunks: buildChunkTree(topChunks, allBySource[source]),
		}

		insertIntoTree(root, source, fileNode)
	}

	// 将 README.md 文件节点的摘要折叠到父目录节点
	foldReadmeIntoDirectories(root)

	return root, nil
}

// buildChunkTree 为单个文件构建 Chunk 子树。
func buildChunkTree(parentChunks []core.Chunk, allChunks []core.Chunk) []SourceChunkNode {
	childMap := make(map[string][]core.Chunk)
	for _, c := range allChunks {
		if c.ParentID != "" {
			childMap[c.ParentID] = append(childMap[c.ParentID], c)
		}
	}

	nodes := make([]SourceChunkNode, 0, len(parentChunks))
	for _, pc := range parentChunks {
		nodes = append(nodes, chunkToNode(pc, childMap))
	}
	return nodes
}

// chunkToNode 递归构建单个 Chunk 节点及其子块。
func chunkToNode(chunk core.Chunk, childMap map[string][]core.Chunk) SourceChunkNode {
	node := SourceChunkNode{
		Type:      chunkTypeFromSource(chunk.Source, chunk.Language),
		Title:     chunk.Title,
		Summary:   chunk.Summary,
		StartLine: chunk.StartLine,
		EndLine:   chunk.EndLine,
	}
	for _, child := range childMap[chunk.ID] {
		node.Children = append(node.Children, chunkToNode(child, childMap))
	}
	return node
}

// chunkTypeFromSource 根据 Source 路径和 Language 判断 Chunk 显示类型。
func chunkTypeFromSource(source, language string) string {
	if language != "" {
		return "代码"
	}
	ext := strings.ToLower(filepath.Ext(source))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".tif":
		return "图片"
	case ".csv", ".xlsx", ".json", ".yaml", ".yml", ".xml", ".toml", ".eml", ".msg", ".log":
		return "数据"
	default:
		return "文档"
	}
}

// insertIntoTree 将文件节点按 Source 路径插入到目录树中。
func insertIntoTree(root *SourceTreeNode, source string, fileNode *SourceTreeNode) {
	dir := filepath.Dir(source)
	if dir == "." || dir == "/" || dir == "" {
		root.Children = append(root.Children, fileNode)
		return
	}

	parts := strings.Split(dir, string(filepath.Separator))
	current := root
	for _, part := range parts {
		if part == "" {
			continue
		}
		found := false
		for _, child := range current.Children {
			if child.IsDir && child.Name == part {
				current = child
				found = true
				break
			}
		}
		if !found {
			dirNode := &SourceTreeNode{Name: part, IsDir: true}
			current.Children = append(current.Children, dirNode)
			current = dirNode
		}
	}
	current.Children = append(current.Children, fileNode)
}

// foldReadmeIntoDirectories 递归将 README.md 文件节点的摘要折叠到父目录节点，
// 并移除 README.md 文件节点，使其不在 tree 中单独显示。
func foldReadmeIntoDirectories(node *SourceTreeNode) {
	if node == nil {
		return
	}

	var filtered []*SourceTreeNode
	for _, child := range node.Children {
		if child.IsDir {
			foldReadmeIntoDirectories(child)
			filtered = append(filtered, child)
			continue
		}
		if strings.EqualFold(child.Name, "README.md") {
			node.Summary = collectReadmeSummary(child)
			continue
		}
		filtered = append(filtered, child)
	}
	node.Children = filtered
}

// collectReadmeSummary 收集 README.md 文件节点下顶层 Chunk 的摘要，
// 去重后拼接并截断到 200 字符。
func collectReadmeSummary(readmeNode *SourceTreeNode) string {
	if readmeNode == nil || len(readmeNode.Chunks) == 0 {
		return ""
	}

	var summaries []string
	for _, chunk := range readmeNode.Chunks {
		if chunk.Summary == "" {
			continue
		}
		if !contains(summaries, chunk.Summary) {
			summaries = append(summaries, chunk.Summary)
		}
	}

	if len(summaries) == 0 {
		return ""
	}

	summary := strings.Join(summaries, "；")
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	return summary
}

// collectIndexedDirs 从本次索引的文件路径中提取所有触及的目录，
// 并向上回收到 targetPath 根目录，去重后按深度从深到浅排序。
func collectIndexedDirs(files []string, targetPath string) []string {
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil
	}
	rootDir := targetPath
	if !info.IsDir() {
		rootDir = filepath.Dir(targetPath)
	}
	rootDir = filepath.Clean(rootDir)

	seen := make(map[string]bool)
	var dirs []string
	for _, f := range files {
		dir := filepath.Dir(f)
		for dir != "" && dir != "/" && dir != "." {
			dir = filepath.Clean(dir)
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
			if dir == rootDir {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 按目录深度从深到浅排序
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if strings.Count(dirs[i], string(filepath.Separator)) < strings.Count(dirs[j], string(filepath.Separator)) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}
	return dirs
}

// generateRegionReadme 为指定目录生成摘要式 README.md 并索引。
// 若目录下已存在 README.md 或没有可摘要内容，则生成默认摘要。
func (s *IndexingService) generateRegionReadme(ctx context.Context, dir string) error {
	readmePath := filepath.Join(dir, "README.md")
	if fileExists(readmePath) {
		return nil
	}

	admin, ok := s.indexer.(indexer.IndexerAdmin)
	if !ok {
		return fmt.Errorf("索引器不支持列表查询")
	}

	total, err := admin.Count(ctx)
	if err != nil {
		return fmt.Errorf("获取 Chunk 总数失败: %w", err)
	}
	if total == 0 {
		return nil
	}

	allChunks, _, err := admin.List(ctx, 0, total, nil)
	if err != nil {
		return fmt.Errorf("获取 Chunk 列表失败: %w", err)
	}

	// 去重并筛选当前目录下的顶层 Chunk
	seen := make(map[string]bool)
	var summaries []string
	prefix := dir + string(filepath.Separator)
	for _, c := range allChunks {
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		if c.ParentID != "" {
			continue
		}
		if !strings.HasPrefix(c.Source, prefix) {
			continue
		}
		if c.Summary == "" {
			continue
		}
		if !contains(summaries, c.Summary) {
			summaries = append(summaries, c.Summary)
		}
	}

	regionName := filepath.Base(dir)
	var content string
	if len(summaries) > 0 {
		var b strings.Builder
		b.WriteString("# ")
		b.WriteString(regionName)
		b.WriteString("\n\n")
		b.WriteString(core.RegionDescriptorMarker)
		b.WriteString("\n\n")
		for _, summary := range summaries {
			b.WriteString("- ")
			b.WriteString(summary)
			b.WriteString("\n")
		}
		content = b.String()
	} else {
		content = fmt.Sprintf("# %s\n\n%s\n\n_该目录暂无摘要。_\n", regionName, core.RegionDescriptorMarker)
	}

	if err := os.WriteFile(readmePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("写入 README.md 失败: %w", err)
	}

	if _, err := s.processFile(ctx, readmePath); err != nil {
		return fmt.Errorf("索引生成的 README.md 失败: %w", err)
	}

	s.logger.Info("目录 README 生成完成", "dir", dir)
	return nil
}
