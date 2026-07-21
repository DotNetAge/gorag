package gorag

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/embedder"
	"github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/llm"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/store/graph/gograph"
	"github.com/DotNetAge/gorag/v2/store/vector/govector"
	"github.com/DotNetAge/gorag/v2/utils"
	"gopkg.in/yaml.v3"
)

// ragSuffix .rag 库文件的后缀（目录实现，文件认知）。
const ragSuffix = ".rag"

// configFileName 配置文件名。
const configFileName = "config.yml"

// 环境变量常量
const (
	GORAG_MODEL_PATH = "GORAG_MODEL_PATH"
	GORAG_BASE_URL   = "GORAG_BASE_URL"
	GORAG_API_KEY    = "GORAG_API_KEY"
	GORAG_AUTH_TOKEN = "GORAG_AUTH_TOKEN"
	GORAG_MODEL      = "GORAG_MODEL"
)

// Config .rag 库的配置文件结构（config.yml）。
// 分层结构 = storage + embedding + llm + indexer + query。
// api_key 不写入 config.yml，独立存于 .rag/.api_key（权限 600）。
type Config struct {
	Version  int           `yaml:"version"`           // 配置版本号
	Storage  StorageConfig `yaml:"storage"`           // 存储路径配置
	Embedding EmbeddingConfig `yaml:"embedding"`       // 向量模型配置
	LLM      LLMConfig     `yaml:"llm"`               // LLM 配置（不含 APIKey）
	Indexer  IndexerConfig `yaml:"indexer"`           // 索引器配置
	Query    QueryConfig   `yaml:"query"`             // 查询配置
}

// StorageConfig 存储路径配置
type StorageConfig struct {
	VectorsDir string `yaml:"vectors_dir"` // 向量库目录
	GraphsDir  string `yaml:"graphs_dir"`  // 图库目录
	CachesDir  string `yaml:"caches_dir"`  // 缓存目录
	LogsDir    string `yaml:"logs_dir"`    // 日志目录
	MetaDB     string `yaml:"meta_db"`     // 元数据 SQLite 文件名
}

// EmbeddingConfig 向量模型配置
type EmbeddingConfig struct {
	ModelFile  string `yaml:"model_file"`  // ONNX 模型文件路径
	Dimension  int    `yaml:"dimension"`   // 向量维度
}

// LLMConfig LLM 配置（APIKey 不在此处，存于 .rag/.api_key）
type LLMConfig struct {
	BaseURL        string `yaml:"base_url"`
	Model          string `yaml:"model"`
	Language       string `yaml:"language"`         // 内容语言（如 Chinese）
	MaxTokens      int    `yaml:"max_tokens"`       // 模型最大输出 token
	ContextLength  int    `yaml:"context_length"`   // 模型上下文长度
	ThinkingBudget int    `yaml:"thinking_budget"`  // 思考模式 token 预算（0=默认）
	APIKeyFile     string `yaml:"api_key_file,omitempty"` // 外部 API Key 文件路径（可选）
}

// IndexerConfig 索引器配置
type IndexerConfig struct {
	Type string `yaml:"type"` // semantic | graph | hyper
}

// QueryConfig 查询配置
type QueryConfig struct {
	SemanticWeight float32 `yaml:"semantic_weight"` // 语义检索权重
	GraphWeight    float32 `yaml:"graph_weight"`    // 图检索权重
}

// RAGOption Open 函数的配置选项
type RAGOption func(*Config)

// WithLLM 注入 gochat 客户端对应的 LLM 配置。
// 注：实际的 chat.Client 通过此函数从配置实例化并返回，供 indexer 使用。
// LLM 在应用层实例化，indexer 包不再创建 LLM 客户端。
func WithLLM(llmCfg LLMConfig) RAGOption {
	return func(cfg *Config) {
		cfg.LLM = llmCfg
	}
}

// WithEmbeddingModelFile 设置向量模型文件路径
func WithEmbeddingModelFile(modelFile string) RAGOption {
	return func(cfg *Config) {
		cfg.Embedding.ModelFile = modelFile
	}
}

// WithIndexType 设置索引器类型
func WithIndexType(indexType string) RAGOption {
	return func(cfg *Config) {
		cfg.Indexer.Type = indexType
	}
}

// WithName 设置 RAG 库命名（兼容旧 API，仅写入 Storage 命名，无实际作用）
func WithName(name string) RAGOption {
	// 库名即目录名，此选项仅为兼容旧调用方
	return func(cfg *Config) {}
}

