package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	embedder "github.com/DotNetAge/gorag/embedding/openai"
	llm "github.com/DotNetAge/gorag/llm/openai"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/vectorstore/memory"
)

type RAGService struct {
	engine *rag.Engine
}

type IndexRequest struct {
	Content string `json:"content"`
}

type QueryRequest struct {
	Question string `json:"question"`
	TopK     int    `json:"top_k"`
}

type QueryResponse struct {
	Answer  string `json:"answer"`
	Sources []struct {
		Content string  `json:"content"`
		Score   float32 `json:"score"`
	} `json:"sources"`
}

func NewRAGService() (*RAGService, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}

	// Create embedder
	embedderInstance, err := embedder.New(embedder.Config{APIKey: apiKey})
	if err != nil {
		return nil, err
	}

	// Create LLM client
	llmInstance, err := llm.New(llm.Config{APIKey: apiKey})
	if err != nil {
		return nil, err
	}

	engine, err := rag.New(
		rag.WithParser(text.NewParser()),
		rag.WithVectorStore(memory.NewStore()),
		rag.WithEmbedder(embedderInstance),
		rag.WithLLM(llmInstance),
	)
	if err != nil {
		return nil, err
	}

	return &RAGService{engine: engine}, nil
}

func (s *RAGService) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	err := s.engine.Index(ctx, rag.Source{
		Type:    "text",
		Content: req.Content,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *RAGService) queryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Question == "" {
		http.Error(w, "Question is required", http.StatusBadRequest)
		return
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 3
	}

	ctx := context.Background()
	resp, err := s.engine.Query(ctx, req.Question, rag.QueryOptions{
		TopK: topK,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := QueryResponse{
		Answer:  resp.Answer,
		Sources: make([]struct {
			Content string  `json:"content"`
			Score   float32 `json:"score"`
		}, len(resp.Sources)),
	}

	for i, source := range resp.Sources {
		response.Sources[i].Content = source.Content
		response.Sources[i].Score = source.Score
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	service, err := NewRAGService()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/api/index", service.indexHandler)
	http.HandleFunc("/api/query", service.queryHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server running on http://localhost:%s\n", port)
	fmt.Println("Endpoints:")
	fmt.Println("  POST /api/index - Index document content")
	fmt.Println("  POST /api/query - Query the RAG engine")
	fmt.Println()
	fmt.Println("Example index request:")
	fmt.Println("  curl -X POST http://localhost:8080/api/index -H 'Content-Type: application/json' -d '{\"content\": \"Go is an open source programming language...\"}'")
	fmt.Println()
	fmt.Println("Example query request:")
	fmt.Println("  curl -X POST http://localhost:8080/api/query -H 'Content-Type: application/json' -d '{\"question\": \"What is Go?\", \"top_k\": 3}'")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
