package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/chunker/semantic"
	"github.com/DotNetAge/gorag/infra/indexing"
	"github.com/DotNetAge/gorag/infra/parser/config/types"
	"github.com/DotNetAge/gorag/infra/steps"
	"github.com/DotNetAge/gorag/infra/vectorstore"
	"github.com/DotNetAge/gorag/pkg/di"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
)

// IndexerOption Indexer 配置选项
type IndexerOption func(*defaultIndexer)

// defaultIndexer 默认索引器实现
type defaultIndexer struct {
	mu sync.RWMutex

	// 核心组件
	parsers     []dataprep.Parser // 多个解析器（实际使用）
	chunker     dataprep.SemanticChunker
	embedder    embedding.Provider
	vectorStore abstraction.VectorStore
	graphStore  abstraction.GraphStore
	extractor   dataprep.GraphExtractor

	// 可选组件
	metrics   abstraction.Metrics
	logger    logging.Logger
	container *di.Container

	// 配置
	watchDirs []string
	batchSize int

	// 运行时状态
	indexedFiles map[string]time.Time // 已索引的文件及其时间
	isRunning    bool                 // 是否正在运行监控
}

// WithAllParsers 加载所有内置解析器 (20+)
func WithAllParsers() IndexerOption {
	return func(idx *defaultIndexer) {
		idx.parsers = types.AllParsers()
	}
}

// WithParsers 指定多个解析器
func WithParsers(parsers ...dataprep.Parser) IndexerOption {
	return func(idx *defaultIndexer) {
		if len(parsers) > 0 {
			idx.parsers = parsers
		}
	}
}

// WithWatchDir 指定需要监控的目录
func WithWatchDir(dirs ...string) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.watchDirs = append(idx.watchDirs, dirs...)
	}
}

// WithStore 设置向量存储
func WithStore(store abstraction.VectorStore) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.vectorStore = store
	}
}

// WithGraph 设置图存储（启用 GraphRAG）
func WithGraph(store abstraction.GraphStore) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.graphStore = store
	}
}

// WithEmbedding 设置 Embedding Provider
func WithEmbedding(provider embedding.Provider) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.embedder = provider
	}
}

// WithLLM 设置 LLM 客户端（用于智能分块和图提取）
func WithLLM(client interface{}) IndexerOption {
	// 保留接口，未来实现 LLM 客户端适配
	return func(idx *defaultIndexer) {
		// 预留位置，等待 LLM 接口定义完成后实现
	}
}

// WithMetrics 设置指标收集器
func WithMetrics(metrics abstraction.Metrics) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.metrics = metrics
	}
}

// WithLogger 设置日志记录器
func WithLogger(logger logging.Logger) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.logger = logger
	}
}

// WithContainer 设置 DI 容器
func WithContainer(container *di.Container) IndexerOption {
	return func(idx *defaultIndexer) {
		idx.container = container
	}
}

// DefaultIndexer 创建默认索引器
func DefaultIndexer(opts ...IndexerOption) dataprep.Indexer {
	idx := &defaultIndexer{
		batchSize:    100,
		indexedFiles: make(map[string]time.Time),
	}

	// 应用所有选项
	for _, opt := range opts {
		opt(idx)
	}

	return idx
}