// defaultConfig 生成默认配置
func defaultConfig() *Config {
	return &Config{
		Version: 1,
		Storage: StorageConfig{
			VectorsDir: "vectors",
			GraphsDir:  "graphs",
			CachesDir:  "caches",
			LogsDir:    "logs",
			MetaDB:     "meta.db",
		},
		Embedding: EmbeddingConfig{
			Dimension: 512,
		},
		LLM: LLMConfig{
			Language:      "Chinese",
			MaxTokens:     128000,
			ContextLength: 128000,
		},
		Indexer: IndexerConfig{
			Type: "semantic",
		},
		Query: QueryConfig{
			SemanticWeight: 0.8,
			GraphWeight:    0.2,
		},
	}
}

// Init 在指定路径创建 .rag 库目录结构。
// 物理结构 = config.yml + .api_key + .ragignore + .lock +
// meta.db + vectors/ + graphs/ + caches/ + logs/。
//
// ragDir 必须以 .rag 结尾，否则返回错误。
// 已存在的 .rag 目录会被复用（不报错），但缺失的子目录和文件会被补齐。
func Init(ragDir string) error {
	if !strings.HasSuffix(ragDir, ragSuffix) {
		return fmt.Errorf("路径必须以 .rag 结尾（RAG 库文件）: %s", ragDir)
	}

	// 1. 创建根目录
	if err := os.MkdirAll(ragDir, 0755); err != nil {
		return fmt.Errorf("创建 .rag 目录失败: %w", err)
	}

	// 2. 创建子目录
	subDirs := []string{"vectors", "graphs", "caches", "logs"}
	for _, sub := range subDirs {
		if err := os.MkdirAll(filepath.Join(ragDir, sub), 0755); err != nil {
			return fmt.Errorf("创建 %s 子目录失败: %w", sub, err)
		}
	}

	// 3. 写入默认 config.yml（已存在则跳过）
	configPath := filepath.Join(ragDir, configFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := defaultConfig()
		if err := saveConfig(ragDir, cfg); err != nil {
			return fmt.Errorf("写入 config.yml 失败: %w", err)
		}
	}

	// 4. 生成 .ragignore（已存在则跳过）
	ragignorePath := filepath.Join(ragDir, ".ragignore")
	if _, err := os.Stat(ragignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(ragignorePath, []byte(defaultRagignoreContent), 0644); err != nil {
			return fmt.Errorf("写入 .ragignore 失败: %w", err)
		}
	}

	// 5. 创建空 .lock 文件（运行时通过 flock 加锁）
	lockPath := filepath.Join(ragDir, ".lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		if err := os.WriteFile(lockPath, []byte{}, 0644); err != nil {
			return fmt.Errorf("写入 .lock 失败: %w", err)
		}
	}

	return nil
}

// defaultRagignoreContent .ragignore 默认内容
const defaultRagignoreContent = `# 敏感信息
.api_key

# 运行时锁文件
.lock

# 数据库（体积大，不入版本控制）
vectors/
graphs/
caches/
meta.db
meta.db-wal
meta.db-shm

# 日志
logs/
`

