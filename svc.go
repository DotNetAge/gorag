package gorag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/store/meta"
)

// IndexingService 作为能力聚合根，对外暴露统一入口。
//
// 具体的领域能力由子服务实现：
//   - IndexerService: 索引与增量更新
//   - QueryService:   语义查询、多关键字查询、Chunk 列表
//   - GraphService:   图探索、Cypher 查询
//   - AdminService:   库信息、诊断、日志、目录树
//   - RegionService:  Region README 自动生成
//   - LLMService:     LLM 组件注入与使用记录
type IndexingService struct {
	dataDir   string          // 索引数据目录
	metaStore meta.Store      // 元数据存储
	indexer   indexer.Indexer // 索引器实例
	logger    logging.Logger  // 日志记录器

	mu sync.RWMutex // 保护内部状态

	// 子服务懒加载
	indexerSvc *IndexerService
	querySvc   *QueryService
	graphSvc   *GraphService
	adminSvc   *AdminService
	regionSvc  *RegionService
	llmSvc     *LLMService
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
			// 日志器创建失败前，先关闭已创建的索引器与元数据存储，避免资源泄漏
			closeServiceResources(svc)
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}
		logFile := filepath.Join(logDir, "gorag.log")
		logger, err := logging.DefaultFileLogger(logFile)
		if err != nil {
			// 日志器创建失败前，先关闭已创建的索引器与元数据存储，避免资源泄漏
			closeServiceResources(svc)
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

// closeServiceResources 关闭索引器与元数据存储，用于构造函数失败时的资源清理。
func closeServiceResources(svc *IndexingService) {
	if svc.indexer != nil {
		if closer, ok := svc.indexer.(indexer.IndexerCloser); ok {
			_ = closer.Close(context.Background())
		}
	}
	if svc.metaStore != nil {
		_ = svc.metaStore.Close()
	}
}

// Stop 停止服务，关闭所有资源。
// 返回关闭过程中收集到的所有错误。
func (s *IndexingService) Stop() error {
	var errs []error

	// 关闭索引器
	if closer, ok := s.indexer.(indexer.IndexerCloser); ok {
		if err := closer.Close(context.Background()); err != nil {
			s.logger.Error("关闭索引器失败", err)
			errs = append(errs, fmt.Errorf("关闭索引器失败: %w", err))
		}
	}

	// 关闭元数据存储
	if s.metaStore != nil {
		if err := s.metaStore.Close(); err != nil {
			s.logger.Error("关闭元数据存储失败", err)
			errs = append(errs, fmt.Errorf("关闭元数据存储失败: %w", err))
		}
	}

	return errors.Join(errs...)
}

// IndexerSvc 返回索引服务。
func (s *IndexingService) IndexerSvc() *IndexerService {
	if s.indexerSvc == nil {
		s.indexerSvc = &IndexerService{svc: s}
	}
	return s.indexerSvc
}

// Querier 返回查询服务。
func (s *IndexingService) Querier() *QueryService {
	if s.querySvc == nil {
		s.querySvc = &QueryService{svc: s}
	}
	return s.querySvc
}

// Explorer 返回图探索服务。
func (s *IndexingService) Explorer() *GraphService {
	if s.graphSvc == nil {
		s.graphSvc = &GraphService{svc: s}
	}
	return s.graphSvc
}

// Admin 返回管理服务。
func (s *IndexingService) Admin() *AdminService {
	if s.adminSvc == nil {
		s.adminSvc = &AdminService{svc: s}
	}
	return s.adminSvc
}

// Region 返回 Region 服务。
func (s *IndexingService) Region() *RegionService {
	if s.regionSvc == nil {
		s.regionSvc = &RegionService{svc: s}
	}
	return s.regionSvc
}

// LLM 返回 LLM 服务。
func (s *IndexingService) LLM() *LLMService {
	if s.llmSvc == nil {
		s.llmSvc = &LLMService{svc: s}
	}
	return s.llmSvc
}