// Init 初始化索引器环境（完全基于 DI 容器）
func (idx *defaultIndexer) Init() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 确保有 DI 容器
	if idx.container == nil {
		idx.container = di.New()
	}

	var err error

	// 从容器解析 Logger
	idx.logger, err = di.ResolveTyped[logging.Logger](idx.container)
	if err != nil {
		// 容器中不存在，创建默认日志记录器并注册
		logPath := "./logs/rag.log"
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
		logger, _ := logging.NewDefaultLogger(logPath)
		idx.container.RegisterInstance((*logging.Logger)(nil), logger)
		idx.logger = logger
	}

	// 从容器解析 Embedding Provider
	idx.embedder, err = di.ResolveTyped[embedding.Provider](idx.container)
	if err != nil {
		// 容器中不存在，创建默认 embedding 并注册
		idx.logger.Info("Embedding provider not registered, using default BEG model")
		provider, err := embedding.WithBEG("bge-small-zh-v1.5", "")
		if err != nil {
			idx.logger.Error("Failed to create default embedding provider", err)
			return fmt.Errorf("failed to create default embedding provider: %w", err)
		}
		idx.container.RegisterInstance((*embedding.Provider)(nil), provider)
		idx.embedder = provider
	}

	// 从容器解析 VectorStore
	idx.vectorStore, err = di.ResolveTyped[abstraction.VectorStore](idx.container)
	if err != nil {
		// 容器中不存在，创建默认 vector store 并注册
		idx.logger.Info("Vector store not registered, using default govector")
		store, err := vectorstore.DefaultVectorStore("")
		if err != nil {
			idx.logger.Error("Failed to create default vector store", err)
			return fmt.Errorf("failed to create default vector store: %w", err)
		}
		idx.container.RegisterInstance((*abstraction.VectorStore)(nil), store)
		idx.vectorStore = store
	}

	// 从容器解析 Chunker
	idx.chunker, err = di.ResolveTyped[dataprep.SemanticChunker](idx.container)
	if err != nil {
		// 容器中不存在，创建默认 chunker 并注册（依赖 embedder）
		idx.logger.Info("Using default semantic chunker")
		chunker := semantic.DefaultSemanticChunker(idx.embedder)
		idx.container.RegisterInstance((*dataprep.SemanticChunker)(nil), chunker)
		idx.chunker = chunker
	}

	// 如果指定了 GraphStore，从容器解析 GraphExtractor
	if idx.graphStore != nil && idx.extractor == nil {
		idx.extractor, err = di.ResolveTyped[dataprep.GraphExtractor](idx.container)
		if err != nil {
			idx.logger.Info("Graph extractor not registered, skipping graph extraction")
			// 暂不创建默认 GraphExtractor，需要 LLM 客户端支持
		}
	}

	idx.logger.Info("DefaultIndexer initialized successfully with DI container")
	return nil
}

// Index 执行增量索引（只索引新增文件）
func (idx *defaultIndexer) Index() error {
	ctx := context.Background()

	for _, dir := range idx.watchDirs {
		if err := idx.indexDirectory(ctx, dir, true, false); err != nil {
			return err
		}
	}
	return nil
}

// IndexAll 索引所有监控目录（阻塞式，但内部并行处理每个文件）
func (idx *defaultIndexer) IndexAll() error {
	ctx := context.Background()

	// ✅ 并行索引所有监控目录
	for _, dir := range idx.watchDirs {
		if err := idx.indexDirectory(ctx, dir, true, true); err != nil {
			return err
		}
	}
	return nil
}

// Start 启动文件监控服务（自动索引变化的文件）
func (idx *defaultIndexer) Start() error {
	// 初始化索引器（从 DI 容器加载所有组件）
	if err := idx.Init(); err != nil {
		return err
	}

	idx.mu.Lock()
	idx.isRunning = true
	idx.mu.Unlock()

	// 创建文件监控器
	watcher, err := NewFileWatcher(idx, idx.logger)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// 添加监控配置
	var configs []WatchConfig
	for _, dir := range idx.watchDirs {
		configs = append(configs, WatchConfig{
			Path:             dir,
			Recursive:        true,
			Patterns:         nil,                        // 监控所有文件
			Exclude:          []string{"*.tmp", "*.swp"}, // 排除临时文件
			DebounceInterval: 500 * time.Millisecond,
		})
	}
	watcher.AddConfigs(configs...)

	idx.logger.Info("Starting file watcher service", map[string]interface{}{
		"watch_dirs": idx.watchDirs,
	})

	// ✅ 启动监控（阻塞）
	// 当文件发生变化时，watcher 会自动调用 indexer.IndexFile() 触发完整索引流程：
	// 1. watcher 检测到文件变化 → 2. 调用 IndexFile()
	// 3. 执行 Pipeline → 4. Parse → Chunk → Embed → Store
	return watcher.Start()
}

// IndexDirectory 索引目录（实现 dataprep.Indexer 接口）
func (idx *defaultIndexer) IndexDirectory(ctx context.Context, dirPath string, recursive bool) error {
	return idx.indexDirectory(ctx, dirPath, recursive, false)
}

// GetMetrics 获取当前索引器的指标数据
func (idx *defaultIndexer) GetMetrics() map[string]interface{} {
	if idx.metrics == nil {
		return nil
	}

	return idx.metrics.GetMetrics()
}

