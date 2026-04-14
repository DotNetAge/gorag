package gorag

import (
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/indexer"
	"github.com/DotNetAge/gorag/logging"
)

// GoRAG 的服务应用入口
type IndexingService struct {
	dataDir string   // 索引数据目录
	watchs  []string // 监控的文件目录
	indexer *indexer.HybridIndexer
	logger  logging.Logger
}

func Start(dataDir string) *IndexingService {
	return &IndexingService{
		dataDir: dataDir,
		watchs:  []string{},
	}
}

func (i *IndexingService) Watch() {
	// 监控指定文件目录，
	// TODO: 当文件发生变更时就进行自动索引；
	// TODO: 首次启动时进行全量索引
}

type Config struct {
	Name string
}

func New(dataDir string) core.Indexer {
	// TODO: 如果 Data目录存在就要检查该目录下的config.yml文件
	// name, type: hybrid, se, graph
	return nil
}

func Open(dataDir string) *indexer.HybridIndexer {

	return indexer.NewHybridIndexer()
}

// TODO: 如何能让开发人员同时实例化多个RAG实例，每个RAG实例应该对应一个独立的数据文件夹
// TODO: 语义化分块器是应该使用全模态还是单模态？
// TODO: 索引时貌似没有将RawDocument.Images加入索引
// TODO: 查询时要对查询进行语义化（向量化) - 要使用Query对象进行查询
// TODO: 提供输出格式化器 Formatter
