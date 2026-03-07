package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader("config.yaml")
	assert.NotNil(t, loader)
	assert.Equal(t, "config.yaml", loader.configPath)
}

func TestLoader_Load_EmptyPath(t *testing.T) {
	loader := NewLoader("")
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoader_Load_FileNotFound(t *testing.T) {
	loader := NewLoader("/nonexistent/path/config.yaml")
	_, err := loader.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestLoader_Load_ValidYAML(t *testing.T) {
	// Create temp YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
rag:
  topK: 10
  chunkSize: 500
  chunkOverlap: 50
  useSemanticChunking: true
  useHyDE: false
  useRAGFusion: true
  ragFusionQueries: 3
  ragFusionWeight: 0.6
embedding:
  provider: openai
  openai:
    apiKey: test-key
    model: text-embedding-ada-002
    baseURL: https://api.openai.com/v1
llm:
  provider: anthropic
  anthropic:
    apiKey: test-anthropic-key
    model: claude-3-opus-20240229
vectorstore:
  type: memory
logging:
  level: debug
  format: text
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	loader := NewLoader(configPath)
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, 10, cfg.RAG.TopK)
	assert.Equal(t, 500, cfg.RAG.ChunkSize)
	assert.Equal(t, 50, cfg.RAG.ChunkOverlap)
	assert.True(t, cfg.RAG.UseSemanticChunking)
	assert.False(t, cfg.RAG.UseHyDE)
	assert.True(t, cfg.RAG.UseRAGFusion)
	assert.Equal(t, 3, cfg.RAG.RAGFusionQueries)
	assert.InDelta(t, 0.6, cfg.RAG.RAGFusionWeight, 0.001)

	assert.Equal(t, "openai", cfg.Embedding.Provider)
	assert.Equal(t, "test-key", cfg.Embedding.OpenAI.APIKey)
	assert.Equal(t, "text-embedding-ada-002", cfg.Embedding.OpenAI.Model)

	assert.Equal(t, "anthropic", cfg.LLM.Provider)
	assert.Equal(t, "test-anthropic-key", cfg.LLM.Anthropic.APIKey)

	assert.Equal(t, "memory", cfg.VectorStore.Type)

	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
}

func TestLoader_Load_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte("invalid: yaml: content: [broken"), 0644)
	require.NoError(t, err)

	loader := NewLoader(configPath)
	_, err = loader.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal config")
}

func TestLoader_LoadFromEnv(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"GORAG_EMBEDDING_PROVIDER": "cohere",
		"GORAG_OPENAI_API_KEY":     "env-openai-key",
		"GORAG_COHERE_API_KEY":     "env-cohere-key",
		"GORAG_VOYAGE_API_KEY":     "env-voyage-key",
		"GORAG_LLM_PROVIDER":       "anthropic",
		"GORAG_ANTHROPIC_API_KEY":  "env-anthropic-key",
		"GORAG_VECTORSTORE_TYPE":   "milvus",
		"GORAG_PINECONE_API_KEY":   "env-pinecone-key",
	}

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	loader := NewLoader("")
	cfg, err := loader.Load()
	require.NoError(t, err)

	assert.Equal(t, "cohere", cfg.Embedding.Provider)
	assert.Equal(t, "env-openai-key", cfg.Embedding.OpenAI.APIKey)
	assert.Equal(t, "env-openai-key", cfg.LLM.OpenAI.APIKey)
	assert.Equal(t, "env-cohere-key", cfg.Embedding.Cohere.APIKey)
	assert.Equal(t, "env-voyage-key", cfg.Embedding.Voyage.APIKey)
	assert.Equal(t, "anthropic", cfg.LLM.Provider)
	assert.Equal(t, "env-anthropic-key", cfg.LLM.Anthropic.APIKey)
	assert.Equal(t, "milvus", cfg.VectorStore.Type)
	assert.Equal(t, "env-pinecone-key", cfg.VectorStore.Pinecone.APIKey)
}

