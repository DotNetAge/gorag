package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/DotNetAge/gorag/errors"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validator validates configuration
type Validator struct {
	errors []ValidationError
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		errors: []ValidationError{},
	}
}

// AddError adds a validation error
func (v *Validator) AddError(field, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Errors returns all validation errors
func (v *Validator) Errors() []ValidationError {
	return v.errors
}

// Error returns a formatted error message
func (v *Validator) Error() *errors.GoRAGError {
	if !v.HasErrors() {
		return nil
	}

	var messages []string
	for _, err := range v.errors {
		messages = append(messages, fmt.Sprintf("  • %s", err.Error()))
	}

	return errors.ErrConfiguration(fmt.Sprintf("Configuration validation failed:\n%s",
		strings.Join(messages, "\n")))
}

// Validate validates the entire configuration
func Validate(cfg *Config) error {
	v := NewValidator()

	// Validate RAG config
	v.validateRAG(&cfg.RAG)

	// Validate embedding config
	v.validateEmbedding(&cfg.Embedding)

	// Validate LLM config
	v.validateLLM(&cfg.LLM)

	// Validate vector store config
	v.validateVectorStore(&cfg.VectorStore)

	// Validate logging config
	v.validateLogging(&cfg.Logging)

	if v.HasErrors() {
		return v.Error()
	}

	return nil
}

// validateRAG validates RAG configuration
func (v *Validator) validateRAG(cfg *RAGConfig) {
	if cfg.TopK <= 0 {
		v.AddError("rag.topK", "must be greater than 0")
	}

	if cfg.ChunkSize <= 0 {
		v.AddError("rag.chunkSize", "must be greater than 0")
	}

	if cfg.ChunkOverlap < 0 {
		v.AddError("rag.chunkOverlap", "must be non-negative")
	}

	if cfg.ChunkOverlap >= cfg.ChunkSize {
		v.AddError("rag.chunkOverlap", "must be less than chunkSize")
	}

	if cfg.RAGFusionQueries <= 0 {
		v.AddError("rag.ragFusionQueries", "must be greater than 0")
	}

	if cfg.RAGFusionWeight < 0 || cfg.RAGFusionWeight > 1 {
		v.AddError("rag.ragFusionWeight", "must be between 0 and 1")
	}
}

// validateEmbedding validates embedding configuration
func (v *Validator) validateEmbedding(cfg *EmbeddingConfig) {
	// Validate provider
	validProviders := []string{"openai", "ollama", "cohere", "voyage"}
	if !contains(validProviders, cfg.Provider) {
		v.AddError("embedding.provider",
			fmt.Sprintf("must be one of: %s", strings.Join(validProviders, ", ")))
	}

	// Validate provider-specific config
	switch cfg.Provider {
	case "openai":
		v.validateOpenAI("embedding.openai", &cfg.OpenAI)
	case "ollama":
		v.validateOllama("embedding.ollama", &cfg.Ollama)
	case "cohere":
		v.validateCohere("embedding.cohere", &cfg.Cohere)
	case "voyage":
		v.validateVoyage("embedding.voyage", &cfg.Voyage)
	}
}

// validateLLM validates LLM configuration
func (v *Validator) validateLLM(cfg *LLMConfig) {
	// Validate provider
	validProviders := []string{"openai", "anthropic", "ollama", "azure_openai", "compatible"}
	if !contains(validProviders, cfg.Provider) {
		v.AddError("llm.provider",
			fmt.Sprintf("must be one of: %s", strings.Join(validProviders, ", ")))
	}

	// Validate provider-specific config
	switch cfg.Provider {
	case "openai":
		v.validateOpenAI("llm.openai", &cfg.OpenAI)
	case "anthropic":
		v.validateAnthropic("llm.anthropic", &cfg.Anthropic)
	case "ollama":
		v.validateOllama("llm.ollama", &cfg.Ollama)
	case "azure_openai":
		v.validateAzureOpenAI("llm.azure_openai", &cfg.AzureOpenAI)
	}
}

// validateVectorStore validates vector store configuration
func (v *Validator) validateVectorStore(cfg *VectorStoreConfig) {
	// Validate type
	validTypes := []string{"memory", "milvus", "qdrant", "weaviate", "pinecone", "govector"}
	if !contains(validTypes, cfg.Type) {
		v.AddError("vectorstore.type",
			fmt.Sprintf("must be one of: %s", strings.Join(validTypes, ", ")))
	}

	// Validate type-specific config
	switch cfg.Type {
	case "milvus":
		v.validateMilvus(&cfg.Milvus)
	case "qdrant":
		v.validateQdrant(&cfg.Qdrant)
	case "weaviate":
		v.validateWeaviate(&cfg.Weaviate)
	case "pinecone":
	case "govector":
		v.validateGoVector(&cfg.GoVector)
		v.validatePinecone(&cfg.Pinecone)
	}
}

// validateLogging validates logging configuration
func (v *Validator) validateLogging(cfg *LoggingConfig) {
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, cfg.Level) {
		v.AddError("logging.level",
			fmt.Sprintf("must be one of: %s", strings.Join(validLevels, ", ")))
	}

	validFormats := []string{"json", "text"}
	if !contains(validFormats, cfg.Format) {
		v.AddError("logging.format",
			fmt.Sprintf("must be one of: %s", strings.Join(validFormats, ", ")))
	}
}