// Open 打开已存在的 .rag 库。
// 强制 .rag 后缀校验，不兼容旧 dataDir。
// opts 可注入 WithLLM、WithEmbeddingModelFile 等。
//
// 行为：
//   - 校验路径以 .rag 结尾
//   - 加载 config.yml
//   - 根据 cfg.Indexer.Type 创建索引器
//   - 若有 LLM 配置，则创建 gochat 客户端并注入 GraphIndexer/HyperIndexer
func Open(ragDir string, opts ...RAGOption) (indexer.Indexer, error) {
	if !strings.HasSuffix(ragDir, ragSuffix) {
		return nil, fmt.Errorf("路径必须以 .rag 结尾（RAG 库文件）: %s", ragDir)
	}

	// 1. 检查目录存在
	info, err := os.Stat(ragDir)
	if err != nil {
		return nil, fmt.Errorf(".rag 库不存在: %w（提示：请先运行 grag init）", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s 不是目录（.rag 库必须是目录）", ragDir)
	}

	// 2. 加载配置文件
	cfg, err := loadConfig(ragDir)
	if err != nil {
		return nil, err
	}

	// 3. 应用传入的选项（覆盖配置文件中的字段）
	for _, opt := range opts {
		opt(cfg)
	}

	// 4. 校验 embedder 配置（必须有 model_file）
	if cfg.Embedding.ModelFile == "" {
		return nil, fmt.Errorf("未配置 embedder，请运行: grag config embedder <model-path>")
	}
	if _, err := os.Stat(cfg.Embedding.ModelFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("embedder 模型文件不存在: %s", cfg.Embedding.ModelFile)
	}

	// 5. 实例化索引器
	return createIndexer(ragDir, cfg)
}

// createIndexer 根据 cfg.Indexer.Type 实例化索引器
//
// 策略：
//   - type=semantic → SemanticIndexer（仅向量库 + embedder）
//   - type=graph    → GraphIndexer（纯图谱模式，不需要 LLM/extractor，独立使用一般不推荐）
//   - type=hyper    → HyperIndexer（语义线 + 关系线，生产推荐模式）
//   - 未显式配置 type 时：有 LLM 默认 hyper，无 LLM 默认 semantic
func createIndexer(ragDir string, cfg *Config) (indexer.Indexer, error) {
	// 创建 embedder
	clip, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(cfg.Embedding.ModelFile))
	if err != nil {
		return nil, fmt.Errorf("创建 embedder 失败: %w", err)
	}

	// 创建向量库
	vectorStore, err := createVectorDB(ragDir, clip)
	if err != nil {
		return nil, fmt.Errorf("创建向量库失败: %w", err)
	}

	// 判断是否有 LLM 配置
	hasLLM := cfg.LLM.Model != "" && cfg.LLM.BaseURL != ""

	// 自动选择索引器类型：有 LLM 默认 hyper（生产推荐），无 LLM 默认 semantic
	idxType := cfg.Indexer.Type
	if idxType == "" {
		if hasLLM {
			idxType = "hyper"
		} else {
			idxType = "semantic"
		}
	}

	switch idxType {
	case "semantic":
		// 纯语义模式：仅向量库 + embedder
		return indexer.NewSemanticIndexer(vectorStore, clip)

	case "graph":
		// 纯图谱模式（独立使用，一般不推荐）
		// GraphIndexer 不需要 LLM/extractor，只需要 GraphStore
		graphStore, gErr := createGraphDB(ragDir)
		if gErr != nil {
			return nil, fmt.Errorf("创建图库失败: %w", gErr)
		}
		return indexer.New(graphStore)

	case "hyper":
		// 生产推荐模式：语义线 + 关系线
		// 1. 可选创建 Summarizer（hasLLM 时尝试创建，失败降级为 nil，不阻塞主流程）
		var summarizer llm.Summarizer
		if hasLLM {
			apiKey, keyErr := ResolveAPIKey(ragDir)
			if keyErr != nil {
				return nil, fmt.Errorf("解析 APIKey 失败: %w", keyErr)
			}
			summarizerCfg := llm.Config{
				APIKey:        apiKey,
				BaseURL:       cfg.LLM.BaseURL,
				Model:         cfg.LLM.Model,
				Language:      cfg.LLM.Language,
				MaxTokens:     cfg.LLM.MaxTokens,
				ContextLength: cfg.LLM.ContextLength,
			}
			if sm, smErr := llm.NewSummarizer(summarizerCfg, logging.DefaultNoopLogger()); smErr == nil {
				summarizer = sm
			}
			// Summarizer 创建失败不阻塞，summarizer 保持为 nil，降级为不带摘要增强的 SemanticIndexer
		}

		// 2. 创建 SemanticIndexer（summarizer 为 nil 时不注入）
		var semantic indexer.Indexer
		if summarizer != nil {
			semantic, err = indexer.NewSemanticIndexer(
				vectorStore, clip,
				indexer.WithSemanticSummarizer(summarizer),
			)
		} else {
			semantic, err = indexer.NewSemanticIndexer(vectorStore, clip)
		}
		if err != nil {
			return nil, fmt.Errorf("创建 SemanticIndexer 失败: %w", err)
		}

		// 3. 创建 GraphIndexer（不需要 LLM/extractor，只持有 GraphStore）
		graphStore, gErr := createGraphDB(ragDir)
		if gErr != nil {
			return nil, fmt.Errorf("创建图库失败: %w", gErr)
		}
		graph, gErr := indexer.New(graphStore)
		if gErr != nil {
			return nil, fmt.Errorf("创建 GraphIndexer 失败: %w", gErr)
		}

		// 4. 组合为 HyperIndexer（semantic 必传，graph 可为 nil 但此处不为 nil）
		return indexer.NewHyperIndexer(semantic, graph)

	default:
		return nil, fmt.Errorf("不支持的索引器类型: %s（仅支持 semantic/graph/hyper）", idxType)
	}
}

