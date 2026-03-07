# GoRAG Configuration Guide

## Overview

GoRAG provides a flexible configuration system that allows you to customize various aspects of the RAG engine through YAML files and environment variables. This guide explains all available configuration options and how to use them.

## Configuration Sources

GoRAG loads configuration from two sources, in order of precedence:

1. **Environment variables** - Override any configuration values
2. **YAML configuration file** - Main configuration source
3. **Default values** - Fallback when no other values are specified

## Configuration File Structure

Create a `config.yaml` file in one of the following locations:

- Current working directory: `./config.yaml`
- Config directory: `./config/config.yaml`
- User home directory: `~/.gorag/config.yaml`

### Example Configuration

```yaml
rag:
  topK: 5
  chunkSize: 1000
  chunkOverlap: 100
  useSemanticChunking: false
  useHyDE: false
  useRAGFusion: false
  useContextCompression: false
  ragFusionQueries: 4
  ragFusionWeight: 0.5

embedding:
  provider: "openai"
  openai:
    apiKey: "your-api-key"
    model: "text-embedding-ada-002"
    baseURL: "https://api.openai.com/v1"
  ollama:
    model: "qllama/bge-small-zh-v1.5:latest"
    baseURL: "http://localhost:11434"
  cohere:
    apiKey: "your-api-key"
    model: "embed-english-v3.0"
    baseURL: "https://api.cohere.ai/v1"
  voyage:
    apiKey: "your-api-key"
    model: "voyage-2"
    baseURL: "https://api.voyageai.com/v1"

llm:
  provider: "openai"
  openai:
    apiKey: "your-api-key"
    model: "gpt-3.5-turbo"
    baseURL: "https://api.openai.com/v1"
  anthropic:
    apiKey: "your-api-key"
    model: "claude-3-opus-20240229"
    baseURL: "https://api.anthropic.com/v1"
  ollama:
    model: "qwen3:7b"
    baseURL: "http://localhost:11434"
  azure_openai:
    apiKey: "your-api-key"
    model: "gpt-35-turbo"
    endpoint: "https://your-resource.openai.azure.com/"
    apiVersion: "2024-03-01-preview"

vectorstore:
  type: "memory"
  memory:
    maxSize: 10000
  milvus:
    host: "localhost"
    port: "19530"
    username: ""
    password: ""
    database: "default"
  qdrant:
    url: "http://localhost:6333"
    apiKey: ""
  weaviate:
    url: "http://localhost:8080"
    apiKey: ""
  pinecone:
    apiKey: "your-api-key"
    environment: "gcp-starter"

logging:
  level: "info"
  format: "json"
```

## Configuration Options

### RAG Configuration

| Option | Description | Default Value | Environment Variable |
|--------|-------------|---------------|----------------------|
| `topK` | Number of top results to retrieve | 5 | `GORAG_RAG_TOPK` |
| `chunkSize` | Size of document chunks in characters | 1000 | `GORAG_RAG_CHUNKSIZE` |
| `chunkOverlap` | Overlap between chunks in characters | 100 | `GORAG_RAG_CHUNKOVERLAP` |
| `useSemanticChunking` | Enable semantic chunking | false | `GORAG_RAG_USESEMANTICCHUNKING` |
| `useHyDE` | Enable Hypothetical Document Embeddings | false | `GORAG_RAG_USEHYDE` |
| `useRAGFusion` | Enable RAG-Fusion retrieval | false | `GORAG_RAG_USERAGFUSION` |
| `useContextCompression` | Enable context compression | false | `GORAG_RAG_USECONTEXTCOMPRESSION` |
| `ragFusionQueries` | Number of queries to generate for RAG-Fusion | 4 | `GORAG_RAG_RAGFUSIONQUERIES` |
| `ragFusionWeight` | Weight for RAG-Fusion results | 0.5 | `GORAG_RAG_RAGFUSIONWEIGHT` |

### Embedding Configuration

