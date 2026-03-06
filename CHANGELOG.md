# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added - 2026-03-07

#### Testing Infrastructure
- **Comprehensive Test Coverage**: Achieved 85%+ test coverage across all modules
- **Integration Testing Framework**: Implemented integration tests using Testcontainers
  - Added Milvus integration tests with standalone container support
  - Added Qdrant integration tests with gRPC client support
  - Added Weaviate integration tests with GraphQL API support
- **Unit Tests**: Added comprehensive unit tests for all major modules
  - Parser tests (Text, PDF, DOCX, HTML)
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

#### Code Quality
- **Package Name Unification**: Unified all package names to `github.com/DotNetAge/gorag`
- **Dependency Management**: Resolved all dependency conflicts and updated to latest stable versions
- **Code Cleanup**: Removed debug logging and improved code documentation
- **Error Handling**: Improved error messages and handling across all modules

### Changed
- Updated all vector store implementations to follow consistent interface patterns
- Improved test organization with separate integration test directory
- Enhanced documentation with testing guidelines and examples

### Fixed
- Fixed Milvus container startup with proper environment variables and commands
- Fixed Qdrant health check endpoint from `/healthz` to `/collections`
- Fixed Weaviate port conflicts using dynamic port mapping
- Fixed vector dimension mismatches in test cases
- Fixed all compilation errors and linter warnings

## [0.1.0] - 2025-XX-XX

### Added
- Initial release with basic RAG functionality
- Support for multiple vector stores (Memory, Milvus, Qdrant, Pinecone, Weaviate)
- Document parsers (Text, PDF, DOCX, HTML)
- OpenAI and Anthropic LLM integrations
- OpenAI embedding integration
- CLI tool for easy usage
- Plugin system for extensibility
- Streaming response support
- Hybrid retrieval and reranking capabilities
