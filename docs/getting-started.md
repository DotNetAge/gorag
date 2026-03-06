# Getting Started with GoRAG

## Installation

```bash
go get github.com/DotNetAge/gorag
```

## Basic Usage

### 1. Create a RAG Engine

```go
import (
    "context"
    "log"
    
    "github.com/DotNetAge/gorag/rag"
    "github.com/DotNetAge/gorag/parser/text"
    "github.com/DotNetAge/gorag/vectorstore/memory"
    "github.com/DotNetAge/gorag/embedding/openai"
    "github.com/DotNetAge/gorag/llm/openai"
)

ctx := context.Background()

// Create RAG engine
engine, err := rag.New(
    rag.WithParser(text.NewParser()),
    rag.WithVectorStore(memory.NewStore()),
    rag.WithEmbedder(openai.NewEmbedder("your-api-key")),
    rag.WithLLM(openai.NewClient("your-api-key")),
)
if err != nil {
    log.Fatal(err)
}
```

### 2. Index Documents

```go
// Index text content
err := engine.Index(ctx, rag.Source{
    Type:    "text",
    Content: "Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.",
})

// Index multiple documents
documents := []string{
    "RAG (Retrieval-Augmented Generation) is an AI technique that combines information retrieval with text generation.",
    "Vector databases are specialized databases designed to store and query high-dimensional vectors efficiently.",
}

for _, doc := range documents {
    err := engine.Index(ctx, rag.Source{
        Type:    "text",
        Content: doc,
    })
    if err != nil {
        log.Printf("Error indexing document: %v", err)
    }
}
```

### 3. Query

```go
// Basic query
resp, err := engine.Query(ctx, "What is Go?", rag.QueryOptions{
    TopK: 5,
})

if err != nil {
    log.Fatal(err)
}

fmt.Println("Answer:", resp.Answer)
fmt.Println("Sources:")
for i, source := range resp.Sources {
    fmt.Printf("[%d] Score: %.4f - %s...\n", i+1, source.Score, source.Content[:50])
}

// Query with custom prompt template
resp, err = engine.Query(ctx, "How does RAG work?", rag.QueryOptions{
    TopK: 3,
    PromptTemplate: "Answer the question based on the following context:\n\n{context}\n\nQuestion: {question}\nAnswer:",
})
```

## Components

### Document Parsers

- **Text Parser**: Plain text and markdown
- **PDF Parser**: PDF documents (coming soon)
- **DOCX Parser**: Word documents (coming soon)
- **HTML Parser**: HTML documents (coming soon)

### Vector Stores

- **Memory Store**: In-memory for testing and development
- **Pinecone**: Cloud vector database (coming soon)
- **Weaviate**: Open source vector database (coming soon)
- **Milvus**: Open source vector database (coming soon)
- **Qdrant**: Open source vector database (coming soon)

### Embedding Providers

- **OpenAI**: OpenAI embeddings (text-embedding-ada-002)
- **Ollama**: Local models (coming soon)
- **Hugging Face**: Open source models (coming soon)

### LLM Clients

- **OpenAI**: GPT models (GPT-3.5, GPT-4)
- **Anthropic**: Claude models (coming soon)
- **Ollama**: Local models (coming soon)

## Examples

See the [examples/](../examples/) directory for more examples:

- [Basic Example](../examples/basic/) - Simple RAG usage with mock implementations
- [Advanced Example](../examples/advanced/) - Advanced RAG with OpenAI integration
- [Web Service Example](../examples/web/) - RESTful API service

## Next Steps

- Read the [API Reference](api.md)
- Check out more [Examples](../examples/)
- Explore the [Core Concepts](core-concepts.md)
- Learn about [Advanced Retrieval Strategies](advanced-retrieval.md)