func TestLoader_EnvOverridesFile(t *testing.T) {
	// Create temp YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
embedding:
  provider: openai
  openai:
    apiKey: file-key
llm:
  provider: openai
vectorstore:
  type: memory
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Set env to override
	t.Setenv("GORAG_EMBEDDING_PROVIDER", "ollama")
	t.Setenv("GORAG_OPENAI_API_KEY", "env-key")

	loader := NewLoader(configPath)
	cfg, err := loader.Load()
	require.NoError(t, err)

	// Env should override file
	assert.Equal(t, "ollama", cfg.Embedding.Provider)
	assert.Equal(t, "env-key", cfg.Embedding.OpenAI.APIKey)
}

func TestGetDefaultConfigPath(t *testing.T) {
	// This test depends on the current directory
	path := GetDefaultConfigPath()
	// Should return either "config.yaml", "gorag.yaml", or ""
	assert.Contains(t, []string{"config.yaml", "gorag.yaml", ""}, path)
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "env set",
			key:          "GORAG_TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "env not set",
			key:          "GORAG_TEST_VAR_UNSET",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}
			result := GetEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "long key",
			apiKey:   "sk-1234567890abcdef",
			expected: "sk-1****cdef",
		},
		{
			name:     "short key",
			apiKey:   "short",
			expected: "****",
		},
		{
			name:     "exactly 8 chars",
			apiKey:   "12345678",
			expected: "1234****5678",
		},
		{
			name:     "empty key",
			apiKey:   "",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeAPIKey(tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_ToMap(t *testing.T) {
	cfg := &Config{
		RAG: RAGConfig{
			TopK:      5,
			ChunkSize: 1000,
		},
		Embedding: EmbeddingConfig{
			Provider: "openai",
		},
		LLM: LLMConfig{
			Provider: "openai",
		},
		VectorStore: VectorStoreConfig{
			Type: "memory",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	m := cfg.ToMap()
	assert.NotNil(t, m)
	assert.Contains(t, m, "rag")
	assert.Contains(t, m, "embedding")
	assert.Contains(t, m, "llm")
	assert.Contains(t, m, "vectorstore")
	assert.Contains(t, m, "logging")
}

func TestConfig_FromMap(t *testing.T) {
	cfg := &Config{}
	err := cfg.FromMap(map[string]interface{}{})
	assert.NoError(t, err)
}

func TestConfig_ValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid openai config",
			cfg: &Config{
				Embedding: EmbeddingConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test-key",
					},
				},
				LLM: LLMConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test-key",
					},
				},
				VectorStore: VectorStoreConfig{
					Type: "memory",
				},
			},
			wantError: false,
		},
		{
			name: "missing openai embedding key",
			cfg: &Config{
				Embedding: EmbeddingConfig{
					Provider: "openai",
				},
				LLM: LLMConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test-key",
					},
				},
				VectorStore: VectorStoreConfig{
					Type: "memory",
				},
			},
			wantError: true,
			errorMsg:  "OpenAI API key",
		},
		{
			name: "missing llm openai key",
			cfg: &Config{
				Embedding: EmbeddingConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test-key",
					},
				},
				LLM: LLMConfig{
					Provider: "openai",
				},
				VectorStore: VectorStoreConfig{
					Type: "memory",
				},
			},
			wantError: true,
			errorMsg:  "OpenAI API key",
		},
		{
			name: "missing anthropic key",
			cfg: &Config{
				Embedding: EmbeddingConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test-key",
					},
				},
				LLM: LLMConfig{
					Provider: "anthropic",
				},
				VectorStore: VectorStoreConfig{
					Type: "memory",
				},
			},
			wantError: true,
			errorMsg:  "Anthropic API key",
		},
		{
			name: "missing pinecone key",
			cfg: &Config{
				Embedding: EmbeddingConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test-key",
					},
				},
				LLM: LLMConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test-key",
					},
				},
				VectorStore: VectorStoreConfig{
					Type: "pinecone",
				},
			},
			wantError: true,
			errorMsg:  "Pinecone API key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
