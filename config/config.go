package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure for GoRAG
type Config struct {
	RAG         RAGConfig         `yaml:"rag"`
	Embedding   EmbeddingConfig   `yaml:"embedding"`
	LLM         LLMConfig         `yaml:"llm"`
	VectorStore VectorStoreConfig `yaml:"vectorstore"`
	Logging     LoggingConfig     `yaml:"logging"`
}

// RAGConfig represents RAG engine configuration
type RAGConfig struct {
	TopK                  int     `yaml:"topK" default:"5"`
	ChunkSize             int     `yaml:"chunkSize" default:"1000"`
	ChunkOverlap          int     `yaml:"chunkOverlap" default:"100"`
	UseSemanticChunking   bool    `yaml:"useSemanticChunking" default:"false"`
	UseHyDE               bool    `yaml:"useHyDE" default:"false"`
	UseRAGFusion          bool    `yaml:"useRAGFusion" default:"false"`
	UseContextCompression bool    `yaml:"useContextCompression" default:"false"`
	RAGFusionQueries      int     `yaml:"ragFusionQueries" default:"4"`
	RAGFusionWeight       float64 `yaml:"ragFusionWeight" default:"0.5"`
}

// EmbeddingConfig represents embedding provider configuration
type EmbeddingConfig struct {
	Provider string       `yaml:"provider" default:"openai"`
	OpenAI   OpenAIConfig `yaml:"openai"`
	Ollama   OllamaConfig `yaml:"ollama"`
	Cohere   CohereConfig `yaml:"cohere"`
	Voyage   VoyageConfig `yaml:"voyage"`
}

// LLMConfig represents LLM client configuration
type LLMConfig struct {
	Provider    string            `yaml:"provider" default:"openai"`
	OpenAI      OpenAIConfig      `yaml:"openai"`
	Anthropic   AnthropicConfig   `yaml:"anthropic"`
	Ollama      OllamaConfig      `yaml:"ollama"`
	AzureOpenAI AzureOpenAIConfig `yaml:"azure_openai"`
}

// VectorStoreConfig represents vector store configuration
type VectorStoreConfig struct {
	Type     string         `yaml:"type" default:"memory"`
	Memory   MemoryConfig   `yaml:"memory"`
	Milvus   MilvusConfig   `yaml:"milvus"`
	Qdrant   QdrantConfig   `yaml:"qdrant"`
	Weaviate WeaviateConfig `yaml:"weaviate"`
	Pinecone PineconeConfig `yaml:"pinecone"`
	GoVector GoVectorConfig `yaml:"govector"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" default:"info"`
	Format string `yaml:"format" default:"json"`
}

// OpenAIConfig represents OpenAI configuration
type OpenAIConfig struct {
	APIKey  string `yaml:"apiKey"`
	Model   string `yaml:"model" default:"text-embedding-ada-002"`
	BaseURL string `yaml:"baseURL" default:"https://api.openai.com/v1"`
}

// AnthropicConfig represents Anthropic configuration
type AnthropicConfig struct {
	APIKey  string `yaml:"apiKey"`
	Model   string `yaml:"model" default:"claude-3-opus-20240229"`
	BaseURL string `yaml:"baseURL" default:"https://api.anthropic.com/v1"`
}

// OllamaConfig represents Ollama configuration
type OllamaConfig struct {
	Model   string `yaml:"model" default:"qwen3:7b"`
	BaseURL string `yaml:"baseURL" default:"http://localhost:11434"`
}

// AzureOpenAIConfig represents Azure OpenAI configuration
type AzureOpenAIConfig struct {
	APIKey     string `yaml:"apiKey"`
	Model      string `yaml:"model"`
	Endpoint   string `yaml:"endpoint"`
	APIVersion string `yaml:"apiVersion" default:"2024-03-01-preview"`
}

// CohereConfig represents Cohere configuration
type CohereConfig struct {
	APIKey  string `yaml:"apiKey"`
	Model   string `yaml:"model" default:"embed-english-v3.0"`
	BaseURL string `yaml:"baseURL" default:"https://api.cohere.ai/v1"`
}

// VoyageConfig represents Voyage configuration
type VoyageConfig struct {
	APIKey  string `yaml:"apiKey"`
	Model   string `yaml:"model" default:"voyage-2"`
	BaseURL string `yaml:"baseURL" default:"https://api.voyageai.com/v1"`
}

// MemoryConfig represents memory vector store configuration
type MemoryConfig struct {
	MaxSize int `yaml:"maxSize" default:"10000"`
}

