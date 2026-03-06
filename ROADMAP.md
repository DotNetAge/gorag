# GoRAG - Project Roadmap

## Version 0.1.0 (MVP)

### Core Features
- [x] Basic RAG engine
- [x] Text parser
- [x] In-memory vector store
- [x] Mock embedding provider
- [x] Mock LLM client
- [x] Basic examples
- [x] Documentation

## Version 0.2.0

### Parsers (9 types)
- [x] Text parser
- [x] PDF parser
- [x] DOCX parser
- [x] HTML parser
- [x] JSON parser
- [x] YAML parser
- [x] Excel parser
- [x] PPT parser
- [x] Image parser

### Vector Stores (5 backends)
- [x] Memory store
- [x] Pinecone integration
- [x] Weaviate integration
- [x] Milvus integration
- [x] Qdrant integration

### Embedding Providers (2 providers)
- [x] OpenAI embeddings
- [x] Ollama local embeddings

### LLM Clients (5 clients)
- [x] OpenAI client
- [x] Anthropic client
- [x] Azure OpenAI client
- [x] Ollama client
- [x] Compatible API client (supports domestic LLMs: qwen, seed2, minmax, kimi, glm5, deepseek, etc.)

### Features
- [x] Streaming responses
- [x] Custom prompt templates
- [x] Query caching

## Version 0.3.0

### Advanced Features
- [x] Hybrid retrieval (keyword + semantic)
- [x] Reranking
- [x] Query routing
- [x] Multi-modal support (images, audio)

### Performance
- [x] Batch processing
- [x] Connection pooling
- [x] Async indexing

### Observability
- [x] Metrics (Prometheus)
- [x] Structured logging
- [x] Distributed tracing

## Version 0.4.0

### Production Features
- [x] Plugin system
- [x] Configuration management
- [ ] Rate limiting
- [ ] Authentication/Authorization

### CLI Tool
- [x] Index command
- [x] Query command
- [x] Export/Import
- [x] Custom prompt template support

## Version 0.5.0

### Testing & Quality
- [x] Full test coverage (85%+)
- [x] Integration tests with Testcontainers
- [x] Performance benchmarks

### Documentation
- [x] Production deployment guides
- [x] API documentation
- [x] Getting started guide

## Version 1.0.0

### Stable Release
- [ ] Multi-tenancy support
- [ ] Advanced security features
- [ ] Enterprise support
- [ ] Cloud-native deployment templates

## Future Plans

### Version 1.1.0
- [ ] Graph RAG support
- [ ] Advanced chunking strategies
- [ ] Multi-language support
- [ ] Custom embedding models

### Version 1.2.0
- [ ] Real-time indexing
- [ ] Distributed deployment
- [ ] Advanced monitoring
- [ ] Performance optimization
