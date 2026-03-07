package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		RAG: RAGConfig{
			TopK:             5,
			ChunkSize:        1000,
			ChunkOverlap:     100,
			RAGFusionQueries: 4,
			RAGFusionWeight:  0.5,
		},
		Embedding: EmbeddingConfig{
			Provider: "openai",
			OpenAI: OpenAIConfig{
				APIKey:  "test-key",
				Model:   "text-embedding-ada-002",
				BaseURL: "https://api.openai.com/v1",
			},
		},
		LLM: LLMConfig{
			Provider: "openai",
			OpenAI: OpenAIConfig{
				APIKey:  "test-key",
				Model:   "gpt-4",
				BaseURL: "https://api.openai.com/v1",
			},
		},
		VectorStore: VectorStoreConfig{
			Type: "memory",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	err := Validate(cfg)
	assert.NoError(t, err)
}

func TestValidate_RAGConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       RAGConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid config",
			cfg: RAGConfig{
				TopK:             5,
				ChunkSize:        1000,
				ChunkOverlap:     100,
				RAGFusionQueries: 4,
				RAGFusionWeight:  0.5,
			},
			wantError: false,
		},
		{
			name: "invalid topK",
			cfg: RAGConfig{
				TopK:             0,
				ChunkSize:        1000,
				ChunkOverlap:     100,
				RAGFusionQueries: 4,
				RAGFusionWeight:  0.5,
			},
			wantError: true,
			errorMsg:  "topK",
		},
		{
			name: "invalid chunkSize",
			cfg: RAGConfig{
				TopK:             5,
				ChunkSize:        0,
				ChunkOverlap:     100,
				RAGFusionQueries: 4,
				RAGFusionWeight:  0.5,
			},
			wantError: true,
			errorMsg:  "chunkSize",
		},
		{
			name: "chunkOverlap >= chunkSize",
			cfg: RAGConfig{
				TopK:             5,
				ChunkSize:        1000,
				ChunkOverlap:     1000,
				RAGFusionQueries: 4,
				RAGFusionWeight:  0.5,
			},
			wantError: true,
			errorMsg:  "chunkOverlap",
		},
		{
			name: "invalid ragFusionWeight",
			cfg: RAGConfig{
				TopK:             5,
				ChunkSize:        1000,
				ChunkOverlap:     100,
				RAGFusionQueries: 4,
				RAGFusionWeight:  1.5,
			},
			wantError: true,
			errorMsg:  "ragFusionWeight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				RAG: tt.cfg,
				Embedding: EmbeddingConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test",
						Model:  "test",
					},
				},
				LLM: LLMConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test",
						Model:  "test",
					},
				},
				VectorStore: VectorStoreConfig{
					Type: "memory",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			err := Validate(cfg)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_EmbeddingConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       EmbeddingConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid openai",
			cfg: EmbeddingConfig{
				Provider: "openai",
				OpenAI: OpenAIConfig{
					APIKey: "test-key",
					Model:  "text-embedding-ada-002",
				},
			},
			wantError: false,
		},
		{
			name: "invalid provider",
			cfg: EmbeddingConfig{
				Provider: "invalid",
			},
			wantError: true,
			errorMsg:  "provider",
		},
		{
			name: "missing API key",
			cfg: EmbeddingConfig{
				Provider: "openai",
				OpenAI: OpenAIConfig{
					Model: "test",
				},
			},
			wantError: true,
			errorMsg:  "apiKey",
		},
		{
			name: "missing model",
			cfg: EmbeddingConfig{
				Provider: "openai",
				OpenAI: OpenAIConfig{
					APIKey: "test",
				},
			},
			wantError: true,
			errorMsg:  "model",
		},
		{
			name: "invalid base URL",
			cfg: EmbeddingConfig{
				Provider: "openai",
				OpenAI: OpenAIConfig{
					APIKey:  "test",
					Model:   "test",
					BaseURL: "invalid-url",
				},
			},
			wantError: true,
			errorMsg:  "baseURL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				RAG: RAGConfig{
					TopK:             5,
					ChunkSize:        1000,
					ChunkOverlap:     100,
					RAGFusionQueries: 4,
					RAGFusionWeight:  0.5,
				},
				Embedding: tt.cfg,
				LLM: LLMConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test",
						Model:  "test",
					},
				},
				VectorStore: VectorStoreConfig{
					Type: "memory",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			err := Validate(cfg)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_VectorStoreConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       VectorStoreConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid memory",
			cfg: VectorStoreConfig{
				Type: "memory",
			},
			wantError: false,
		},
		{
			name: "valid milvus",
			cfg: VectorStoreConfig{
				Type: "milvus",
				Milvus: MilvusConfig{
					Host: "localhost",
					Port: "19530",
				},
			},
			wantError: false,
		},
		{
			name: "invalid type",
			cfg: VectorStoreConfig{
				Type: "invalid",
			},
			wantError: true,
			errorMsg:  "type",
		},
		{
			name: "milvus missing host",
			cfg: VectorStoreConfig{
				Type: "milvus",
				Milvus: MilvusConfig{
					Port: "19530",
				},
			},
			wantError: true,
			errorMsg:  "host",
		},
		{
			name: "milvus missing port",
			cfg: VectorStoreConfig{
				Type: "milvus",
				Milvus: MilvusConfig{
					Host: "localhost",
					Port: "",
				},
			},
			wantError: true,
			errorMsg:  "port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				RAG: RAGConfig{
					TopK:             5,
					ChunkSize:        1000,
					ChunkOverlap:     100,
					RAGFusionQueries: 4,
					RAGFusionWeight:  0.5,
				},
				Embedding: EmbeddingConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test",
						Model:  "test",
					},
				},
				LLM: LLMConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test",
						Model:  "test",
					},
				},
				VectorStore: tt.cfg,
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			err := Validate(cfg)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_LoggingConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       LoggingConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid config",
			cfg: LoggingConfig{
				Level:  "info",
				Format: "json",
			},
			wantError: false,
		},
		{
			name: "invalid level",
			cfg: LoggingConfig{
				Level:  "invalid",
				Format: "json",
			},
			wantError: true,
			errorMsg:  "level",
		},
		{
			name: "invalid format",
			cfg: LoggingConfig{
				Level:  "info",
				Format: "invalid",
			},
			wantError: true,
			errorMsg:  "format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				RAG: RAGConfig{
					TopK:             5,
					ChunkSize:        1000,
					ChunkOverlap:     100,
					RAGFusionQueries: 4,
					RAGFusionWeight:  0.5,
				},
				Embedding: EmbeddingConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test",
						Model:  "test",
					},
				},
				LLM: LLMConfig{
					Provider: "openai",
					OpenAI: OpenAIConfig{
						APIKey: "test",
						Model:  "test",
					},
				},
				VectorStore: VectorStoreConfig{
					Type: "memory",
				},
				Logging: tt.cfg,
			}

			err := Validate(cfg)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_MultipleErrors(t *testing.T) {
	cfg := &Config{
		RAG: RAGConfig{
			TopK:             0, // Invalid
			ChunkSize:        0, // Invalid
			ChunkOverlap:     100,
			RAGFusionQueries: 4,
			RAGFusionWeight:  1.5, // Invalid
		},
		Embedding: EmbeddingConfig{
			Provider: "invalid", // Invalid
		},
		LLM: LLMConfig{
			Provider: "openai",
			OpenAI: OpenAIConfig{
				// Missing APIKey and Model
			},
		},
		VectorStore: VectorStoreConfig{
			Type: "memory",
		},
		Logging: LoggingConfig{
			Level:  "invalid", // Invalid
			Format: "json",
		},
	}

	err := Validate(cfg)
	require.Error(t, err)

	// Should contain multiple error messages
	errMsg := err.Error()
	assert.Contains(t, errMsg, "topK")
	assert.Contains(t, errMsg, "chunkSize")
	assert.Contains(t, errMsg, "ragFusionWeight")
	assert.Contains(t, errMsg, "provider")
	assert.Contains(t, errMsg, "apiKey")
	assert.Contains(t, errMsg, "model")
	assert.Contains(t, errMsg, "level")
}

func TestValidateURL(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		url       string
		wantError bool
	}{
		{
			name:      "valid http URL",
			url:       "http://localhost:8080",
			wantError: false,
		},
		{
			name:      "valid https URL",
			url:       "https://api.example.com/v1",
			wantError: false,
		},
		{
			name:      "empty URL",
			url:       "",
			wantError: true,
		},
		{
			name:      "invalid scheme",
			url:       "ftp://example.com",
			wantError: true,
		},
		{
			name:      "no host",
			url:       "http://",
			wantError: true,
		},
		{
			name:      "invalid format",
			url:       "not-a-url",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.validateURL(tt.url)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
