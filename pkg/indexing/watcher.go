package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/fsnotify/fsnotify"
)

// WatchConfig 文件监控配置
type WatchConfig struct {
	Path             string        // 监控目录
	Recursive        bool          // 是否递归监控子目录
	Patterns         []string      // 文件匹配模式（例如：[]string{"*.pdf", "*.md"}）
	Exclude          []string      // 排除的文件模式
	DebounceInterval time.Duration // 防抖间隔，默认 500ms
}

// FileWatcher 文件监控器
type FileWatcher struct {
	mu           sync.RWMutex
	watcher      *fsnotify.Watcher
	configs      []WatchConfig
	indexer      Indexer
	isRunning    bool
	cancel       context.CancelFunc
	ctx          context.Context
	indexedFiles map[string]time.Time // 已索引文件的最后修改时间
	logger       logging.Logger
}

// NewFileWatcher 创建文件监控器
func NewFileWatcher(indexer Indexer, logger logging.Logger) (*FileWatcher, error) {
	fw := &FileWatcher{
		indexer:      indexer,
		indexedFiles: make(map[string]time.Time),
		logger:       logger,
	}

	var err error
	fw.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return fw, nil
}

// AddConfigs 添加多个监控配置
func (fw *FileWatcher) AddConfigs(configs ...WatchConfig) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.configs = append(fw.configs, configs...)
}

// Start 启动文件监控（阻塞式）
func (fw *FileWatcher) Start() error {
	fw.mu.Lock()
	if fw.isRunning {
		fw.mu.Unlock()
		return fmt.Errorf("watcher is already running")
	}
	fw.isRunning = true
	fw.ctx, fw.cancel = context.WithCancel(context.Background())
	fw.mu.Unlock()

	// 先添加所有监控目录
	for _, config := range fw.configs {
		if err := fw.addWatchPath(config); err != nil {
			fw.logger.Error("Failed to add watch path", err, map[string]interface{}{
				"path": config.Path,
			})
			return err
		}
	}

	fw.logger.Info("File watcher started", map[string]interface{}{
		"watch_count": len(fw.configs),
	})

	// 处理事件
	return fw.handleEvents()
}

// Stop 停止文件监控
func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if !fw.isRunning {
		return nil
	}

	fw.isRunning = false
	if fw.cancel != nil {
		fw.cancel()
	}

	return fw.watcher.Close()
}

// addWatchPath 添加监控路径
func (fw *FileWatcher) addWatchPath(config WatchConfig) error {
	// 检查目录是否存在
	info, err := os.Stat(config.Path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", config.Path)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", config.Path)
	}

	// 添加监控
	err = fw.watcher.Add(config.Path)
	if err != nil {
		return fmt.Errorf("failed to add watch: %w", err)
	}

	fw.logger.Info("Added watch path", map[string]interface{}{
		"path":      config.Path,
		"recursive": config.Recursive,
	})

	// 如果是递归监控，添加所有子目录
	if config.Recursive {
		err = filepath.Walk(config.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && path != config.Path {
				return fw.watcher.Add(path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk directory: %w", err)
		}
	}

	return nil
}

// handleEvents 处理文件事件
func (fw *FileWatcher) handleEvents() error {
	// 防抖定时器
	var debounceTimer *time.Timer
	var debounceMu sync.Mutex
	eventsToProcess := make([]fsnotify.Event, 0)

	for {
		select {
		case <-fw.ctx.Done():
			return nil

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return nil
			}

			// 检查是否需要处理该事件
			if !fw.shouldProcessEvent(event) {
				continue
			}

			// 防抖处理
			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			eventsToProcess = append(eventsToProcess, event)

			interval := 500 * time.Millisecond
			for _, config := range fw.configs {
				if config.DebounceInterval > 0 {
					interval = config.DebounceInterval
					break
				}
			}

			debounceTimer = time.AfterFunc(interval, func() {
				debounceMu.Lock()
				eventsCopy := make([]fsnotify.Event, len(eventsToProcess))
				copy(eventsCopy, eventsToProcess)
				eventsToProcess = eventsToProcess[:0]
				debounceMu.Unlock()

				// 处理累积的事件
				for _, e := range eventsCopy {
					fw.processEvent(e)
				}
			})
			debounceMu.Unlock()

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return nil
			}
			fw.logger.Error("Watcher error", err, nil)
		}
	}
}

// shouldProcessEvent 检查是否应该处理该事件
func (fw *FileWatcher) shouldProcessEvent(event fsnotify.Event) bool {
	// 只处理文件事件
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}

	// 跳过目录事件
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		return false
	}

	// 检查文件模式
	for _, config := range fw.configs {
		if len(config.Patterns) > 0 {
			matched := false
			for _, pattern := range config.Patterns {
				if matched, _ := filepath.Match(pattern, filepath.Base(event.Name)); matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 检查排除模式
		for _, exclude := range config.Exclude {
			if matched, _ := filepath.Match(exclude, filepath.Base(event.Name)); matched {
				return false
			}
		}

		// 检查是否在监控路径下
		eventPath := event.Name
		watchPath := config.Path

		// 规范化路径
		eventPath = filepath.Clean(eventPath)
		watchPath = filepath.Clean(watchPath)

		// 精确匹配或以 watchPath 为前缀（后跟路径分隔符）
		if eventPath == watchPath || strings.HasPrefix(eventPath, watchPath+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// processEvent 处理单个文件事件
func (fw *FileWatcher) processEvent(event fsnotify.Event) {
	fw.logger.Debug("Processing file event", map[string]interface{}{
		"name": event.Name,
		"op":   event.Op.String(),
	})

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write:
		// 文件或目录被创建或修改
		fw.handleCreateOrModify(event.Name)
	case event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename:
		// 文件或目录被删除或重命名
		fw.handleRemoveOrRename(event.Name)
	}
}

// handleCreateOrModify 处理创建或修改事件
func (fw *FileWatcher) handleCreateOrModify(filePath string) {
	// 获取文件信息
	info, err := os.Stat(filePath)
	if err != nil {
		fw.logger.Error("Failed to stat file", err, map[string]interface{}{
			"path": filePath,
		})
		return
	}

	// 检查是否是新增或更新
	lastModTime, exists := fw.indexedFiles[filePath]
	if exists && !info.ModTime().After(lastModTime) {
		fw.logger.Debug("Skipping unchanged file", map[string]interface{}{
			"path": filePath,
		})
		return
	}

	// ✅ 索引文件（异步执行）
	go func() {
		fw.logger.Info("Indexing new/modified file", map[string]interface{}{
			"path": filePath,
		})

		// 初始化索引器
		if indexerWithInit, ok := fw.indexer.(interface{ Init() error }); ok {
			if err := indexerWithInit.Init(); err != nil {
				fw.logger.Error("Failed to init indexer", err, nil)
				return
			}
		}

		// ✅ 触发完整索引流程
		if err := fw.indexer.IndexFile(fw.ctx, filePath); err != nil {
			fw.logger.Error("Failed to index file", err, map[string]interface{}{
				"path": filePath,
			})
			return
		}

		fw.indexedFiles[filePath] = info.ModTime()
		fw.logger.Info("File indexed successfully", map[string]interface{}{
			"path": filePath,
		})
	}()
}

// handleRemoveOrRename 处理删除或重命名事件
func (fw *FileWatcher) handleRemoveOrRename(filePath string) {
	// 从已索引列表中移除
	delete(fw.indexedFiles, filePath)

	fw.logger.Info("File removed from index", map[string]interface{}{
		"path": filePath,
	})
}
