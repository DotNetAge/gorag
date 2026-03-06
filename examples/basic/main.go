package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/raya-dev/gorag/parser/text"
	"github.com/raya-dev/gorag/rag"
	"github.com/raya-dev/gorag/vectorstore/memory"
)

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range embeddings {
		embeddings[i] = make([]float32, 128)
		for j := range embeddings[i] {
			embeddings[i][j] = float32(i+j) / 100.0
		}
	}
	return embeddings, nil
}

func (m *mockEmbedder) Dimension() int {
	return 128
}

type mockLLM struct{}

func (m *mockLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return fmt.Sprintf("Based on the provided context, here's the answer to your question.\n\nPrompt length: %d characters", len(prompt)), nil
}

func (m *mockLLM) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		defer close(ch)
		answer, _ := m.Complete(ctx, prompt)
		for _, word := range strings.Split(answer, " ") {
			ch <- word + " "
		}
	}()
	return ch, nil
}

func main() {
	ctx := context.Background()

	fmt.Println("=== GoRAG Basic Example ===")
	fmt.Println()

	engine, err := rag.New(
		rag.WithParser(text.NewParser()),
		rag.WithVectorStore(memory.NewStore()),
		rag.WithEmbedder(&mockEmbedder{}),
		rag.WithLLM(&mockLLM{}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("1. Indexing documents...")
	documents := []string{
		"Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.",
		"RAG (Retrieval-Augmented Generation) is an AI technique that combines information retrieval with text generation.",
		"Vector databases are specialized databases designed to store and query high-dimensional vectors efficiently.",
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

	fmt.Println("\n2. Querying the RAG engine...")
	questions := []string{
		"What is Go?",
		"How does RAG work?",
	}

	for _, question := range questions {
		fmt.Printf("\nQuestion: %s\n", question)
		fmt.Println(strings.Repeat("-", 50))

		resp, err := engine.Query(ctx, question, rag.QueryOptions{
			TopK: 3,
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
