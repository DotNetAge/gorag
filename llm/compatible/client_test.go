package compatible

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	// Test cases
	testCases := []struct {
		name        string
		config      Config
		expectedErr bool
	}{
		{
			name: "Missing API key",
			config: Config{
				Model:   "gpt-3.5-turbo",
				BaseURL: "https://api.openai.com",
			},
			expectedErr: true,
		},
		{
			name: "Minimal config",
			config: Config{
				APIKey: "test-api-key",
			},
			expectedErr: false,
		},
		{
			name: "Full config",
			config: Config{
				APIKey:      "test-api-key",
				Model:       "gpt-3.5-turbo",
				BaseURL:     "https://api.example.com",
				Temperature: 0.8,
				MaxTokens:   1000,
			},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(tc.config)
			if tc.expectedErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestClient_Complete(t *testing.T) {
	// This is a basic test to ensure the method signature is correct
	// Actual API calls would require mocking or integration tests
	client, err := New(Config{
		APIKey: "test-api-key",
		Model:  "gpt-3.5-turbo",
	})
	require.NoError(t, err)

	// We expect this to fail since it's a test API key, but the method should be callable
	_, err = client.Complete(context.Background(), "Hello, world!")
	assert.Error(t, err)
}

func TestClient_CompleteStream(t *testing.T) {
	// This is a basic test to ensure the method signature is correct
	// Actual API calls would require mocking or integration tests
	client, err := New(Config{
		APIKey: "test-api-key",
		Model:  "gpt-3.5-turbo",
	})
	require.NoError(t, err)

	// We expect this to fail since it's a test API key, but the method should be callable
	_, err = client.CompleteStream(context.Background(), "Hello, world!")
	assert.Error(t, err)
}
