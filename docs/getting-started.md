# Getting Started with GoRAG

## Installation

```bash
go get github.com/raya-dev/gorag
```

## Basic Usage

### 1. Create a RAG Engine

```go
import (
    "github.com/raya-dev/gorag/rag"
    "github.com/raya-dev/gorag/parser/text"
    "github.com/raya-dev/gorag/vectorstore/memory"
    "github.com/raya-dev/gorag/embedding/openai"
    "github.com/raya-dev/gorag/llm/openai"
)

engine, err := rag.New(
    rag.WithParser(text.NewParser()),
    rag.WithVectorStore(memory.NewStore()),
    rag.WithEmbedder(openai.NewEmbedder(apiKey)),
    rag.WithLLM(openai.NewClient(apiKey)),
)
```

### 2. Index Documents

```go
err := engine.Index(ctx, rag.Source{
    Type:    "text",
    Content: "Your document content here...",
})
```

### 3. Query

```go
resp, err := engine.Query(ctx, "Your question here?", rag.QueryOptions{
    TopK: 5,
})

fmt.Println(resp.Answer)
for _, source := range resp.Sources {
    fmt.Printf("Source: %s (Score: %.2f)\n", source.Content, source.Score)
}
```

## Components

### Document Parsers

- **Text Parser**: Plain text and markdown
- **PDF Parser**: PDF documents (coming soon)
- **DOCX Parser**: Word documents (coming soon)

### Vector Stores

- **Memory Store**: In-memory for testing
- **Pinecone**: Cloud vector database (coming soon)
- **Weaviate**: Open source vector database (coming soon)

### Embedding Providers

- **OpenAI**: OpenAI embeddings (coming soon)
- **Ollama**: Local models (coming soon)

### LLM Clients

- **OpenAI**: GPT models (coming soon)
- **Anthropic**: Claude models (coming soon)

## Examples

See the [examples/](../examples/) directory for more examples:

- [Basic Example](../examples/basic/)
- [Advanced Example](../examples/advanced/)
- [Web Service Example](../examples/web/)

## Next Steps

- Read the [API Reference](api.md)
- Check out more [Examples](../examples/)
- Learn about [Advanced Topics](advanced.md)
