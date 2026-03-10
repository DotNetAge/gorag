package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	embedOllama "github.com/DotNetAge/gorag/embedding/ollama"
	llmOllama "github.com/DotNetAge/gochat/pkg/client/ollama"
	"github.com/DotNetAge/gochat/pkg/client/base"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/vectorstore/memory"
)

// generateTestData generates multiple test documents for benchmarking
func generateTestData(count int) []Source {
	sources := make([]Source, count)

	// Base content in both English and Chinese
	baseContent := "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support. Go语言是一种开源编程语言，它能让构造简单、可靠且高效的软件变得容易。Go语言具有垃圾回收、类型安全和并发支持等特性。Go语言的设计理念是简洁、高效和可靠性。Go语言的语法简洁明了，易于学习和使用。Go语言的标准库非常丰富，提供了很多实用的功能。Go语言的编译速度非常快，生成的可执行文件体积小，运行效率高。"

	for i := 0; i < count; i++ {
		sources[i] = Source{
			Type:    "text",
			Content: fmt.Sprintf("Document %d: %s", i+1, baseContent),
		}
	}

	return sources
}

// loadGeneratedTestData loads test data from generated files
func loadGeneratedTestData() ([]Source, error) {
	var sources []Source

	// Try different paths to find test_data directory, prioritize tools/test_data
	testDataDirs := []string{
		"../tools/test_data",
		"../../tools/test_data",
		"test_data",
		"../test_data",
		"../../test_data",
	}

	var testDataDir string
	var err error
	var files []os.DirEntry

	for _, dir := range testDataDirs {
		files, err = os.ReadDir(dir)
		if err == nil {
			testDataDir = dir
			break
		}
	}

	if testDataDir == "" {
		return nil, fmt.Errorf("failed to find test data directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".txt" {
			filePath := filepath.Join(testDataDir, file.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read test file %s: %w", filePath, err)
			}

			sources = append(sources, Source{
				Type:    "text",
				Content: string(content),
			})
		}
	}

	return sources, nil
}

func BenchmarkEngine_Index_SingleDocument(b *testing.B) {
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
		Config: base.Config{
		Model: "qwen3:0.6b",
	},
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

	// Test data - English and Chinese mixed content
	source := Source{
		Type:    "text",
		Content: "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support. Go语言是一种开源编程语言，它能让构造简单、可靠且高效的软件变得容易。Go语言具有垃圾回收、类型安全和并发支持等特性。Go语言的设计理念是简洁、高效和可靠性。Go语言的语法简洁明了，易于学习和使用。Go语言的标准库非常丰富，提供了很多实用的功能。Go语言的编译速度非常快，生成的可执行文件体积小，运行效率高。",
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

func BenchmarkEngine_Index_MultipleDocuments(b *testing.B) {
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
		Config: base.Config{
		Model: "qwen3:0.6b",
	},
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

	// Generate test data (10 documents)
	sources := generateTestData(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		// Clear the store before each iteration
		vectorStore = memory.NewStore()
		engine, _ = New(
			WithParser(parser),
			WithEmbedder(embedder),
			WithVectorStore(vectorStore),
			WithLLM(llmClient),
		)
		// Index all documents
		for _, source := range sources {
			err := engine.Index(ctx, source)
			if err != nil {
				b.Fatalf("Failed to index: %v", err)
			}
		}
	}
}

func BenchmarkEngine_Query_SingleDocument(b *testing.B) {
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
		Config: base.Config{
		Model: "qwen3:0.6b",
	},
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

	// Index test data - English and Chinese mixed content
	source := Source{
		Type:    "text",
		Content: "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support. Go语言是一种开源编程语言，它能让构造简单、可靠且高效的软件变得容易。Go语言具有垃圾回收、类型安全和并发支持等特性。Go语言的设计理念是简洁、高效和可靠性。Go语言的语法简洁明了，易于学习和使用。Go语言的标准库非常丰富，提供了很多实用的功能。Go语言的编译速度非常快，生成的可执行文件体积小，运行效率高。",
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

func BenchmarkEngine_Query_MultipleDocuments(b *testing.B) {
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
		Config: base.Config{
		Model: "qwen3:0.6b",
	},
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

	// Generate and index test data (10 documents)
	sources := generateTestData(10)
	ctx := context.Background()
	for _, source := range sources {
		err := engine.Index(ctx, source)
		if err != nil {
			b.Fatalf("Failed to index: %v", err)
		}
	}

	// Test query
	question := "What is Go programming language?"

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

func BenchmarkEngine_Index_LargeScale(b *testing.B) {
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
		Config: base.Config{
		Model: "qwen3:0.6b",
	},
	})
	if err != nil {
		b.Fatalf("Failed to create LLM client: %v", err)
	}

	// Load generated test data
	sources, err := loadGeneratedTestData()
	if err != nil {
		b.Fatalf("Failed to load test data: %v", err)
	}

	b.Logf("Loaded %d test documents for large-scale benchmark", len(sources))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		// Clear the store before each iteration
		vectorStore = memory.NewStore()
		engine, _ := New(
			WithParser(parser),
			WithEmbedder(embedder),
			WithVectorStore(vectorStore),
			WithLLM(llmClient),
		)
		// Index all documents
		for _, source := range sources {
			err := engine.Index(ctx, source)
			if err != nil {
				b.Fatalf("Failed to index: %v", err)
			}
		}
	}
}

func BenchmarkEngine_Query_LargeScale(b *testing.B) {
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
		Config: base.Config{
		Model: "qwen3:0.6b",
	},
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

	// Load generated test data
	sources, err := loadGeneratedTestData()
	if err != nil {
		b.Fatalf("Failed to load test data: %v", err)
	}

	// Index all documents
	ctx := context.Background()
	for _, source := range sources {
		err := engine.Index(ctx, source)
		if err != nil {
			b.Fatalf("Failed to index: %v", err)
		}
	}

	b.Logf("Indexed %d test documents for large-scale query benchmark", len(sources))

	// Test query
	question := "What is Go programming language?"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Query(ctx, question, QueryOptions{
			TopK: 5,
		})
		if err != nil {
			b.Fatalf("Failed to query: %v", err)
		}
	}
}