// MilvusConfig represents Milvus vector store configuration
type MilvusConfig struct {
	Host     string `yaml:"host" default:"localhost"`
	Port     string `yaml:"port" default:"19530"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database" default:"default"`
}

// QdrantConfig represents Qdrant vector store configuration
type QdrantConfig struct {
	URL    string `yaml:"url" default:"http://localhost:6333"`
	APIKey string `yaml:"apiKey"`
}

// WeaviateConfig represents Weaviate vector store configuration
type WeaviateConfig struct {
	URL    string `yaml:"url" default:"http://localhost:8080"`
	APIKey string `yaml:"apiKey"`
}

// PineconeConfig represents Pinecone vector store configuration
type PineconeConfig struct {
	APIKey      string `yaml:"apiKey"`
	Environment string `yaml:"environment"`
}

// Loader represents a configuration loader
type Loader struct {
	configPath string
}

// NewLoader creates a new configuration loader
func NewLoader(configPath string) *Loader {
	return &Loader{
		configPath: configPath,
	}
}

// Load loads the configuration from file and environment variables
func (l *Loader) Load() (*Config, error) {
	config := &Config{}

	if l.configPath != "" {
		if err := l.loadFromFile(config); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	l.loadFromEnv(config)

	return config, nil
}

// loadFromFile loads configuration from YAML file
func (l *Loader) loadFromFile(config *Config) error {
	if _, err := os.Stat(l.configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", l.configPath)
	}

	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func (l *Loader) loadFromEnv(config *Config) {
	if provider := os.Getenv("GORAG_EMBEDDING_PROVIDER"); provider != "" {
		config.Embedding.Provider = provider
	}
	if apiKey := os.Getenv("GORAG_OPENAI_API_KEY"); apiKey != "" {
		config.Embedding.OpenAI.APIKey = apiKey
		config.LLM.OpenAI.APIKey = apiKey
	}
	if apiKey := os.Getenv("GORAG_COHERE_API_KEY"); apiKey != "" {
		config.Embedding.Cohere.APIKey = apiKey
	}
	if apiKey := os.Getenv("GORAG_VOYAGE_API_KEY"); apiKey != "" {
		config.Embedding.Voyage.APIKey = apiKey
	}

	if provider := os.Getenv("GORAG_LLM_PROVIDER"); provider != "" {
		config.LLM.Provider = provider
	}
	if apiKey := os.Getenv("GORAG_ANTHROPIC_API_KEY"); apiKey != "" {
		config.LLM.Anthropic.APIKey = apiKey
	}

	if storeType := os.Getenv("GORAG_VECTORSTORE_TYPE"); storeType != "" {
		config.VectorStore.Type = storeType
	}
	if apiKey := os.Getenv("GORAG_PINECONE_API_KEY"); apiKey != "" {
		config.VectorStore.Pinecone.APIKey = apiKey
	}
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	if _, err := os.Stat("config/config.yaml"); err == nil {
		return "config/config.yaml"
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, ".gorag", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	return ""
}

// SetDefaults sets default values for configuration
func (c *Config) SetDefaults() {
	if c.RAG.TopK == 0 {
		c.RAG.TopK = 5
	}
	if c.RAG.ChunkSize == 0 {
		c.RAG.ChunkSize = 1000
	}
	if c.RAG.ChunkOverlap == 0 {
		c.RAG.ChunkOverlap = 100
	}
	if c.RAG.RAGFusionQueries == 0 {
		c.RAG.RAGFusionQueries = 4
	}
	if c.RAG.RAGFusionWeight == 0 {
		c.RAG.RAGFusionWeight = 0.5
	}

	if c.Embedding.Provider == "" {
		c.Embedding.Provider = "openai"
	}
	if c.Embedding.OpenAI.Model == "" {
		c.Embedding.OpenAI.Model = "text-embedding-ada-002"
	}
	if c.Embedding.OpenAI.BaseURL == "" {
		c.Embedding.OpenAI.BaseURL = "https://api.openai.com/v1"
	}
	if c.Embedding.Ollama.Model == "" {
		c.Embedding.Ollama.Model = "qllama/bge-small-zh-v1.5:latest"
	}
	if c.Embedding.Ollama.BaseURL == "" {
		c.Embedding.Ollama.BaseURL = "http://localhost:11434"
	}
	if c.Embedding.Cohere.Model == "" {
		c.Embedding.Cohere.Model = "embed-english-v3.0"
	}
	if c.Embedding.Cohere.BaseURL == "" {
		c.Embedding.Cohere.BaseURL = "https://api.cohere.ai/v1"
	}
	if c.Embedding.Voyage.Model == "" {
		c.Embedding.Voyage.Model = "voyage-2"
	}
	if c.Embedding.Voyage.BaseURL == "" {
		c.Embedding.Voyage.BaseURL = "https://api.voyageai.com/v1"
	}

	if c.LLM.Provider == "" {
		c.LLM.Provider = "openai"
	}
	if c.LLM.OpenAI.Model == "" {
		c.LLM.OpenAI.Model = "gpt-3.5-turbo"
	}
	if c.LLM.OpenAI.BaseURL == "" {
		c.LLM.OpenAI.BaseURL = "https://api.openai.com/v1"
	}
	if c.LLM.Anthropic.Model == "" {
		c.LLM.Anthropic.Model = "claude-3-opus-20240229"
	}
	if c.LLM.Anthropic.BaseURL == "" {
		c.LLM.Anthropic.BaseURL = "https://api.anthropic.com/v1"
	}
	if c.LLM.Ollama.Model == "" {
		c.LLM.Ollama.Model = "qwen3:7b"
	}
	if c.LLM.Ollama.BaseURL == "" {
		c.LLM.Ollama.BaseURL = "http://localhost:11434"
	}
	if c.LLM.AzureOpenAI.APIVersion == "" {
		c.LLM.AzureOpenAI.APIVersion = "2024-03-01-preview"
	}

	if c.VectorStore.Type == "" {
		c.VectorStore.Type = "memory"
	}
	if c.VectorStore.Milvus.Host == "" {
		c.VectorStore.Milvus.Host = "localhost"
	}
	if c.VectorStore.Milvus.Port == "" {
		c.VectorStore.Milvus.Port = "19530"
	}
	if c.VectorStore.Milvus.Database == "" {
		c.VectorStore.Milvus.Database = "default"
	}
	if c.VectorStore.Qdrant.URL == "" {
		c.VectorStore.Qdrant.URL = "http://localhost:6333"
	}
	if c.VectorStore.Weaviate.URL == "" {
		c.VectorStore.Weaviate.URL = "http://localhost:8080"
	}
	if c.VectorStore.Memory.MaxSize == 0 {
		c.VectorStore.Memory.MaxSize = 10000
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Embedding.Provider == "openai" && c.Embedding.OpenAI.APIKey == "" {
		return fmt.Errorf("OpenAI API key is required for embedding")
	}
	if c.Embedding.Provider == "cohere" && c.Embedding.Cohere.APIKey == "" {
		return fmt.Errorf("Cohere API key is required for embedding")
	}
	if c.Embedding.Provider == "voyage" && c.Embedding.Voyage.APIKey == "" {
		return fmt.Errorf("Voyage API key is required for embedding")
	}

	if c.LLM.Provider == "openai" && c.LLM.OpenAI.APIKey == "" {
		return fmt.Errorf("OpenAI API key is required for LLM")
	}
	if c.LLM.Provider == "anthropic" && c.LLM.Anthropic.APIKey == "" {
		return fmt.Errorf("Anthropic API key is required for LLM")
	}
	if c.LLM.Provider == "azure_openai" && (c.LLM.AzureOpenAI.APIKey == "" || c.LLM.AzureOpenAI.Endpoint == "") {
		return fmt.Errorf("Azure OpenAI API key and endpoint are required for LLM")
	}

	if c.VectorStore.Type == "pinecone" && c.VectorStore.Pinecone.APIKey == "" {
		return fmt.Errorf("Pinecone API key is required")
	}

	return nil
}

// GetEnv returns the value of an environment variable or the default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SanitizeAPIKey sanitizes API keys for logging
func SanitizeAPIKey(apiKey string) string {
	if len(apiKey) < 8 {
		return "****"
	}
	return apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
}

// ToMap converts the config to a map for use in plugins
func (c *Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"rag":         c.RAG,
		"embedding":   c.Embedding,
		"llm":         c.LLM,
		"vectorstore": c.VectorStore,
		"logging":     c.Logging,
	}
}

// FromMap loads config from a map
func (c *Config) FromMap(data map[string]interface{}) error {
	return nil
}

// GoVectorConfig represents GoVector configuration
type GoVectorConfig struct {
	Collection string `yaml:"collection" default:"gorag"`
	Dimension  int    `yaml:"dimension" default:"1536"`
	DBPath     string `yaml:"dbPath" default:"gorag_vectors.db"`
	UseHNSW    bool   `yaml:"useHNSW" default:"true"`
}
