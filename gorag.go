// Package gorag provides a Retrieval-Augmented Generation (RAG) framework for Go
//
// GoRAG is a comprehensive framework for building RAG applications that combine
// large language models (LLMs) with vector databases for efficient information retrieval.
//
// Key features include:
// - Circuit breaker pattern for service resilience
// - Graceful degradation for unreliable services
// - Lazy loading for efficient memory usage
// - Observability with metrics, logging, and tracing
// - Plugin system for extensibility
// - Connection pooling for efficient resource management
// - Support for multiple vector stores (Memory, Milvus, Pinecone, Qdrant, Weaviate)
// - Support for multiple embedding providers (Cohere, Ollama, OpenAI, Voyage)
// - Support for multiple LLM clients (Anthropic, Azure OpenAI, Ollama, OpenAI)
// - Support for multiple document parsers (CSV, JSON, Markdown, PDF, etc.)
//
// To get started, see the examples in the cmd/gorag directory or refer to the documentation
// in the docs directory.
package gorag
