package retrieval

import (
	"context"
	gochatcore "github.com/DotNetAge/gochat/pkg/core"

	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLM struct {
	response string
}

func (m *mockLLM) Chat(ctx context.Context, messages []gochatcore.Message, opts ...gochatcore.Option) (*gochatcore.Response, error) {
	return &gochatcore.Response{Content: m.response}, nil
}

func (m *mockLLM) ChatStream(ctx context.Context, messages []gochatcore.Message, opts ...gochatcore.Option) (*gochatcore.Stream, error) {
	return nil, nil
}

func TestReranker_Rerank(t *testing.T) {
	results := []core.Result{
		{
			Chunk: core.Chunk{
				ID:      "doc1",
				Content: "Document 1",
			},
			Score: 0.9,
		},
		{
			Chunk: core.Chunk{
				ID:      "doc2",
				Content: "Document 2",
			},
			Score: 0.8,
		},
		{
			Chunk: core.Chunk{
				ID:      "doc3",
				Content: "Document 3",
			},
			Score: 0.7,
		},
	}

	tests := []struct {
		name     string
		llmResp  string
		topK     int
		expected int
	}{
		{
			name:     "rerank with valid scores",
			llmResp:  "0.5, 0.9, 0.3",
			topK:     2,
			expected: 2,
		},
		{
			name:     "rerank with no scores",
			llmResp:  "",
			topK:     2,
			expected: 2,
		},
		{
			name:     "topK greater than results",
			llmResp:  "0.5, 0.9, 0.3",
			topK:     5,
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := &mockLLM{response: tt.llmResp}
			reranker := NewReranker(llm, tt.topK)
			reranked, err := reranker.Rerank(context.Background(), "test query", results)
			require.NoError(t, err)
			assert.Len(t, reranked, tt.expected)
		})
	}
}

func TestReranker_BuildPrompt(t *testing.T) {
	results := []core.Result{
		{
			Chunk: core.Chunk{
				ID:      "doc1",
				Content: "Test content 1",
			},
			Score: 0.9,
		},
	}

	llm := &mockLLM{response: "0.9"}
	reranker := NewReranker(llm, 1)
	prompt := reranker.buildRerankPrompt("test query", results)

	assert.Contains(t, prompt, "test query")
	assert.Contains(t, prompt, "Test content 1")
}

func TestReranker_WithPrompt(t *testing.T) {
	llm := &mockLLM{response: "0.9"}
	reranker := NewReranker(llm, 1).WithPrompt("Custom prompt: {query}")
	assert.Contains(t, reranker.prompt, "Custom prompt")
}
