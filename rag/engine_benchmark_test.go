package rag

import (
	"context"
	"testing"

	embedOllama "github.com/DotNetAge/gorag/embedding/ollama"
	llmOllama "github.com/DotNetAge/gorag/llm/ollama"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/vectorstore/memory"
)

func BenchmarkEngine_Index(b *testing.B) {
	// Create components
	parser := text.NewParser()
	embedder, err := embedOllama.New(embedOllama.Config{
		Model: "qllama/bge-small-zh-v1.5:latest",
	})
	if err != nil {
		b.Fatalf("Failed to create embedder: %v", err)
	}

	vectorStore := memory.NewStore()
	llmClient, err := llmOllama.New(llmOllama.Config{
		Model: "qwen3:0.6b",
	})
	if err != nil {
		b.Fatalf("Failed to create LLM client: %v", err)
	}

	// Create RAG engine
	engine, err := New(
		WithParser(parser),
		WithEmbedder(embedder),
		WithVectorStore(vectorStore),
		WithLLM(llmClient),
	)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	// Test data
	source := Source{
		Type:    "text",
		Content: "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support.",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		err := engine.Index(ctx, source)
		if err != nil {
			b.Fatalf("Failed to index: %v", err)
		}
	}
}

func BenchmarkEngine_Query(b *testing.B) {
	// Create components
	parser := text.NewParser()
	embedder, err := embedOllama.New(embedOllama.Config{
		Model: "qllama/bge-small-zh-v1.5:latest",
	})
	if err != nil {
		b.Fatalf("Failed to create embedder: %v", err)
	}

	vectorStore := memory.NewStore()
	llmClient, err := llmOllama.New(llmOllama.Config{
		Model: "qwen3:0.6b",
	})
	if err != nil {
		b.Fatalf("Failed to create LLM client: %v", err)
	}

	// Create RAG engine
	engine, err := New(
		WithParser(parser),
		WithEmbedder(embedder),
		WithVectorStore(vectorStore),
		WithLLM(llmClient),
	)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	// Index test data
	source := Source{
		Type:    "text",
		Content: "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support.",
	}
	ctx := context.Background()
	err = engine.Index(ctx, source)
	if err != nil {
		b.Fatalf("Failed to index: %v", err)
	}

	// Test query
	question := "What is Go?"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Query(ctx, question, QueryOptions{
			TopK: 3,
		})
		if err != nil {
			b.Fatalf("Failed to query: %v", err)
		}
	}
}
