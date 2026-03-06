package main

import (
	"context"
	"fmt"
	"log"
	"os"

	embedder "github.com/DotNetAge/gorag/embedding/openai"
	llm "github.com/DotNetAge/gorag/llm/openai"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/vectorstore/memory"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== GoRAG Advanced Example ===")
	fmt.Println()

	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set OPENAI_API_KEY environment variable")
	}

	// Create RAG engine with OpenAI
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

	// Create RAG engine with OpenAI
	engine, err := rag.New(
		rag.WithParser(text.NewParser()),
		rag.WithVectorStore(memory.NewStore()),
		rag.WithEmbedder(embedderInstance),
		rag.WithLLM(llmInstance),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("1. Indexing documents...")

	documents := []string{
		"Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.",
		"Go was created by Robert Griesemer, Rob Pike, and Ken Thompson at Google in 2007.",
		"Go is statically typed, compiled language with garbage collection and memory safety.",
		"Go has built-in support for concurrency with goroutines and channels.",
		"The Go standard library provides many useful packages for web development, networking, and more.",
	}

	for _, doc := range documents {
		err := engine.Index(ctx, rag.Source{
			Type:    "text",
			Content: doc,
		})
		if err != nil {
			log.Printf("Error indexing document: %v", err)
		} else {
			fmt.Printf("   Indexed: %s...\n", doc[:50])
		}
	}

	fmt.Println("\n2. Querying with OpenAI...")
	questions := []string{
		"What is Go?",
		"Who created Go?",
		"What are the key features of Go?",
	}

	for _, question := range questions {
		fmt.Printf("\nQuestion: %s\n", question)
		fmt.Println("==================================")

		resp, err := engine.Query(ctx, question, rag.QueryOptions{
			TopK: 3,
			PromptTemplate: "Answer the question based on the following context:\n\n{context}\n\nQuestion: {question}\nAnswer:",
		})
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Printf("Answer: %s\n", resp.Answer)
		fmt.Printf("Sources: %d\n", len(resp.Sources))
		for i, source := range resp.Sources {
			fmt.Printf("  [%d] Score: %.4f - %s...\n", i+1, source.Score, source.Content[:40])
		}
	}

	fmt.Println("\n=== Example Complete ===")
}
