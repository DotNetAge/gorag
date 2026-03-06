package ollama

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Run("Default configuration", func(t *testing.T) {
		client, err := New(Config{})
		require.NoError(t, err)
		assert.Equal(t, "llama2", client.config.Model)
		assert.Equal(t, "http://localhost:11434", client.config.BaseURL)
		assert.Equal(t, 60*time.Second, client.config.Timeout)
		assert.Equal(t, 3, client.config.MaxRetries)
		assert.Equal(t, 0.7, client.config.Temperature)
	})

	t.Run("Custom configuration", func(t *testing.T) {
		client, err := New(Config{
			Model:       "mistral",
			BaseURL:     "http://localhost:11435",
			Timeout:     30 * time.Second,
			MaxRetries:  5,
			Temperature: 0.5,
			MaxTokens:   1000,
		})
		require.NoError(t, err)
		assert.Equal(t, "mistral", client.config.Model)
		assert.Equal(t, "http://localhost:11435", client.config.BaseURL)
		assert.Equal(t, 30*time.Second, client.config.Timeout)
		assert.Equal(t, 5, client.config.MaxRetries)
		assert.Equal(t, 0.5, client.config.Temperature)
		assert.Equal(t, 1000, client.config.MaxTokens)
	})
}

func TestClient_Complete(t *testing.T) {
	client, err := New(Config{
		Model: "functiongemma:270m",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := client.Complete(ctx, "Hello, who are you?")
	require.NoError(t, err)
	assert.NotEmpty(t, response)
}

func TestClient_CompleteStream(t *testing.T) {
	client, err := New(Config{
		Model: "functiongemma:270m",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch, err := client.CompleteStream(ctx, "Hello, who are you?")
	require.NoError(t, err)

	var response string
	for chunk := range ch {
		response += chunk
	}

	assert.NotEmpty(t, response)
}
