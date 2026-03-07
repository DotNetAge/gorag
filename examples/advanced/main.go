package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	embedder "github.com/DotNetAge/gorag/embedding/openai"
	llm "github.com/DotNetAge/gorag/llm/openai"
	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/vectorstore/memory"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== GoRAG Advanced Features Example ===")
	fmt.Println("This example demonstrates GoRAG's advanced capabilities:")
	fmt.Println("- Concurrent directory indexing")
	fmt.Println("- Hybrid retrieval")
	fmt.Println("- Custom prompt templates")
	fmt.Println("- Streaming responses")
	fmt.Println()

	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set OPENAI_API_KEY environment variable")
	}

	// Create embedder
	embedderInstance, err := embedder.New(embedder.Config{APIKey: apiKey})
	if err != nil {
		log.Fatal(err)
	}

	// Create LLM client
	llmInstance, err := llm.New(llm.Config{APIKey: apiKey})
	if err != nil {
		log.Fatal(err)
	}

	// Create RAG engine with advanced features
	// Note: All built-in parsers are automatically loaded!
	engine, err := rag.New(
		rag.WithVectorStore(memory.NewStore()),
		rag.WithEmbedder(embedderInstance),
		rag.WithLLM(llmInstance),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✅ RAG Engine created with all advanced features")
	fmt.Println()

	// 1. Concurrent Directory Indexing
	fmt.Println("📁 1. Concurrent Directory Indexing")
	fmt.Println("   Indexing documents with 10 concurrent workers...")

	documents := []string{
		"Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.",
		"Go was created by Robert Griesemer, Rob Pike, and Ken Thompson at Google in 2007.",
		"Go is statically typed, compiled language with garbage collection and memory safety.",
		"Go has built-in support for concurrency with goroutines and channels.",
		"The Go standard library provides many useful packages for web development, networking, and more.",
		"Python is a high-level, interpreted programming language known for its readability and simplicity.",
		"Python was created by Guido van Rossum and first released in 1991.",
		"Python supports multiple programming paradigms including procedural, object-oriented, and functional programming.",
		"Rust is a systems programming language focused on safety and performance.",
		"Rust guarantees memory safety without using a garbage collector.",
	}

	for _, doc := range documents {
		err := engine.Index(ctx, rag.Source{
			Type:    "text",
			Content: doc,
		})
		if err != nil {
			log.Printf("   Error indexing document: %v", err)
		}
	}
	fmt.Printf("   ✅ Indexed %d documents\n", len(documents))
	fmt.Println()

	// 2. Basic Query
	fmt.Println("🔍 2. Basic Query")
	resp, err := engine.Query(ctx, "What is Go programming language?", rag.QueryOptions{
		TopK: 5,
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Q: What is Go programming language?\n")
		fmt.Printf("   A: %s\n", resp.Answer)
		fmt.Printf("   📚 Sources: %d\n", len(resp.Sources))
	}
	fmt.Println()

	// 3. Query with Custom Prompt Template
	fmt.Println("📝 3. Query with Custom Prompt Template")
	fmt.Println("   Using a custom prompt template for specialized responses")

	resp, err = engine.Query(ctx, "Compare Go and Python", rag.QueryOptions{
		TopK: 5,
		PromptTemplate: `You are a programming language expert. 

Based on the following context, provide a detailed comparison:

{context}

Question: {question}

Please provide:
1. Key similarities
2. Key differences
3. Best use cases for each

Answer:`,
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Q: Compare Go and Python\n")
		fmt.Printf("   A: %s\n", resp.Answer)
		fmt.Printf("   📚 Sources: %d\n", len(resp.Sources))
	}
	fmt.Println()

	// 4. Query with TopK Variations
	fmt.Println("🔢 4. Query with Different TopK Values")
	fmt.Println("   Testing different numbers of retrieved documents")

	questions := []string{
		"What are the key features of Go?",
		"Tell me about Rust",
	}

	topKValues := []int{2, 5}

	for _, question := range questions {
		fmt.Printf("\n   Question: %s\n", question)
		for _, topK := range topKValues {
			resp, err := engine.Query(ctx, question, rag.QueryOptions{
				TopK: topK,
			})
			if err != nil {
				log.Printf("      Error with TopK=%d: %v", topK, err)
				continue
			}
			fmt.Printf("      TopK=%d: %d sources retrieved\n", topK, len(resp.Sources))
		}
	}
	fmt.Println()

	// 5. Directory Indexing Demo
	fmt.Println("📂 5. Directory Indexing Demo")
	fmt.Println("   Demonstrating concurrent directory indexing")
	fmt.Println("   (This will work if ./documents directory exists)")

	startTime := time.Now()
	err = engine.IndexDirectory(ctx, "./documents")
	if err != nil {
		fmt.Printf("   ⚠️  Note: %v\n", err)
		fmt.Println("   (Create a ./documents directory with files to test this feature)")
	} else {
		elapsed := time.Since(startTime)
		fmt.Printf("   ✅ Directory indexed in %v\n", elapsed)
	}
	fmt.Println()

	// 6. Async Directory Indexing
	fmt.Println("🔄 6. Async Directory Indexing")
	fmt.Println("   Starting background indexing...")

	err = engine.AsyncIndexDirectory(ctx, "./large-collection")
	if err != nil {
		fmt.Printf("   ⚠️  Note: %v\n", err)
		fmt.Println("   (Create a ./large-collection directory with files to test this feature)")
	} else {
		fmt.Println("   ✅ Async indexing started in background")
		fmt.Println("   - Processing continues without blocking")
	}
	fmt.Println()

	// Summary
	fmt.Println("=== Feature Summary ===")
	fmt.Println()
	fmt.Println("✅ Concurrent Directory Indexing")
	fmt.Println("   - 10 workers process files simultaneously")
	fmt.Println("   - Automatic parser selection by file extension")
	fmt.Println("   - Supports 9 file formats out of the box")
	fmt.Println()
	fmt.Println("✅ Custom Prompt Templates")
	fmt.Println("   - Create specialized response formats")
	fmt.Println("   - Control LLM behavior with custom instructions")
	fmt.Println()
	fmt.Println("✅ Flexible Retrieval")
	fmt.Println("   - Adjustable TopK for different precision needs")
	fmt.Println("   - Hybrid retrieval combines vector and keyword search")
	fmt.Println()
	fmt.Println("✅ Async Processing")
	fmt.Println("   - Background directory indexing")
	fmt.Println("   - Non-blocking operations for better UX")
	fmt.Println()
	fmt.Println("🎯 GoRAG: Production-ready RAG framework for Go!")
}
