# API Reference

## Core Interfaces

### Parser

```go
type Parser interface {
    Parse(ctx context.Context, r io.Reader) ([]Chunk, error)
    SupportedFormats() []string
}
```

### VectorStore

```go
type Store interface {
    Add(ctx context.Context, chunks []Chunk, embeddings [][]float32) error
    Search(ctx context.Context, query []float32, opts SearchOptions) ([]Result, error)
    Delete(ctx context.Context, ids []string) error
}
```

### EmbeddingProvider

```go
type Provider interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
}
```

### LLMClient

```go
type Client interface {
    Complete(ctx context.Context, prompt string) (string, error)
    CompleteStream(ctx context.Context, prompt string) (<-chan string, error)
}
```

## RAG Engine

### New

```go
func New(opts ...Option) (*Engine, error)
```

Creates a new RAG engine with the given options.

### Options

- `WithParser(parser core.Parser)` - Set the document parser
- `WithVectorStore(store core.VectorStore)` - Set the vector store
- `WithEmbedder(embedder core.EmbeddingProvider)` - Set the embedding provider
- `WithLLM(llm core.LLMClient)` - Set the LLM client

### Index

```go
func (e *Engine) Index(ctx context.Context, source core.Source) error
```

Adds documents to the RAG engine.

### Query

```go
func (e *Engine) Query(ctx context.Context, question string, opts QueryOptions) (*Response, error)
```

Performs a RAG query.

### QueryOptions

```go
type QueryOptions struct {
    TopK           int
    PromptTemplate string
    Stream         bool
}
```

### Response

```go
type Response struct {
    Answer  string
    Sources []core.Result
}
```

## Types

### Chunk

```go
type Chunk struct {
    ID       string
    Content  string
    Metadata map[string]string
}
```

### Result

```go
type Result struct {
    Chunk
    Score float32
}
```

### SearchOptions

```go
type SearchOptions struct {
    TopK      int
    Filter    map[string]interface{}
    MinScore  float32
}
```

### Source

```go
type Source struct {
    Type    string
    Path    string
    Content string
    Reader  interface{}
}
```
