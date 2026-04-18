package gorag

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gochat/client/openai"
	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedder"
	"github.com/DotNetAge/gorag/indexer"
	"github.com/DotNetAge/gorag/logging"
	"github.com/DotNetAge/gorag/store/doc/bleve"
	"github.com/DotNetAge/gorag/store/graph/gograph"
	"github.com/DotNetAge/gorag/store/vector/govector"
	"github.com/DotNetAge/gorag/utils"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Name      string `yaml:"name"`       // RAG 名称
	Type      string `yaml:"type"`       // 索引器类型：hybrid, semantic, graph
	ModelFile string `yaml:"model_file"` // 向量化模型
}

const (
	configFileName   = "config.yml"
	GORAG_MODEL_PATH = "GORAG_MODEL_PATH"
	GORAG_BASE_URL   = "GORAG_BASE_URL"
	GORAG_API_KEY    = "GORAG_API_KEY"
	GORAG_AUTH_TOKEN = "GORAG_AUTH_TOKEN"
	GORAG_MODEL      = "GORAG_MODEL"
)

// New 创建新的 RAG 索引实例
// 如果数据目录不存在则创建，生成配置文件和子目录结构
func New(dataDir string, cfg *Config) (core.Indexer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// 设置默认值
	if cfg.Type == "" {
		cfg.Type = "hybrid"
	}

	// 1. 创建数据目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// 2. 检查模型文件是否存在
	if cfg.ModelFile != "" {
		if _, err := os.Stat(cfg.ModelFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("model file not found: %s", cfg.ModelFile)
		}
	}

	// 3. 保存配置文件
	if err := saveConfig(dataDir, cfg); err != nil {
		return nil, err
	}

	// 4. 创建子目录
	subDirs := []string{"vectors", "graphs", "fulltexts"}
	for _, subDir := range subDirs {
		dirPath := filepath.Join(dataDir, subDir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s directory: %w", subDir, err)
		}
	}

	// 5. 实例化索引器
	return createIndexerByName(cfg.Type, dataDir, cfg.ModelFile)
}

// Open 打开已存在的 RAG 索引实例
// 从数据目录读取配置文件并恢复索引器
func Open(dataDir string) (core.Indexer, error) {
	// 1. 检查数据目录是否存在
	info, err := os.Stat(dataDir)
	if err != nil {
		return nil, fmt.Errorf("data directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dataDir)
	}

	// 2. 加载配置文件
	cfg, err := loadConfig(dataDir)
	if err != nil {
		return nil, err
	}

	// 3. 检查模型文件是否存在
	if cfg.ModelFile != "" {
		if _, err := os.Stat(cfg.ModelFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("model file not found: %s", cfg.ModelFile)
		}
	}

	// 4. 实例化索引器
	return createIndexerByName(cfg.Type, dataDir, cfg.ModelFile)
}

// loadConfig 从数据目录加载配置文件
func loadConfig(dataDir string) (*Config, error) {
	configPath := filepath.Join(dataDir, configFileName)
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return &cfg, nil
}

// saveConfig 保存配置文件到数据目录
func saveConfig(dataDir string, cfg *Config) error {
	configPath := filepath.Join(dataDir, configFileName)
	configData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(configPath, configData, 0644)
}

// CheckModel 检查模型文件是否存在，如果不存在则从 HuggingFace 下载
// modelId: HuggingFace 模型 ID，如 "Xenova/chinese-clip-vit-base-patch16"
// modelFile: 模型文件路径，如 "onnx/model.onnx"
// 返回模型文件的完整路径
func CheckModel(modelId, modelFile string) (string, error) {
	baseDir := os.Getenv(GORAG_MODEL_PATH)
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			baseDir = "./models"
		} else {
			baseDir = filepath.Join(homeDir, ".embeddings")
		}
	}

	// 如果 BaseDir 不存在就创建
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create model directory: %w", err)
	}

	// 模型文件完整路径
	onnxFile := filepath.Join(baseDir, modelId, modelFile)

	// 检查模型文件是否存在
	if _, err := os.Stat(onnxFile); os.IsNotExist(err) {
		slog.Info("Model file not found, downloading from HuggingFace", "model", modelId, "file", modelFile)

		// 使用 ModelDownloader 下载模型文件
		downloader, err := utils.NewModelDownloader(baseDir)
		if err != nil {
			return "", fmt.Errorf("failed to create model downloader: %w", err)
		}

		// 下载模型文件（包括必要的配置文件）
		files := []string{modelFile}
		// 如果是 ONNX 模型，通常还需要下载配置文件
		if filepath.Ext(modelFile) == ".onnx" {
			files = append(files, "config.json", "tokenizer.json", "vocab.txt")
		}

		if _, err := downloader.Download(modelId, files); err != nil {
			slog.Error("Failed to download model", "error", err)
			return "", fmt.Errorf("failed to download model: %w", err)
		}

		slog.Info("Model downloaded successfully", "path", onnxFile)
	}

	return onnxFile, nil
}

