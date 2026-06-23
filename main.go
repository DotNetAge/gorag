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
	"github.com/DotNetAge/gorag/utils"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Name     string `yaml:"name"`      // RAG 名称
	Type     string `yaml:"type"`      // 索引器类型：hybrid, semantic, graph
	EmbeddingModelFile string `yaml:"embedding_model_file"` // 向量化模型（onnx 文件路径）

	// LLM 模型配置（用于 GraphIndexer）
	APIKey          string `yaml:"api_key,omitempty"`
	BaseURL         string `yaml:"base_url,omitempty"`
	Model           string `yaml:"model,omitempty"`
	Language        string `yaml:"language,omitempty"`
	MaxTokens       int    `yaml:"max_tokens,omitempty"`
	ThinkingBudget  int    `yaml:"thinking_budget,omitempty"`
}

const (
	configFileName   = "config.yml"
	GORAG_MODEL_PATH = "GORAG_MODEL_PATH"
	GORAG_BASE_URL   = "GORAG_BASE_URL"
	GORAG_API_KEY    = "GORAG_API_KEY"
	GORAG_AUTH_TOKEN = "GORAG_AUTH_TOKEN"
	GORAG_MODEL      = "GORAG_MODEL"
)

type RAGOption func(*Config)

func WithEmbeddingModelFile(modelFile string) RAGOption {
	return func(cfg *Config) {
		cfg.EmbeddingModelFile = modelFile
	}
}

func WithIndexType(indexType string) RAGOption {
	return func(cfg *Config) {
		cfg.Type = indexType
	}
}

func WithName(name string) RAGOption {
	return func(cfg *Config) {
		cfg.Name = name
	}
}

// WithLLMModel 设置 LLM 模型配置（用于 GraphIndexer）。
// 配置内容会持久化到 config.yml 中，Open 时自动恢复。
func WithLLMModel(modelCfg indexer.ModelConfig) RAGOption {
	return func(cfg *Config) {
		cfg.APIKey = modelCfg.APIKey
		cfg.BaseURL = modelCfg.BaseURL
		cfg.Model = modelCfg.Model
		cfg.Language = modelCfg.Language
		cfg.MaxTokens = modelCfg.MaxTokens
		cfg.ThinkingBudget = modelCfg.ThinkingBudget
	}
}

// ToLLMConfig 将 Config 中的 LLM 字段转换为 indexer.ModelConfig。
// 如果 model 为空，返回 nil。
func (c *Config) ToLLMConfig() *indexer.ModelConfig {
	if c.Model == "" {
		return nil
	}
	lang := c.Language
	if lang == "" {
		lang = "Chinese"
	}
	maxTokens := c.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 128000
	}
	return &indexer.ModelConfig{
		APIKey:         c.APIKey,
		BaseURL:        c.BaseURL,
		Model:          c.Model,
		Language:       lang,
		MaxTokens:      maxTokens,
		ThinkingBudget: c.ThinkingBudget,
	}
}

