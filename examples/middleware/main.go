package main

import (
	"context"
	"fmt"
	"log"
	"os"

	embedder "github.com/DotNetAge/gorag/embedding/openai"
	llm "github.com/DotNetAge/gorag/llm/openai"
	"github.com/DotNetAge/gorag/middleware"
	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/vectorstore/memory"
)

// Example logger implementation
type simpleLogger struct{}

func (l *simpleLogger) Info(ctx context.Context, message string, fields map[string]interface{}) {
	fmt.Printf("[INFO] %s %v\n", message, fields)
}

func (l *simpleLogger) Debug(ctx context.Context, message string, fields map[string]interface{}) {
	fmt.Printf("[DEBUG] %s %v\n", message, fields)
}

func (l *simpleLogger) Error(ctx context.Context, message string, err error, fields map[string]interface{}) {
	fmt.Printf("[ERROR] %s: %v %v\n", message, err, fields)
}

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create components
	embedderInstance, err := embedder.New(embedder.Config{APIKey: apiKey})
	if err != nil {
		log.Fatal(err)
	}

	llmInstance, err := llm.New(llm.Config{APIKey: apiKey})
	if err != nil {
		log.Fatal(err)
	}

	// Create middleware chain
	logger := &simpleLogger{}
	loggingMiddleware := middleware.NewLoggingMiddleware(logger)
	validationMiddleware := middleware.NewValidationMiddleware(1000, 10*1024*1024)

	// Custom transform middleware
	transformMiddleware := middleware.NewTransformMiddleware(
		// Query transformer: add context to queries
		func(ctx context.Context, query *middleware.Query) error {
			query.Question = fmt.Sprintf("Based on the indexed documents, %s", query.Question)
			return nil
		},
		// Response transformer: add disclaimer
		func(ctx context.Context, response *middleware.Response) error {
			response.Answer = response.Answer + "\n\n(This answer is generated based on indexed documents)"
			return nil
		},
	)

	chain := middleware.NewChain(
		validationMiddleware,
		loggingMiddleware,
		transformMiddleware,
	)

	// Create RAG engine (middleware integration would be added to engine)
	engine, err := rag.New(
		rag.WithVectorStore(memory.NewStore()),
		rag.WithEmbedder(embedderInstance),
		rag.WithLLM(llmInstance),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Example: Index a document
	source := &middleware.Source{
		Type:    "text",
		Content: "Go is a statically typed, compiled programming language designed at Google. It is syntactically similar to C, but with memory safety, garbage collection, structural typing, and CSP-style concurrency.",
	}

	fmt.Println("=== Indexing Document ===")
	if err := chain.BeforeIndex(ctx, source); err != nil {
		log.Fatal(err)
	}

	// Index using engine
	if err := engine.Index(ctx, rag.Source{
		Type:    source.Type,
		Content: source.Content,
	}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Querying ===")
	query := &middleware.Query{
		Question: "What is Go?",
	}

	if err := chain.BeforeQuery(ctx, query); err != nil {
		log.Fatal(err)
	}

	// Query using engine
	resp, err := engine.Query(ctx, query.Question, rag.QueryOptions{
		TopK: 3,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Transform response
	response := &middleware.Response{
		Answer:  resp.Answer,
		Sources: resp.Sources,
	}

	if err := chain.AfterQuery(ctx, response); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nAnswer: %s\n", response.Answer)
	fmt.Printf("Sources: %d documents\n", len(response.Sources))
}
