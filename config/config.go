package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	RAG        RAGConfig        `yaml:"rag"`
	Embedding  EmbeddingConfig  `yaml:"embedding"`
	LLM        LLMConfig        `yaml:"llm"`
	VectorStore VectorStoreConfig `yaml:"vectorstore"`
	Logging    LoggingConfig    `yaml:"logging"`
	Metrics    MetricsConfig    `yaml:"metrics"`
	Auth       AuthConfig       `yaml:"auth"`
	RateLimit  RateLimitConfig  `yaml:"rate_limit"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port string `yaml:"port" default:"8080"`
	Host string `yaml:"host" default:"0.0.0.0"`
}

// RAGConfig represents RAG engine configuration
type RAGConfig struct {
	TopK        int    `yaml:"topK" default:"5"`
	ChunkSize   int    `yaml:"chunkSize" default:"1000"`
	ChunkOverlap int   `yaml:"chunkOverlap" default:"100"`
}

// EmbeddingConfig represents embedding provider configuration
type EmbeddingConfig struct {
	Provider string            `yaml:"provider" default:"openai"`
	OpenAI   OpenAIConfig      `yaml:"openai"`
	Ollama   OllamaConfig      `yaml:"ollama"`
}

// LLMConfig represents LLM client configuration
type LLMConfig struct {
	Provider   string            `yaml:"provider" default:"openai"`
	OpenAI     OpenAIConfig      `yaml:"openai"`
	Anthropic  AnthropicConfig   `yaml:"anthropic"`
	Ollama     OllamaConfig      `yaml:"ollama"`
	AzureOpenAI AzureOpenAIConfig `yaml:"azure_openai"`
}

// VectorStoreConfig represents vector store configuration
type VectorStoreConfig struct {
	Type     string          `yaml:"type" default:"memory"`
	Memory   MemoryConfig    `yaml:"memory"`
	Milvus   MilvusConfig    `yaml:"milvus"`
	Qdrant   QdrantConfig    `yaml:"qdrant"`
	Weaviate WeaviateConfig  `yaml:"weaviate"`
	Pinecone PineconeConfig  `yaml:"pinecone"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" default:"info"`
	Format string `yaml:"format" default:"json"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" default:"true"`
	Port    string `yaml:"port" default:"9090"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled  bool           `yaml:"enabled" default:"false"`
	Providers []AuthProvider `yaml:"providers"`
}

// AuthProvider represents an authentication provider
type AuthProvider struct {
	Type     string            `yaml:"type"`
	APIKey   APIKeyConfig      `yaml:"api_key"`
	OAuth2   OAuth2Config      `yaml:"oauth2"`
}

// APIKeyConfig represents API key authentication configuration
type APIKeyConfig struct {
	Keys []string `yaml:"keys"`
}

// OAuth2Config represents OAuth2 authentication configuration
type OAuth2Config struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	AuthURL      string `yaml:"auth_url"`
	TokenURL     string `yaml:"token_url"`
	RedirectURL  string `yaml:"redirect_url"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled      bool             `yaml:"enabled" default:"false"`
	RequestsPerMinute int         `yaml:"requests_per_minute" default:"60"`
	Burst        int             `yaml:"burst" default:"10"`
	Exemptions   []string        `yaml:"exemptions"`
}

// OpenAIConfig represents OpenAI configuration
type OpenAIConfig struct {
	APIKey string `yaml:"apiKey"`
	Model  string `yaml:"model" default:"text-embedding-ada-002"`
	BaseURL string `yaml:"baseURL" default:"https://api.openai.com/v1"`
}

// AnthropicConfig represents Anthropic configuration
type AnthropicConfig struct {
	APIKey string `yaml:"apiKey"`
	Model  string `yaml:"model" default:"claude-3-opus-20240229"`
	BaseURL string `yaml:"baseURL" default:"https://api.anthropic.com/v1"`
}

// OllamaConfig represents Ollama configuration
type OllamaConfig struct {
	Model   string `yaml:"model" default:"qwen3:7b"`
	BaseURL string `yaml:"baseURL" default:"http://localhost:11434"`
}

// AzureOpenAIConfig represents Azure OpenAI configuration
type AzureOpenAIConfig struct {
	APIKey      string `yaml:"apiKey"`
	Model       string `yaml:"model"`
	Endpoint    string `yaml:"endpoint"`
	APIVersion  string `yaml:"apiVersion" default:"2024-03-01-preview"`
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
	URL      string `yaml:"url" default:"http://localhost:6333"`
	APIKey   string `yaml:"apiKey"`
}

