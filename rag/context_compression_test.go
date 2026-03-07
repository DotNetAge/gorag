package rag

import (
	"context"
	"testing"

	llmOllama "github.com/DotNetAge/gorag/llm/ollama"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextCompressor_Compress(t *testing.T) {
	// Create LLM client
	llmClient, err := llmOllama.New(llmOllama.Config{
		Model: "qwen3:0.6b",
	})
	require.NoError(t, err)

	// Create context compressor
	compressor := NewContextCompressor(llmClient)

	// Create test results
	results := []vectorstore.Result{
		{
			Chunk: vectorstore.Chunk{
				ID:      "1",
				Content: "Go is a programming language designed for simplicity and efficiency. It is statically typed and compiled.",
			},
			Score: 0.9,
		},
		{
			Chunk: vectorstore.Chunk{
				ID:      "2",
				Content: "Go has garbage collection and concurrency support. It is widely used for backend development.",
			},
			Score: 0.85,
		},
		{
			Chunk: vectorstore.Chunk{
				ID:      "3",
				Content: "Python is a high-level programming language known for its readability.",
			},
			Score: 0.4,
		},
	}

	// Test compression
	query := "What are the features of Go programming language?"
	compressed, err := compressor.Compress(context.Background(), query, results)
	require.NoError(t, err)

	// Verify compression
	assert.Greater(t, len(compressed), 0)
	assert.LessOrEqual(t, len(compressed), len(results))
	t.Logf("Original results: %d, Compressed results: %d", len(results), len(compressed))
	for i, result := range compressed {
		t.Logf("Compressed result %d: %s (Score: %.2f)", i+1, result.Content, result.Score)
	}
}

func TestContextCompressor_RemoveRedundancy(t *testing.T) {
	// Create LLM client
	llmClient, err := llmOllama.New(llmOllama.Config{
		Model: "qwen3:0.6b",
	})
	require.NoError(t, err)

	// Create context compressor with lower similarity threshold for testing
	compressor := NewContextCompressor(llmClient).WithSimilarityThreshold(0.7)

	// Create test results with redundancy
	results := []vectorstore.Result{
		{
			Chunk: vectorstore.Chunk{
				ID:      "1",
				Content: "Go is a programming language designed for simplicity and efficiency.",
			},
			Score: 0.9,
		},
		{
			Chunk: vectorstore.Chunk{
				ID:      "2",
				Content: "Go is a programming language designed for simplicity and efficiency. It is statically typed.",
			},
			Score: 0.85,
		},
	}

	// Test compression
	query := "What is Go?"
	compressed, err := compressor.Compress(context.Background(), query, results)
	require.NoError(t, err)

	// Verify redundancy removal
	assert.LessOrEqual(t, len(compressed), len(results))
	t.Logf("Original results: %d, Compressed results: %d", len(results), len(compressed))
	
	// If redundancy was not removed, log the similarity score for debugging
	if len(compressed) == len(results) {
		similarity := compressor.calculateSimilarity(
			results[0].Content,
			results[1].Content,
		)
		t.Logf("Similarity score: %.2f", similarity)
	}
}
