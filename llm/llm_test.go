package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockClient implements the Client interface for testing
type mockClient struct{}

func (m *mockClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "Mock completion", nil
}

func (m *mockClient) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "Mock stream"
	close(ch)
	return ch, nil
}

func TestClient_Complete(t *testing.T) {
	client := &mockClient{}
	result, err := client.Complete(context.Background(), "Hello")
	assert.NoError(t, err)
	assert.Equal(t, "Mock completion", result)
}

func TestClient_CompleteStream(t *testing.T) {
	client := &mockClient{}
	stream, err := client.CompleteStream(context.Background(), "Hello")
	assert.NoError(t, err)

	var results []string
	for s := range stream {
		results = append(results, s)
	}

	assert.Len(t, results, 1)
	assert.Equal(t, "Mock stream", results[0])
}
