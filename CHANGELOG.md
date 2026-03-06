# Changelog

All notable changes to this project will be documented in this file.

## [0.5.0] - 2026-03-07

### Added

#### New Features
- **Azure OpenAI Client**: Full support for Azure OpenAI Service with streaming and standard completion
- **Configuration Management**: Flexible YAML and environment variable configuration system
- **Custom Prompt Templates**: Support for custom prompt formats with `{question}` and `{context}` placeholders
- **Performance Benchmarks**: Built-in benchmark tests for Index and Query operations
- **Production Deployment Guide**: Comprehensive deployment documentation for production environments

#### Parser Enhancements
- **Excel Parser**: Support for Excel file parsing with multiple sheets
- **PPT Parser**: Support for PowerPoint presentation parsing
- **Image Parser**: Multi-modal support for image processing with OCR capabilities

#### LLM Support
- **Domestic LLM Support**: Support for popular Chinese LLMs (qwen, seed2, minmax, kimi, glm5, etc.)
- **Ollama Integration**: Local LLM support with Ollama runtime

#### CLI Tool Improvements
- **Custom Prompt Templates**: CLI support for custom prompt templates
- **File Indexing**: Index documents from files directly
- **Export/Import**: Export and import indexed documents for backup and migration

#### Testing Infrastructure
- **Comprehensive Test Coverage**: Achieved 85%+ test coverage across all modules
- **Integration Testing Framework**: Implemented integration tests using Testcontainers
  - Added Milvus integration tests with standalone container support
  - Added Qdrant integration tests with gRPC client support
  - Added Weaviate integration tests with GraphQL API support
- **Unit Tests**: Added comprehensive unit tests for all major modules
  - Parser tests (Text, PDF, DOCX, HTML, Excel, PPT, Image)
  - Vector store tests (Memory, Milvus, Qdrant, Pinecone, Weaviate)
  - Core module tests (Chunk, Document, Result)
  - CLI tool tests
  - Embedding module tests

#### Vector Store Improvements
- **Milvus**: 
  - Fixed collection creation and schema management
  - Implemented proper Flush and LoadCollection for data persistence
  - Added support for FLAT index type
  - Fixed search result handling with ID field
- **Qdrant**: 
  - Updated to v1.12.0 client
  - Fixed UUID handling for point IDs
  - Implemented proper vector search with payload retrieval
  - Fixed collection creation with correct distance metric
- **Weaviate**: 
  - Implemented complete CRUD operations (Add, Search, Delete)
  - Added automatic collection creation with proper schema
  - Implemented GraphQL-based vector search
  - Added support for custom vector dimensions
  - Fixed class naming convention (must start with uppercase)

### Changed
- **Package Name Unification**: Unified all package names to `github.com/DotNetAge/gorag`
- **Dependency Management**: Resolved all dependency conflicts and updated to latest stable versions
- **Code Cleanup**: Removed debug logging and improved code documentation
- **Error Handling**: Improved error messages and handling across all modules
- Updated all vector store implementations to follow consistent interface patterns
- Improved test organization with separate integration test directory
- Enhanced documentation with testing guidelines and examples

### Fixed
- Fixed Milvus container startup with proper environment variables and commands
- Fixed Qdrant health check endpoint from `/healthz` to `/collections`
- Fixed Weaviate port conflicts using dynamic port mapping
- Fixed vector dimension mismatches in test cases
- Fixed all compilation errors and linter warnings

### Documentation
- Added comprehensive production deployment guide
- Updated API documentation with new features
- Enhanced getting started guide with configuration examples
- Added performance benchmarking documentation

## [0.4.0] - 2025-XX-XX

### Added
- Plugin system for extensibility
- CLI tool with Index, Query, and Export/Import commands
- Streaming response support
- Hybrid retrieval and reranking capabilities

## [0.3.0] - 2025-XX-XX

### Added
- Multi-modal support (Image, Audio)
- Batch processing
- Connection pooling
- Async indexing
- Metrics (Prometheus)
- Structured logging
- Distributed tracing

## [0.2.0] - 2025-XX-XX

### Added
- PDF parser
- DOCX parser
- HTML parser
- Pinecone integration
- Weaviate integration
- Milvus integration
- Qdrant integration
- OpenAI embeddings
- Ollama local embeddings
- OpenAI client
- Anthropic client
- Query caching

## [0.1.0] - 2025-XX-XX

### Added
- Initial release with basic RAG functionality
- Basic RAG engine
- Text parser
- In-memory vector store
- Mock embedding provider
- Mock LLM client
- Basic examples
- Documentation
