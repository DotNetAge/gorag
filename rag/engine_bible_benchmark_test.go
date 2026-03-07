package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	embedOllama "github.com/DotNetAge/gorag/embedding/ollama"
	llmOllama "github.com/DotNetAge/gorag/llm/ollama"
	"github.com/DotNetAge/gorag/parser/html"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/vectorstore/memory"
)

// loadBibleTestData loads bible test data from files with multiple formats
func loadBibleTestData() ([]Source, error) {
	var sources []Source

	// Path to the bible test data
	testDataDir := "/Users/ray/workspaces/gorag/gorag/test_data/cus"

	// Read all files in the directory
	files, err := os.ReadDir(testDataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read test data directory: %w", err)
	}

	// Collect HTML and text files
	for _, file := range files {
		if !file.IsDir() {
			ext := filepath.Ext(file.Name())
			if ext == ".htm" || ext == ".html" || ext == ".txt" {
				filePath := filepath.Join(testDataDir, file.Name())
				content, err := os.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("failed to read test file %s: %w", filePath, err)
				}

				// Determine source type based on file extension
				sourceType := "text"
				if ext == ".htm" || ext == ".html" {
					sourceType = "html"
				}

				sources = append(sources, Source{
					Type:    sourceType,
					Content: string(content),
				})
			}
		}
	}

	return sources, nil
}

func BenchmarkEngine_Index_BibleScale_MixedFormats(b *testing.B) {
	// Create components
	textParser := text.NewParser()
	htmlParser := html.NewParser()

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

	// Load bible test data
	sources, err := loadBibleTestData()
	if err != nil {
		b.Fatalf("Failed to load test data: %v", err)
	}

	b.Logf("Loaded %d bible files (mixed formats) for benchmark", len(sources))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		// Clear the store before each iteration
		vectorStore = memory.NewStore()
		engine, _ := New(
			WithParser(textParser), // Set text parser as default
			WithEmbedder(embedder),
			WithVectorStore(vectorStore),
			WithLLM(llmClient),
		)
		// Add html parser for html files
		engine.AddParser("html", htmlParser)
		// Index all documents
		for _, source := range sources {
			err := engine.Index(ctx, source)
			if err != nil {
				b.Fatalf("Failed to index: %v", err)
			}
		}
	}
}

func BenchmarkEngine_Query_BibleScale_MixedFormats(b *testing.B) {
	// Create components
	textParser := text.NewParser()
	htmlParser := html.NewParser()

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
		WithParser(textParser), // Set text parser as default
		WithEmbedder(embedder),
		WithVectorStore(vectorStore),
		WithLLM(llmClient),
	)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	// Add html parser for html files
	engine.AddParser("html", htmlParser)

	// Load bible test data
	sources, err := loadBibleTestData()
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

	b.Logf("Indexed %d bible files (mixed formats) for query benchmark", len(sources))

	// Test query
	question := "What is the beginning of the Bible?"

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


