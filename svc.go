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

	"github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/indexer"
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

// Index 对指定路径执行批量索引。
// targetPath 可以是文件或目录。
func (s *IndexingService) Index(ctx context.Context, targetPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 扫描文件
	var files []string
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("无法访问目标路径: %w", err)
	}
	if info.IsDir() {
		files, err = scanDir(targetPath)
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

	s.logger.Info("开始批量索引", "target", targetPath, "files", len(files))

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

	s.logger.Info("批量索引完成",
		"total", len(files),
		"failed", failedCount,
		"success", len(files)-failedCount)

	return nil
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

// scanDir 扫描目录下的所有文本文件。
func scanDir(dir string) ([]string, error) {
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
			return nil
		}
		if isTextFile(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
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

	return s.metaStore.DeleteDocument(absPath)
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

// Query 执行搜索查询。
func (s *IndexingService) Query(ctx context.Context, text string) (*core.Hit, error) {
	if text == "" {
		return nil, fmt.Errorf("查询内容不能为空")
	}
	return s.indexer.Search(ctx, s.indexer.NewQuery(text))
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

// Update 执行跨文件实体发现与关系线重建。
// 对已索引的文档集合执行全量实体关系提取，
// 将发现的新实体和关系追加到 GraphStore。
// stub — 待实现。
func (s *IndexingService) Update(ctx context.Context, path string) error {
	return fmt.Errorf("Update: 暂未实现")
}

// ── 目录树 ────────────────────────────────────────────────────────────

// SourceTreeNode 目录树节点，对应一个文件或目录。
type SourceTreeNode struct {
	Name     string            // 文件/目录名
	Path     string            // 完整路径
	Size     int64             // 文件内容总大小（所有 Chunk Content 长度之和）
	IsDir    bool              // 是否为目录
	Chunks   []SourceChunkNode // 该文件下的顶层 Chunk（ParentID==""）
	Children []*SourceTreeNode // 子目录
}

// SourceChunkNode Chunk 树节点。
type SourceChunkNode struct {
	Type     string            // 数据|文档|图片|代码
	Title    string            // Chunk 标题
	Summary  string            // Chunk 摘要
	Children []SourceChunkNode // 通过 ParentID 连结的子块
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
		Type:    chunkTypeFromSource(chunk.Source, chunk.Language),
		Title:   chunk.Title,
		Summary: chunk.Summary,
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