// WeaviateConfig represents Weaviate vector store configuration
type WeaviateConfig struct {
	URL      string `yaml:"url" default:"http://localhost:8080"`
	APIKey   string `yaml:"apiKey"`
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
	// Create default config
	config := &Config{}

	// Load from file if it exists
	if l.configPath != "" {
		if err := l.loadFromFile(config); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Override with environment variables
	l.loadFromEnv(config)

	return config, nil
}

// loadFromFile loads configuration from YAML file
func (l *Loader) loadFromFile(config *Config) error {
	// Check if file exists
	if _, err := os.Stat(l.configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", l.configPath)
	}

	// Read file content
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func (l *Loader) loadFromEnv(config *Config) {
	// Server config
	if port := os.Getenv("GORAG_SERVER_PORT"); port != "" {
		config.Server.Port = port
	}
	if host := os.Getenv("GORAG_SERVER_HOST"); host != "" {
		config.Server.Host = host
	}

	// RAG config
	if topK := os.Getenv("GORAG_RAG_TOPK"); topK != "" {
		// Parse and set
	}

	// Embedding config
	if provider := os.Getenv("GORAG_EMBEDDING_PROVIDER"); provider != "" {
		config.Embedding.Provider = provider
	}
	if apiKey := os.Getenv("GORAG_OPENAI_API_KEY"); apiKey != "" {
		config.Embedding.OpenAI.APIKey = apiKey
		config.LLM.OpenAI.APIKey = apiKey
	}

	// LLM config
	if provider := os.Getenv("GORAG_LLM_PROVIDER"); provider != "" {
		config.LLM.Provider = provider
	}
	if apiKey := os.Getenv("GORAG_ANTHROPIC_API_KEY"); apiKey != "" {
		config.LLM.Anthropic.APIKey = apiKey
	}

	// Vector store config
	if storeType := os.Getenv("GORAG_VECTORSTORE_TYPE"); storeType != "" {
		config.VectorStore.Type = storeType
	}
	if apiKey := os.Getenv("GORAG_PINECONE_API_KEY"); apiKey != "" {
		config.VectorStore.Pinecone.APIKey = apiKey
	}

	// Auth config
	if enabled := os.Getenv("GORAG_AUTH_ENABLED"); enabled == "true" {
		config.Auth.Enabled = true
	}

	// Rate limit config
	if enabled := os.Getenv("GORAG_RATE_LIMIT_ENABLED"); enabled == "true" {
		config.RateLimit.Enabled = true
	}
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	// Check current directory
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	// Check config directory
	if _, err := os.Stat("config/config.yaml"); err == nil {
		return "config/config.yaml"
	}

	// Check home directory
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
	// Server defaults
	if c.Server.Port == "" {
		c.Server.Port = "8080"
	}
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}

	// RAG defaults
	if c.RAG.TopK == 0 {
		c.RAG.TopK = 5
	}
	if c.RAG.ChunkSize == 0 {
		c.RAG.ChunkSize = 1000
	}
	if c.RAG.ChunkOverlap == 0 {
		c.RAG.ChunkOverlap = 100
	}

	// Embedding defaults
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

	// LLM defaults
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

	// Vector store defaults
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

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	// Metrics defaults
	if c.Metrics.Port == "" {
		c.Metrics.Port = "9090"
	}

	// Rate limit defaults
	if c.RateLimit.RequestsPerMinute == 0 {
		c.RateLimit.RequestsPerMinute = 60
	}
	if c.RateLimit.Burst == 0 {
		c.RateLimit.Burst = 10
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	// Validate embedding config
	if c.Embedding.Provider == "openai" && c.Embedding.OpenAI.APIKey == "" {
		return fmt.Errorf("OpenAI API key is required for embedding")
	}

	// Validate LLM config
	if c.LLM.Provider == "openai" && c.LLM.OpenAI.APIKey == "" {
		return fmt.Errorf("OpenAI API key is required for LLM")
	}
	if c.LLM.Provider == "anthropic" && c.LLM.Anthropic.APIKey == "" {
		return fmt.Errorf("Anthropic API key is required for LLM")
	}
	if c.LLM.Provider == "azure_openai" && (c.LLM.AzureOpenAI.APIKey == "" || c.LLM.AzureOpenAI.Endpoint == "") {
		return fmt.Errorf("Azure OpenAI API key and endpoint are required for LLM")
	}

	// Validate vector store config
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
	// Implementation would convert struct to map
	// For simplicity, we'll return a basic map
	return map[string]interface{}{
		"server":      c.Server,
		"rag":         c.RAG,
		"embedding":   c.Embedding,
		"llm":         c.LLM,
		"vectorstore": c.VectorStore,
		"logging":     c.Logging,
		"metrics":     c.Metrics,
		"auth":        c.Auth,
		"rate_limit":  c.RateLimit,
	}
}

// FromMap loads config from a map
func (c *Config) FromMap(data map[string]interface{}) error {
	// Implementation would convert map to struct
	// For simplicity, we'll skip this for now
	return nil
}
