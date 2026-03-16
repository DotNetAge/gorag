// Package hybridrag demonstrates how to compose Hybrid RAG pipeline with multiple retrieval strategies.
//
// This example shows how to combine these Steps:
// QueryToFilterStep → StepBackStep → HyDEStep →
// [VectorSearchStep + SparseSearchStep] →
// RAGFusionStep → RerankStep → GenerationStep
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/infra/searcher/hybrid"
	"github.com/DotNetAge/gorag/infra/service"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	govector "github.com/DotNetAge/govector/core"
)

// govectorAdapter wraps govector.Collection to implement abstraction.VectorStore
type govectorAdapter struct {
	collection *govector.Collection
}

func (a *govectorAdapter) Add(ctx context.Context, vector *entity.Vector) error {
	return nil // TODO: Implement based on govector API
}

func (a *govectorAdapter) AddBatch(ctx context.Context, vectors []*entity.Vector) error {
	return nil // TODO: Implement based on govector API
}

func (a *govectorAdapter) Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
	return nil, nil, nil // TODO: Implement based on govector API
}

func (a *govectorAdapter) Delete(ctx context.Context, id string) error {
	return nil // TODO: Implement based on govector API
}

func (a *govectorAdapter) DeleteBatch(ctx context.Context, ids []string) error {
	return nil // TODO: Implement based on govector API
}

func (a *govectorAdapter) Close(ctx context.Context) error {
	return nil // TODO: Implement based on govector API
}

func main() {
	ctx := context.Background()

	// ========================================================================
	// Step 1: Initialize Core Components
	// ========================================================================

	// Embedding model (using local BGE for demonstration)
	embedder, err := embedding.WithBEG("bge-small-zh-v1.5", "")
	if err != nil {
		log.Fatalf("Failed to initialize embedder: %v", err)
	}

	// LLM (using Mock for demonstration)
	llm := &MockLLM{}

	// ========================================================================
	// Step 2: Initialize Storage Components
	// ========================================================================

	// Dense vector store using govector
	denseStore := &govectorAdapter{collection: initVectorStore()}

	// ========================================================================
	// Step 3: Build the Hybrid RAG Searcher
	// ========================================================================

	searcher := hybrid.New(
		// Core components
		hybrid.WithEmbedding(embedder),
		hybrid.WithVectorStore(denseStore),
		hybrid.WithGenerator(service.NewGenerator(llm)),

		// Configuration
		hybrid.WithDenseTopK(10),
	)

	// ========================================================================
	// Step 4: Execute Queries
	// ========================================================================

	queries := []string{
		"GoRAG 支持哪些高级检索策略？",
		"如何实现混合检索？",
	}

	for _, query := range queries {
		fmt.Printf("\n=== 查询：%s ===\n", query)

		answer, err := searcher.Search(ctx, query)
		if err != nil {
			log.Printf("Search failed: %v", err)
			continue
		}

		fmt.Printf("\n答案:\n%s\n", answer)
	}

	fmt.Println("\n=== Hybrid RAG Example Complete ===")
}

// initVectorStore initializes the dense vector store
func initVectorStore() *govector.Collection {
	store, _ := govector.NewStorage("./data/example_vectors")
	collection, _ := govector.NewCollection("example", 512, govector.Cosine, store, true)
	return collection
}

// MockLLM is a simple mock for demonstration
type MockLLM struct{}

func (m *MockLLM) Chat(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error) {
	return &core.Response{
		Content: "这是一个示例答案。在实际应用中，系统会基于检索到的文档内容生成准确的回答。",
	}, nil
}

func (m *MockLLM) ChatStream(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Stream, error) {
	ch := make(chan core.StreamEvent, 1)
	go func() {
		ch <- core.StreamEvent{
			Type:    core.EventContent,
			Content: "这是一个示例答案。在实际应用中，系统会基于检索到的文档内容生成准确的回答。",
		}
		close(ch)
	}()
	return core.NewStream(ch, nil), nil
}