| Option | Description | Default Value | Environment Variable |
|--------|-------------|---------------|----------------------|
| `provider` | Embedding provider (openai, ollama, cohere, voyage) | openai | `GORAG_EMBEDDING_PROVIDER` |
| `openai.apiKey` | OpenAI API key | - | `GORAG_OPENAI_API_KEY` |
| `openai.model` | OpenAI embedding model | text-embedding-ada-002 | `GORAG_OPENAI_MODEL` |
| `openai.baseURL` | OpenAI API base URL | https://api.openai.com/v1 | `GORAG_OPENAI_BASEURL` |
| `ollama.model` | Ollama embedding model | qllama/bge-small-zh-v1.5:latest | `GORAG_OLLAMA_MODEL` |
| `ollama.baseURL` | Ollama API base URL | http://localhost:11434 | `GORAG_OLLAMA_BASEURL` |
| `cohere.apiKey` | Cohere API key | - | `GORAG_COHERE_API_KEY` |
| `cohere.model` | Cohere embedding model | embed-english-v3.0 | `GORAG_COHERE_MODEL` |
| `cohere.baseURL` | Cohere API base URL | https://api.cohere.ai/v1 | `GORAG_COHERE_BASEURL` |
| `voyage.apiKey` | Voyage API key | - | `GORAG_VOYAGE_API_KEY` |
| `voyage.model` | Voyage embedding model | voyage-2 | `GORAG_VOYAGE_MODEL` |
| `voyage.baseURL` | Voyage API base URL | https://api.voyageai.com/v1 | `GORAG_VOYAGE_BASEURL` |

### LLM Configuration

| Option | Description | Default Value | Environment Variable |
|--------|-------------|---------------|----------------------|
| `provider` | LLM provider (openai, anthropic, ollama, azure_openai) | openai | `GORAG_LLM_PROVIDER` |
| `openai.apiKey` | OpenAI API key | - | `GORAG_OPENAI_API_KEY` |
| `openai.model` | OpenAI LLM model | gpt-3.5-turbo | `GORAG_OPENAI_MODEL` |
| `openai.baseURL` | OpenAI API base URL | https://api.openai.com/v1 | `GORAG_OPENAI_BASEURL` |
| `anthropic.apiKey` | Anthropic API key | - | `GORAG_ANTHROPIC_API_KEY` |
| `anthropic.model` | Anthropic LLM model | claude-3-opus-20240229 | `GORAG_ANTHROPIC_MODEL` |
| `anthropic.baseURL` | Anthropic API base URL | https://api.anthropic.com/v1 | `GORAG_ANTHROPIC_BASEURL` |
| `ollama.model` | Ollama LLM model | qwen3:7b | `GORAG_OLLAMA_MODEL` |
| `ollama.baseURL` | Ollama API base URL | http://localhost:11434 | `GORAG_OLLAMA_BASEURL` |
| `azure_openai.apiKey` | Azure OpenAI API key | - | `GORAG_AZURE_OPENAI_API_KEY` |
| `azure_openai.model` | Azure OpenAI model | - | `GORAG_AZURE_OPENAI_MODEL` |
| `azure_openai.endpoint` | Azure OpenAI endpoint | - | `GORAG_AZURE_OPENAI_ENDPOINT` |
| `azure_openai.apiVersion` | Azure OpenAI API version | 2024-03-01-preview | `GORAG_AZURE_OPENAI_API_VERSION` |

### Vector Store Configuration

