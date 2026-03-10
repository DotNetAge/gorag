# Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] - 2026-03-10

### 🚀 Major Architectural Updates
- **Complete LLM Engine Migration**: Deprecated and removed the internal `llm` package in favor of the unified [`gochat`](https://github.com/DotNetAge/gochat) SDK. This brings enterprise-grade stability, unified message structures, streaming events, and out-of-the-box support for OpenAI, Anthropic, Ollama, Azure, and multiple domestic Chinese LLMs (Kimi, DeepSeek, GLM-4, etc.).
- **Native Vector Store Integration**: Introduced [`govector`](https://github.com/DotNetAge/govector) as a first-class citizen. `GoRAG` now ships with a pure Go, zero-dependency embedded vector database, allowing developers to run a full RAG pipeline locally without setting up external databases like Milvus, Qdrant, or Pinecone.
- **Parser Ecosystem Decoupling**: Moved heavy CGO-dependent parsers (Audio, Video, Webpage) into independent plugin repositories (`gorag-audio`, `gorag-video`, `gorag-webpage`) to keep the core framework ultra-lightweight and compilation times fast.

### ✨ Added
- **Ollama Client Upgrades**: Native integration via `gochat`, providing robust support for running local open-source models (Llama 3, Qwen, Mistral).
- **16 Native Parsers**: Built-in, streaming-supported, pure Go parsers for `txt`, `md`, `csv`, `json`, `yaml`, `html`, `xml`, `log`, `sql`, and various programming languages (`go`, `py`, `js`, `ts`, `java`, `email`).
- **Concurrent Directory Indexing**: Added a powerful 10-worker concurrent processing engine (`IndexDirectory` and `AsyncIndexDirectory`) capable of ingesting entire codebases or 100M+ files rapidly.
- **Advanced RAG Features**: Native implementation of Multi-hop RAG, Agentic RAG, Semantic Chunking, HyDE (Hypothetical Document Embeddings), and RAG-Fusion.
- **Resilience Mechanisms**: Added Circuit Breaker, rate-limiting, connection pooling, and graceful degradation strategies for high-availability production deployments.

### 🧹 Removed
- **Internal `llm` package**: Completely deleted. Replaced by `github.com/DotNetAge/gochat/pkg/core` interfaces.
- Legacy prompt formatting wrappers that didn't align with the standard multi-turn Chat structures.

### 🐛 Fixed
- Resolved integration test flakiness with Testcontainers for Milvus, Qdrant, and Weaviate.
- Fixed `mockLLM` implementations in test suites to correctly emulate `gochat`'s new stream chunk structures (`gochatcore.StreamEvent`).
- Fixed vector dimension mismatches and improved test coverage to reliably stay above 85%.