// New 创建新的 RAG 索引实例。
//
// 按代际自动选择索引器：
//   - 有 LLM 模型配置（WithLLMModel）→ GraphIndexer（第三代，最强）
//   - 无 LLM 模型配置 → HybridIndexer（第二代，semantic + fulltext）
//
// 如果数据目录不存在则创建，生成配置文件和子目录结构。
func New(dataDir string, opts ...RAGOption) (core.Indexer, error) {
	cfg := &Config{
		Name: "gorag",
		Type: "hybrid",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// 1. 创建数据目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// 2. 检查模型文件是否存在
	if cfg.EmbeddingModelFile != "" {
		if _, err := os.Stat(cfg.EmbeddingModelFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("model file not found: %s", cfg.EmbeddingModelFile)
		}
	} else {
		return nil, fmt.Errorf("model file is empty")
	}

	// 3. 检测 LLM 配置 → 选择代际
	hasLLM := cfg.Model != ""
	if hasLLM {
		cfg.Type = "graph"
	} else {
		cfg.Type = "hybrid"
	}

	// 4. 保存配置文件
	if err := saveConfig(dataDir, cfg); err != nil {
		return nil, err
	}

	// 5. 创建子目录
	subDirs := []string{"vectors", "graphs", "fulltexts", "caches"}
	for _, subDir := range subDirs {
		dirPath := filepath.Join(dataDir, subDir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s directory: %w", subDir, err)
		}
	}

	// 6. 实例化索引器
	return createIndexerByName(cfg.Type, dataDir, cfg.EmbeddingModelFile)
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
	if cfg.EmbeddingModelFile != "" {
		if _, err := os.Stat(cfg.EmbeddingModelFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("model file not found: %s", cfg.EmbeddingModelFile)
		}
	}

	// 4. 实例化索引器
	return createIndexerByName(cfg.Type, dataDir, cfg.EmbeddingModelFile)
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

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create model directory: %w", err)
	}

	onnxFile := filepath.Join(baseDir, modelId, modelFile)

	if _, err := os.Stat(onnxFile); os.IsNotExist(err) {
		slog.Info("Model file not found, downloading from HuggingFace", "model", modelId, "file", modelFile)

		downloader, err := utils.NewModelDownloader(baseDir)
		if err != nil {
			return "", fmt.Errorf("failed to create model downloader: %w", err)
		}

		files := []string{modelFile}
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
	case "graph":
		return createGraphIndexer(dataDir)
	case "hybrid":
		return createHybridIndexer(dataDir, modelFile)
	case "semantic":
		return createSemanticIndexer(dataDir, modelFile)
	case "fulltext":
		return createFulltextIndexer(dataDir)
	default:
		return nil, fmt.Errorf("unknown indexer type: %s", name)
	}
}

func createSemanticIndexer(dataDir, modelFile string) (core.Indexer, error) {
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

// createGraphIndexer 创建独立 GraphIndexer（第三代）。
// 从 dataDir/config.yml 加载 LLM 配置和 embedding 模型路径。
func createGraphIndexer(dataDir string) (core.Indexer, error) {
	cfg, err := loadConfig(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	llmCfg := cfg.ToLLMConfig()
	if llmCfg == nil {
		return nil, fmt.Errorf("graph indexer requires LLM model configuration (set model in config)")
	}

	if cfg.EmbeddingModelFile == "" {
		return nil, fmt.Errorf("embedding model file is required")
	}

	clip, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(cfg.EmbeddingModelFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	vectorStore, err := createVectorDB(dataDir, cfg.EmbeddingModelFile, clip)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	graphStore, err := createGraphDB(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph store: %w", err)
	}

	slog.Info("GraphIndexer created (standalone mode)",
		"model", llmCfg.Model, "base_url", llmCfg.BaseURL)
	return indexer.New(*llmCfg, clip, vectorStore, graphStore), nil
}

func createFulltextIndexer(dataDir string) (core.Indexer, error) {
	fullTextStore, err := createFullTextDB(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create fulltext store: %w", err)
	}
	return indexer.NewFulltextIndexer(fullTextStore)
}

// createHybridIndexer 创建第二代混合索引器（semantic + fulltext），无 LLM 依赖。
func createHybridIndexer(dataDir string, modelFile string) (*HybridIndexer, error) {
	clip, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(modelFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	vectorStore, err := createVectorDB(dataDir, modelFile, clip)
	if err != nil {
		slog.Error("Failed to init vector store", "error", err)
		return nil, fmt.Errorf("failed to init vector store: %w", err)
	}

	fullTextStore, err := createFullTextDB(dataDir)
	if err != nil {
		slog.Error("Failed to init fulltext store", "error", err)
		return nil, err
	}

	return NewHybridIndexer(logging.DefaultConsoleLogger(), vectorStore, fullTextStore, clip)
}

func getName(dataDir string) string {
	return filepath.Base(dataDir)
}

func createVectorDB(dataDir string, modelFile string, clip *embedder.ChineseClipEmbedder) (core.VectorStore, error) {
	name := getName(dataDir)
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
