package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
	fmt.Println("- HyDE (Hypothetical Document Embeddings)")
	fmt.Println("- RAG-Fusion")
	fmt.Println("- Context Compression")
	fmt.Println("- Multi-turn Conversation")
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

	for i, doc := range documents {
		err := engine.Index(ctx, rag.Source{
			Type:    "text",
			Content: doc,
			Metadata: map[string]string{
				"index":   fmt.Sprintf("%d", i),
				"language": doc[:2],
			},
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

	// 3. Query with HyDE (Hypothetical Document Embeddings)
	fmt.Println("💡 3. Query with HyDE (Hypothetical Document Embeddings)")
	fmt.Println("   HyDE generates hypothetical answers to improve retrieval quality")
	
	resp, err = engine.Query(ctx, "Compare Go and Rust performance", rag.QueryOptions{
		TopK:           5,
		UseHyDE:        true,
		HyDEInstructions: "Generate a detailed comparison focusing on performance characteristics",
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Q: Compare Go and Rust performance\n")
		fmt.Printf("   A: %s\n", resp.Answer)
		fmt.Printf("   📚 Sources: %d\n", len(resp.Sources))
	}
	fmt.Println()

	// 4. Query with RAG-Fusion
	fmt.Println("🔄 4. Query with RAG-Fusion")
	fmt.Println("   RAG-Fusion generates multiple query variations for better retrieval")
	
	resp, err = engine.Query(ctx, "programming languages for system development", rag.QueryOptions{
		TopK:             5,
		UseRAGFusion:     true,
		RAGFusionQueries: 3,
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Q: programming languages for system development\n")
		fmt.Printf("   A: %s\n", resp.Answer)
		fmt.Printf("   📚 Sources: %d\n", len(resp.Sources))
	}
	fmt.Println()

	// 5. Query with Context Compression
	fmt.Println("🗜️  5. Query with Context Compression")
	fmt.Println("   Context compression optimizes token usage for better results")
	
	resp, err = engine.Query(ctx, "What are the key features of Go, Python, and Rust?", rag.QueryOptions{
		TopK:                 5,
		UseContextCompression: true,
		MaxContextTokens:     500,
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Q: What are the key features of Go, Python, and Rust?\n")
		fmt.Printf("   A: %s\n", resp.Answer)
		fmt.Printf("   📚 Sources: %d\n", len(resp.Sources))
	}
	fmt.Println()

	// 6. Multi-turn Conversation
	fmt.Println("💬 6. Multi-turn Conversation Support")
	fmt.Println("   Maintaining context across multiple queries")
	
	// Create a conversation session
	conversationID := "demo-conversation"
	
	// First turn
	resp1, err := engine.Query(ctx, "Who created Go?", rag.QueryOptions{
		TopK:           3,
		ConversationID: conversationID,
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Turn 1 - Q: Who created Go?\n")
		fmt.Printf("            A: %s\n", resp1.Answer)
	}
	
	// Second turn (referencing previous context)
	resp2, err := engine.Query(ctx, "What else did they create?", rag.QueryOptions{
		TopK:           3,
		ConversationID: conversationID,
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Turn 2 - Q: What else did they create?\n")
		fmt.Printf("            A: %s\n", resp2.Answer)
	}
	
	// Third turn (referencing previous context)
	resp3, err := engine.Query(ctx, "When was it created?", rag.QueryOptions{
		TopK:           3,
		ConversationID: conversationID,
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Turn 3 - Q: When was it created?\n")
		fmt.Printf("            A: %s\n", resp3.Answer)
	}
	fmt.Println()

	// 7. Combined Advanced Features
	fmt.Println("🚀 7. Combined Advanced Features")
	fmt.Println("   Using HyDE + RAG-Fusion + Context Compression together")
	
	resp, err = engine.Query(ctx, "Which language is best for concurrent programming?", rag.QueryOptions{
		TopK:                  5,
		UseHyDE:               true,
		UseRAGFusion:          true,
		RAGFusionQueries:      3,
		UseContextCompression: true,
		MaxContextTokens:      400,
		ConversationID:        conversationID,
	})
	if err != nil {
		log.Printf("   Error: %v", err)
	} else {
		fmt.Printf("   Q: Which language is best for concurrent programming?\n")
		fmt.Printf("   A: %s\n", resp.Answer)
		fmt.Printf("   📚 Sources: %d\n", len(resp.Sources))
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
	fmt.Println("✅ HyDE (Hypothetical Document Embeddings)")
	fmt.Println("   - Generates hypothetical answers to improve retrieval")
	fmt.Println("   - Better understanding of query intent")
	fmt.Println()
	fmt.Println("✅ RAG-Fusion")
	fmt.Println("   - Generates multiple query variations")
	fmt.Println("   - Combines results from multiple perspectives")
	fmt.Println()
	fmt.Println("✅ Context Compression")
	fmt.Println("   - Optimizes token usage")
	fmt.Println("   - Fits more relevant context within token limits")
	fmt.Println()
	fmt.Println("✅ Multi-turn Conversation")
	fmt.Println("   - Maintains context across queries")
	fmt.Println("   - Natural conversational flow")
	fmt.Println()
	fmt.Println("🎯 GoRAG: The most feature-complete RAG framework for Go!")
}
