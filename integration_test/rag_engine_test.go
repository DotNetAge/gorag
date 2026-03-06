package integration_test

import (
	"context"
	"testing"
	"time"

	embedOllama "github.com/DotNetAge/gorag/embedding/ollama"
	llmOllama "github.com/DotNetAge/gorag/llm/ollama"
	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/rag/retrieval"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/DotNetAge/gorag/vectorstore/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRAGEngine_CompleteFlow(t *testing.T) {
	// Create components
	parser := text.NewParser()
	embedder, err := embedOllama.New(embedOllama.Config{
		Model: "qllama/bge-small-zh-v1.5:latest",
	})
	require.NoError(t, err)

	vectorStore := memory.NewStore()
	llmClient, err := llmOllama.New(llmOllama.Config{
		Model: "qwen3:0.6b",
	})
	require.NoError(t, err)

	retriever := retrieval.NewHybridRetriever(vectorStore, &mockKeywordStore{}, 0.5)
	reranker := retrieval.NewReranker(llmClient, 3)
	cache := rag.NewMemoryCache(1 * time.Hour)
	router := rag.NewDefaultRouter()
	metrics := observability.NewPrometheusMetrics()
	logger := observability.NewJSONLogger()
	tracer := observability.NewNoopTracer()

	// Register metrics
	err = metrics.Register()
	require.NoError(t, err)

	// Create RAG engine
	rengine, err := rag.New(
		rag.WithParser(parser),
		rag.WithEmbedder(embedder),
		rag.WithVectorStore(vectorStore),
		rag.WithLLM(llmClient),
		rag.WithRetriever(retriever),
		rag.WithReranker(reranker),
		rag.WithCache(cache),
		rag.WithRouter(router),
		rag.WithMetrics(metrics),
		rag.WithLogger(logger),
		rag.WithTracer(tracer),
	)
	require.NoError(t, err)

	// Test indexing
	source := rag.Source{
		Type:    "text",
		Content: "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support.",
	}

	err = rengine.Index(context.Background(), source)
	require.NoError(t, err)

	// Test query
	question := "What is Go?"
	response, err := rengine.Query(context.Background(), question, rag.QueryOptions{
		TopK: 3,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, response.Answer)
	assert.GreaterOrEqual(t, len(response.Sources), 1)

	// Test batch index
	sources := []rag.Source{
		{
			Type:    "text",
			Content: "Python is a high-level programming language known for its readability and simplicity. It is widely used in data science and machine learning.",
		},
		{
			Type:    "text",
			Content: "JavaScript is a scripting language primarily used for web development. It is dynamic and interpreted.",
		},
	}

	err = rengine.BatchIndex(context.Background(), sources)
	require.NoError(t, err)

	// Test batch query
	questions := []string{
		"What is Python?",
		"What is JavaScript?",
	}

	responses, err := rengine.BatchQuery(context.Background(), questions, rag.QueryOptions{
		TopK: 3,
	})
	require.NoError(t, err)

	assert.Len(t, responses, 2)
	assert.NotEmpty(t, responses[0].Answer)
	assert.NotEmpty(t, responses[1].Answer)

	// Test async index
	source4 := rag.Source{
		Type:    "text",
		Content: "Java is a class-based, object-oriented programming language. It is designed to have as few implementation dependencies as possible.",
	}

	err = rengine.AsyncIndex(context.Background(), source4)
	require.NoError(t, err)

	// Wait for async index to complete
	time.Sleep(1 * time.Second)

	// Test query with cached response
	response2, err := rengine.Query(context.Background(), question, rag.QueryOptions{
		TopK: 3,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, response2.Answer)
	assert.Equal(t, response.Answer, response2.Answer) // Should be same as cached response
}

// mockKeywordStore implements the retrieval.KeywordStore interface
type mockKeywordStore struct{}

func (m *mockKeywordStore) Search(ctx context.Context, query string, topK int) ([]vectorstore.Result, error) {
	return []vectorstore.Result{}, nil
}
