package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_SetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// Test RAG defaults
	assert.Equal(t, 5, cfg.RAG.TopK)
	assert.Equal(t, 1000, cfg.RAG.ChunkSize)
	assert.Equal(t, 100, cfg.RAG.ChunkOverlap)
	assert.Equal(t, 4, cfg.RAG.RAGFusionQueries)
	assert.InDelta(t, 0.5, cfg.RAG.RAGFusionWeight, 0.001)

	// Test Embedding defaults
	assert.Equal(t, "openai", cfg.Embedding.Provider)
	assert.Equal(t, "text-embedding-ada-002", cfg.Embedding.OpenAI.Model)
	assert.Equal(t, "https://api.openai.com/v1", cfg.Embedding.OpenAI.BaseURL)
	assert.Equal(t, "qllama/bge-small-zh-v1.5:latest", cfg.Embedding.Ollama.Model)
	assert.Equal(t, "http://localhost:11434", cfg.Embedding.Ollama.BaseURL)
	assert.Equal(t, "embed-english-v3.0", cfg.Embedding.Cohere.Model)
	assert.Equal(t, "https://api.cohere.ai/v1", cfg.Embedding.Cohere.BaseURL)
	assert.Equal(t, "voyage-2", cfg.Embedding.Voyage.Model)
	assert.Equal(t, "https://api.voyageai.com/v1", cfg.Embedding.Voyage.BaseURL)

	// Test LLM defaults
	assert.Equal(t, "openai", cfg.LLM.Provider)
	assert.Equal(t, "gpt-3.5-turbo", cfg.LLM.OpenAI.Model)
	assert.Equal(t, "https://api.openai.com/v1", cfg.LLM.OpenAI.BaseURL)
	assert.Equal(t, "claude-3-opus-20240229", cfg.LLM.Anthropic.Model)
	assert.Equal(t, "https://api.anthropic.com/v1", cfg.LLM.Anthropic.BaseURL)
	assert.Equal(t, "qwen3:7b", cfg.LLM.Ollama.Model)
	assert.Equal(t, "http://localhost:11434", cfg.LLM.Ollama.BaseURL)
	assert.Equal(t, "2024-03-01-preview", cfg.LLM.AzureOpenAI.APIVersion)

	// Test VectorStore defaults
	assert.Equal(t, "memory", cfg.VectorStore.Type)
	assert.Equal(t, "localhost", cfg.VectorStore.Milvus.Host)
	assert.Equal(t, "19530", cfg.VectorStore.Milvus.Port)
	assert.Equal(t, "default", cfg.VectorStore.Milvus.Database)
	assert.Equal(t, "http://localhost:6333", cfg.VectorStore.Qdrant.URL)
	assert.Equal(t, "http://localhost:8080", cfg.VectorStore.Weaviate.URL)
	assert.Equal(t, 10000, cfg.VectorStore.Memory.MaxSize)

	// Test Logging defaults
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
}

func TestConfig_SetDefaults_PartialConfig(t *testing.T) {
	// Test that SetDefaults only sets missing values
	cfg := &Config{
		RAG: RAGConfig{
			TopK:      10, // Custom value
			ChunkSize: 0,  // Should be set to default
		},
		Embedding: EmbeddingConfig{
			Provider: "cohere", // Custom value
			OpenAI: OpenAIConfig{
				Model: "custom-model", // Custom value
			},
		},
		LLM: LLMConfig{
			Provider: "anthropic", // Custom value
		},
		VectorStore: VectorStoreConfig{
			Type: "milvus", // Custom value
		},
		Logging: LoggingConfig{
			Level: "debug", // Custom value
		},
	}

	cfg.SetDefaults()

	// Custom values should be preserved
	assert.Equal(t, 10, cfg.RAG.TopK)
	assert.Equal(t, "cohere", cfg.Embedding.Provider)
	assert.Equal(t, "custom-model", cfg.Embedding.OpenAI.Model)
	assert.Equal(t, "anthropic", cfg.LLM.Provider)
	assert.Equal(t, "milvus", cfg.VectorStore.Type)
	assert.Equal(t, "debug", cfg.Logging.Level)

	// Missing values should be set to defaults
	assert.Equal(t, 1000, cfg.RAG.ChunkSize)
	assert.Equal(t, "https://api.openai.com/v1", cfg.Embedding.OpenAI.BaseURL)
	assert.Equal(t, "json", cfg.Logging.Format)
}

func TestConfig_SetDefaults_Idempotent(t *testing.T) {
	// Test that calling SetDefaults multiple times is safe
	cfg := &Config{}

	cfg.SetDefaults()
	firstTopK := cfg.RAG.TopK
	firstProvider := cfg.Embedding.Provider

	cfg.SetDefaults()
	secondTopK := cfg.RAG.TopK
	secondProvider := cfg.Embedding.Provider

	assert.Equal(t, firstTopK, secondTopK)
	assert.Equal(t, firstProvider, secondProvider)
}