func createIndexerByName(name, dataDir, modelFile string) (core.Indexer, error) {
	switch name {
	case "hybrid":
		return createHybridIndexer(dataDir, modelFile)
	case "semantic":
		return createSemanticIndexer(dataDir, modelFile)
	case "graph":
		return createGraphIndexer(dataDir)
	case "fulltext":
		return createFulltextIndexer(dataDir)
	default:
		return nil, fmt.Errorf("unknown indexer type: %s", name)
	}
}

func createSemanticIndexer(dataDir, modelFile string) (core.Indexer, error) {
	// 创建 embedder（全局唯一一次 ONNX 初始化）
	clip, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(modelFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	vectorStore, err := createVectorDB(dataDir, modelFile, clip)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	return indexer.NewSemanticIndexer(vectorStore, clip), nil
}

func createGraphIndexer(dataDir string) (core.Indexer, error) {
	graphStore, err := createGraphDB(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph store: %w", err)
	}
	return indexer.NewGraphIndexer(graphStore), nil
}

func createFulltextIndexer(dataDir string) (core.Indexer, error) {
	fullTextStore, err := createFullTextDB(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create fulltext store: %w", err)
	}
	return indexer.NewFulltextIndexer(fullTextStore)
}

func createHybridIndexer(dataDir string, modelFile string) (*HybridIndexer, error) {

	// 创建 embedder（全局唯一一次 ONNX 初始化）
	clip, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(modelFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	vectorStore, err := createVectorDB(dataDir, modelFile, clip)
	if err != nil {
		slog.Error("Failed to init vector store", "error", err)
		return nil, fmt.Errorf("failed to init vector store: %w", err)
	}

	graphStore, err := createGraphDB(dataDir)

	if err != nil {
		slog.Error("Failed to init graph store", "error", err)
		return nil, fmt.Errorf("Failed to init graph store: %w", err)
	}

	fullTextStore, err := createFullTextDB(dataDir)
	if err != nil {
		slog.Error("Failed to init fulltext store", "error", err)
		return nil, err
	}

	llm := createLLM()

	idx, err := NewHybridIndexer(logging.DefaultConsoleLogger(), vectorStore, graphStore, fullTextStore, llm, clip)
	if err != nil {
		slog.Error("Failed to init indexer", "error", err)
		return nil, err
	}
	return idx, nil
}

func getName(dataDir string) string {
	return filepath.Dir(dataDir)
}

func createLLM() chat.Client {
	baseURL := os.Getenv(GORAG_BASE_URL)
	apiKey := os.Getenv(GORAG_API_KEY)
	authToken := os.Getenv(GORAG_AUTH_TOKEN)
	model := os.Getenv(GORAG_MODEL)

	if model == "" || baseURL == "" {
		return nil
	}

	c, err := openai.NewOpenAI(chat.Config{
		APIKey:    apiKey,
		Model:     model,
		BaseURL:   baseURL,
		AuthToken: authToken,
	})

	if err != nil {
		panic(err)
	}

	return c
}

func createVectorDB(dataDir string, modelFile string, clip *embedder.ChineseClipEmbedder) (core.VectorStore, error) {
	name := filepath.Dir(dataDir)

	vectorDbFile := filepath.Join(dataDir, "vectors", name+".db")

	return govector.NewStore(
		govector.WithCollection(name),
		govector.WithDimension(clip.Dim()),
		govector.WithDBPath(vectorDbFile),
		govector.WithHNSW(true),
	)
}

func createFullTextDB(dataDir string) (core.FullTextStore, error) {

	name := getName(dataDir)
	bleveDBFile := filepath.Join(dataDir, "fulltexts", name+".bleve")
	return bleve.NewBleveStore(bleveDBFile)
}

func createGraphDB(dataDir string) (core.GraphStore, error) {
	name := getName(dataDir)
	graphDbFile := filepath.Join(dataDir, "graphs", name+".db")
	return gograph.NewGraphStore(graphDbFile)
}