// Provider-specific validators

func (v *Validator) validateOpenAI(prefix string, cfg *OpenAIConfig) {
	if cfg.APIKey == "" {
		v.AddError(prefix+".apiKey", "is required")
	}

	if cfg.Model == "" {
		v.AddError(prefix+".model", "is required")
	}

	if cfg.BaseURL != "" {
		if err := v.validateURL(cfg.BaseURL); err != nil {
			v.AddError(prefix+".baseURL", err.Error())
		}
	}
}

func (v *Validator) validateAnthropic(prefix string, cfg *AnthropicConfig) {
	if cfg.APIKey == "" {
		v.AddError(prefix+".apiKey", "is required")
	}

	if cfg.Model == "" {
		v.AddError(prefix+".model", "is required")
	}

	if cfg.BaseURL != "" {
		if err := v.validateURL(cfg.BaseURL); err != nil {
			v.AddError(prefix+".baseURL", err.Error())
		}
	}
}

func (v *Validator) validateOllama(prefix string, cfg *OllamaConfig) {
	if cfg.Model == "" {
		v.AddError(prefix+".model", "is required")
	}

	if cfg.BaseURL == "" {
		v.AddError(prefix+".baseURL", "is required")
	} else {
		if err := v.validateURL(cfg.BaseURL); err != nil {
			v.AddError(prefix+".baseURL", err.Error())
		}
	}
}

func (v *Validator) validateAzureOpenAI(prefix string, cfg *AzureOpenAIConfig) {
	if cfg.APIKey == "" {
		v.AddError(prefix+".apiKey", "is required")
	}

	if cfg.Model == "" {
		v.AddError(prefix+".model", "is required")
	}

	if cfg.Endpoint == "" {
		v.AddError(prefix+".endpoint", "is required")
	} else {
		if err := v.validateURL(cfg.Endpoint); err != nil {
			v.AddError(prefix+".endpoint", err.Error())
		}
	}

	if cfg.APIVersion == "" {
		v.AddError(prefix+".apiVersion", "is required")
	}
}

func (v *Validator) validateCohere(prefix string, cfg *CohereConfig) {
	if cfg.APIKey == "" {
		v.AddError(prefix+".apiKey", "is required")
	}

	if cfg.Model == "" {
		v.AddError(prefix+".model", "is required")
	}

	if cfg.BaseURL != "" {
		if err := v.validateURL(cfg.BaseURL); err != nil {
			v.AddError(prefix+".baseURL", err.Error())
		}
	}
}

func (v *Validator) validateVoyage(prefix string, cfg *VoyageConfig) {
	if cfg.APIKey == "" {
		v.AddError(prefix+".apiKey", "is required")
	}

	if cfg.Model == "" {
		v.AddError(prefix+".model", "is required")
	}

	if cfg.BaseURL != "" {
		if err := v.validateURL(cfg.BaseURL); err != nil {
			v.AddError(prefix+".baseURL", err.Error())
		}
	}
}

func (v *Validator) validateMilvus(cfg *MilvusConfig) {
	if cfg.Host == "" {
		v.AddError("vectorstore.milvus.host", "is required")
	}

	if cfg.Port == "" {
		v.AddError("vectorstore.milvus.port", "is required")
	}
}

func (v *Validator) validateQdrant(cfg *QdrantConfig) {
	if cfg.URL == "" {
		v.AddError("vectorstore.qdrant.url", "is required")
	} else {
		if err := v.validateURL(cfg.URL); err != nil {
			v.AddError("vectorstore.qdrant.url", err.Error())
		}
	}
}

func (v *Validator) validateWeaviate(cfg *WeaviateConfig) {
	if cfg.URL == "" {
		v.AddError("vectorstore.weaviate.url", "is required")
	} else {
		if err := v.validateURL(cfg.URL); err != nil {
			v.AddError("vectorstore.weaviate.url", err.Error())
		}
	}
}

func (v *Validator) validatePinecone(cfg *PineconeConfig) {
	if cfg.APIKey == "" {
		v.AddError("vectorstore.pinecone.apiKey", "is required")
	}

	if cfg.Environment == "" {
		v.AddError("vectorstore.pinecone.environment", "is required")
	}
}

// Helper functions

func (v *Validator) validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (v *Validator) validateGoVector(cfg *GoVectorConfig) {
	if cfg.DBPath == "" {
		v.AddError("vectorstore.govector.dbPath", "is required")
	}
	if cfg.Dimension <= 0 {
		v.AddError("vectorstore.govector.dimension", "must be greater than 0")
	}
	if cfg.Collection == "" {
		v.AddError("vectorstore.govector.collection", "is required")
	}
}
