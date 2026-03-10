package rag

import (
	"context"
	"testing"

	llmOllama "github.com/DotNetAge/gochat/pkg/client/ollama"
	"github.com/DotNetAge/gochat/pkg/client/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHyDE_EnhanceQuery(t *testing.T) {
	// Create LLM client
	llmClient, err := llmOllama.New(llmOllama.Config{
		Config: base.Config{
		Model: "qwen3:0.6b",
	},
	})
	require.NoError(t, err)

	// Create HyDE
	hydration := NewHyDE(llmClient)

	// Test query
	query := "What is the capital of France?"
	enhancedQuery, err := hydration.EnhanceQuery(context.Background(), query)
	require.NoError(t, err)

	// Verify enhanced query
	assert.NotEmpty(t, enhancedQuery)
	assert.Contains(t, enhancedQuery, query)
	assert.Contains(t, enhancedQuery, "Hypothetical document:")
	t.Logf("Original query: %s", query)
	t.Logf("Enhanced query length: %d", len(enhancedQuery))
}

func TestHyDE_CustomPrompt(t *testing.T) {
	// Create LLM client
	llmClient, err := llmOllama.New(llmOllama.Config{
		Config: base.Config{
		Model: "qwen3:0.6b",
	},
	})
	require.NoError(t, err)

	// Create HyDE with custom prompt
	customPrompt := "Generate a detailed answer for: {query}\n\nAnswer:"
	hydration := NewHyDE(llmClient).WithPromptTemplate(customPrompt)

	// Test query
	query := "What is Go programming language?"
	enhancedQuery, err := hydration.EnhanceQuery(context.Background(), query)
	require.NoError(t, err)

	// Verify enhanced query
	assert.NotEmpty(t, enhancedQuery)
	assert.Contains(t, enhancedQuery, query)
	t.Logf("Enhanced query with custom prompt: %s", enhancedQuery)
}
