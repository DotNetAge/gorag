package rag

import (
	"context"
	"testing"

	embedOllama "github.com/DotNetAge/gorag/embedding/ollama"
	llmOllama "github.com/DotNetAge/gorag/llm/ollama"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/vectorstore/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Query_CustomPromptTemplate(t *testing.T) {
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

	// Create RAG engine
	rengine, err := New(
		WithParser(parser),
		WithEmbedder(embedder),
		WithVectorStore(vectorStore),
		WithLLM(llmClient),
	)
	require.NoError(t, err)

	// Test indexing
	source := Source{
		Type:    "text",
		Content: "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled. Go has garbage collection and concurrency support.",
	}

	err = rengine.Index(context.Background(), source)
	require.NoError(t, err)

	// Test query with custom prompt template
	question := "What is Go?"
	customTemplate := "You are a helpful assistant. Please answer the following question based on the provided context.\n\nContext:\n{context}\n\nQuestion: {question}\n\nAnswer:"

	response, err := rengine.Query(context.Background(), question, QueryOptions{
		TopK:           3,
		PromptTemplate: customTemplate,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, response.Answer)
	assert.GreaterOrEqual(t, len(response.Sources), 1)
}
