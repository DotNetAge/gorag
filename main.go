package gorag

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedder"
	"github.com/DotNetAge/gorag/indexer"
	"github.com/DotNetAge/gorag/logging"
	"github.com/DotNetAge/gorag/store/doc/bleve"
	"github.com/DotNetAge/gorag/store/graph/gograph"
	"github.com/DotNetAge/gorag/store/vector/govector"
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
	Name      string // RAG 名称
	Type      string // 索引器类型：hybrid, semantic, graph
	ModelFile string // 向量化模型
}

func New(dataDir string) core.Indexer {
	// TODO: 如果 Data目录存在就要检查该目录下的config.yml文件
	// name, type: hybrid, se, graph

	return nil
}

func Open(dataDir string) *HybridIndexer {
	return indexer.NewHybridIndexer()
}

func createHybridIndexer(dataDir string) (*HybridIndexer, error) {

	name := filepath.Dir(dataDir)

	baseDir := "./embeddings"
	modelDir := "Xenova/chinese-clip-vit-base-patch16"
	modelFile := "onnx/model.onnx"

	onnxFile := filepath.Join(baseDir, modelDir, modelFile)
	if _, err := os.Stat(onnxFile); os.IsNotExist(err) {
		// TODO: 使用 utils.HuggingFaceDownloader 下载模型文件
		// downloader := utils.NewModelDownloader(baseDir)
		// if err := downloader.Download(modelDir, modelFile); err != nil {
		// 	slog.Error("Failed to download model", "error", err)
		// 	return nil, fmt.Errorf("failed to download model: %w", err)
		// }
	}

	clip, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(onnxFile))

	vectorDbFile := filepath.Join(dataDir, "vectors", name+".db")
	bleveDBFile := filepath.Join(dataDir, "fulltexts", name+".bleve")
	graphDbFile := filepath.Join(dataDir, "graphs", name+".db")

	vectorStore, err := govector.NewStore(
		govector.WithCollection(name),
		govector.WithDimension(clip.Dim()),
		govector.WithDBPath(vectorDbFile),
		govector.WithHNSW(true),
	)

	if err != nil {
		slog.Error("Failed to init vector store", "error", err)
		return nil, fmt.Errorf("failed to init vector store: %w", err)
	}

	graphStore, err := gograph.NewGraphStore(graphDbFile)
	if err != nil {
		slog.Error("Failed to init graph store", "error", err)
		return nil, err
	}

	fullTextStore, err := bleve.NewBleveStore(bleveDBFile)
	if err != nil {
		slog.Error("Failed to init fulltext store", "error", err)
		return nil, err
	}

	indexer, err := NewHybridIndexer(vectorStore, graphStore, fullTextStore, clip)
	if err != nil {
		slog.Error("Failed to init indexer", "error", err)
		return nil, err
	}
	return indexer, nil
}

// TODO: 如何能让开发人员同时实例化多个RAG实例，每个RAG实例应该对应一个独立的数据文件夹
// TODO: 语义化分块器是应该使用全模态还是单模态？
// TODO: 索引时貌似没有将RawDocument.Images加入索引
// TODO: 提供输出格式化器 Formatter