| Option | Description | Default Value | Environment Variable |
|--------|-------------|---------------|----------------------|
| `type` | Vector store type (memory, milvus, qdrant, weaviate, pinecone) | memory | `GORAG_VECTORSTORE_TYPE` |
| `memory.maxSize` | Maximum number of chunks in memory store | 10000 | `GORAG_VECTORSTORE_MEMORY_MAXSIZE` |
| `milvus.host` | Milvus server host | localhost | `GORAG_VECTORSTORE_MILVUS_HOST` |
| `milvus.port` | Milvus server port | 19530 | `GORAG_VECTORSTORE_MILVUS_PORT` |
| `milvus.username` | Milvus username | - | `GORAG_VECTORSTORE_MILVUS_USERNAME` |
| `milvus.password` | Milvus password | - | `GORAG_VECTORSTORE_MILVUS_PASSWORD` |
| `milvus.database` | Milvus database name | default | `GORAG_VECTORSTORE_MILVUS_DATABASE` |
| `qdrant.url` | Qdrant server URL | http://localhost:6333 | `GORAG_VECTORSTORE_QDRANT_URL` |
| `qdrant.apiKey` | Qdrant API key | - | `GORAG_VECTORSTORE_QDRANT_APIKEY` |
| `weaviate.url` | Weaviate server URL | http://localhost:8080 | `GORAG_VECTORSTORE_WEAVIATE_URL` |
| `weaviate.apiKey` | Weaviate API key | - | `GORAG_VECTORSTORE_WEAVIATE_APIKEY` |
| `pinecone.apiKey` | Pinecone API key | - | `GORAG_PINECONE_API_KEY` |
| `pinecone.environment` | Pinecone environment | - | `GORAG_PINECONE_ENVIRONMENT` |

### Logging Configuration

| Option | Description | Default Value | Environment Variable |
|--------|-------------|---------------|----------------------|
| `level` | Log level (debug, info, warn, error) | info | `GORAG_LOGGING_LEVEL` |
| `format` | Log format (json, text) | json | `GORAG_LOGGING_FORMAT` |

## Using Environment Variables

You can override any configuration value using environment variables. Environment variable names are derived from the configuration path, converted to uppercase, and prefixed with `GORAG_`.

### Example

```bash
# Set embedding provider to Cohere
export GORAG_EMBEDDING_PROVIDER=cohere

# Set Cohere API key
export GORAG_COHERE_API_KEY=your-api-key

# Enable semantic chunking
export GORAG_RAG_USESEMANTICCHUNKING=true

# Enable HyDE
export GORAG_RAG_USEHYDE=true
```

## Configuration Loading Order

1. **Default values** - Hardcoded in the config package
2. **YAML file** - Loaded from one of the config file locations
3. **Environment variables** - Override values from YAML file

## Validating Configuration

GoRAG automatically validates the configuration when loading it. It will check for required fields like API keys for the selected providers.

## Using Configuration in Code

You can load the configuration in your code using the config package:

```go
import "github.com/DotNetAge/gorag/config"

// Create a config loader
loader := config.NewLoader("path/to/config.yaml")

// Load the configuration
cfg, err := loader.Load()
if err != nil {
    log.Fatal(err)
}

// Set defaults (optional)
cfg.SetDefaults()

// Validate the configuration
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}

// Use the configuration
fmt.Printf("Embedding provider: %s\n", cfg.Embedding.Provider)
fmt.Printf("TopK: %d\n", cfg.RAG.TopK)
```

## Best Practices

1. **Use environment variables for sensitive information** - API keys and other secrets should be set via environment variables, not stored in YAML files

2. **Use YAML files for non-sensitive configuration** - Settings like chunk size, topK, and provider selection can be stored in YAML files

3. **Start with default values** - Most configuration options have reasonable default values that work well for common use cases

4. **Test different configurations** - Experiment with different values for `topK`, `chunkSize`, and other parameters to find the best configuration for your use case

5. **Enable advanced features gradually** - Features like HyDE, RAG-Fusion, and context compression can improve results but may increase latency

## Troubleshooting

### Configuration Not Loading

- Check that your config file is in one of the expected locations
- Verify that environment variables are set correctly
- Check for syntax errors in your YAML file

### API Key Errors

- Make sure you've set the correct API key for your selected provider
- Check that the API key has the necessary permissions
- Verify that the API key is not expired

### Performance Issues

- Adjust `chunkSize` and `chunkOverlap` for better performance
- Consider using a local embedding provider like Ollama for faster embeddings
- Disable resource-intensive features like RAG-Fusion if latency is a concern

## Conclusion

The GoRAG configuration system provides a flexible way to customize the RAG engine to your specific needs. By understanding and properly configuring the available options, you can optimize the performance and accuracy of your RAG applications.
