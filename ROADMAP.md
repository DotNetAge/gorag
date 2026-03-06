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

### Advanced Retrieval
- [x] Hybrid retrieval (keyword + semantic)
- [x] Reranking
- [x] Query routing
- [x] Multi-modal support (images)

### Performance
- [x] Batch processing
- [x] Connection pooling
- [x] Async indexing

### Observability
- [x] Metrics (Prometheus)
- [x] Structured logging
- [x] Distributed tracing

## Version 0.4.0

### Extensibility
- [x] Plugin system
- [x] Configuration management

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
- [x] Plugin development guide

## Version 0.6.0 (Next Release)

### More Parsers
- [ ] Markdown parser (with frontmatter support)
- [ ] CSV parser
- [ ] RTF parser
- [ ] EPUB parser

### More Vector Stores
- [ ] Chroma integration
- [ ] Elasticsearch integration
- [ ] PostgreSQL pgvector integration
- [ ] Redis Vector integration

### More Embedding Providers
- [ ] Cohere embeddings
- [ ] HuggingFace embeddings
- [ ] Jina AI embeddings
- [ ] Voyage AI embeddings

### More LLM Clients
- [ ] Google Gemini client
- [ ] AWS Bedrock client
- [ ] Cohere client
- [ ] Mistral AI client

## Version 0.7.0

### Advanced Chunking
- [ ] Semantic chunking
- [ ] Recursive character text splitter
- [ ] Code-aware chunking
- [ ] Markdown-aware chunking

### Query Enhancement
- [ ] Query rewriting
- [ ] Query expansion
- [ ] Multi-query retrieval
- [ ] HyDE (Hypothetical Document Embeddings)

### Retrieval Optimization
- [ ] Maximal marginal relevance (MMR)
- [ ] Similarity score threshold filtering
- [ ] Metadata filtering
- [ ] Time-weighted retrieval

## Version 0.8.0

### Performance Optimization
- [ ] Parallel document processing
- [ ] Embedding caching
- [ ] Query result caching
- [ ] Lazy loading for large documents

### Developer Tools
- [ ] RAG pipeline visualization
- [ ] Performance profiling tools
- [ ] Debug mode with detailed logging
- [ ] Query analysis dashboard

### Testing Tools
- [ ] RAG evaluation metrics
- [ ] Retrieval quality benchmarks
- [ ] Answer relevance scoring
- [ ] Ground truth comparison tools

## Version 0.9.0

### Advanced Features
- [ ] Document versioning
- [ ] Incremental indexing
- [ ] Document deletion and updates
- [ ] Collection management

### Multi-modal Enhancement
- [ ] Audio transcription support
- [ ] Video frame extraction
- [ ] Table extraction from images
- [ ] Chart recognition

### Integration
- [ ] LangChain compatibility layer
- [ ] LlamaIndex compatibility layer
- [ ] OpenAI Assistants API integration
- [ ] Custom callback system

## Version 1.0.0

### Stability & Production Ready
- [ ] API stability guarantee
- [ ] Comprehensive error handling
- [ ] Graceful degradation
- [ ] Production-ready documentation

### Ecosystem
- [ ] Example applications gallery
- [ ] Community plugin repository
- [ ] Integration templates
- [ ] Best practices guide

## Future Considerations

### Research & Innovation
- [ ] Graph RAG support
- [ ] Adaptive retrieval strategies
- [ ] Self-querying retrieval
- [ ] Contextual compression

### Community
- [ ] Plugin marketplace
- [ ] Community contributions guide
- [ ] Regular release schedule
- [ ] Semantic versioning commitment