// loadConfig 从 .rag 目录加载配置文件
func loadConfig(ragDir string) (*Config, error) {
	configPath := filepath.Join(ragDir, configFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取 config.yml 失败: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析 config.yml 失败: %w", err)
	}
	// 兼容默认值
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Storage.VectorsDir == "" {
		cfg.Storage.VectorsDir = "vectors"
	}
	if cfg.Storage.GraphsDir == "" {
		cfg.Storage.GraphsDir = "graphs"
	}
	if cfg.Storage.CachesDir == "" {
		cfg.Storage.CachesDir = "caches"
	}
	if cfg.Storage.LogsDir == "" {
		cfg.Storage.LogsDir = "logs"
	}
	if cfg.Storage.MetaDB == "" {
		cfg.Storage.MetaDB = "meta.db"
	}
	if cfg.LLM.Language == "" {
		cfg.LLM.Language = "Chinese"
	}
	if cfg.LLM.MaxTokens <= 0 {
		cfg.LLM.MaxTokens = 128000
	}
	if cfg.LLM.ContextLength <= 0 {
		cfg.LLM.ContextLength = 128000
	}
	if cfg.Query.SemanticWeight == 0 && cfg.Query.GraphWeight == 0 {
		cfg.Query.SemanticWeight = 0.8
		cfg.Query.GraphWeight = 0.2
	}
	return &cfg, nil
}

// saveConfig 保存配置到 .rag 目录
func saveConfig(ragDir string, cfg *Config) error {
	configPath := filepath.Join(ragDir, configFileName)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化 config.yml 失败: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

// SaveConfig 更新 .rag 目录的 config.yml（公开 API，供 grag config 调用）
func SaveConfig(ragDir string, cfg *Config) error {
	return saveConfig(ragDir, cfg)
}

// LoadConfig 从 .rag 目录加载 config.yml（公开 API）
func LoadConfig(ragDir string) (*Config, error) {
	return loadConfig(ragDir)
}

// createVectorDB 创建向量库
func createVectorDB(ragDir string, clip *embedder.ChineseClipEmbedder) (core.VectorStore, error) {
	name := filepath.Base(ragDir)
	cfg, _ := loadConfig(ragDir)
	vectorsDir := cfg.Storage.VectorsDir
	if vectorsDir == "" {
		vectorsDir = "vectors"
	}
	vectorDbFile := filepath.Join(ragDir, vectorsDir, name+".db")
	return govector.NewStore(
		govector.WithCollection(name),
		govector.WithDimension(clip.Dim()),
		govector.WithDBPath(vectorDbFile),
		govector.WithHNSW(true),
	)
}

// createGraphDB 创建图库
func createGraphDB(ragDir string) (core.GraphStore, error) {
	name := filepath.Base(ragDir)
	cfg, _ := loadConfig(ragDir)
	graphsDir := cfg.Storage.GraphsDir
	if graphsDir == "" {
		graphsDir = "graphs"
	}
	graphDbFile := filepath.Join(ragDir, graphsDir, name+".db")
	return gograph.NewGraphStore(graphDbFile)
}

// CheckModel 检查模型文件是否存在，不存在则从 HuggingFace 下载
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
		return "", fmt.Errorf("创建模型目录失败: %w", err)
	}

	onnxFile := filepath.Join(baseDir, modelId, modelFile)

	if _, err := os.Stat(onnxFile); os.IsNotExist(err) {
		downloader, err := utils.NewModelDownloader(baseDir)
		if err != nil {
			return "", fmt.Errorf("创建模型下载器失败: %w", err)
		}

		files := []string{modelFile}
		if filepath.Ext(modelFile) == ".onnx" {
			files = append(files, "config.json", "tokenizer.json", "vocab.txt")
		}

		if _, err := downloader.Download(modelId, files); err != nil {
			return "", fmt.Errorf("下载模型失败: %w", err)
		}
	}

	return onnxFile, nil
}

// 兼容旧代码：DefaultConsoleLogger 等日志便捷函数仍可用
var _ = logging.DefaultConsoleLogger