// IndexFile 索引单个文件（带指标收集）
func (idx *defaultIndexer) IndexFile(ctx context.Context, filePath string) error {
	startTime := time.Now()

	idx.logger.Info("Indexing file", map[string]interface{}{
		"path": filePath,
	})

	// 执行管线
	state := indexing.DefaultState(ctx, filePath)
	if err := idx.ExecutePipeline(ctx, filePath); err != nil {
		// ✅ 记录错误指标
		if idx.metrics != nil {
			idx.metrics.RecordParsingErrors(filePath, err)
		}
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// ✅ 记录成功指标
	duration := time.Since(startTime)
	if idx.metrics != nil {
		idx.metrics.RecordIndexingDuration(filePath, duration)

		// 记录 chunk 数量（用于统计 embedding 数量）
		if state.TotalChunks > 0 {
			idx.metrics.RecordEmbeddingCount(state.TotalChunks)
		}
	}

	idx.logger.Info("File indexed successfully", map[string]interface{}{
		"path":         filePath,
		"total_chunks": state.TotalChunks,
		"duration_ms":  duration.Milliseconds(),
	})

	return nil
}

// assemblyPipeline 装配标准的索引阶段管线
// 使用 gochat/pkg/pipeline 的泛型 Pipeline
func (idx *defaultIndexer) assemblyPipeline() *pipeline.Pipeline[*indexing.State] {
	// 创建泛型 Pipeline[*indexing.State]
	p := pipeline.New[*indexing.State]()

	// 添加标准 Step（使用 infra/steps 中的通用组件，并传递 metrics）
	p.AddSteps(
		steps.NewFileDiscoveryStep(),
		steps.NewParseStep(idx.parsers...), // ✅ 传入所有解析器，自动根据文件类型选择
		steps.NewChunkStep(idx.chunker),
		steps.NewEmbedStep(idx.embedder, idx.metrics),    // ✅ 传递 metrics
		steps.NewStoreStep(idx.vectorStore, idx.metrics), // ✅ 传递 metrics
	)

	return p
}

// ExecutePipeline 执行管线
func (idx *defaultIndexer) ExecutePipeline(ctx context.Context, filePath string) error {
	p := idx.assemblyPipeline()
	state := indexing.DefaultState(ctx, filePath)
	return p.Execute(ctx, state)
}

// indexDirectory 索引目录的内部实现（并行处理文件）
func (idx *defaultIndexer) indexDirectory(ctx context.Context, dir string, recursive, force bool) error {
	idx.logger.Info("Indexing directory", map[string]interface{}{
		"dir":       dir,
		"recursive": recursive,
		"force":     force,
	})

	// 收集所有需要索引的文件
	var filesToIndex []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 检查是否需要索引（增量模式）
		modTime := info.ModTime()
		if lastIndexed, exists := idx.indexedFiles[path]; exists && !force {
			if modTime.Before(lastIndexed) || modTime.Equal(lastIndexed) {
				idx.logger.Debug("Skipping already indexed file", map[string]interface{}{
					"path": path,
				})
				return nil
			}
		}

		filesToIndex = append(filesToIndex, path)
		return nil
	})

	if err != nil {
		return err
	}

	idx.logger.Info("Found files to index", map[string]interface{}{
		"count": len(filesToIndex),
	})

	// ✅ 并行索引所有文件（使用 worker pool 模式）
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // 最多 10 个并发
	errorChan := make(chan error, len(filesToIndex))

	for _, filePath := range filesToIndex {
		wg.Add(1)
		go func(fp string) {
			defer wg.Done()

			// 获取信号量（控制并发数）
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 索引文件
			if err := idx.IndexFile(ctx, fp); err != nil {
				idx.logger.Error("Failed to index file", err, map[string]interface{}{
					"path": fp,
				})
				errorChan <- err
				return
			}

			idx.indexedFiles[fp] = time.Now()
			idx.logger.Debug("File indexed successfully", map[string]interface{}{
				"path": fp,
			})
		}(filePath)
	}

	// 等待所有任务完成
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// 收集错误
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("indexed %d files with %d errors: %v", len(filesToIndex), len(errors), errors[0])
	}

	idx.logger.Info("Directory indexing completed", map[string]interface{}{
		"total_files": len(filesToIndex),
		"success":     len(filesToIndex) - len(errors),
		"errors":      len(errors),
	})

	return nil
}
