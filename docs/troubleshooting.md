# GoRAG Troubleshooting Guide

This guide helps you diagnose and resolve common issues when using GoRAG.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Configuration Issues](#configuration-issues)
- [Indexing Issues](#indexing-issues)
- [Query Issues](#query-issues)
- [Performance Issues](#performance-issues)
- [Vector Store Issues](#vector-store-issues)
- [API Issues](#api-issues)
- [Debugging](#debugging)

## Installation Issues

### Go Module Not Found

**Problem**: `go: module github.com/DotNetAge/gorag@latest found (v0.x.x), but does not contain package`

**Solution**:
```bash
# Clean module cache
go clean -modcache

# Update go.mod
go mod tidy

# Download dependencies
go mod download
```

### Build Errors

**Problem**: `undefined: rag.New` or similar build errors

**Solution**:
1. Ensure you're using Go 1.21 or later
2. Check imports:
```go
import "github.com/DotNetAge/gorag/rag"
```
3. Run `go mod tidy` to resolve dependencies

## Configuration Issues

### Missing API Key

**Problem**: `api key is required`

**Solution**:
```go
// Set environment variable
export OPENAI_API_KEY="your-api-key"

// Or pass in config
config := openai.Config{
    APIKey: "your-api-key",
}
```

### Invalid Configuration

**Problem**: `invalid configuration: chunkOverlap >= chunkSize`

**Solution**:
```go
config := rag.Config{
    ChunkSize:      1000,
    ChunkOverlap:   200, // Must be less than ChunkSize
    TopK:           5,
}
```

## Indexing Issues

### File Not Found

**Problem**: `failed to open file at path /path/to/file: open /path/to/file: no such file or directory`

**Solution**:
1. Check file path is correct
2. Ensure file exists:
```bash
ls -la /path/to/file
```
3. Use absolute path or resolve relative path:
```go
absPath, err := filepath.Abs("relative/path/to/file")
```

### Parser Not Found

**Problem**: `no parser found for extension .xyz`

**Solution**:
```go
// Register custom parser
engine.AddParser(".xyz", customParser)

// Or use default parser
engine.SetDefaultParser(textParser)
```

### Memory Issues During Indexing

**Problem**: Out of memory when indexing large files

**Solution**:
1. Use lazy loading:
```go
loader := lazyloader.NewLazyDocumentManager(100 * 1024 * 1024) // 100MB limit
```

2. Reduce worker count:
```go
config := indexing.IndexerConfig{
    WorkerCount: 5, // Default is 10
}
```

3. Process files in smaller batches

## Query Issues

### No Results Returned

**Problem**: Query returns empty results

**Solution**:
1. Check documents are indexed:
```go
// Verify indexing
err := engine.Index(ctx, source)
if err != nil {
    log.Printf("Indexing failed: %v", err)
}
```

2. Adjust TopK parameter:
```go
resp, err := engine.Query(ctx, question, rag.QueryOptions{
    TopK: 10, // Increase from default 5
})
```

3. Check similarity threshold in retriever configuration

### Slow Query Performance

**Problem**: Queries take too long

**Solution**:
1. Enable query caching:
```go
cache := rag.NewMemoryCache(5 * time.Minute)
engine.WithCache(cache)
```

2. Use hybrid retrieval with tuned weights:
```go
retriever := retrieval.NewHybridRetriever(
    vectorWeight: 0.7,
    keywordWeight: 0.3,
)
```

3. Enable connection pooling for vector stores

## Performance Issues

### High Memory Usage

**Problem**: Application uses too much memory

**Solution**:
1. Enable lazy loading for documents
2. Reduce batch sizes:
```go
config := embedding.Config{
    BatchSize: 50, // Default is 100
}
```

3. Unload documents when not needed:
```go
doc.Unload()
```

### Slow Indexing

**Problem**: Indexing large document collections is slow

**Solution**:
1. Use concurrent indexing:
```go
err := engine.IndexDirectory(ctx, "./documents")
```

2. Optimize embedding batch size:
```go
config := embedding.Config{
    BatchSize: 100, // Adjust based on API limits
}
```

3. Use local embedding models (Ollama) instead of API calls

## Vector Store Issues

### Connection Failed

**Problem**: Cannot connect to vector store

**Solution**:

**Pinecone**:
```go
store, err := pinecone.New(pinecone.Config{
    APIKey:   "your-api-key",
    IndexName: "your-index",
    Dimension: 1536,
})
```

**Milvus**:
```go
store, err := milvus.New(milvus.Config{
    Address: "localhost:19530",
    Dimension: 1536,
})
```

**Weaviate**:
```go
store, err := weaviate.New(weaviate.Config{
    Host:   "localhost:8080",
    Scheme: "http",
})
```

### Dimension Mismatch

**Problem**: `dimension mismatch: expected 1536, got 768`

**Solution**:
```go
// Ensure embedding dimension matches vector store
embedder, err := openai.New(openai.Config{
    Model: "text-embedding-3-small", // 1536 dimensions
})

// Or adjust vector store dimension
store, err := pinecone.New(pinecone.Config{
    Dimension: 1536, // Match embedder dimension
})
```

## API Issues

### Rate Limiting

**Problem**: `rate limit exceeded`

**Solution**:
1. Implement retry logic with backoff:
```go
config := llm.Config{
    MaxRetries: 5,
}
```

2. Use circuit breaker:
```go
breaker := circuitbreaker.New(circuitbreaker.Config{
    MaxFailures: 5,
    Timeout:     30 * time.Second,
})
```

3. Reduce request frequency

### Timeout Errors

**Problem**: `context deadline exceeded`

**Solution**:
```go
// Increase timeout
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// Or use longer timeout for specific operations
config := llm.Config{
    Timeout: 60 * time.Second,
}
```

## Debugging

### Enable Debug Mode

```go
import "github.com/DotNetAge/gorag/debug"

// Enable debugging
debug.Enable()

// Log messages
debug.Log("INFO", "Starting operation", map[string]interface{}{
    "file": "document.txt",
})

// Trace function execution
defer debug.Trace("ProcessDocument")()

// Print runtime stats
debug.PrintStats()
```

### Profile Performance

```go
// CPU profiling
stopCPU, err := debugger.StartProfile(debug.ProfileCPU, "cpu.prof")
if err != nil {
    log.Fatal(err)
}
defer stopCPU()

// Memory profiling
stopMem, err := debugger.StartProfile(debug.ProfileMemory, "mem.prof")
if err != nil {
    log.Fatal(err)
}
defer stopMem()
```

### Check Health Status

```go
healthChecker := rag.NewHealthChecker(engine)
report := healthChecker.Check(ctx)

fmt.Printf("Status: %s\n", report.Status)
for _, component := range report.Components {
    fmt.Printf("  %s: %s (latency: %v)\n", 
        component.Name, 
        component.Status, 
        component.Latency)
}
```

### Common Error Codes

| Error | Description | Solution |
|-------|-------------|----------|
| `ErrInvalidSource` | Invalid document source | Check source type and content/path |
| `ErrIndexerNotConfigured` | Missing required component | Provide embedder, store, and parsers |
| `ErrStorage` | Vector store error | Check store connection and configuration |
| `ErrEmbedding` | Embedding generation failed | Check API key and model availability |
| `ErrLLM` | LLM request failed | Check API key, model, and rate limits |

### Getting Help

If you encounter issues not covered in this guide:

1. Check the [GitHub Issues](https://github.com/DotNetAge/gorag/issues)
2. Review the [API Documentation](./api.md)
3. Look at [Examples](../gorag-examples/)
4. Enable debug mode and check logs
5. Submit a new issue with:
   - Go version
   - GoRAG version
   - Error message
   - Minimal reproducible example
