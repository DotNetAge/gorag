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

### Parsers
- [x] Text parser
- [x] PDF parser
- [x] DOCX parser
- [x] HTML parser
- [x] JSON parser
- [x] YAML parser
- [x] Excel parser
- [x] PPT parser
- [x] Image parser
- [x] Config parser
- [x] CSV parser
- [x] DB schema parser
- [x] Email parser
- [x] Go code parser
- [x] Java code parser
- [x] JS code parser
- [x] Log parser
- [x] Markdown parser
- [x] Python code parser
- [x] TypeScript code parser
- [x] XML parser

### Vector Stores
- [x] Memory store
- [x] Pinecone integration
- [x] Weaviate integration
- [x] Milvus integration
- [x] Qdrant integration

### Embedding Providers
- [x] OpenAI embeddings
- [x] Ollama local embeddings
- [x] Cohere embeddings
- [x] Voyage embeddings

### LLM Clients
- [x] OpenAI client
- [x] Anthropic client
- [x] Azure OpenAI client
- [x] Ollama client
- [x] Compatible API client (supports domestic LLMs)

### Features
- [x] Streaming responses
- [x] Custom prompt templates
- [x] Query caching

## Version 0.3.0

### Advanced Retrieval
- [x] Hybrid retrieval (keyword + semantic)
- [x] Reranking
- [x] Query routing
- [ ] Multi-modal support (images)

### Performance
- [x] Batch processing
- [ ] Connection pooling
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
- [ ] Export/Import
- [x] Custom prompt template support

## Version 0.5.0

### Testing & Quality
- [ ] Full test coverage (85%+)
- [x] Integration tests with Testcontainers
- [x] Performance benchmarks

### Documentation
- [x] Production deployment guides
- [x] API documentation
- [x] Getting started guide
- [x] Plugin development guide

## Version 0.6.0 (Next Release - Quality Improvement)

### Test Coverage Improvement
- [ ] Add tests for config package (current: 0%)
- [ ] Add tests for Azure OpenAI client (current: 0%)
- [ ] Improve test coverage for Excel parser (current: 13.5%)
- [ ] Improve test coverage for Milvus store (current: 58.7%)
- [ ] Improve test coverage for Qdrant store (current: 66.1%)
- [ ] Improve test coverage for Weaviate store (current: 42.6%)
- [ ] Improve test coverage for RAG engine (current: 56.8%)

### Code Quality
- [x] Implement proper LLM response parsing for reranker scores
- [ ] Add error handling for edge cases
- [ ] Improve code documentation

### Bug Fixes
- [x] Fix any issues found during testing
- [x] Handle concurrent access safely
- [x] Improve error messages

## Version 0.7.0 (Performance & Reliability)

### Performance Optimization
- [x] Optimize embedding batch processing
- [x] Add connection pooling for vector stores
- [x] Implement query result caching
- [x] Add lazy loading for large documents

### Reliability
- [x] Add retry logic for API calls
- [x] Implement graceful degradation
- [x] Add circuit breaker pattern
- [x] Improve timeout handling

### Developer Experience
- [x] Add more code examples
- [x] Improve error messages
- [x] Add debugging utilities
- [x] Create troubleshooting guide

## Version 0.8.0 (Documentation & Examples)

### Documentation
- [ ] Add architecture decision records (ADRs)
- [x] Create contribution guidelines
- [ ] Add performance tuning guide
- [ ] Create FAQ section

### Examples
- [x] Add real-world use case examples
- [x] Create step-by-step tutorials
- [x] Add integration examples
- [x] Create best practices guide

### Community
- [ ] Set up issue templates
- [ ] Create pull request template
- [ ] Add code of conduct
- [x] Set up GitHub Actions for CI/CD

## Version 0.9.0 (Stability)

### API Stability
- [ ] Review and finalize public APIs
- [ ] Add deprecation warnings
- [ ] Ensure backward compatibility
- [ ] Document API changes

### Production Readiness
- [x] Add health check endpoints
- [x] Implement graceful shutdown


## Version 1.0.0 (Stable Release)

### Stability
- [ ] API stability guarantee
- [ ] No breaking changes
- [ ] Production-ready documentation
- [ ] Comprehensive test suite

### Support
- [ ] Long-term support commitment
- [ ] Regular security updates
- [ ] Community support channels
- [ ] Enterprise support options

## Future Considerations

### Research
- [ ] Evaluate Graph RAG feasibility
- [ ] Research adaptive retrieval
- [ ] Explore multi-modal enhancements

### Community
- [ ] Plugin marketplace
- [ ] Community contributions
- [ ] Regular releases
- [ ] Semantic versioning
